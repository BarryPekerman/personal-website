// Route map for the front page: the visitor's real path to this server
// (their country -> the Cloudflare edge that took the request -> the cluster),
// drawn on a dot-matrix world from Natural Earth's land outline. The view
// zooms to fit the route. Positions come from the server (/api/status); a hop
// whose location isn't known is left off the map rather than guessed.

class RouteMap {
  constructor(svg) {
    this.svg = svg;
    this.W = 600; this.H = 260;  // replaced by the element's size in resize()
    this.landLayer = this.el('g', {});
    this.routeLayer = this.el('g', {});
    this.nodeLayer = this.el('g', {});
    this.rings = null;  // land polygons as [lon, lat] rings, with bounding boxes
    this.hops = [];
    this.key = '';
    this.path = null;     // the drawn route, for packets to travel along
    this.inFlight = 0;
    // one map unit per screen pixel, so the map fills its panel at any shape
    this.resize();
    new ResizeObserver(() => this.resize()).observe(svg);
    this.loadLand();
  }

  resize() {
    const r = this.svg.getBoundingClientRect();
    if (!r.width) return;  // hidden (the other layout's map): drawn once it shows
    const W = Math.max(200, Math.round(r.width)), H = Math.max(120, Math.round(r.height));
    if (W === this.W && H === this.H && this.svg.hasAttribute('viewBox')) return;
    this.W = W; this.H = H;
    this.svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
    this.render();
  }

  el(tag, attrs, parent = this.svg) {
    const e = document.createElementNS('http://www.w3.org/2000/svg', tag);
    for (const k in attrs) e.setAttribute(k, attrs[k]);
    parent.appendChild(e);
    return e;
  }

  async loadLand() {
    try {
      // one download for every map on the page
      RouteMap.land = RouteMap.land || fetch('/static/vendor/land-110m.json').then(r => r.json());
      const topo = await RouteMap.land;
      const land = topojson.feature(topo, topo.objects.land);
      const polys = land.features.flatMap(f =>
        f.geometry.type === 'Polygon' ? [f.geometry.coordinates] : f.geometry.coordinates);
      this.rings = polys.map(p => p[0]).map(ring => {
        let x0 = 180, x1 = -180, y0 = 90, y1 = -90;
        for (const [x, y] of ring) { x0 = Math.min(x0, x); x1 = Math.max(x1, x); y0 = Math.min(y0, y); y1 = Math.max(y1, y); }
        return { ring, x0, x1, y0, y1 };
      });
      this.render();
    } catch (e) { /* no land outline: the route still draws */ }
  }

  // hops: [{ pos: [lat, lon] | null, label, you? }]; redraws only when they change
  draw(hops) {
    const key = JSON.stringify(hops);
    if (key === this.key) return;
    this.key = key;
    this.hops = hops;
    this.render();
  }

  // the window (in degrees) that fits the route, never closer than a region (about Europe and the Middle East), so it
  // always reads as a map; a local route is a short line inside it
  view() {
    const pts = this.hops.filter(h => h.pos).map(h => h.pos);
    if (!pts.length) return { lon0: -180, lat1: 90, span: 360 };
    const lats = pts.map(p => p[0]), lons = pts.map(p => p[1]);
    const cLat = (Math.min(...lats) + Math.max(...lats)) / 2;
    const cLon = (Math.min(...lons) + Math.max(...lons)) / 2;
    const aspect = this.W / this.H;
    // 10 degrees of latitude spare, so hops near the top or bottom keep room for labels
    let span = Math.max(40, (Math.max(...lons) - Math.min(...lons)) * 1.3,
      (Math.max(...lats) - Math.min(...lats) + 10) * aspect);
    span = Math.min(span, 360);
    return { lon0: cLon - span / 2, lat1: cLat + span / aspect / 2, span };
  }

  render() {
    // a hidden map isn't drawn; the ResizeObserver redraws it when it shows
    if (!this.svg.getBoundingClientRect().width) return;
    const v = this.view();
    const k = this.W / v.span;  // pixels per degree
    const px = ([lat, lon]) => [(lon - v.lon0) * k, (v.lat1 - lat) * k];

    this.landLayer.replaceChildren();
    if (this.rings) {
      const step = v.span / (this.W / 9);  // a dot every 9 px, whatever the zoom
      for (let x = step / 2; x < this.W; x += step * k) {
        for (let y = step / 2; y < this.H; y += step * k) {
          const lon = v.lon0 + x / k, lat = v.lat1 - y / k;
          if (this.onLand(lon, lat)) this.el('circle', { cx: x, cy: y, r: 1.5, fill: '#24507e' }, this.landLayer);
        }
      }
    }

    this.routeLayer.replaceChildren();
    this.nodeLayer.replaceChildren();
    this.labels = [];
    const shown = this.hops.filter(h => h.pos);
    this.path = null;
    if (shown.length > 1) {
      const d = shown.map((h, i) => (i ? 'L' : 'M') + px(h.pos).join(' ')).join(' ');
      this.path = this.el('path', { d, class: 'route-line' }, this.routeLayer);
    }
    shown.forEach((h, i) => {
      const [x, y] = px(h.pos);
      const g = this.el('g', { class: 'node' + (h.you ? ' you' : '') }, this.nodeLayer);
      if (i === shown.length - 1) this.el('circle', { cx: x, cy: y, r: 5, class: 'ping' }, g);
      this.el('circle', { cx: x, cy: y, r: 5 }, g);
      this.label(g, x + 9, y + 4, 'start', h.label);
    });
  }

  // label places a text, moved down or up (nearest first, staying on the
  // map) until it clears the ones already placed
  label(parent, x, y, anchor, text) {
    const t = this.el('text', { x, y, 'text-anchor': anchor }, parent);
    t.textContent = text;
    // past the right edge: put it on the node's left instead
    if (anchor === 'start' && t.getBBox().x + t.getBBox().width > this.W - 4) {
      t.setAttribute('text-anchor', 'end');
      t.setAttribute('x', x - 18);
    }
    const hits = b => this.labels.some(o => b.x < o.x + o.width && o.x < b.x + b.width && b.y < o.y + o.height && o.y < b.y + b.height);
    for (const dy of [0, 12, -12, 24, -24, 36, -36]) {
      const ty = Math.min(this.H - 4, Math.max(11, y + dy));
      t.setAttribute('y', ty);
      if (!hits(t.getBBox())) break;
    }
    this.labels.push(t.getBBox());
  }

  // packet animates one real message along the route: 'out' is the
  // visitor's keystroke going to the server, 'in' is the server's reply. The
  // real round trip takes milliseconds; drawn slowed down, a reply still
  // can't set off before the keystroke behind it has arrived, so it waits for
  // the last outgoing packet to reach the server.
  packet(direction) {
    if (!this.path || this.inFlight >= 8) return;
    const dur = 700, now = performance.now();
    let delay = 0;
    if (direction === 'out') this.outArrives = now + dur;
    else delay = Math.max(0, (this.outArrives || 0) - now);
    this.inFlight++;
    setTimeout(() => this.fly(direction, dur), delay);
  }

  fly(direction, dur) {
    const path = this.path;
    if (!path) { this.inFlight--; return; }
    const len = path.getTotalLength();
    const dot = this.el('circle', { r: 2.6, class: 'packet ' + direction }, this.nodeLayer);
    let t0 = null;
    const step = ts => {
      if (t0 === null) t0 = ts;
      const f = Math.min(1, (ts - t0) / dur);
      const at = path.getPointAtLength((direction === 'out' ? f : 1 - f) * len);
      dot.setAttribute('cx', at.x); dot.setAttribute('cy', at.y);
      if (f < 1 && dot.isConnected) requestAnimationFrame(step);
      else { dot.remove(); this.inFlight--; }
    };
    requestAnimationFrame(step);
  }

  onLand(lon, lat) {
    for (const r of this.rings) {
      if (lon < r.x0 || lon > r.x1 || lat < r.y0 || lat > r.y1) continue;
      let inside = false;
      const ring = r.ring;
      for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
        const [xi, yi] = ring[i], [xj, yj] = ring[j];
        if ((yi > lat) !== (yj > lat) && lon < (xj - xi) * (lat - yi) / (yj - yi) + xi) inside = !inside;
      }
      if (inside) return true;
    }
    return false;
  }
}
