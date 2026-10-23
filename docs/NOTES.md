# Project notes

Working notes and decisions. Last updated 2026-10-20.

**Current direction: see "No fake widgets" below. It overrides older sections where they disagree.**

## What this project is

A portfolio site styled after *Uplink*: visitors explore my CV and projects
in a terminal in the browser. The shell is simulated, so nothing a visitor
types runs anywhere. The real, isolated shell is The Handover
(play.barrypekerman.com), which the site links to.

## Decisions so far

- **v1 shell is virtual** (`internal/shell/virtual.go`). It's cheap to run,
  has nothing to break into, and is easy to test.
- **Command blocklists don't work.** xterm.js sends one keystroke per message,
  and blocklists can be bypassed anyway. Isolation is the only real defense.
- **No fabricated content.** The old placeholder CV ("Senior Engineer,
  Stanford") is gone. Everything in `content/` must be true.
- **Go stays** (not a static JS site): the server owns the shell and serves the
  live status.
- **No real shell here** (2026-10-23): the `local` and `k8s` backends,
  `connect sandbox` and the guest image are deleted. A real bash was one env
  var away in the production image; The Handover is the real shell.

## Real BusyBox: options considered

> **Superseded.** The Handover (play.barrypekerman.com) already is the real,
> isolated shell, with one network-less Incus container per player. This site
> links to it instead of building its own sandbox. Kept for the reasoning.

| Option | Where it runs | Verdict |
|---|---|---|
| A. BusyBox pod per visitor | my k3s cluster | **v2.** Needs gVisor, per-IP rate limits, quotas, idle timeout, warm pod pool |
| B. BusyBox in the browser (WASM / v86) | visitor's machine | Zero risk, but heavy download and mostly frontend work |
| C. One shared container | one pod | No: visitors would interfere with each other |

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
| Metrics | `:9090/metrics` (`METRICS_ADDR`), separate from the site port so the tunnel never serves it; scraping it needs the port on the Service and a ServiceMonitor in `oke-gitops` (review H5) |
| Status panel | made-up numbers | **make real:** pod + node via the Downward API, git commit baked in at build, real uptime |
| Route map | decoration | **make real or cut:** draw the actual path from Cloudflare headers (visitor country, edge location such as TLV) → OKE region → this pod |
| Trace meter | meaningless | cut |
| Locked sandbox | promises something absent | cut |
| Console | repeated the panels | keep, with commands that query real things (`status`, `route`, `connect handover`) |

Result: fewer elements, all working. The look and sound stay; everything you
read or click is true. The panels showing live infrastructure are what make it
a DevOps portfolio instead of a theme.

Open question to settle with fresh eyes: does this match what felt wrong?

## Where it runs (live since 2026-10-20)

https://barrypekerman.com (and www). Nothing about the site lives in The
Handover's infra (handover-hosting), so taking the game down can't take the
site with it.

| Piece | Where |
|---|---|
| Image | `ghcr.io/barrypekerman/personal-website:<commit>`, multi-arch (the nodes are ARM), built by CI on every merge to `main`; `ghcr-retention.yml` keeps the newest 10 |
| Deploy | `oke-gitops` `portfolio/`: ArgoCD Application; deploying is bumping the image commit in `deployment.yaml` |
| Exposure | Cloudflare Tunnel `oke-portfolio`: cloudflared runs in the cluster (outbound only); token is a SOPS secret in `oke-gitops` |
| DNS, tunnel | `oci-oke-infra` `terraform/cloudflare.tf` (local runs; HCP state `Berries-/oci-oke-infra`) |
| Status panel | `/api/status`: pod and node (Downward API), commit (`-X main.commit`), uptime, open terminals, The Handover's seats (its `/healthz`) |
| Route map | Visitor country (`CF-IPCountry`) → edge (`CF-Ray` suffix of the page request, then of the terminal's WebSocket; on a phone, of the status request it draws) → the tunnel connection it enters at (cloudflared metrics) → cluster; positions from `internal/geo` (generated: OurAirports, mledoze/countries) |

## Roadmap

1. [x] Virtual shell, tests, lint, CI, Docker image (PR #1)
2. [x] "No fake widgets" direction: settled and built
3. [ ] Review `content/`: every line true (done); the real CV and its PDF, email and LinkedIn still to come
4. [x] Page layout: HUD (name, title, CV, links), projects, contract card, console
5. [x] Real data: game seats from `/healthz`; pod/node/commit/uptime; route from Cloudflare headers and the tunnel's connections
6. [x] Deploy to OKE via ArgoCD (`oke-gitops` `portfolio/`; the stale `k8s/` and `argocd/` manifests are deleted)
7. [x] Uplink styling, music/sfx toggles
8. [x] Remove k8s mode and `connect sandbox` (The Handover is the real shell)
9. [x] Make the repo public
