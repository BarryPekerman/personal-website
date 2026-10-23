// Package status serves /api/status: facts about this deployment and the
// visitor's request that the front page shows. Everything in it is read from
// the running system; nothing is made up.
package status

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"portfolio/internal/geo"
)

// upstreamErrors counts failed reads of the status sources, served on /metrics
var upstreamErrors = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "portfolio_status_upstream_errors_total",
	Help: "Failed reads of a status source: handover (the game's /healthz) or tunnel (cloudflared's /metrics).",
}, []string{"source"})

func init() {
	upstreamErrors.WithLabelValues("handover")
	upstreamErrors.WithLabelValues("tunnel")
}

// Info is fixed for the life of the process
type Info struct {
	Pod     string // from the Downward API (POD_NAME)
	Node    string // from the Downward API (NODE_NAME)
	Region  string // where the cluster runs (REGION)
	Commit  string // git commit baked in at build time
	Started time.Time
	// DevVisitor stands in for Cloudflare's headers when a request has none
	// (local development only; behind Cloudflare every request has them)
	DevVisitor Visitor
}

// Seats is The Handover's /healthz reply
type Seats struct {
	Players int `json:"players"`
	Max     int `json:"max"`
}

// Visitor is what Cloudflare tells us about the request
type Visitor struct {
	Country string `json:"country,omitempty"` // CF-IPCountry
	Colo    string `json:"colo,omitempty"`    // edge location, the suffix of CF-Ray
}

// TunnelLink is one Cloudflare edge location the cluster's tunnel is
// connected to, and how many of its connections land there
type TunnelLink struct {
	Colo        string
	Connections int
}

// Hop is one step of the visitor's route, for the map and the route panel
type Hop struct {
	Kind  string   `json:"kind"`  // you, edge, forward (Cloudflare carries it on to a tunnel connection), cluster
	Label string   `json:"label"` // country, edge or region code
	Pos   *geo.Pos `json:"pos"`   // null when the place isn't known: the map leaves it out
	// Tunnel (edge only): the cluster's tunnel connects at this edge, so the
	// request goes straight in
	Tunnel bool `json:"tunnel,omitempty"`
}

type response struct {
	Pod      string    `json:"pod"`
	Node     string    `json:"node"`
	Region   string    `json:"region"`
	Commit   string    `json:"commit"`
	Started  time.Time `json:"started"`
	Sessions int64     `json:"sessions"`
	Visitor  Visitor   `json:"visitor"`
	Handover *Seats    `json:"handover"` // null when the game can't be reached
	Route    []Hop     `json:"route"`
}

// Handler serves the status JSON
type Handler struct {
	info     Info
	sessions func() int64
	seats    *cache[*Seats]
	tunnel   *cache[[]TunnelLink]
}

// Sources are the URLs the handler reads, each at most once per TTL whatever
// the traffic. An empty URL leaves that part of the status null.
type Sources struct {
	HandoverHealthz string // The Handover's /healthz, for its seats
	TunnelMetrics   string // cloudflared's Prometheus /metrics, for its edge locations
	TTL             time.Duration
}

// NewHandler reports info, the live session count, The Handover's seats and
// where the cluster's tunnel is connected to Cloudflare
func NewHandler(info Info, sessions func() int64, src Sources) *Handler {
	client := &http.Client{Timeout: 3 * time.Second}
	return &Handler{
		info:     info,
		sessions: sessions,
		seats:    &cache[*Seats]{name: "handover", url: src.HandoverHealthz, ttl: src.TTL, client: client, parse: parseSeats},
		tunnel:   &cache[[]TunnelLink]{name: "tunnel", url: src.TunnelMetrics, ttl: src.TTL, client: client, parse: parseTunnel},
	}
}

// ServeHTTP reports the status. The route is for the request's own edge, or,
// given ?country=IL&colo=TLV, for that one: the front page passes the edge
// that served the page or the terminal, which can differ from the one that
// takes a later status request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v := h.Visitor(r)
	if q := (Visitor{Country: r.URL.Query().Get("country"), Colo: r.URL.Query().Get("colo")}); validVisitor(q) {
		v = q
	}
	resp := response{
		Pod:      h.info.Pod,
		Node:     h.info.Node,
		Region:   h.info.Region,
		Commit:   h.info.Commit,
		Started:  h.info.Started,
		Sessions: h.sessions(),
		Visitor:  v,
		Handover: h.seats.get(r.Context()),
		Route:    route(v, h.tunnel.get(r.Context()), h.info.Region),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

// route is the visitor's path: their country, the edge that took the request,
// then into the tunnel. A request enters at its own edge when the tunnel has a
// connection there; otherwise Cloudflare carries it on to one that has, which
// the origin isn't told, so the route assumes the nearest. links is nil when
// cloudflared can't be read: then nothing is assumed.
func route(v Visitor, links []TunnelLink, region string) []Hop {
	at := func(p geo.Pos, ok bool) *geo.Pos {
		if !ok {
			return nil
		}
		return &p
	}
	edgePos, edgeKnown := geo.Colo(v.Colo)
	hops := []Hop{
		{Kind: "you", Label: v.Country, Pos: at(geo.Country(v.Country))},
		{Kind: "edge", Label: v.Colo, Pos: at(edgePos, edgeKnown)},
	}
	var nearest *TunnelLink
	best := 0.0
	for i, l := range links {
		if l.Colo == v.Colo {
			hops[1].Tunnel = true
			nearest = nil
			break
		}
		if p, ok := geo.Colo(l.Colo); ok && edgeKnown {
			if d := geo.Distance(edgePos, p); nearest == nil || d < best {
				nearest, best = &links[i], d
			}
		}
	}
	if nearest != nil {
		hops = append(hops, Hop{Kind: "forward", Label: nearest.Colo, Pos: at(geo.Colo(nearest.Colo))})
	}
	return append(hops, Hop{Kind: "cluster", Label: region, Pos: at(geo.Region(region))})
}

var (
	countryCode = regexp.MustCompile(`^[A-Z0-9]{2}$`) // ISO alpha-2, or Cloudflare's XX and T1
	coloCode    = regexp.MustCompile(`^[A-Z]{3}$`)    // IATA
)

func validVisitor(v Visitor) bool {
	return countryCode.MatchString(v.Country) && coloCode.MatchString(v.Colo)
}

// Visitor is what Cloudflare says about the request (DevVisitor when it went
// around Cloudflare)
func (h *Handler) Visitor(r *http.Request) Visitor {
	if r.Header.Get("CF-Ray") == "" {
		return h.info.DevVisitor
	}
	return visitorFrom(r)
}

func visitorFrom(r *http.Request) Visitor {
	v := Visitor{Country: r.Header.Get("CF-IPCountry")}
	// CF-Ray looks like "8d2f1c3b5e6a7f80-TLV"
	if ray := r.Header.Get("CF-Ray"); ray != "" {
		if i := strings.LastIndexByte(ray, '-'); i >= 0 && i < len(ray)-1 {
			v.Colo = ray[i+1:]
		}
	}
	return v
}

// cache remembers the last answer (or failure, as T's zero value) for ttl
type cache[T any] struct {
	name   string // the source, for the error metric
	url    string
	ttl    time.Duration
	client *http.Client
	parse  func(io.Reader) (T, bool)

	mu      sync.Mutex
	fetched time.Time
	val     T
}

func (c *cache[T]) get(ctx context.Context) T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.url == "" {
		var zero T
		return zero
	}
	if time.Since(c.fetched) < c.ttl {
		return c.val
	}
	c.fetched = time.Now()
	var ok bool
	if c.val, ok = c.fetch(ctx); !ok {
		upstreamErrors.WithLabelValues(c.name).Inc()
	}
	return c.val
}

// fetch reads and parses the source; false when either fails
func (c *cache[T]) fetch(ctx context.Context) (T, bool) {
	var zero T
	// not the visitor's context: a closed tab shouldn't cache a failure
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return zero, false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return zero, false
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body
	if resp.StatusCode != http.StatusOK {
		return zero, false
	}
	return c.parse(io.LimitReader(resp.Body, 1<<20))
}

func parseSeats(r io.Reader) (*Seats, bool) {
	var s Seats
	if err := json.NewDecoder(r).Decode(&s); err != nil || s.Max <= 0 {
		return nil, false
	}
	return &s, true
}

// parseTunnel reads cloudflared's metric lines like
//
//	cloudflared_tunnel_server_locations{connection_id="1",edge_location="mrs06"} 1
//
// and counts the live connections per location, by its IATA code ("MRS")
func parseTunnel(r io.Reader) ([]TunnelLink, bool) {
	const metric = "cloudflared_tunnel_server_locations{"
	const label = `edge_location="`
	counts := map[string]int{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, metric) || !strings.HasSuffix(line, "} 1") {
			continue
		}
		_, loc, ok := strings.Cut(line, label)
		if !ok {
			continue
		}
		loc, _, _ = strings.Cut(loc, `"`)
		// "mrs06" is the sixth Marseille data centre: keep the airport code
		code := strings.ToUpper(strings.TrimRight(loc, "0123456789"))
		if code != "" {
			counts[code]++
		}
	}
	if sc.Err() != nil || len(counts) == 0 {
		return nil, false
	}
	links := make([]TunnelLink, 0, len(counts))
	for colo, n := range counts {
		links = append(links, TunnelLink{Colo: colo, Connections: n})
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].Connections != links[j].Connections {
			return links[i].Connections > links[j].Connections
		}
		return links[i].Colo < links[j].Colo
	})
	return links, true
}
