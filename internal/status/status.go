// Package status serves /api/status: facts about this deployment and the
// visitor's request that the front page shows. Everything in it is read from
// the running system; nothing is made up.
package status

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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

type response struct {
	Pod      string    `json:"pod"`
	Node     string    `json:"node"`
	Region   string    `json:"region"`
	Commit   string    `json:"commit"`
	Started  time.Time `json:"started"`
	Sessions int64     `json:"sessions"`
	Visitor  Visitor   `json:"visitor"`
	Handover *Seats    `json:"handover"` // null when the game can't be reached
}

// Handler serves the status JSON
type Handler struct {
	info     Info
	sessions func() int64
	seats    *seatCache
}

// NewHandler reports info, the live session count, and The Handover's seats
// from handoverURL (fetched at most once per ttl, whatever the traffic)
func NewHandler(info Info, sessions func() int64, handoverURL string, ttl time.Duration) *Handler {
	return &Handler{
		info:     info,
		sessions: sessions,
		seats:    &seatCache{url: handoverURL, ttl: ttl, client: &http.Client{Timeout: 3 * time.Second}},
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
		Visitor:  visitorFrom(r),
		Handover: h.seats.get(r.Context()),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
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

// seatCache remembers the last answer (or failure) for ttl
type seatCache struct {
	url    string
	ttl    time.Duration
	client *http.Client

	mu      sync.Mutex
	fetched time.Time
	seats   *Seats
}

func (c *seatCache) get(ctx context.Context) *Seats {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.url == "" {
		return nil
	}
	if time.Since(c.fetched) < c.ttl {
		return c.seats
	}
	c.fetched = time.Now()
	c.seats = c.fetch(ctx)
	return c.seats
}

func (c *seatCache) fetch(ctx context.Context) *Seats {
	// not the visitor's context: a closed tab shouldn't cache a failure
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var s Seats
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&s); err != nil || s.Max <= 0 {
		return nil
	}
	return &s
}
