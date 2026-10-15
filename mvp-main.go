package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// --- 1. THE "DUNGEON MASTER" MEMORY ---
// Session state tracks both directory and environment variables
type SessionState struct {
	CurrentDir string
	Env        map[string]string
}

var sessions = make(map[string]*SessionState)
var mu sync.Mutex

// --- 2. API STRUCTURES ---
type CmdRequest struct {
	SessionID string `json:"session_id"`
	Command   string `json:"command"`
}

type CmdResponse struct {
	Output string `json:"output"`
	Prompt string `json:"prompt"` // Return the new path for the prompt
}

type CompletionsRequest struct {
	SessionID string `json:"session_id"`
	Partial   string `json:"partial"`
}

type CompletionsResponse struct {
	Completions []string `json:"completions"`
}

func handleCommand(w http.ResponseWriter, r *http.Request) {
	// A. Parse Request
	var req CmdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	// B. Get User's "State" (Current Directory + Environment)
	mu.Lock()
	session, exists := sessions[req.SessionID]
	if !exists {
		// Initialize new session
		cwd, _ := os.Getwd()
		session = &SessionState{
			CurrentDir: cwd,
			Env:        make(map[string]string),
		}
		// Copy system environment
		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				session.Env[parts[0]] = parts[1]
			}
		}
		sessions[req.SessionID] = session
	}
	currentDir := session.CurrentDir
	env := session.Env
	mu.Unlock()

	// C. Analyze the Command
	parts := strings.Fields(req.Command)
	if len(parts) == 0 {
		return
	}
	binary := parts[0]
	args := parts[1:]

	var output string

	// --- D. Handle Shell Built-ins ---
	if binary == "cd" {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		
		// Expand ~ to home directory
		if target == "~" || strings.HasPrefix(target, "~/") {
			home, _ := os.UserHomeDir()
			if target == "~" {
				target = home
			} else {
				target = filepath.Join(home, target[2:])
			}
		}
		
		// Calculate the new absolute path
		var newPath string
		if filepath.IsAbs(target) {
			newPath = target
		} else {
			newPath = filepath.Join(currentDir, target)
		}
		newPath = filepath.Clean(newPath)
		
		// Verify it actually exists before updating state
		if info, err := os.Stat(newPath); err == nil && info.IsDir() {
			mu.Lock()
			session.CurrentDir = newPath
			mu.Unlock()
			currentDir = newPath
		} else {
			output = fmt.Sprintf("cd: %s: No such file or directory", target)
		}

	} else if binary == "export" {
		// Handle export VAR=value
		if len(args) > 0 {
			for _, arg := range args {
				if strings.Contains(arg, "=") {
					parts := strings.SplitN(arg, "=", 2)
					if len(parts) == 2 {
						mu.Lock()
						session.Env[parts[0]] = parts[1]
						mu.Unlock()
					}
				}
			}
		} else {
			// Show all environment variables
			mu.Lock()
			for k, v := range session.Env {
				output += fmt.Sprintf("export %s=%s\n", k, v)
			}
			mu.Unlock()
		}

	} else if binary == "pwd" {
		output = currentDir + "\n"

	} else if binary == "echo" {
		// Handle echo with variable expansion
		echoText := strings.Join(args, " ")
		// Expand environment variables
		echoText = expandVariables(echoText, env)
		output = echoText + "\n"

	} else if binary == "env" {
		// Show environment variables
		mu.Lock()
		for k, v := range session.Env {
			output += fmt.Sprintf("%s=%s\n", k, v)
		}
		mu.Unlock()

	} else {
		// --- E. EXECUTE: Run standard binaries (ls, cat, grep) ---
		// Expand variables in command first
		expandedCommand := expandVariables(req.Command, env)
		
		// Build environment slice for command
		cmdEnv := os.Environ()
		mu.Lock()
		for k, v := range session.Env {
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
		}
		mu.Unlock()
		
		// We use "sh -c" so things like wildcards (*) work
		cmd := exec.Command("sh", "-c", expandedCommand)
		cmd.Dir = currentDir
		cmd.Env = cmdEnv
		
		out, err := cmd.CombinedOutput()
		output = string(out)
		if err != nil {
			// Don't show exit code for normal failures
			if exitError, ok := err.(*exec.ExitError); ok {
				// Only show exit code if it's not 0 or 1 (common for grep, etc.)
				if exitError.ExitCode() != 0 && exitError.ExitCode() != 1 {
					output += fmt.Sprintf("\n(Exit Code: %d)", exitError.ExitCode())
				}
			}
		}
	}

	// F. Return Output + New Prompt Location
	json.NewEncoder(w).Encode(CmdResponse{
		Output: output,
		Prompt: fmt.Sprintf("guest@portfolio:%s$ ", filepath.Base(currentDir)),
	})
}

func handleCompletions(w http.ResponseWriter, r *http.Request) {
	var req CompletionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	// Get user's current directory
	mu.Lock()
	session, exists := sessions[req.SessionID]
	if !exists {
		cwd, _ := os.Getwd()
		session = &SessionState{
			CurrentDir: cwd,
			Env:        make(map[string]string),
		}
		// Copy system environment
		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				session.Env[parts[0]] = parts[1]
			}
		}
		sessions[req.SessionID] = session
	}
	currentDir := session.CurrentDir
	mu.Unlock()

	parts := strings.Fields(req.Partial)
	var completions []string

	if len(parts) == 0 {
		// No command yet - suggest common commands
		commonCommands := []string{"ls", "cd", "cat", "pwd", "echo", "grep", "head", "tail", "less"}
		completions = commonCommands
	} else if len(parts) == 1 {
		// First word - could be command or file
		partial := parts[0]
		
		// Check if it's a partial command
		commonCommands := []string{"ls", "cd", "cat", "pwd", "echo", "grep", "head", "tail", "less", "export"}
		for _, cmd := range commonCommands {
			if strings.HasPrefix(cmd, partial) {
				completions = append(completions, cmd)
			}
		}
		
		// Also check files/directories
		entries, err := os.ReadDir(currentDir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, partial) {
					if entry.IsDir() {
						completions = append(completions, name+"/")
					} else {
						completions = append(completions, name)
					}
				}
			}
		}
	} else {
		// Multiple parts - completing file/directory path
		lastPart := parts[len(parts)-1]
		
		// Determine the base directory for completion
		var baseDir string
		if filepath.IsAbs(lastPart) {
			baseDir = filepath.Dir(lastPart)
			if baseDir == "/" {
				baseDir = "/"
			}
		} else {
			// Relative path
			if strings.Contains(lastPart, "/") {
				dirPart := filepath.Dir(lastPart)
				baseDir = filepath.Join(currentDir, dirPart)
			} else {
				baseDir = currentDir
			}
		}
		
		// Get the prefix to match
		prefix := filepath.Base(lastPart)
		
		entries, err := os.ReadDir(baseDir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, prefix) {
					if entry.IsDir() {
						// Reconstruct the path
						if strings.Contains(lastPart, "/") {
							dirPart := filepath.Dir(lastPart)
							completions = append(completions, filepath.Join(dirPart, name)+"/")
						} else {
							completions = append(completions, name+"/")
						}
					} else {
						if strings.Contains(lastPart, "/") {
							dirPart := filepath.Dir(lastPart)
							completions = append(completions, filepath.Join(dirPart, name))
						} else {
							completions = append(completions, name)
						}
					}
				}
			}
		}
	}

	json.NewEncoder(w).Encode(CompletionsResponse{
		Completions: completions,
	})
}

// expandVariables replaces $VAR and ${VAR} with environment variable values
func expandVariables(text string, env map[string]string) string {
	result := text
	
	// Handle ${VAR} syntax
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		varName := result[start+2 : end]
		if val, ok := env[varName]; ok {
			result = result[:start] + val + result[end+1:]
		} else {
			result = result[:start] + result[end+1:]
		}
	}
	
	// Handle $VAR syntax (but not $$)
	for {
		start := strings.Index(result, "$")
		if start == -1 || start == len(result)-1 {
			break
		}
		// Skip $$ (escaped dollar)
		if start > 0 && result[start-1] == '$' {
			result = result[:start-1] + result[start+1:]
			continue
		}
		
		// Find end of variable name (non-alphanumeric or underscore)
		end := start + 1
		for end < len(result) && (result[end] == '_' || 
			(result[end] >= 'a' && result[end] <= 'z') ||
			(result[end] >= 'A' && result[end] <= 'Z') ||
			(result[end] >= '0' && result[end] <= '9')) {
			end++
		}
		
		if end > start+1 {
			varName := result[start+1 : end]
			if val, ok := env[varName]; ok {
				result = result[:start] + val + result[end:]
			} else {
				// Variable not found, remove the $VAR
				result = result[:start] + result[end:]
			}
		} else {
			break
		}
	}
	
	return result
}

// --- 3. THE FRONTEND (Embedded for simplicity) ---
func serveHTML(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <style>
        * { font-family: 'Courier New', Courier, monospace; }
        body { 
            background: #111; 
            color: #0f0; 
            font-family: 'Courier New', Courier, monospace; 
            padding: 20px;
            font-size: 14px;
            line-height: 1.4;
        }
        #output { 
            white-space: pre-wrap;
            font-family: 'Courier New', Courier, monospace !important;
            font-size: 14px !important;
            font-weight: normal !important;
            letter-spacing: 0 !important;
        }
        #prompt { 
            font-family: 'Courier New', Courier, monospace !important;
            font-size: 14px !important;
            font-weight: normal !important;
            letter-spacing: 0 !important;
        }
        input { 
            background: transparent; 
            border: none; 
            color: #0f0 !important; 
            outline: none; 
            width: 80%; 
            font-family: 'Courier New', Courier, monospace !important;
            font-size: 14px !important;
            font-weight: normal !important;
            letter-spacing: 0 !important;
            text-rendering: auto;
        }
        #completions {
            color: #888;
            font-size: 12px;
            margin-top: 2px;
        }
    </style>
</head>
<body>
    <div id="output">Welcome to the Stateless Terminal MVP. Try 'ls', 'cd', 'cat'.<br></div>
    <div>
        <span id="prompt">guest@portfolio:~$ </span>
        <input type="text" id="cmd" autofocus>
        <div id="completions"></div>
    </div>
    <script>
        const input = document.getElementById('cmd');
        const output = document.getElementById('output');
        const promptLabel = document.getElementById('prompt');
        const completionsDiv = document.getElementById('completions');
        // Generate a random session ID for this browser tab
        const sessionID = Math.random().toString(36).substring(7);
        let commandHistory = [];
        let historyIndex = -1;
        let currentCompletions = [];
        let completionIndex = -1;

        // Get completions from server
        async function getCompletions(partial) {
            try {
                const res = await fetch('/api/completions', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({session_id: sessionID, partial: partial})
                });
                const data = await res.json();
                return data.completions || [];
            } catch (e) {
                return [];
            }
        }

        // Show completions
        function showCompletions(completions) {
            if (completions.length > 0) {
                completionsDiv.textContent = completions.join('  ');
            } else {
                completionsDiv.textContent = '';
            }
        }

        input.addEventListener('keydown', async (e) => {
            if (e.key === 'Enter') {
                const command = input.value.trim();
                if (command) {
                    commandHistory.push(command);
                    historyIndex = commandHistory.length;
                }
                output.innerHTML += promptLabel.innerText + command + '\n';
                input.value = '';
                completionsDiv.textContent = '';
                currentCompletions = [];
                completionIndex = -1;

                const res = await fetch('/api/cmd', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({session_id: sessionID, command: command})
                });
                
                const data = await res.json();
                output.innerText += data.output;
                if (!data.output.endsWith('\n') && data.output !== "") output.innerText += '\n';
                
                promptLabel.innerText = data.prompt;
                window.scrollTo(0, document.body.scrollHeight);
            } else if (e.key === 'Tab') {
                e.preventDefault();
                const text = input.value;
                const cursorPos = input.selectionStart;
                const textBeforeCursor = text.substring(0, cursorPos);
                const parts = textBeforeCursor.split(' ');
                const lastPart = parts[parts.length - 1] || '';
                
                if (lastPart.length > 0) {
                    const completions = await getCompletions(textBeforeCursor);
                    if (completions.length > 0) {
                        if (currentCompletions.length === 0 || currentCompletions.join('') !== completions.join('')) {
                            // New completions, reset index
                            currentCompletions = completions;
                            completionIndex = 0;
                        } else {
                            // Same completions, cycle through
                            completionIndex = (completionIndex + 1) % completions.length;
                        }
                        
                        const completion = completions[completionIndex];
                        
                        // Replace the last part with the completion
                        parts[parts.length - 1] = completion;
                        const newValue = parts.join(' ') + text.substring(cursorPos);
                        input.value = newValue;
                        input.setSelectionRange(parts.join(' ').length, parts.join(' ').length);
                        showCompletions(completions);
                    }
                } else {
                    // Empty last part - show all commands
                    const completions = await getCompletions('');
                    if (completions.length > 0) {
                        showCompletions(completions);
                    }
                }
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                if (commandHistory.length > 0) {
                    if (historyIndex > 0) historyIndex--;
                    input.value = commandHistory[historyIndex] || '';
                }
            } else if (e.key === 'ArrowDown') {
                e.preventDefault();
                if (historyIndex < commandHistory.length - 1) {
                    historyIndex++;
                    input.value = commandHistory[historyIndex] || '';
                } else {
                    historyIndex = commandHistory.length;
                    input.value = '';
                }
            } else {
                // Reset completion index when typing
                if (e.key.length === 1 || e.key === 'Backspace' || e.key === 'Delete') {
                    completionIndex = -1;
                    currentCompletions = [];
                    completionsDiv.textContent = '';
                }
            }
        });

        // Show completions as user types (debounced)
        let completionTimeout;
        input.addEventListener('input', async (e) => {
            clearTimeout(completionTimeout);
            const text = input.value;
            if (text && text.length > 0) {
                completionTimeout = setTimeout(async () => {
                    const parts = text.split(' ');
                    const lastPart = parts[parts.length - 1] || '';
                    if (lastPart.length > 0) {
                        const completions = await getCompletions(text);
                        if (completions.length > 0) {
                            showCompletions(completions);
                        } else {
                            completionsDiv.textContent = '';
                        }
                    } else {
                        completionsDiv.textContent = '';
                    }
                }, 300);
            } else {
                completionsDiv.textContent = '';
            }
        });
    </script>
</body>
</html>`
	w.Write([]byte(html))
}

func main() {
	http.HandleFunc("/api/cmd", handleCommand)
	http.HandleFunc("/api/completions", handleCompletions)
	http.HandleFunc("/", serveHTML)
	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

