// Front page: projects, boot animation, clock, live status and the phone
// layout. Loaded after terminal.js, audio.js and routemap.js.
// ---------- projects: the write-ups in content/projects, listed by /api/projects ----------
const projectsReady = fetch('/api/projects').then(r => {
  if (!r.ok) throw new Error(r.status);
  return r.json();
});

const $ = id => document.getElementById(id);

// A phone gets its own layout (see the CSS) and no terminal: the same media
// query decides both
const PHONE = matchMedia('(max-width: 700px), (max-height: 500px) and (pointer: coarse)');

// ---------- the edge the desktop route shows ----------
// The page's own request, then the terminal's connection once a session is
// open: those are what the visitor uses. A status request every 15 s can
// enter Cloudflare somewhere else, so the desktop asks for this edge's route.
// The phone draws the status request itself as its traffic, so it shows that
// request's own route.
let edge = { country: document.body.dataset.country, colo: document.body.dataset.colo, via: 'page request' };

// ---------- maps: the terminal reports its traffic to the desktop one ----------
const routeMap = new RouteMap($('map'));
const phoneMap = new RouteMap($('p-map'));

// ---------- terminal: desktop only, started when the desktop layout shows ----------
// xterm.js and the terminal client are loaded only for the desktop layout:
// a phone never shows the terminal, so it doesn't download them
const VERSION = new URL(document.currentScript.src).searchParams.get('v');
function loadScript(path) {
  return new Promise((resolve, reject) => {
    const s = document.createElement('script');
    s.src = path + '?v=' + VERSION;
    s.async = false;  // run in the order added
    s.onload = resolve; s.onerror = reject;
    document.body.append(s);
  });
}
let terminal = null, loading = false;
async function initDesktop() {
  if (terminal || loading || PHONE.matches) return;
  loading = true;
  const css = document.createElement('link');
  css.rel = 'stylesheet'; css.href = '/static/vendor/xterm.css?v=' + VERSION;
  document.head.append(css);
  try {
    await Promise.all(['/static/vendor/xterm.js', '/static/vendor/addon-fit.js',
      '/static/vendor/addon-web-links.js', '/static/js/terminal.js'].map(loadScript));
  } catch (e) {
    $('terminal').textContent = 'The terminal could not load. Reload to try again.';
    loading = false;
    return;
  }
  terminal = new PortfolioTerminal($('terminal'), {
    fontSize: 13,
    fontFamily: 'ui-monospace, "SFMono-Regular", "JetBrains Mono", Menlo, Consolas, monospace',
    theme: { background: '#03060b', foreground: '#b9d6ee', cursor: '#5fd3ff', selectionBackground: '#1d3557' },
    lazy: true,
    onInput: data => { if (data === '\r') Sfx.enter(); else Sfx.key(); routeMap.packet('out'); },
    onOutput: () => routeMap.packet('in'),
    onEdge: e => { edge = { country: e.country, colo: e.colo, via: 'terminal connection' }; refreshStatus(); },
  });
  if (bootDone) terminal.term.focus();
}
initDesktop();
PHONE.addEventListener('change', initDesktop);
document.addEventListener('click', e => {
  const b = e.target.closest('[data-run]');
  if (b && terminal) terminal.run(b.dataset.run);
});
['help', 'cv', 'projects', 'contact', 'clear'].forEach(c => {
  const b = document.createElement('button');
  b.className = 'btn'; b.textContent = c; b.dataset.run = c;
  document.getElementById('chips').appendChild(b);
});

const sysBox = document.getElementById('systems');
projectsReady.then(list => {
  document.getElementById('sys-count').textContent = list.length;
  list.forEach(s => {
    const d = document.createElement('div');
    d.className = 'sys';
    const name = document.createElement('div');
    name.className = 'name'; name.textContent = s.name;
    const meta = document.createElement('div');
    meta.className = 'meta';
    const open = document.createElement('span'); open.textContent = 'OPEN';
    meta.append(open);
    if (s.link) {
      const a = document.createElement('a');
      a.href = s.link; a.target = '_blank'; a.rel = 'noopener'; a.textContent = s.label + ' ↗';
      a.onclick = e => e.stopPropagation();
      meta.append(a);
    }
    d.append(name, meta);
    d.onclick = () => {
      sysBox.querySelectorAll('.sys').forEach(x => x.classList.toggle('active', x === d));
      Sfx.connected();
      if (terminal) terminal.run('cat ~/projects/' + s.file);
    };
    sysBox.appendChild(d);
  });
}).catch(() => { sysBox.textContent = 'could not load the projects'; });

// ---------- boot: password breaker ----------
const NAME = 'BARRY PEKERMAN';
const GLYPHS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789#$%&@';
const crack = document.getElementById('crack');
const cells = [...NAME].map(ch => {
  const s = document.createElement('span');
  if (ch === ' ') s.className = 'gap';
  crack.appendChild(s);
  return { el: s, ch, locked: ch === ' ' };
});
let bootDone = false;
const spin = setInterval(() => {
  cells.forEach(c => { if (!c.locked) c.el.textContent = GLYPHS[Math.random() * GLYPHS.length | 0]; });
}, 45);
cells.filter(c => !c.locked).forEach((c, i) => setTimeout(() => lock(c), 250 + i * 85));
function lock(c) { c.locked = true; c.el.textContent = c.ch; c.el.classList.add('locked'); }
setTimeout(() => { document.getElementById('boot-status').textContent = 'IDENTITY CONFIRMED · CONNECTION ESTABLISHED'; }, 1450);
setTimeout(finishBoot, 1900);
function finishBoot() {
  if (bootDone) return;
  bootDone = true;
  clearInterval(spin);
  document.getElementById('boot').classList.add('gone');
  if (terminal) terminal.term.focus();
}
document.getElementById('boot').addEventListener('click', finishBoot);
window.addEventListener('keydown', finishBoot, { once: true });

// ---------- clock ----------
setInterval(() => {
  document.getElementById('clock').textContent = new Date().toISOString().slice(11, 19);
}, 1000);

// ---------- status: /api/status, refreshed every 15 s ----------
let started = null;
function fmtUptime(ms) {
  const s = Math.max(0, ms / 1000 | 0);
  const d = s / 86400 | 0, h = s / 3600 % 24 | 0, m = s / 60 % 60 | 0, sec = s % 60;
  const pad = n => String(n).padStart(2, '0');
  return (d ? d + 'd ' : '') + pad(h) + ':' + pad(m) + ':' + pad(sec);
}
setInterval(() => { if (started) $('s-uptime').textContent = fmtUptime(Date.now() - started); }, 1000);

function routeItem(title, detail) {
  const li = document.createElement('li');
  const b = document.createElement('b'); b.textContent = title;
  const small = document.createElement('small'); small.textContent = detail;
  li.append(b, small);
  return li;
}

async function refreshStatus() {
  // on the phone the status request is the traffic: it's drawn going out and back
  phoneMap.packet('out');
  try {
    const desktopEdge = !PHONE.matches && /^[A-Z]{3}$/.test(edge.colo || '');
    const query = desktopEdge ? '?' + new URLSearchParams({ country: edge.country, colo: edge.colo }) : '';
    const r = await fetch('/api/status' + query, { cache: 'no-store' });
    if (!r.ok) throw new Error(r.status);
    const s = await r.json();
    $('s-pod').textContent = s.pod || 'n/a (not in a cluster)';
    $('s-pod').title = s.pod || '';
    $('s-node').textContent = s.node || 'n/a';
    $('s-region').textContent = s.region || 'n/a';
    const c = $('s-commit');
    c.textContent = '';
    if (/^[0-9a-f]{40}$/.test(s.commit)) {
      const a = document.createElement('a');
      a.href = 'https://github.com/BarryPekerman/personal-website/commit/' + s.commit;
      a.target = '_blank'; a.rel = 'noopener'; a.textContent = s.commit.slice(0, 7);
      c.appendChild(a);
    } else c.textContent = s.commit;
    started = Date.parse(s.started);
    $('s-sessions').textContent = s.sessions;

    // the route, worked out by the server: country -> the edge that took the
    // request -> (when the tunnel has no connection there) the edge Cloudflare
    // most likely carries it on to -> the cluster
    const route = s.route || [];
    const took = 'Cloudflare edge that took your ' + (desktopEdge ? edge.via : 'request');
    const detail = {
      you: () => 'your browser',
      edge: h => h.tunnel ? took + '; the tunnel connects here, so it goes straight in' : took,
      forward: h => `the tunnel has no connection at ${route[1].label}: Cloudflare carries it on, most likely to ${h.label} (nearest)`,
      cluster: () => 'pod ' + (s.pod || 'local') + ', node ' + (s.node || 'n/a'),
    };
    const title = {
      you: h => 'you' + (h.label ? ' · ' + h.label : ''),
      edge: h => 'Cloudflare' + (h.label ? ' · ' + h.label : ''),
      forward: h => 'Cloudflare · ' + h.label,
      cluster: h => 'OKE' + (h.label ? ' · ' + h.label : ''),
    };
    $('route').replaceChildren(...route.map(h => routeItem(title[h.kind](h), detail[h.kind](h))));
    const hops = route.map(h => ({ pos: h.pos, label: title[h.kind](h), you: h.kind === 'you' }));
    routeMap.draw(hops);
    phoneMap.draw(hops);
    phoneMap.packet('in');
    $('p-route').textContent = route.map(h => title[h.kind](h)).join('  →  ');
    const strip = $('p-strip');
    strip.replaceChildren();
    const line = (label, value) => {
      const d = document.createElement('div');
      d.append(label + ' ');
      const b = document.createElement('b');
      if (value instanceof Node) b.append(value); else b.textContent = value;
      d.append(b);
      strip.append(d);
    };
    line('served by pod', s.pod || 'local');
    line('on', 'OKE ' + (s.region || '') + ', node ' + (s.node || 'n/a'));
    line('built from', c.firstChild ? c.firstChild.cloneNode(true) : s.commit);
    $('p-seats').textContent = s.handover
      ? `${s.handover.players} / ${s.handover.max} seats taken right now`
      : 'seat count unavailable';

    $('seats').textContent = s.handover
      ? `${s.handover.players} / ${s.handover.max} seats taken right now`
      : 'seat count unavailable';
    for (const id of ['live', 'p-live']) { $(id).textContent = '● LIVE'; $(id).className = 'live'; }
  } catch (e) {
    for (const id of ['live', 'p-live']) { $(id).textContent = '● OFFLINE'; $(id).className = 'stale'; }
  }
}
refreshStatus();
// no polling while the tab is hidden; catch up when it shows again
setInterval(() => { if (!document.hidden) refreshStatus(); }, 15000);
document.addEventListener('visibilitychange', () => { if (!document.hidden) refreshStatus(); });

// ---------- phone: projects and CV as text, from the files the terminal shows ----------
async function contentText(path) {
  const r = await fetch('/api/content/' + path);
  if (!r.ok) throw new Error(r.status);
  return r.text();
}

// appendText adds text to an element, with URLs as links
function appendText(el, text) {
  for (const part of text.split(/(https?:\/\/\S+)/)) {
    if (/^https?:\/\//.test(part)) {
      const a = document.createElement('a');
      a.href = part; a.target = '_blank'; a.rel = 'noopener'; a.textContent = part;
      el.append(a);
    } else if (part) el.append(part);
  }
}

// renderText turns a terminal write-up (hard-wrapped at 80 columns, HEADINGS
// underlined with dashes, "- " bullets) into paragraphs that wrap on a phone
function renderText(text, box) {
  const lines = text.replace(/\r/g, '').split('\n');
  let para = null, list = null, item = null;
  const add = (tag, cls) => { const e = document.createElement(tag); if (cls) e.className = cls; box.append(e); return e; };
  for (let i = 0; i < lines.length; i++) {
    const l = lines[i], t = l.trim();
    if (!t || /^# /.test(l)) { para = list = item = null; continue; }
    if (/^-{3,}$/.test((lines[i + 1] || '').trim())) {
      para = list = item = null;
      add('h4').textContent = t; i++;
      continue;
    }
    if (/^- /.test(l)) {
      para = null;
      if (!list) list = add('ul');
      item = document.createElement('li'); appendText(item, t.slice(2)); list.append(item);
      continue;
    }
    if (item && /^\s/.test(l)) { appendText(item, ' ' + t); continue; }
    // "Key: value" lines (contact, STACK) stand alone
    if (/^[A-Za-z][\w-]*( [\w-]+)?:\s/.test(t) || /^https?:\/\//.test(t)) {
      list = item = null;
      para = add('p', /^STACK:/.test(t) ? 'stack' : '');
      appendText(para, t);
      if (!/^STACK:/.test(t)) para = null;
      continue;
    }
    list = item = null;
    if (para) appendText(para, ' ' + t); else { para = add('p'); appendText(para, t); }
  }
}

projectsReady.then(list => list.forEach(async s => {
  const d = document.createElement('details');
  const sum = document.createElement('summary');
  const name = document.createElement('div'); name.className = 'name'; name.textContent = s.name;
  const g = document.createElement('div'); g.className = 'gist'; g.textContent = s.gist;
  sum.append(name, g);
  const body = document.createElement('div'); body.className = 'text';
  d.append(sum, body);
  $('p-projects').append(d);
  try {
    renderText(await contentText('projects/' + s.file), body);
  } catch (e) {
    body.textContent = 'could not load this write-up';
  }
})).catch(() => { $('p-projects').textContent = 'could not load the projects'; });

// the CV (without its first lines, the name and title the page shows above)
// and how to reach me, from the same files as the terminal's cv and contact
(async () => {
  const box = $('p-cv-text');
  try {
    const cv = await contentText('cv.txt');
    renderText(cv.replace(/\r/g, '').split(/\n\s*\n/).slice(1).join('\n\n'), box);
    renderText(await contentText('contact_info.md'), box);
  } catch (e) { box.textContent = 'could not load the CV'; }
})();
$('p-cv-btn').addEventListener('click', () => {
  $('p-cv').open = true;
  $('p-cv').scrollIntoView({ behavior: 'smooth', block: 'start' });
});
