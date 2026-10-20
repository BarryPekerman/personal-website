# portfolio-terminal

This website.

PROBLEM
-------
A DevOps portfolio should prove its claims, not list them. A scrolling resume
tells you I know Go, Kubernetes and security; it doesn't show you. I wanted the
site itself to be the evidence -- something a technical reader could poke at.

BUILD
-----
- A Go backend streams a terminal over WebSockets to xterm.js in the browser;
  the server owns the shell (echo, prompt, history, tab completion), the
  browser only renders
- The shell you are using is a virtual shell written in Go: it reads files
  embedded in the binary, so nothing you type ever runs on a real system
- One Session interface, three swappable backends (virtual / local / k8s), so
  the safe simulation and a future real sandbox share the same contract
- Tested with the race detector, linted, and scanned with govulncheck on every
  push through GitHub Actions

RESULT
------
The page you are reading is the demo: a running, tested, sandboxed Go service
rather than a screenshot of one. Next: a real shell in one locked-down
Kubernetes pod per visitor (gVisor runtime, per-IP limits, warm pod pool).

STACK: Go, WebSockets, xterm.js, Docker, Kubernetes, GitHub Actions, govulncheck
