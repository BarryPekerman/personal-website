# Project notes

Working notes and decisions. Last updated 2026-10-16.

## What this project is

A portfolio site styled after *Uplink*: visitors explore my CV and projects
in a terminal in the browser. v1 is a simulated shell, so nothing a visitor
types runs anywhere. v2 adds a real shell in an isolated pod per visitor.

## Decisions so far

- **v1 shell is virtual** (`internal/shell/virtual.go`). It's cheap to run,
  has nothing to break into, and is easy to test.
- **Command blocklists don't work.** xterm.js sends one keystroke per message,
  and blocklists can be bypassed anyway. Isolation is the only real defense.
- **No fabricated content.** The old placeholder CV ("Senior Engineer,
  Stanford") is gone. Everything in `content/` must be true.
- **Go stays** (not a static JS site). The shell backends share one `Session`
  interface, so v2 is a backend swap, not a rewrite.

## Real BusyBox: options considered

| Option | Where it runs | Verdict |
|---|---|---|
| A. BusyBox pod per visitor | my k3s cluster | **v2.** Needs gVisor, per-IP rate limits, quotas, idle timeout, warm pod pool |
| B. BusyBox in the browser (WASM / v86) | visitor's machine | Zero risk, but heavy download and mostly frontend work |
| C. One shared container | one pod | No: visitors would interfere with each other |

`connect sandbox` exists as the v2 hook (prints "offline" for now).

## Who visits and what they need

| Visitor | What happens today | What would help |
|---|---|---|
| Random visitor | Black screen, doesn't know what to type, leaves in ~10s | Guided start: clickable commands, a short auto-typed intro |
| Non-technical HR | Won't type anything. **The terminal hurts here** | Name, title and contact visible immediately; a "plain CV / PDF" link |
| Programmer | Pokes at it, finds there are no pipes or grep, decides "cute fake shell" | Clean public repo and tests. Don't pretend to be a real shell |
| DevOps engineer | The page itself shows nothing about infrastructure | **Show the real infrastructure**: deployment, CI, live status |

## The idea that makes it worth showing: a live console

Today it's a clone of a common genre ("terminal portfolio"). It becomes
distinctive if the terminal is a **window into the infrastructure running it**:

- `status`: real pod name, node, uptime, git commit, image tag, ArgoCD sync state
- `deploys`: the last few CI runs and commits
- `top` / `metrics`: live numbers from Prometheus (requests, sessions, memory)
- `kubectl get pods`: read-only, through a ServiceAccount restricted to its own namespace

These are read-only and safe, and they prove the GitOps / CI / monitoring
claims instead of stating them.

## Roadmap

1. [x] Virtual shell, tests, lint, CI, Docker image
2. [ ] Review `content/` (every line true; fill the TODOs)
3. [ ] HR-proof the landing page: visible name, title, contact, plain-CV link, clickable commands
4. [ ] Deploy to k3s via ArgoCD (rewrite the stale `k8s/` and `argocd/` manifests)
5. [ ] `status` / `deploys` live commands
6. [ ] Uplink styling
7. [ ] v2: real sandbox (option A), behind `connect sandbox`
8. [ ] Make the repo public once 2–4 are done

Timebox: steps 2–4 are about one weekend. This is a side project next to
LFCS (main priority) and the Ethereum node project.
