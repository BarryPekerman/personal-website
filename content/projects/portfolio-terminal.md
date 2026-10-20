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
- Runs on my OKE (Oracle Kubernetes) cluster, deployed by ArgoCD from a GitOps
  repo; Cloudflare reaches it through an outbound-only tunnel, so the site
  needs no inbound port or load balancer
- The panels around the terminal show the live deployment: the pod and node
  serving you, the commit it was built from, and your real route to it
- Tested with the race detector, linted, and scanned with govulncheck on every
  push through GitHub Actions

RESULT
------
The page you are reading is the demo: a running, tested, sandboxed Go service
rather than a screenshot of one. For a real shell, see the-handover.

STACK: Go, WebSockets, xterm.js, Docker, Kubernetes (OKE), ArgoCD, Terraform,
Cloudflare Tunnel, GitHub Actions, govulncheck
