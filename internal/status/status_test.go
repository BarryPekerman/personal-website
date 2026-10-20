package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	info := Info{Pod: "portfolio-abc", Node: "10.0.1.212", Region: "il-jerusalem-1", Commit: "107745f"}
	h := NewHandler(info, func() int64 { return 2 }, game.URL, time.Minute)

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

	r := get(t, NewHandler(Info{}, func() int64 { return 0 }, game.URL, time.Minute), nil)
	if r.Visitor != (Visitor{}) {
		t.Errorf("visitor without Cloudflare headers: %+v", r.Visitor)
	}
	if r.Handover != nil {
		t.Errorf("handover when the game is down: %+v", r.Handover)
	}
}
