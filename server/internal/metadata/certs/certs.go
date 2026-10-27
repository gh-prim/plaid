// Package certs maps content certifications (PG-13, FSK 12, TV-MA…) to the
// minimum ages F003 profiles use (F005 R24).
package certs

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// Systems: films and TV shows are rated differently in most countries.
const (
	Movie = "movie"
	TV    = "tv"
)

// Ages are the only values a profile's age limit can take (F003 R18).
var Ages = []int{0, 7, 10, 12, 13, 16, 18}

//go:embed certifications.json
var raw []byte

// table is country → system → certification → age.
var table = func() map[string]map[string]map[string]int {
	var t map[string]json.RawMessage
	if err := json.Unmarshal(raw, &t); err != nil {
		panic("certs: " + err.Error())
	}
	out := map[string]map[string]map[string]int{}
	for country, body := range t {
		if strings.HasPrefix(country, "_") {
			continue
		}
		var systems map[string]map[string]int
		if err := json.Unmarshal(body, &systems); err != nil {
			panic("certs: " + country + ": " + err.Error())
		}
		out[country] = systems
	}
	return out
}()

// Age returns the minimum age for cert in country's system. ok is false for
// an unknown country, system or certification, which counts as unrated.
func Age(country, system, cert string) (age int, ok bool) {
	age, ok = table[strings.ToUpper(country)][system][normalize(cert)]
	return age, ok
}

// Countries lists the countries the table covers.
func Countries() []string {
	out := make([]string, 0, len(table))
	for c := range table {
		out = append(out, c)
	}
	return out
}

func normalize(cert string) string {
	cert = strings.ToUpper(strings.TrimSpace(cert))
	cert = strings.TrimPrefix(cert, "FSK")
	cert = strings.TrimPrefix(cert, "RATED")
	return strings.TrimSpace(cert)
}

// ParseNFO splits a Kodi .nfo <mpaa> value into a country and a
// certification. Kodi writes "Rated PG-13" (US), "FR:12" or "DE:FSK 16",
// and some tools a bare "PG-13"; a value without a country gets
// defaultCountry.
func ParseNFO(mpaa, defaultCountry string) (country, cert string) {
	v := strings.TrimSpace(mpaa)
	if c, rest, found := strings.Cut(v, ":"); found && len(strings.TrimSpace(c)) == 2 {
		return strings.ToUpper(strings.TrimSpace(c)), normalize(rest)
	}
	if strings.HasPrefix(strings.ToUpper(v), "RATED ") {
		return "US", normalize(v)
	}
	return strings.ToUpper(defaultCountry), normalize(v)
}
