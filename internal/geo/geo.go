// Package geo places the hops of a visitor's route on the map: Cloudflare
// edge locations by their airport code, visitors' countries by ISO code, and
// the cluster by its OCI region. places.json is generated (see gen/).
package geo

import (
	_ "embed"
	"encoding/json"
	"math"
)

//go:generate go run ./gen

//go:embed places.json
var placesJSON []byte

// Pos is [latitude, longitude]
type Pos [2]float64

var places struct {
	Airports  map[string]Pos `json:"airports"`
	Countries map[string]Pos `json:"countries"`
}

// where each OCI region's data centre is
var regions = map[string]Pos{
	"il-jerusalem-1": {31.77, 35.21},
}

func init() {
	if err := json.Unmarshal(placesJSON, &places); err != nil {
		panic("geo: places.json: " + err.Error())
	}
}

// Colo is where a Cloudflare edge location is, by its IATA code ("TLV")
func Colo(code string) (Pos, bool) {
	p, ok := places.Airports[code]
	return p, ok
}

// Country is a centre point for an ISO 3166 alpha-2 code ("IL")
func Country(code string) (Pos, bool) {
	p, ok := places.Countries[code]
	return p, ok
}

// Region is where an OCI region is ("il-jerusalem-1")
func Region(name string) (Pos, bool) {
	p, ok := regions[name]
	return p, ok
}

// Distance is the great-circle distance between two points, in km
func Distance(a, b Pos) float64 {
	const r = 6371
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLon := rad(b[0]-a[0]), rad(b[1]-a[1])
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(rad(a[0]))*math.Cos(rad(b[0]))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
