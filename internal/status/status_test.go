package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

	r := get(t, NewHandler(Info{}, func() int64 { return 0 }, Sources{HandoverHealthz: game.URL, TTL: time.Minute}), nil)
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
