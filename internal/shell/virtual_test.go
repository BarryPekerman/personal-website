package shell

import (
	"io"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sirupsen/logrus"
)

var testFS = fstest.MapFS{
	".motd":                {Data: []byte("WELCOME-BANNER\n")},
	"cv.txt":               {Data: []byte("CV-CONTENT\n")},
	"contact_info.md":      {Data: []byte("CONTACT-CONTENT\n")},
	"projects/alpha.md":    {Data: []byte("ALPHA-CONTENT\n")},
	"projects/portal.md":   {Data: []byte("PORTAL-CONTENT\n")},
	"projects/deep/one.md": {Data: []byte("ONE\n")},
}

// harness drives a VirtualSession and collects everything it prints
type harness struct {
	t       *testing.T
	session Session
	mu      sync.Mutex
	out     strings.Builder
	eof     chan struct{}
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	session, err := NewVirtualManager(testFS, logger).CreateSession("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Start(); err != nil {
		t.Fatal(err)
	}

	h := &harness{t: t, session: session, eof: make(chan struct{})}
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := session.Reader().Read(buf)
			h.mu.Lock()
			h.out.Write(buf[:n])
			h.mu.Unlock()
			if err != nil {
				close(h.eof)
				return
			}
		}
	}()
	t.Cleanup(func() { _ = session.Close() })

	h.waitUntil(0, func(out string) bool { return strings.HasSuffix(out, "$ ") })
	return h
}

func (h *harness) output() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.out.String()
}

// send types keys and returns the output they produced, once it contains until
func (h *harness) send(keys, until string) string {
	h.t.Helper()
	return h.typeKeys(keys, func(out string) bool { return strings.Contains(out, until) })
}

// run types a command plus Enter and returns its output, without the
// echoed command line, once the next prompt appears
func (h *harness) run(cmd string) string {
	h.t.Helper()
	out := h.typeKeys(cmd+"\r", func(out string) bool {
		return strings.Contains(out, "\r\n") && strings.HasSuffix(out, "$ ")
	})
	return out[strings.Index(out, "\r\n")+2:]
}

// typeKeys writes keys, then waits until the output produced after the write satisfies done
func (h *harness) typeKeys(keys string, done func(string) bool) string {
	h.t.Helper()
	before := len(h.output())
	if _, err := h.session.Writer().Write([]byte(keys)); err != nil {
		h.t.Fatalf("write %q: %v", keys, err)
	}
	return h.waitUntil(before, done)
}

// waitUntil polls the output written after offset from until done reports true
func (h *harness) waitUntil(from int, done func(string) bool) string {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if out := h.output()[from:]; done(out) {
			return out
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.t.Fatalf("timed out; output so far: %q", h.output()[from:])
	return ""
}

func (h *harness) waitForExit() {
	h.t.Helper()
	select {
	case <-h.eof:
	case <-time.After(5 * time.Second):
		h.t.Fatal("session did not end")
	}
}

func TestBannerAndPrompt(t *testing.T) {
	h := newHarness(t)
	out := h.output()
	if !strings.Contains(out, "WELCOME-BANNER") {
		t.Errorf("banner missing: %q", out)
	}
	if !strings.Contains(out, "guest@portfolio") {
		t.Errorf("prompt missing: %q", out)
	}
}

func TestCommands(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"ls", "cv.txt"},
		{"ls", "projects/"},
		{"ls -a", ".motd"},
		{"ls projects", "alpha.md"},
		{"cat cv.txt", "CV-CONTENT"},
		{"cv", "CV-CONTENT"},
		{"contact", "CONTACT-CONTENT"},
		{"projects", "portal.md"},
		{"cat projects/alpha.md", "ALPHA-CONTENT"},
		{"cat ~/projects/alpha.md", "ALPHA-CONTENT"},
		{"cat /home/guest/cv.txt", "CV-CONTENT"},
		{"cat nope.txt", "No such file or directory"},
		{"cat projects", "Is a directory"},
		{"cat /etc/passwd", "Permission denied"},
		{"cat ../../etc/passwd", "Permission denied"},
		{"ls /", "Permission denied"},
		{"pwd", "/home/guest"},
		{"whoami", "guest"},
		{"echo hello world", "hello world"},
		{"sudo su", "not in the sudoers file"},
		{"rm cv.txt", "Read-only file system"},
		{"frobnicate", "frobnicate: command not found"},
		{"help", "projects"},
	}
	h := newHarness(t)
	for _, tt := range tests {
		if out := h.run(tt.cmd); !strings.Contains(out, tt.want) {
			t.Errorf("%q: want output containing %q, got %q", tt.cmd, tt.want, out)
		}
	}
}

func TestHiddenFilesNotListedByDefault(t *testing.T) {
	h := newHarness(t)
	if out := h.run("ls"); strings.Contains(out, ".motd") {
		t.Errorf("ls shows hidden file: %q", out)
	}
}

func TestCd(t *testing.T) {
	h := newHarness(t)

	h.run("cd projects")
	if out := h.run("pwd"); !strings.Contains(out, "/home/guest/projects") {
		t.Errorf("pwd after cd: %q", out)
	}
	if out := h.run("cat alpha.md"); !strings.Contains(out, "ALPHA-CONTENT") {
		t.Errorf("relative cat after cd: %q", out)
	}
	if out := h.run("cd .."); !strings.HasSuffix(out, "~"+colorReset+"$ ") {
		t.Errorf("prompt after cd ..: %q", out)
	}
	if out := h.run("cd .."); !strings.Contains(out, "Permission denied") {
		t.Errorf("cd above home should be denied: %q", out)
	}
	if out := h.run("cd cv.txt"); !strings.Contains(out, "Not a directory") {
		t.Errorf("cd into file: %q", out)
	}
	h.run("cd projects/deep")
	h.run("cd")
	if out := h.run("pwd"); !strings.Contains(out, "/home/guest\r\n") {
		t.Errorf("bare cd should go home: %q", out)
	}
}

func TestBackspaceAndCtrlC(t *testing.T) {
	h := newHarness(t)

	if out := h.run("cat cv.txtx\x7f"); !strings.Contains(out, "CV-CONTENT") {
		t.Errorf("backspace not applied: %q", out)
	}
	h.send("cat cv", "cat cv")
	h.send("\x03", "^C")
	if out := h.run("whoami"); !strings.Contains(out, "guest") || strings.Contains(out, "cat") {
		t.Errorf("ctrl-c should discard the line: %q", out)
	}
}

func TestTabCompletion(t *testing.T) {
	h := newHarness(t)

	// Unique command
	h.send("who\t", "whoami ")
	if out := h.run(""); !strings.Contains(out, "guest") {
		t.Errorf("completed command did not run: %q", out)
	}

	// Unique file inside a directory
	h.send("cat projects/al\t", "projects/alpha.md ")
	if out := h.run(""); !strings.Contains(out, "ALPHA-CONTENT") {
		t.Errorf("completed path did not run: %q", out)
	}

	// Ambiguous: lists the options
	if out := h.send("cat projects/\t", "portal.md"); !strings.Contains(out, "alpha.md") {
		t.Errorf("ambiguous completion should list options: %q", out)
	}
}

func TestHistory(t *testing.T) {
	h := newHarness(t)

	h.run("whoami")
	h.run("pwd")
	if out := h.send("\x1b[A", "pwd"); !strings.Contains(out, "pwd") {
		t.Errorf("up arrow should recall last command: %q", out)
	}
	h.send("\x1b[A", "whoami")
	if out := h.run(""); !strings.Contains(out, "guest") {
		t.Errorf("recalled command did not run: %q", out)
	}
	if out := h.run("history"); !strings.Contains(out, "1  whoami") || !strings.Contains(out, "2  pwd") {
		t.Errorf("history output: %q", out)
	}
}

func TestLineLengthCapped(t *testing.T) {
	h := newHarness(t)
	long := strings.Repeat("a", maxLineLength*2)
	out := h.run("echo " + long)
	if strings.Count(out, "a") > maxLineLength {
		t.Errorf("line not capped: got %d chars", strings.Count(out, "a"))
	}
}

func TestExit(t *testing.T) {
	h := newHarness(t)
	h.send("exit\r", "logout")
	h.waitForExit()
	if err := h.session.Wait(); err != nil {
		t.Errorf("Wait: %v", err)
	}
}

func TestCtrlDExits(t *testing.T) {
	h := newHarness(t)
	h.send("\x04", "logout")
	h.waitForExit()
}

func TestSelfDestruct(t *testing.T) {
	h := newHarness(t)
	h.send("sudo rm -rf /\r", "SECURITY VIOLATION")
	h.waitForExit()
	if !strings.Contains(h.output(), "SESSION TERMINATED") {
		t.Errorf("self-destruct sequence incomplete: %q", h.output())
	}
}

func TestCloseEndsSession(t *testing.T) {
	h := newHarness(t)
	if err := h.session.Close(); err != nil {
		t.Fatal(err)
	}
	h.waitForExit()
	if err := h.session.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestResolve(t *testing.T) {
	s := &VirtualSession{cwd: "projects"}
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"", ".", true},
		{"~", ".", true},
		{"alpha.md", "projects/alpha.md", true},
		{"..", ".", true},
		{"../cv.txt", "cv.txt", true},
		{"~/cv.txt", "cv.txt", true},
		{"/home/guest/projects", "projects", true},
		{"../..", "", false},
		{"/etc/passwd", "", false},
		{"/home/guestevil", "", false},
	}
	for _, tt := range tests {
		got, ok := s.resolve(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("resolve(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestClearWipesScrollback(t *testing.T) {
	h := newHarness(t)
	if out := h.run("clear"); !strings.Contains(out, "\x1b[3J") {
		t.Errorf("clear should erase scrollback: %q", out)
	}
	if out := h.send("\x0c", "$ "); strings.Contains(out, "\x1b[3J") {
		t.Errorf("Ctrl-L should keep scrollback: %q", out)
	}
}

func TestConnect(t *testing.T) {
	h := newHarness(t)
	tests := []struct{ cmd, want string }{
		{"connect", "usage: connect"},
		{"connect github", "https://github.com/BarryPekerman"},
		{"connect nsa", "Access denied"},
	}
	for _, tt := range tests {
		if out := h.run(tt.cmd); !strings.Contains(out, tt.want) {
			t.Errorf("%q: want %q, got %q", tt.cmd, tt.want, out)
		}
	}
}
