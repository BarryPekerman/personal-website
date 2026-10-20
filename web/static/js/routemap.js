// Route map for the front page: the visitor's real path to this server
// (their country -> the Cloudflare edge that took the request -> the cluster),
// drawn on a dot-matrix world from Natural Earth's land outline. The view
// zooms to fit the route. A hop whose location isn't known is left off the
// map rather than guessed. Tunnel links are the cluster's live connections to
// Cloudflare (from cloudflared's metrics), drawn faintly from each edge
// location to the cluster.

// Rough centre of each country (lat, lon), by the ISO code in CF-IPCountry
const COUNTRY_POS = {
  IL: [31.4, 35.0], PS: [31.9, 35.2], JO: [31.2, 36.5], LB: [33.9, 35.9], EG: [26.8, 30.8],
  CY: [35.1, 33.4], TR: [39.0, 35.2], GR: [39.1, 22.0], AE: [24.0, 54.0], SA: [24.0, 45.0],
  GB: [54.0, -2.0], IE: [53.4, -8.2], FR: [46.6, 2.4], DE: [51.2, 10.4], NL: [52.1, 5.3],
  BE: [50.6, 4.6], CH: [46.8, 8.2], AT: [47.6, 14.1], IT: [42.8, 12.6], ES: [40.2, -3.7],
  PT: [39.6, -8.0], PL: [52.1, 19.4], CZ: [49.8, 15.5], RO: [45.9, 24.9], BG: [42.7, 25.5],
  HU: [47.2, 19.4], SE: [62.0, 15.0], NO: [61.0, 9.0], FI: [64.0, 26.0], DK: [56.0, 10.0],
  UA: [49.0, 31.4], RU: [56.0, 38.0], IN: [22.0, 79.0], SG: [1.35, 103.8], JP: [36.2, 138.3],
  KR: [36.5, 127.9], CN: [35.0, 104.0], HK: [22.3, 114.2], TW: [23.7, 121.0], AU: [-25.3, 133.8],
  NZ: [-41.0, 174.0], ZA: [-29.0, 24.0], NG: [9.1, 8.7], KE: [0.2, 37.9], BR: [-10.8, -52.9],
  AR: [-34.0, -64.0], MX: [23.6, -102.5], CA: [56.0, -96.0], US: [39.8, -98.6],
};

// Cloudflare edge locations by IATA code (the suffix of CF-Ray)
const COLO_POS = {
  TLV: [32.01, 34.89], AMM: [31.72, 35.99], BEY: [33.82, 35.49], LCA: [34.88, 33.63],
  IST: [41.26, 28.74], ATH: [37.94, 23.94], SOF: [42.70, 23.41], OTP: [44.57, 26.10],
  BUD: [47.44, 19.26], VIE: [48.11, 16.57], PRG: [50.10, 14.26], WAW: [52.17, 20.97],
  FRA: [50.03, 8.57], MUC: [48.35, 11.79], HAM: [53.63, 9.99], DUS: [51.29, 6.77],
  BER: [52.37, 13.50], TXL: [52.56, 13.29], AMS: [52.31, 4.76], BRU: [50.90, 4.48],
  CDG: [49.01, 2.55], MRS: [43.44, 5.22], ZRH: [47.46, 8.55], GVA: [46.24, 6.11],
  MXP: [45.63, 8.72], FCO: [41.80, 12.25], MAD: [40.47, -3.56], BCN: [41.30, 2.08],
  LIS: [38.77, -9.13], LHR: [51.47, -0.45], MAN: [53.35, -2.27], DUB: [53.43, -6.27],
  CPH: [55.62, 12.65], ARN: [59.65, 17.92], OSL: [60.19, 11.10], HEL: [60.32, 24.96],
  KBP: [50.35, 30.89], DME: [55.41, 37.90], DXB: [25.25, 55.36], FJR: [25.11, 56.32],
  RUH: [24.96, 46.70], JED: [21.68, 39.16], CAI: [30.12, 31.41], BOM: [19.09, 72.87],
  DEL: [28.56, 77.10], MAA: [12.99, 80.17], SIN: [1.36, 103.99], HKG: [22.31, 113.91],
  NRT: [35.77, 140.39], KIX: [34.43, 135.24], ICN: [37.46, 126.44], TPE: [25.08, 121.23],
  SYD: [-33.95, 151.18], MEL: [-37.67, 144.84], AKL: [-37.01, 174.79], JNB: [-26.14, 28.24],
  CPT: [-33.97, 18.60], LOS: [6.58, 3.32], NBO: [-1.32, 36.93], GRU: [-23.43, -46.47],
  EZE: [-34.82, -58.54], MEX: [19.44, -99.07], YYZ: [43.68, -79.63], YUL: [45.47, -73.74],
  YVR: [49.19, -123.18], IAD: [38.95, -77.46], EWR: [40.69, -74.17], BOS: [42.37, -71.01],
  ATL: [33.64, -84.43], MIA: [25.79, -80.29], ORD: [41.98, -87.90], DFW: [32.90, -97.04],
  DEN: [39.86, -104.67], PHX: [33.43, -112.01], LAX: [33.94, -118.41], SJC: [37.36, -121.93],
  SEA: [47.45, -122.31],
};

// Where each OCI region's data centre is
const REGION_POS = { 'il-jerusalem-1': [31.77, 35.21] };

class RouteMap {
  constructor(svg) {
    this.svg = svg;
    this.W = 600; this.H = 260;
    svg.setAttribute('viewBox', `0 0 ${this.W} ${this.H}`);
    this.landLayer = this.el('g', {});
    this.routeLayer = this.el('g', {});
    this.nodeLayer = this.el('g', {});
    this.rings = null;  // land polygons as [lon, lat] rings, with bounding boxes
    this.hops = [];
    this.links = [];
    this.key = '';
    this.path = null;     // the drawn route, for packets to travel along
    this.inFlight = 0;
    this.loadLand();
  }

  el(tag, attrs, parent = this.svg) {
    const e = document.createElementNS('http://www.w3.org/2000/svg', tag);
    for (const k in attrs) e.setAttribute(k, attrs[k]);
    parent.appendChild(e);
    return e;
  }

  async loadLand() {
    try {
      const r = await fetch('https://cdn.jsdelivr.net/npm/world-atlas@2.0.2/land-110m.json');
      const topo = await r.json();
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

  // hops: [{ pos: [lat, lon] | null, label, you? }]; links: [{ pos, label }],
  // each joined to the last hop (the cluster). Redraws only when they change.
  draw(hops, links = []) {
    const key = JSON.stringify([hops, links]);
    if (key === this.key) return;
    this.key = key;
    this.hops = hops;
    this.links = links.filter(l => l.pos);
    this.render();
  }

  // the window (in degrees) that fits the known hops, at the map's aspect ratio
  view() {
    const pts = [...this.hops, ...this.links].filter(h => h.pos).map(h => h.pos);
    if (!pts.length) return { lon0: -180, lat1: 90, span: 360 };
    const lats = pts.map(p => p[0]), lons = pts.map(p => p[1]);
    const cLat = (Math.min(...lats) + Math.max(...lats)) / 2;
    const cLon = (Math.min(...lons) + Math.max(...lons)) / 2;
    const aspect = this.W / this.H;
    let span = Math.max(16, (Math.max(...lons) - Math.min(...lons)) * 1.4,
      (Math.max(...lats) - Math.min(...lats)) * 1.4 * aspect);
    span = Math.min(span, 360);
    return { lon0: cLon - span / 2, lat1: cLat + span / aspect / 2, span };
  }

  render() {
    const v = this.view();
    const k = this.W / v.span;  // pixels per degree
    const px = ([lat, lon]) => [(lon - v.lon0) * k, (v.lat1 - lat) * k];

    this.landLayer.replaceChildren();
    if (this.rings) {
      const step = v.span / 66;  // about 66 dots across, whatever the zoom
      for (let x = step / 2; x < this.W; x += step * k) {
        for (let y = step / 2; y < this.H; y += step * k) {
          const lon = v.lon0 + x / k, lat = v.lat1 - y / k;
          if (this.onLand(lon, lat)) this.el('circle', { cx: x, cy: y, r: 1.3, fill: '#16304f' }, this.landLayer);
        }
      }
    }

    this.routeLayer.replaceChildren();
    this.nodeLayer.replaceChildren();
    const shown = this.hops.filter(h => h.pos);
    this.path = null;
    // tunnel links under the route; a link at a hop's location gets no label
    // of its own (the hop's label already names the place)
    const end = shown.length && shown[shown.length - 1];
    for (const l of end ? this.links : []) {
      const [x, y] = px(l.pos), [ex, ey] = px(end.pos);
      this.el('path', { d: `M${x} ${y} L${ex} ${ey}`, class: 'tunnel-link' }, this.routeLayer);
      if (shown.some(h => h.pos[0] === l.pos[0] && h.pos[1] === l.pos[1])) continue;
      const g = this.el('g', { class: 'node tunnel' }, this.nodeLayer);
      this.el('circle', { cx: x, cy: y, r: 3 }, g);
      const t = this.el('text', { x: x + 7, y: y + 12 }, g);
      t.textContent = l.label;
    }
    if (shown.length > 1) {
      const d = shown.map((h, i) => (i ? 'L' : 'M') + px(h.pos).join(' ')).join(' ');
      this.path = this.el('path', { d, class: 'route-line' }, this.routeLayer);
    }
    shown.forEach((h, i) => {
      const [x, y] = px(h.pos);
      const g = this.el('g', { class: 'node' + (h.you ? ' you' : '') }, this.nodeLayer);
      if (i === shown.length - 1) this.el('circle', { cx: x, cy: y, r: 5, class: 'ping' }, g);
      this.el('circle', { cx: x, cy: y, r: 5 }, g);
      // alternate labels left and right so close hops stay readable
      const right = i % 2 === 0;
      const t = this.el('text', { x: right ? x + 9 : x - 9, y: y + 4 - 10 + i * 10, 'text-anchor': right ? 'start' : 'end' }, g);
      t.textContent = h.label;
    });
  }

  // packet animates one real message along the route: 'out' is the
  // visitor's keystroke going to the server, 'in' is the server's reply
  packet(direction) {
    if (!this.path || this.inFlight >= 8) return;
    const path = this.path, len = path.getTotalLength();
    const dot = this.el('circle', { r: 2.6, class: 'packet ' + direction }, this.nodeLayer);
    const dur = 700;
    let t0 = null;
    this.inFlight++;
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
