# Portfolio Terminal

My portfolio website, styled after the hacking game *Uplink*: visitors get a
terminal in the browser and explore my CV and projects with `ls`, `cat` and `cd`.
Live at https://barrypekerman.com, on my OKE cluster; around the terminal, the
page shows the real deployment serving it and the visitor's route to it.

## How it works

```
browser (xterm.js) ──WebSocket──► Go server ──► virtual shell
  keystrokes as {"type":"input"}                 (reads files embedded from content/)
  ◄── raw terminal output (binary frames)
```

The browser only renders. The server owns the shell: echo, prompt, line
editing, history and tab completion all happen server-side. The shell is
simulated, written in Go: nothing a visitor types is executed anywhere. For a
real, isolated shell, see [The Handover](https://play.barrypekerman.com/).

## Run it

```bash
./run-local.sh                    # http://localhost:8080
# or
docker build -t portfolio . && docker run --rm -p 8080:8080 portfolio
```

Edit `content/` to change what visitors see (`.motd` is the login banner),
then restart: the files are compiled into the binary, like `web/`. Pages put
the commit on every static URL (`?v=`), so a deploy never serves stale JS.

## Metrics

Prometheus metrics are on a separate port, `:9090/metrics` (`METRICS_ADDR`),
which the Cloudflare tunnel doesn't reach: terminal sessions (open, started,
refused by limit, ended by reason, setup errors), HTTP requests and latency
per route, failed reads of the status sources, and the Go runtime. Logs are
JSON on stdout (`log/slog`; `LOG_LEVEL=debug` for more).

## Develop

```bash
go test -race ./...
golangci-lint run
```

CI runs the build, tests, linter and `govulncheck`, loads the pages in
headless Chromium at desktop and phone sizes (`scripts/pagecheck.py`: fails on
any console error, screenshots uploaded), and builds the image and scans it
with Grype. On `main` it pushes the image, tagged with the commit, with an
SBOM and provenance attached, and signs it with cosign. Run the page check
locally:

```bash
pip install playwright==1.63.0 && playwright install chromium
./run-local.sh &   # then:
python scripts/pagecheck.py http://localhost:8080 screenshots/
```

## Layout

```
cmd/server/        entry point: HTTP routes
content/           files visitors browse (embedded via go:embed)
internal/handlers/ WebSocket <-> shell session bridge
internal/shell/    the virtual shell
internal/status/   /api/status: the live deployment and the visitor's route
internal/geo/      positions for the route map (generated: go generate ./internal/geo)
web/               pages (templates/) and static files, embedded in the binary;
                   third-party libraries vendored in web/static/vendor/
```

## Deploy

CI pushes `ghcr.io/barrypekerman/personal-website:<commit>` (signed) on every
merge to `main`. Deploying is bumping that commit in `oke-gitops` `portfolio/deployment.yaml`;
ArgoCD does the rest. Where everything runs: [docs/NOTES.md](docs/NOTES.md#where-it-runs-live-since-2026-10-20).

## Roadmap

Details and reasoning: [docs/NOTES.md](docs/NOTES.md).

- [x] Virtual shell, tests, lint and CI
- [x] Deploy to my OKE cluster via ArgoCD
- [x] Uplink styling, live status panel and route map
- [ ] The real CV
- [x] Remove k8s mode (The Handover is the real shell)
