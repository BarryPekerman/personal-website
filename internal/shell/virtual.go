package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	homeDir       = "/home/guest"
	motdFile      = ".motd"
	maxLineLength = 512
	maxHistory    = 100

	// clearScreen matches what `clear` sends: home, erase screen, erase
	// scrollback. Ctrl-L uses clearView, which keeps scrollback, like bash.
	clearScreen = "\x1b[H\x1b[2J\x1b[3J"
	clearView   = "\x1b[H\x1b[2J"
	colorReset  = "\x1b[0m"
	colorRed    = "\x1b[1;31m"
	colorGreen  = "\x1b[1;32m"
	colorYellow = "\x1b[1;33m"
	colorBlue   = "\x1b[1;34m"
)

const defaultMotd = `Connection established. Welcome, guest.
Type 'help' for commands.
`

// VirtualManager creates sessions backed by a built-in command interpreter.
// Nothing is executed on the host: every command only reads from a
// read-only fs.FS, so there is no shell to escape from.
type VirtualManager struct {
	fsys   fs.FS
	logger *logrus.Logger
}

// NewVirtualManager creates a virtual shell manager serving files from fsys
func NewVirtualManager(fsys fs.FS, logger *logrus.Logger) *VirtualManager {
	return &VirtualManager{
		fsys:   fsys,
		logger: logger,
	}
}

// CreateSession creates a new virtual shell session
func (m *VirtualManager) CreateSession(sessionID string) (Session, error) {
	m.logger.WithField("session_id", sessionID).Debug("Creating virtual shell session")

	return &VirtualSession{
		pipes: newPipes(),
		fsys:  m.fsys,
		cwd:   ".",
		done:  make(chan error, 1),
	}, nil
}

// VirtualSession is a simulated shell. It does its own line editing because
// xterm.js sends raw keystrokes, one message per key.
type VirtualSession struct {
	*pipes

	fsys fs.FS
	cwd  string // path inside fsys; "." is the home directory

	line    []rune
	history []string
	histPos int

	done chan error
}

type command struct {
	help   string
	hidden bool
	run    func(s *VirtualSession, args []string)
}

var commands map[string]command

func init() {
	commands = map[string]command{
		"help":     {help: "show this list", run: (*VirtualSession).cmdHelp},
		"ls":       {help: "list files", run: (*VirtualSession).cmdLs},
		"cd":       {help: "change directory", run: (*VirtualSession).cmdCd},
		"pwd":      {help: "print working directory", run: (*VirtualSession).cmdPwd},
		"cat":      {help: "print a file", run: (*VirtualSession).cmdCat},
		"cv":       {help: "show my CV", run: func(s *VirtualSession, _ []string) { s.cmdCat([]string{"~/cv.txt"}) }},
		"contact":  {help: "how to reach me", run: func(s *VirtualSession, _ []string) { s.cmdCat([]string{"~/contact_info.md"}) }},
		"projects": {help: "list my projects", run: func(s *VirtualSession, _ []string) { s.cmdLs([]string{"~/projects"}) }},
		"connect":  {help: "connect to a remote system", run: (*VirtualSession).cmdConnect},
		"clear":    {help: "clear the screen", run: func(s *VirtualSession, _ []string) { s.print(clearScreen) }},
		"history":  {help: "show command history", run: (*VirtualSession).cmdHistory},
		"whoami":   {help: "print user name", run: func(s *VirtualSession, _ []string) { s.print("guest\n") }},
		"exit":     {help: "end the session"}, // handled in execute
		"echo":     {hidden: true, run: func(s *VirtualSession, args []string) { s.print(strings.Join(args, " ") + "\n") }},
		"less":     {hidden: true, run: (*VirtualSession).cmdCat},
		"more":     {hidden: true, run: (*VirtualSession).cmdCat},
		"resume":   {hidden: true, run: func(s *VirtualSession, _ []string) { s.cmdCat([]string{"~/cv.txt"}) }},
		"date":     {hidden: true, run: func(s *VirtualSession, _ []string) { s.print(time.Now().UTC().Format(time.UnixDate) + "\n") }},
		"uname":    {hidden: true, run: (*VirtualSession).cmdUname},
		"sudo":     {hidden: true, run: (*VirtualSession).cmdSudo},
		"rm":       {hidden: true, run: (*VirtualSession).cmdRm},
		"logout":   {hidden: true}, // handled in execute
	}
}

// Start launches the interpreter loop
func (s *VirtualSession) Start() error {
	go func() {
		s.done <- s.run()
	}()
	return nil
}

// Resize is a no-op: output is plain text that the browser terminal wraps
func (s *VirtualSession) Resize(_, _ uint16) error {
	return nil
}

// Wait blocks until the session ends
func (s *VirtualSession) Wait() error {
	return <-s.done
}

// Close stops the session; safe to call more than once
func (s *VirtualSession) Close() error {
	s.closePipes()
	return nil
}

func (s *VirtualSession) run() error {
	// Closing output gives the handler EOF, which ends the connection
	defer func() { _ = s.outW.Close() }()

	s.printMotd()
	s.prompt()

	in := bufio.NewReader(s.inR)
	for {
		r, _, err := in.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				return nil
			}
			return err
		}
		if !s.handleKey(r, in) {
			return nil
		}
	}
}

// handleKey processes one keystroke. It returns false when the session should end.
func (s *VirtualSession) handleKey(r rune, in *bufio.Reader) bool {
	switch {
	case r == '\r' || r == '\n':
		s.print("\n")
		line := strings.TrimSpace(string(s.line))
		s.line = s.line[:0]
		if line != "" {
			s.addHistory(line)
		}
		if !s.execute(line) {
			return false
		}
		s.prompt()
	case r == 0x7f || r == '\b':
		if len(s.line) > 0 {
			s.line = s.line[:len(s.line)-1]
			s.print("\b \b")
		}
	case r == 0x03: // Ctrl-C
		s.print("^C\n")
		s.line = s.line[:0]
		s.prompt()
	case r == 0x04: // Ctrl-D
		if len(s.line) == 0 {
			s.print("logout\n")
			return false
		}
	case r == 0x0c: // Ctrl-L
		s.print(clearView + s.promptString() + string(s.line))
	case r == 0x15: // Ctrl-U
		s.replaceLine("")
	case r == '\t':
		s.complete()
	case r == 0x1b:
		return s.handleEscape(in)
	case r >= 0x20:
		if len(s.line) < maxLineLength {
			s.line = append(s.line, r)
			s.print(string(r))
		}
	}
	return true
}

// handleEscape consumes an ANSI escape sequence (arrow keys etc.). Only
// up/down are used, for history; everything else is ignored.
func (s *VirtualSession) handleEscape(in *bufio.Reader) bool {
	r, _, err := in.ReadRune()
	if err != nil {
		return false
	}
	if r != '[' && r != 'O' {
		// A lone Esc key press followed by a normal key
		return s.handleKey(r, in)
	}

	// Parameters run until a final byte in the range 0x40-0x7e
	for i := 0; i < 16; i++ {
		c, _, err := in.ReadRune()
		if err != nil {
			return false
		}
		if c >= 0x40 && c <= 0x7e {
			switch c {
			case 'A':
				s.historyUp()
			case 'B':
				s.historyDown()
			}
			return true
		}
	}
	return true
}

// execute runs one command line. It returns false when the session should end.
func (s *VirtualSession) execute(line string) bool {
	if line == "" {
		return true
	}
	if isSelfDestruct(line) {
		s.selfDestruct()
		return false
	}

	fields := strings.Fields(line)
	name, args := fields[0], fields[1:]

	if name == "exit" || name == "logout" {
		s.print("logout\n")
		return false
	}

	cmd, ok := commands[name]
	if !ok || cmd.run == nil {
		s.print(fmt.Sprintf("%s: command not found\n", name))
		return true
	}
	cmd.run(s, args)
	return true
}

func (s *VirtualSession) cmdHelp(_ []string) {
	names := make([]string, 0, len(commands))
	for name, cmd := range commands {
		if !cmd.hidden {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, name := range names {
		fmt.Fprintf(&b, "  %s%-10s%s %s\n", colorGreen, name, colorReset, commands[name].help)
	}
	b.WriteString("\nTab completes, arrow keys browse history.\n")
	s.print(b.String())
}

func (s *VirtualSession) cmdLs(args []string) {
	showHidden := false
	var targets []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			showHidden = showHidden || strings.Contains(a, "a")
			continue
		}
		targets = append(targets, a)
	}
	if len(targets) == 0 {
		targets = []string{"."}
	}

	for i, t := range targets {
		p, ok := s.resolve(t)
		if !ok {
			s.print(fmt.Sprintf("ls: cannot open directory '%s': Permission denied\n", t))
			continue
		}
		info, err := fs.Stat(s.fsys, p)
		if err != nil {
			s.print(fmt.Sprintf("ls: cannot access '%s': No such file or directory\n", t))
			continue
		}
		if !info.IsDir() {
			s.print(t + "\n")
			continue
		}
		if len(targets) > 1 {
			if i > 0 {
				s.print("\n")
			}
			s.print(t + ":\n")
		}

		entries, err := fs.ReadDir(s.fsys, p)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") && !showHidden {
				continue
			}
			if e.IsDir() {
				names = append(names, colorBlue+e.Name()+"/"+colorReset)
			} else {
				names = append(names, e.Name())
			}
		}
		if len(names) > 0 {
			s.print(strings.Join(names, "  ") + "\n")
		}
	}
}

func (s *VirtualSession) cmdCd(args []string) {
	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	p, ok := s.resolve(target)
	if !ok {
		s.print(fmt.Sprintf("cd: %s: Permission denied\n", target))
		return
	}
	info, err := fs.Stat(s.fsys, p)
	if err != nil {
		s.print(fmt.Sprintf("cd: %s: No such file or directory\n", target))
		return
	}
	if !info.IsDir() {
		s.print(fmt.Sprintf("cd: %s: Not a directory\n", target))
		return
	}
	s.cwd = p
}

func (s *VirtualSession) cmdPwd(_ []string) {
	s.print(absPath(s.cwd) + "\n")
}

func (s *VirtualSession) cmdCat(args []string) {
	if len(args) == 0 {
		s.print("cat: missing file operand\n")
		return
	}
	for _, a := range args {
		p, ok := s.resolve(a)
		if !ok {
			s.print(fmt.Sprintf("cat: %s: Permission denied\n", a))
			continue
		}
		info, err := fs.Stat(s.fsys, p)
		if err != nil {
			s.print(fmt.Sprintf("cat: %s: No such file or directory\n", a))
			continue
		}
		if info.IsDir() {
			s.print(fmt.Sprintf("cat: %s: Is a directory\n", a))
			continue
		}
		data, err := fs.ReadFile(s.fsys, p)
		if err != nil {
			s.print(fmt.Sprintf("cat: %s: Input/output error\n", a))
			continue
		}
		text := string(data)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		s.print(text)
	}
}

func (s *VirtualSession) cmdHistory(_ []string) {
	var b strings.Builder
	for i, h := range s.history {
		fmt.Fprintf(&b, "%5d  %s\n", i+1, h)
	}
	s.print(b.String())
}

// knownHosts are the systems `connect` knows: it prints where they are, and
// the terminal makes the address a link
var knownHosts = map[string]string{
	"github":   "https://github.com/BarryPekerman",
	"handover": "https://play.barrypekerman.com/",
}

func (s *VirtualSession) cmdConnect(args []string) {
	names := make([]string, 0, len(knownHosts))
	for name := range knownHosts {
		names = append(names, name)
	}
	sort.Strings(names)
	known := strings.Join(names, ", ")

	if len(args) == 0 {
		s.print(fmt.Sprintf("usage: connect <system>   (known: %s)\n", known))
		return
	}
	host := strings.ToLower(args[0])
	if url, ok := knownHosts[host]; ok {
		s.print(fmt.Sprintf("%s: %s (click to open in your browser)\n", host, url))
		return
	}
	s.print(fmt.Sprintf("connect: unknown system '%s' (known: %s)\n", args[0], known))
}

func (s *VirtualSession) cmdUname(args []string) {
	if len(args) > 0 && args[0] == "-a" {
		// The architecture is the server's; there is no kernel behind this shell
		s.print(fmt.Sprintf("Linux portfolio (simulated shell, no kernel) %s\n", runtime.GOARCH))
		return
	}
	s.print("Linux\n")
}

func (s *VirtualSession) cmdSudo(_ []string) {
	s.print("sudo: guest is not in the sudoers file (and nothing typed here runs anywhere: this shell is simulated)\n")
}

func (s *VirtualSession) cmdRm(args []string) {
	if len(args) == 0 {
		s.print("rm: missing operand\n")
		return
	}
	s.print(fmt.Sprintf("rm: cannot remove '%s': Read-only file system\n", args[len(args)-1]))
}

// isSelfDestruct spots the classic destructive one-liners: a fork bomb, or a
// command (in any part of a list or pipeline, sudo or not) that is a
// recursive rm, mkfs or dd. Whole commands only: "echo I love mkfs" is fine.
func isSelfDestruct(line string) bool {
	if strings.HasPrefix(strings.Join(strings.Fields(line), ""), ":(){") {
		return true
	}
	for _, part := range strings.FieldsFunc(strings.ToLower(line), func(r rune) bool {
		return r == ';' || r == '&' || r == '|'
	}) {
		f := strings.Fields(part)
		sudo := len(f) > 0 && f[0] == "sudo"
		if sudo {
			f = f[1:]
		}
		if len(f) == 0 {
			continue
		}
		switch {
		case f[0] == "rm" && (sudo || recursive(f[1:])),
			strings.HasPrefix(f[0], "mkfs"),
			f[0] == "dd" && len(f) > 1:
			return true
		}
	}
	return false
}

// recursive reports whether rm's arguments ask for -r
func recursive(args []string) bool {
	for _, a := range args {
		if a == "--recursive" || (strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR")) {
			return true
		}
	}
	return false
}

func (s *VirtualSession) selfDestruct() {
	steps := []string{
		colorRed + "SECURITY VIOLATION DETECTED" + colorReset + "\n",
		colorYellow + "Initiating self-destruct sequence..." + colorReset + "\n",
		"3...\n",
		"2...\n",
		"1...\n",
		colorRed + "SESSION TERMINATED" + colorReset + "\n",
	}
	for _, step := range steps {
		s.print(step)
		time.Sleep(400 * time.Millisecond)
	}
}

// complete performs tab completion on the last word of the line
func (s *VirtualSession) complete() {
	text := string(s.line)
	start := strings.LastIndexByte(text, ' ') + 1
	word := text[start:]

	var candidates []string
	if start == 0 {
		for name, cmd := range commands {
			if !cmd.hidden && strings.HasPrefix(name, word) {
				candidates = append(candidates, name+" ")
			}
		}
	} else {
		dirPart, base := path.Split(word)
		dir := s.cwd
		if dirPart != "" {
			var ok bool
			if dir, ok = s.resolve(dirPart); !ok {
				return
			}
		}
		entries, err := fs.ReadDir(s.fsys, dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, base) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".")) {
				continue
			}
			if e.IsDir() {
				candidates = append(candidates, dirPart+name+"/")
			} else {
				candidates = append(candidates, dirPart+name+" ")
			}
		}
	}
	if len(candidates) == 0 {
		return
	}
	sort.Strings(candidates)

	prefix := commonPrefix(candidates)
	if len(prefix) > len(word) {
		s.insert(prefix[len(word):])
		return
	}

	// Ambiguous: list the options, then redraw the line
	shown := make([]string, len(candidates))
	for i, c := range candidates {
		shown[i] = path.Base(strings.TrimSpace(c))
		if strings.HasSuffix(c, "/") {
			shown[i] += "/"
		}
	}
	s.print("\n" + strings.Join(shown, "  ") + "\n" + s.promptString() + text)
}

func commonPrefix(items []string) string {
	prefix := items[0]
	for _, it := range items[1:] {
		for !strings.HasPrefix(it, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func (s *VirtualSession) insert(text string) {
	for _, r := range text {
		if len(s.line) >= maxLineLength {
			return
		}
		s.line = append(s.line, r)
		s.print(string(r))
	}
}

func (s *VirtualSession) addHistory(line string) {
	s.history = append(s.history, line)
	if len(s.history) > maxHistory {
		s.history = s.history[len(s.history)-maxHistory:]
	}
	s.histPos = len(s.history)
}

func (s *VirtualSession) historyUp() {
	if s.histPos > 0 {
		s.histPos--
		s.replaceLine(s.history[s.histPos])
	}
}

func (s *VirtualSession) historyDown() {
	if s.histPos < len(s.history)-1 {
		s.histPos++
		s.replaceLine(s.history[s.histPos])
		return
	}
	s.histPos = len(s.history)
	s.replaceLine("")
}

// replaceLine swaps the current input line for text and redraws it
func (s *VirtualSession) replaceLine(text string) {
	s.line = []rune(text)
	s.print("\r\x1b[K" + s.promptString() + text)
}

// resolve maps a user-supplied path to a path inside fsys. It returns false
// when the path points outside the home directory.
func (s *VirtualSession) resolve(p string) (string, bool) {
	var abs string
	switch {
	case p == "" || p == "~":
		abs = homeDir
	case strings.HasPrefix(p, "~/"):
		abs = path.Join(homeDir, p[2:])
	case strings.HasPrefix(p, "/"):
		abs = path.Clean(p)
	default:
		abs = path.Join(absPath(s.cwd), p)
	}

	if abs == homeDir {
		return ".", true
	}
	if rel, ok := strings.CutPrefix(abs, homeDir+"/"); ok {
		return rel, true
	}
	return "", false
}

func absPath(rel string) string {
	if rel == "." {
		return homeDir
	}
	return path.Join(homeDir, rel)
}

func (s *VirtualSession) promptString() string {
	dir := "~"
	if s.cwd != "." {
		dir = "~/" + s.cwd
	}
	return fmt.Sprintf("%sguest@portfolio%s:%s%s%s$ ", colorGreen, colorReset, colorBlue, dir, colorReset)
}

func (s *VirtualSession) prompt() {
	s.print(s.promptString())
}

func (s *VirtualSession) printMotd() {
	motd := defaultMotd
	if data, err := fs.ReadFile(s.fsys, motdFile); err == nil {
		motd = string(data)
	}
	s.print(motd + "\n")
}

// print writes text to the terminal, converting \n to \r\n. A write only
// fails once the session is closed, and run notices that on its next read.
func (s *VirtualSession) print(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	_, _ = s.outW.Write([]byte(text))
}
