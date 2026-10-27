package certs

import (
	"slices"
	"testing"
)

func TestAge(t *testing.T) {
	for _, c := range []struct {
		country, system, cert string
		age                   int
		ok                    bool
	}{
		{"US", Movie, "PG-13", 13, true},
		{"US", Movie, "R", 18, true},
		{"us", Movie, " pg ", 10, true},
		{"US", TV, "TV-MA", 18, true},
		{"US", TV, "TV-14", 16, true},
		{"US", Movie, "TV-MA", 0, false}, // wrong system
		{"US", Movie, "NR", 0, false},
		{"US", Movie, "", 0, false},
		{"GB", Movie, "12A", 12, true},
		{"GB", Movie, "15", 16, true},
		{"FR", Movie, "U", 0, true},
		{"FR", TV, "16", 16, true},
		{"DE", Movie, "FSK 12", 12, true},
		{"DE", Movie, "6", 7, true},
		{"ES", Movie, "APTA", 0, true},
		{"IT", Movie, "VM14", 16, true},
		{"NL", Movie, "9", 10, true},
		{"CA", Movie, "14A", 16, true},
		{"AU", Movie, "MA15+", 16, true},
		{"BR", Movie, "14", 16, true},
		{"JP", Movie, "PG12", 12, true},
		{"XX", Movie, "PG", 0, false},
	} {
		age, ok := Age(c.country, c.system, c.cert)
		if age != c.age || ok != c.ok {
			t.Errorf("Age(%s, %s, %q) = %d, %v; want %d, %v", c.country, c.system, c.cert, age, ok, c.age, c.ok)
		}
	}
}

// TestTableUsesProfileAges: every age must be one F003 profiles can hold,
// or a profile limit could never match it exactly.
func TestTableUsesProfileAges(t *testing.T) {
	want := []string{"AU", "BR", "CA", "DE", "ES", "FR", "GB", "IT", "JP", "NL", "US"}
	got := Countries()
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("countries = %v, want %v", got, want)
	}
	for country, systems := range table {
		for _, system := range []string{Movie, TV} {
			if len(systems[system]) == 0 {
				t.Errorf("%s has no %s ratings", country, system)
			}
		}
		for system, certs := range systems {
			for cert, age := range certs {
				if !slices.Contains(Ages, age) {
					t.Errorf("%s/%s/%s = %d, not a profile age", country, system, cert, age)
				}
				if normalize(cert) != cert {
					t.Errorf("%s/%s key %q isn't normalized", country, system, cert)
				}
			}
		}
	}
}

func TestParseNFO(t *testing.T) {
	for _, c := range []struct{ in, country, cert string }{
		{"Rated PG-13", "US", "PG-13"},
		{"FR:12", "FR", "12"},
		{"de:FSK 16", "DE", "16"},
		{"PG-13", "US", "PG-13"},
		{"12A", "GB", "12A"},
		{"  ", "GB", ""},
	} {
		def := "US"
		if c.country == "GB" {
			def = "gb"
		}
		country, cert := ParseNFO(c.in, def)
		if country != c.country || cert != c.cert {
			t.Errorf("ParseNFO(%q) = %s %q, want %s %q", c.in, country, cert, c.country, c.cert)
		}
	}
	country, cert := ParseNFO("DE:FSK 16", "US")
	if age, ok := Age(country, Movie, cert); age != 16 || !ok {
		t.Errorf("an .nfo FSK value maps to %d, %v; want 16", age, ok)
	}
}
