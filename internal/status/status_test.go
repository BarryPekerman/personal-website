package status

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func get(t *testing.T, h http.Handler, headers map[string]string) response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var r response
	if err := json.NewDecoder(rec.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStatusReportsInfoVisitorAndSeats(t *testing.T) {
	var hits atomic.Int32
	game := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"players": 3, "max": 40}`))
	}))
	defer game.Close()

	metrics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`# TYPE cloudflared_tunnel_server_locations gauge
cloudflared_tunnel_server_locations{connection_id="0",edge_location="tlv02"} 1
cloudflared_tunnel_server_locations{connection_id="1",edge_location="mrs06"} 1
cloudflared_tunnel_server_locations{connection_id="2",edge_location="mrs06"} 1
cloudflared_tunnel_server_locations{connection_id="3",edge_location="fra10"} 0
cloudflared_tunnel_total_requests 42
`))
	}))
	defer metrics.Close()

	info := Info{Pod: "portfolio-abc", Node: "10.0.1.212", Region: "il-jerusalem-1", Commit: "107745f"}
	h := NewHandler(info, func() int64 { return 2 }, Sources{HandoverHealthz: game.URL, TunnelMetrics: metrics.URL, TTL: time.Minute})

	r := get(t, h, map[string]string{"CF-IPCountry": "IL", "CF-Ray": "8d2f1c3b5e6a7f80-TLV"})
	if r.Pod != "portfolio-abc" || r.Region != "il-jerusalem-1" || r.Sessions != 2 {
		t.Errorf("info: %+v", r)
	}
	if r.Visitor != (Visitor{Country: "IL", Colo: "TLV"}) {
		t.Errorf("visitor: %+v", r.Visitor)
	}
	if r.Handover == nil || *r.Handover != (Seats{Players: 3, Max: 40}) {
		t.Errorf("handover: %+v", r.Handover)
	}
	// TLV holds a tunnel connection: straight in
	if got := kinds(r.Route); got != "you:IL edge:TLV+tunnel cluster:il-jerusalem-1" {
		t.Errorf("route: %s", got)
	}

	get(t, h, nil)
	if n := hits.Load(); n != 1 {
		t.Errorf("game fetched %d times within the ttl, want 1", n)
	}
}

func TestStatusWithoutCloudflareOrGame(t *testing.T) {
	game := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer game.Close()

	before := testutil.ToFloat64(upstreamErrors.WithLabelValues("handover"))
	r := get(t, NewHandler(Info{}, func() int64 { return 0 }, Sources{HandoverHealthz: game.URL, TTL: time.Minute}), nil)
	if n := testutil.ToFloat64(upstreamErrors.WithLabelValues("handover")) - before; n != 1 {
		t.Errorf("failed game read counted %v times, want 1", n)
	}
	if r.Visitor != (Visitor{}) {
		t.Errorf("visitor without Cloudflare headers: %+v", r.Visitor)
	}
	if r.Handover != nil {
		t.Errorf("handover when the game is down: %+v", r.Handover)
	}
	// no Cloudflare headers, no tunnel data, no region: nothing is placed
	for _, h := range r.Route {
		if h.Pos != nil {
			t.Errorf("hop placed without data: %+v", h)
		}
	}
}

func kinds(route []Hop) string {
	var parts []string
	for _, h := range route {
		p := h.Kind + ":" + h.Label
		if h.Tunnel {
			p += "+tunnel"
		}
		if h.Pos == nil {
			p += "?"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, " ")
}

func TestParseTunnel(t *testing.T) {
	links, ok := parseTunnel(strings.NewReader(`# TYPE cloudflared_tunnel_server_locations gauge
cloudflared_tunnel_server_locations{connection_id="0",edge_location="tlv02"} 1
cloudflared_tunnel_server_locations{connection_id="1",edge_location="mrs06"} 1
cloudflared_tunnel_server_locations{connection_id="2",edge_location="mrs06"} 1
cloudflared_tunnel_server_locations{connection_id="3",edge_location="fra10"} 0
cloudflared_tunnel_total_requests 42
`))
	// a connection that's down (0) isn't counted
	want := []TunnelLink{{Colo: "MRS", Connections: 2}, {Colo: "TLV", Connections: 1}}
	if !ok || len(links) != 2 || links[0] != want[0] || links[1] != want[1] {
		t.Errorf("parseTunnel: %+v, want %+v", links, want)
	}
}

// With two cloudflared replicas behind a headless Service, each holds its own
// connections: the status adds up every replica's, and skips one that's down
func TestTunnelEveryReplica(t *testing.T) {
	serve := func(addr, loc string) (port string) {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Skipf("listen %s: %v", addr, err)
		}
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, "cloudflared_tunnel_server_locations{connection_id=\"0\",edge_location=%q} 1\n", loc)
			_, _ = fmt.Fprintf(w, "cloudflared_tunnel_server_locations{connection_id=\"1\",edge_location=%q} 1", loc)
		}), ReadHeaderTimeout: time.Second}
		go func() { _ = srv.Serve(l) }()
		t.Cleanup(func() { _ = srv.Close() })
		_, port, _ = net.SplitHostPort(l.Addr().String())
		return port
	}
	port := serve("127.0.0.1:0", "tlv02")
	serve("127.0.0.2:"+port, "mrs06")
	orig := lookupHost
	t.Cleanup(func() { lookupHost = orig })
	lookupHost = func(_ context.Context, host string) ([]string, error) {
		if host != "portfolio-tunnel-metrics" {
			return nil, fmt.Errorf("unexpected host %q", host)
		}
		return []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, nil // .3: nothing listens
	}

	h := NewHandler(Info{}, func() int64 { return 0 }, Sources{TunnelMetrics: "http://portfolio-tunnel-metrics:" + port + "/metrics", TTL: time.Minute})
	links := h.tunnel.get(context.Background())
	want := []TunnelLink{{Colo: "MRS", Connections: 2}, {Colo: "TLV", Connections: 2}}
	if len(links) != 2 || links[0] != want[0] || links[1] != want[1] {
		t.Errorf("links: %+v, want %+v", links, want)
	}
}

func TestRoute(t *testing.T) {
	links := []TunnelLink{{Colo: "TLV", Connections: 2}, {Colo: "MRS", Connections: 2}}
	cases := []struct {
		visitor Visitor
		links   []TunnelLink
		want    string
	}{
		// the edge holds a connection: straight in
		{Visitor{"FR", "MRS"}, links, "you:FR edge:MRS+tunnel cluster:il-jerusalem-1"},
		// no connection at the edge: carried on to the nearest one
		{Visitor{"DE", "FRA"}, links, "you:DE edge:FRA forward:MRS cluster:il-jerusalem-1"},
		{Visitor{"US", "EWR"}, links, "you:US edge:EWR forward:MRS cluster:il-jerusalem-1"},
		{Visitor{"IN", "BOM"}, links, "you:IN edge:BOM forward:TLV cluster:il-jerusalem-1"},
		// countries and edges that weren't in the old hand-made tables
		{Visitor{"VN", "SGN"}, links, "you:VN edge:SGN forward:TLV cluster:il-jerusalem-1"},
		// unknown codes stay on the route, unplaced; tunnel unknown: no guess
		{Visitor{"XX", "ZZZ"}, nil, "you:XX? edge:ZZZ? cluster:il-jerusalem-1"},
	}
	for _, c := range cases {
		if got := kinds(route(c.visitor, c.links, "il-jerusalem-1")); got != c.want {
			t.Errorf("%v: %s, want %s", c.visitor, got, c.want)
		}
	}
}

func TestStatusRouteForGivenEdge(t *testing.T) {
	h := NewHandler(Info{Region: "il-jerusalem-1"}, func() int64 { return 0 }, Sources{TTL: time.Minute})
	cf := map[string]string{"CF-IPCountry": "IL", "CF-Ray": "8d2f1c3b5e6a7f80-MRS"}
	for query, want := range map[string]string{
		"":                     "you:IL edge:MRS cluster:il-jerusalem-1",
		"?country=IL&colo=TLV": "you:IL edge:TLV cluster:il-jerusalem-1",
		"?country=IL&colo=tlv": "you:IL edge:MRS cluster:il-jerusalem-1", // not a code: ignored
		"?country=IL&colo=<b>": "you:IL edge:MRS cluster:il-jerusalem-1",
		"?colo=TLV":            "you:IL edge:MRS cluster:il-jerusalem-1",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/status"+query, nil)
		for k, v := range cf {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var r response
		if err := json.NewDecoder(rec.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
		if got := strings.ReplaceAll(kinds(r.Route), "?", ""); got != want {
			t.Errorf("%q: %s, want %s", query, got, want)
		}
	}
}

func TestCacheServesStaleWhileRefreshing(t *testing.T) {
	var hits atomic.Int32
	release := make(chan struct{})
	game := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := hits.Add(1)
		if n > 1 {
			<-release // every fetch after the first is slow
		}
		_, _ = fmt.Fprintf(w, `{"players": %d, "max": 60}`, n)
	}))
	defer game.Close()
	defer close(release)

	c := &cache[*Seats]{name: "handover", url: game.URL, ttl: 50 * time.Millisecond, client: game.Client(), parse: parseSeats}
	// The first caller waits for the first answer
	if s := c.get(context.Background()); s == nil || s.Players != 1 {
		t.Fatalf("first answer: %+v", s)
	}
	time.Sleep(60 * time.Millisecond) // expired

	// The slow refresh doesn't hold anyone up: all get the old answer at once
	start := time.Now()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if s := c.get(context.Background()); s == nil || s.Players != 1 {
				t.Errorf("stale answer: %+v", s)
			}
		})
	}
	wg.Wait()
	if d := time.Since(start); d > time.Second {
		t.Errorf("callers waited %v for a slow source", d)
	}
	time.Sleep(20 * time.Millisecond)
	if n := hits.Load(); n != 2 {
		t.Errorf("%d fetches, want 2 (one refresh for all callers)", n)
	}
}
