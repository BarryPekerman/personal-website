"""Load the site in headless Chromium at desktop and phone sizes and check that
it works: no console errors, page errors or failed requests (a CSP violation
is a console error), the terminal answers, the phone layout has no terminal.

    pip install playwright==1.63.0 && playwright install chromium
    python scripts/pagecheck.py http://localhost:8080 screenshots/

Screenshots of each page land in the output directory. Exits 1 on any failure.
"""
import os
import sys

from playwright.sync_api import sync_playwright

base, out = sys.argv[1].rstrip("/"), sys.argv[2]
os.makedirs(out, exist_ok=True)
problems, failures = [], []


def check(name, ok):
    print(("ok    " if ok else "FAIL  ") + name)
    if not ok:
        failures.append(name)


def watch(page, tag):
    page.on("console", lambda m: m.type == "error" and problems.append(f"{tag}: console error: {m.text}"))
    page.on("pageerror", lambda e: problems.append(f"{tag}: page error: {e}"))
    page.on("requestfailed", lambda r: problems.append(f"{tag}: request failed: {r.url}"))
    sockets = []
    page.on("websocket", lambda w: sockets.append(w.url))
    return sockets


def terminal_text(page):
    """What the terminal shows (its visible rows)"""
    return page.evaluate("() => [...document.querySelectorAll('.xterm-rows > div')].map(d => d.textContent).join('\\n')")


def wait_for_text(page, text, timeout=5000):
    try:
        page.wait_for_function("t => [...document.querySelectorAll('.xterm-rows > div')].some(d => d.textContent.includes(t))",
                               arg=text, timeout=timeout)
        return True
    except Exception:
        return False


with sync_playwright() as p:
    browser = p.chromium.launch()

    # Desktop front page: HUD, map, terminal opened on the first key press
    page = browser.new_page(viewport={"width": 1400, "height": 900})
    sockets = watch(page, "desktop /")
    page.goto(base + "/")
    page.wait_for_timeout(2500)  # the boot animation
    check("desktop: no session before the visitor types", len(sockets) == 0)
    check("desktop: terminal shows how to start", "Press any key" in terminal_text(page))
    check("desktop: map draws the land", page.locator("#map circle").count() > 100)
    check("desktop: status panel live", page.locator("#live").inner_text().endswith("LIVE"))
    page.screenshot(path=f"{out}/desktop-before.png")
    page.click("#terminal")
    page.keyboard.press("a")
    check("desktop: a key press opens the session", wait_for_text(page, "$ "))
    page.keyboard.type("help")
    page.keyboard.press("Enter")
    check("desktop: help answers", wait_for_text(page, "Available commands"))
    page.click(".sys >> text=the-handover")
    check("desktop: a project opens in the terminal", wait_for_text(page, "STACK:"))
    check("desktop: one session", len(sockets) == 1)
    page.screenshot(path=f"{out}/desktop.png")

    # The desktop route is the page's edge, then the terminal's, never a status
    # request's. Fake Cloudflare headers: the WebSocket enters at FRA (the
    # context's headers), the page at TLV and every status request at MRS.
    ctx = browser.new_context(viewport={"width": 1400, "height": 900},
                              extra_http_headers={"CF-IPCountry": "DE", "CF-Ray": "0000000000000000-FRA"})
    def at(colo):
        return lambda route: route.continue_(headers={**route.request.headers, "cf-ray": "0000000000000000-" + colo})
    page = ctx.new_page()
    watch(page, "desktop edges")
    page.route(lambda url: url.rstrip("/") == base, at("TLV"))
    page.route("**/api/status*", at("MRS"))
    page.goto(base + "/")
    page.wait_for_timeout(2500)
    route_text = lambda: page.locator("#route").inner_text()
    check("desktop route: the page's edge (TLV), not the status request's (MRS)",
          "TLV" in route_text() and "MRS" not in route_text() and "page request" in route_text())
    page.click("#terminal")
    page.keyboard.press("a")
    wait_for_text(page, "$ ")
    page.wait_for_timeout(1000)
    check("desktop route: the terminal's edge (FRA) once a session is open",
          "FRA" in route_text() and "terminal connection" in route_text())
    ctx.close()

    # Full-screen terminal
    page = browser.new_page(viewport={"width": 1200, "height": 800})
    watch(page, "/terminal")
    page.goto(base + "/terminal")
    check("/terminal: connects at once", wait_for_text(page, "$ "))
    page.click("#terminal")
    page.keyboard.type("connect github")
    page.keyboard.press("Enter")
    check("/terminal: connect github", wait_for_text(page, "github.com/BarryPekerman"))
    page.click("#help-popup")
    check("/terminal: help popup opens", page.locator("#help-content").is_visible())
    page.screenshot(path=f"{out}/terminal.png")

    # Phone: a page to read, no terminal
    phone = browser.new_context(viewport={"width": 390, "height": 844}, is_mobile=True, has_touch=True, device_scale_factor=2)
    page = phone.new_page()
    sockets = watch(page, "phone /")
    page.goto(base + "/")
    page.wait_for_timeout(3000)
    check("phone: no terminal session", len(sockets) == 0)
    check("phone: desktop layout hidden", not page.locator(".app").is_visible())
    gists = page.locator("#p-projects .gist").all_text_contents()
    check("phone: every project's summary loaded", len(gists) >= 5 and all(g and "could not load" not in g for g in gists))
    check("phone: nothing wider than the screen", not page.evaluate("document.documentElement.scrollWidth > innerWidth"))
    page.screenshot(path=f"{out}/phone.png", full_page=True)
    page.goto(base + "/terminal")
    page.wait_for_timeout(1500)
    check("phone: /terminal goes to the front page", page.url.rstrip("/") == base)

    browser.close()

check("no console errors, page errors or failed requests", not problems)
for x in problems:
    print("      " + x)
sys.exit(1 if failures else 0)
