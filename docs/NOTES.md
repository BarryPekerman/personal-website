# Project notes

Working notes and decisions. Last updated 2026-10-16.

**Current direction: see "No fake widgets" below. It overrides older sections where they disagree.**

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

> **Superseded.** The Handover (play.barrypekerman.com) already is the real,
> isolated shell, with one network-less Incus container per player. This site
> links to it instead of building its own sandbox. Kept for the reasoning.

| Option | Where it runs | Verdict |
|---|---|---|
| A. BusyBox pod per visitor | my k3s cluster | **v2.** Needs gVisor, per-IP rate limits, quotas, idle timeout, warm pod pool |
| B. BusyBox in the browser (WASM / v86) | visitor's machine | Zero risk, but heavy download and mostly frontend work |
| C. One shared container | one pod | No: visitors would interfere with each other |

`connect sandbox` exists as a placeholder; it goes away (see below).

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

## Direction: no fake widgets (2026-10-16)

The mockup was fun but felt wrong: it has buttons and panels that don't do
anything. In Uplink every element was a tool; here they only looked like tools.

**Rule: every element either does something useful for the visitor or shows
something true. Everything else goes.**

| Element | Was | Verdict |
|---|---|---|
| Name, title, CV, links | real | keep (the CV PDF must exist) |
| Music / sfx toggles | opt-in flavor | keep; honest about being flavor |
| Name-cracking intro | 1.5 s flavor | keep, short and skippable |
| Contract: The Handover | fake | **make real:** live seats from `play.barrypekerman.com/healthz`, button into the game (landing page, never `/play`) |
| Remote systems (projects) | text only | **make real:** each opens repo, short write-up, live demo if any |
| Status panel | made-up numbers | **make real:** pod + node via the Downward API, git commit baked in at build, real uptime |
| Route map | decoration | **make real or cut:** draw the actual path from Cloudflare headers (visitor country, edge location such as TLV) → OKE region → this pod |
| Trace meter | meaningless | cut |
| Locked sandbox | promises something absent | cut |
| Console | repeated the panels | keep, with commands that query real things (`status`, `route`, `connect handover`) |

Result: fewer elements, all working. The look and sound stay; everything you
read or click is true. The panels showing live infrastructure are what make it
a DevOps portfolio instead of a theme.

Open question to settle with fresh eyes: does this match what felt wrong?

## Roadmap

1. [x] Virtual shell, tests, lint, CI, Docker image (PR #1, green)
2. [ ] Fresh-eyes review of the "no fake widgets" direction
3. [ ] Review `content/` (every line true; fill the TODOs; make the CV PDF)
4. [ ] Page layout: HUD (name, title, CV, links), projects, contract card, console
5. [ ] Real data: game status from `/healthz`; pod/node/commit/uptime; route from Cloudflare headers
6. [ ] Deploy to OKE via ArgoCD (rewrite the stale `k8s/` and `argocd/` manifests)
7. [ ] Uplink styling, music/sfx toggles
8. [ ] Remove k8s mode and `connect sandbox` (The Handover is the real shell)
9. [ ] Make the repo public once 3–6 are done

Timebox: steps 2–4 are about one weekend. This is a side project next to
LFCS (main priority) and the Ethereum node project.
