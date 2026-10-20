// Command gen rebuilds internal/geo/places.json, the positions the route map
// needs: every airport with an IATA code (Cloudflare names its edge
// locations after the nearest airport: TLV, MRS, FRA) and a centre point for
// every country (the visitor's CF-IPCountry).
//
// Sources: OurAirports (public domain, updated nightly, no versions) and
// mledoze/countries (ODbL, pinned by tag).
//
//	go generate ./internal/geo
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
)

const (
	airportsURL  = "https://davidmegginson.github.io/ourairports-data/airports.csv"
	countriesURL = "https://raw.githubusercontent.com/mledoze/countries/5.1.0/countries.json"
)

// when two airports share an IATA code (a closed one and its successor),
// the bigger one wins
var rank = map[string]int{"large_airport": 4, "medium_airport": 3, "small_airport": 2, "seaplane_base": 1}

type places struct {
	Airports  map[string][2]float64 `json:"airports"`
	Countries map[string][2]float64 `json:"countries"`
}

func main() {
	out := places{Airports: map[string][2]float64{}, Countries: map[string][2]float64{}}

	body := fetch(airportsURL)
	r := csv.NewReader(body)
	head, err := r.Read()
	if err != nil {
		log.Fatal(err)
	}
	col := map[string]int{}
	for i, h := range head {
		col[h] = i
	}
	best := map[string]int{}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		code, kind := row[col["iata_code"]], row[col["type"]]
		if len(code) != 3 || rank[kind] <= best[code] {
			continue
		}
		lat, err1 := strconv.ParseFloat(row[col["latitude_deg"]], 64)
		lon, err2 := strconv.ParseFloat(row[col["longitude_deg"]], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		best[code] = rank[kind]
		out.Airports[code] = [2]float64{round(lat), round(lon)}
	}
	body.Close() //nolint:errcheck // read-only body

	body = fetch(countriesURL)
	var countries []struct {
		CCA2   string    `json:"cca2"`
		LatLng []float64 `json:"latlng"`
	}
	if err := json.NewDecoder(body).Decode(&countries); err != nil {
		log.Fatal(err)
	}
	body.Close() //nolint:errcheck // read-only body
	for _, c := range countries {
		if len(c.CCA2) == 2 && len(c.LatLng) == 2 {
			out.Countries[c.CCA2] = [2]float64{round(c.LatLng[0]), round(c.LatLng[1])}
		}
	}

	if len(out.Airports) < 5000 || len(out.Countries) < 200 {
		log.Fatalf("suspiciously little data: %d airports, %d countries", len(out.Airports), len(out.Countries))
	}
	b, err := json.Marshal(out)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("places.json", append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("places.json: %d airports, %d countries\n", len(out.Airports), len(out.Countries))
}

func fetch(url string) io.ReadCloser {
	resp, err := http.Get(url) //nolint:gosec,noctx // fixed URLs, a one-off tool
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("%s: %s", url, resp.Status)
	}
	return resp.Body
}

// two decimals is about a kilometre: plenty for a map, and a smaller file
func round(f float64) float64 { return math.Round(f*100) / 100 }
