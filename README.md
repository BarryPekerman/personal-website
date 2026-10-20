# Portfolio Terminal

My portfolio website, styled after the hacking game *Uplink*: visitors get a
terminal in the browser and explore my CV and projects with `ls`, `cat` and `cd`.
Live at https://barrypekerman.com, on my OKE cluster; around the terminal, the
page shows the real deployment serving it and the visitor's route to it.

## How it works

```
browser (xterm.js) ──WebSocket──► Go server ──► shell backend
  keystrokes as {"type":"input"}                 virtual | local | k8s
  ◄── raw terminal output (binary frames)
```

The browser only renders. The server owns the shell: echo, prompt, line
editing, history and tab completion all happen server-side.

### Shell backends (`APP_MODE`)

| Mode | What runs | Use |
|---|---|---|
| `virtual` (default) | A simulated shell written in Go, reading files embedded in the binary from `content/` | Production. Nothing a visitor types is executed anywhere. |
| `local` | Real bash on the host, in `content/` | Development only. **Never expose this.** |
| `k8s` | Real shell in a throwaway, locked-down pod per visitor | Work in progress (v2) |

## Run it

```bash
./run-local.sh                    # http://localhost:8080
# or
docker build -t portfolio . && docker run --rm -p 8080:8080 portfolio
```

Edit `content/` to change what visitors see (`.motd` is the login banner),
then restart: the files are compiled into the binary.

## Develop

```bash
go test -race ./...
golangci-lint run
```

CI runs the build, tests, linter, `govulncheck` and a Docker build on every push.

## Layout

```
cmd/server/        entry point: HTTP routes, picks the shell backend
content/           files visitors browse (embedded via go:embed)
internal/handlers/ WebSocket <-> shell session bridge
internal/shell/    Session interface + virtual, local and k8s backends
internal/k8s/      pod lifecycle for k8s mode
internal/status/   /api/status: the live deployment and the visitor's route
internal/geo/      positions for the route map (generated: go generate ./internal/geo)
web/               front page (home.html), full-screen terminal, xterm.js client
manifests/docker/  guest image for k8s mode
```

## Deploy

CI pushes `ghcr.io/barrypekerman/personal-website:<commit>` on every merge to
`main`. Deploying is bumping that commit in `oke-gitops` `portfolio/deployment.yaml`;
ArgoCD does the rest. Where everything runs: [docs/NOTES.md](docs/NOTES.md#where-it-runs-live-since-2026-10-20).

## Roadmap

Details and reasoning: [docs/NOTES.md](docs/NOTES.md).

- [x] Virtual shell, tests, lint and CI
- [x] Deploy to my OKE cluster via ArgoCD
- [x] Uplink styling, live status panel and route map
- [ ] The real CV
- [ ] Remove k8s mode (The Handover is the real shell)
