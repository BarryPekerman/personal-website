package geo

import (
	"math"
	"testing"
)

func TestLookups(t *testing.T) {
	near := func(p Pos, lat, lon float64) bool {
		return math.Abs(p[0]-lat) < 1 && math.Abs(p[1]-lon) < 1
	}
	if p, ok := Colo("TLV"); !ok || !near(p, 32.0, 34.9) {
		t.Errorf("Colo(TLV) = %v, %v", p, ok)
	}
	if p, ok := Colo("MRS"); !ok || !near(p, 43.4, 5.2) {
		t.Errorf("Colo(MRS) = %v, %v", p, ok)
	}
	if p, ok := Country("IL"); !ok || !near(p, 31.5, 34.75) {
		t.Errorf("Country(IL) = %v, %v", p, ok)
	}
	if p, ok := Region("il-jerusalem-1"); !ok || !near(p, 31.77, 35.21) {
		t.Errorf("Region(il-jerusalem-1) = %v, %v", p, ok)
	}
	for _, f := range []func(string) (Pos, bool){Colo, Country, Region} {
		if _, ok := f("nowhere"); ok {
			t.Error("an unknown code should not be found")
		}
	}
}

func TestDistance(t *testing.T) {
	tlv, _ := Colo("TLV")
	mrs, _ := Colo("MRS")
	// Tel Aviv to Marseille is about 2,880 km
	if d := Distance(tlv, mrs); d < 2800 || d > 2950 {
		t.Errorf("TLV-MRS = %.0f km", d)
	}
	if d := Distance(tlv, tlv); d != 0 {
		t.Errorf("distance to itself = %v", d)
	}
	if a, b := Distance(tlv, mrs), Distance(mrs, tlv); math.Abs(a-b) > 1e-9 {
		t.Errorf("not symmetric: %v vs %v", a, b)
	}
}
