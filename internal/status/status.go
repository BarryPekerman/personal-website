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
	"sort"
	"strings"
	"sync"
	"time"
)

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
	Colo        string `json:"colo"`
	Connections int    `json:"connections"`
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
	// the tunnel's live connections; null when cloudflared can't be reached
	Tunnel []TunnelLink `json:"tunnel"`
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
		seats:    &cache[*Seats]{url: src.HandoverHealthz, ttl: src.TTL, client: client, parse: parseSeats},
		tunnel:   &cache[[]TunnelLink]{url: src.TunnelMetrics, ttl: src.TTL, client: client, parse: parseTunnel},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := response{
		Pod:      h.info.Pod,
		Node:     h.info.Node,
		Region:   h.info.Region,
		Commit:   h.info.Commit,
		Started:  h.info.Started,
		Sessions: h.sessions(),
		Visitor:  h.visitor(r),
		Handover: h.seats.get(r.Context()),
		Tunnel:   h.tunnel.get(r.Context()),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) visitor(r *http.Request) Visitor {
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
	c.val = c.fetch(ctx)
	return c.val
}

func (c *cache[T]) fetch(ctx context.Context) T {
	var zero T
	// not the visitor's context: a closed tab shouldn't cache a failure
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return zero
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return zero
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body
	if resp.StatusCode != http.StatusOK {
		return zero
	}
	v, ok := c.parse(io.LimitReader(resp.Body, 1<<20))
	if !ok {
		return zero
	}
	return v
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
