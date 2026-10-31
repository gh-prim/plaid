package match

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/gh-prim/plaid/server/internal/metadata/provider"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"The Matrix":                          "matrix",
		"Amélie":                              "amelie",
		"Le Fabuleux Destin d'Amélie Poulain": "fabuleux destin d amelie poulain",
		"L'Alliance":                          "alliance",
		"Harry Potter and the Philosopher's Stone": "harry potter and the philosophers stone",
		"Mission: Impossible - Fallout":            "mission impossible fallout",
		"Fast & Furious":                           "fast and furious",
		"Spider-Man: Into the Spider-Verse":        "spider man into the spider verse",
		"The":                                      "the", // a lone article is the title
		"2001: A Space Odyssey":                    "2001 a space odyssey",
		"  Der   Untergang ":                       "untergang",
		"Ça":                                       "ca",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSimilarityAndScore(t *testing.T) {
	if s := Similarity("The Matrix", "Matrix"); s != 1 {
		t.Errorf("article difference: %v", s)
	}
	if s := Similarity("Seven", "Se7en"); s != 0.8 {
		t.Errorf("one letter in five: %v", s)
	}
	if s := Similarity("", "x"); s != 0 {
		t.Errorf("empty title: %v", s)
	}
	c := provider.Candidate{Title: "Heat", OriginalTitle: "Heat", Year: 1995}
	for year, want := range map[int]float64{1995: 1, 1996: 0.91, 1994: 0.91, 1990: 0.7, 0: 0.85} {
		if got := Score("Heat", year, c); got < want-1e-9 || got > want+1e-9 {
			t.Errorf("Score(Heat, %d) = %v, want %v", year, got, want)
		}
	}
	// Original title counts: a French name for a film TMDB calls "The Dinner Game".
	if got := Score("Le Dîner de cons", 1998, provider.Candidate{Title: "The Dinner Game", OriginalTitle: "Le Dîner de cons", Year: 1998}); got != 1 {
		t.Errorf("original title: %v", got)
	}
}

func TestDecide(t *testing.T) {
	cand := func(id int, title string, year int, pop float64) provider.Candidate {
		return provider.Candidate{TMDBID: id, Title: title, Year: year, Popularity: pop}
	}
	for _, c := range []struct {
		name  string
		title string
		year  int
		cands []provider.Candidate
		state string
		best  int
	}{
		{"nothing", "X", 2000, nil, Unmatched, 0},
		{"single exact", "Heat", 1995, []provider.Candidate{cand(949, "Heat", 1995, 1)}, Matched, 949},
		{"single weak", "Heat", 1995, []provider.Candidate{cand(1, "Heathers", 1988, 50)}, NeedsReview, 0},
		{"clear lead", "Dune", 2021, []provider.Candidate{cand(841, "Dune", 1984, 15), cand(438631, "Dune", 2021, 45)}, Matched, 438631},
		{"tie, close popularity", "Dune", 0, []provider.Candidate{cand(438631, "Dune", 2021, 45), cand(841, "Dune", 1984, 15)}, NeedsReview, 0},
		{"tie, 20× popularity", "Alien", 1979, []provider.Candidate{cand(1309211, "The Alien", 1979, 0.4), cand(348, "Alien", 1979, 35)}, Matched, 348},
		{"exactly at the threshold", "Lost", 0, []provider.Candidate{cand(4607, "Lost", 2004, 30)}, Matched, 4607},
	} {
		d := Decide(c.title, c.year, c.cands)
		best := 0
		if d.Best != nil {
			best = d.Best.TMDBID
		}
		if d.State != c.state || best != c.best {
			t.Errorf("%s: %s best %d, want %s best %d", c.name, d.State, best, c.state, c.best)
		}
		if d.State == NeedsReview && (len(d.Candidates) == 0 || len(d.Candidates) > kept) {
			t.Errorf("%s: %d candidates kept", c.name, len(d.Candidates))
		}
	}
}

// TestCases runs the matcher on real TMDB results for 86 names (recorded
// by scripts/record-metadata-fixtures.sh) and checks every decision. When
// a constant changes, update testdata/cases.json and the spec's Revisions.
func TestCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Kind     string `json:"kind"`
		Title    string `json:"title"`
		Year     int    `json:"year"`
		Decision string `json:"decision"`
		TMDBID   int    `json:"tmdbId"`
		Note     string `json:"note"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 50 {
		t.Fatalf("%d cases, the plan asks for 50+", len(cases))
	}
	c := tmdbtest.New(t).Client()
	for _, cs := range cases {
		d, err := Run(context.Background(), c, cs.Kind, cs.Title, cs.Year)
		if err != nil {
			t.Fatalf("%s: %v", cs.Title, err)
		}
		got := 0
		if d.Best != nil {
			got = d.Best.TMDBID
		}
		if d.State != cs.Decision || got != cs.TMDBID {
			t.Errorf("%s %q (%d): %s %d, want %s %d %s", cs.Kind, cs.Title, cs.Year, d.State, got, cs.Decision, cs.TMDBID, cs.Note)
		}
	}
}

type fakeSearch []provider.Candidate

func (f fakeSearch) Search(context.Context, string, string, int, provider.Locale) ([]provider.Candidate, error) {
	return f, nil
}

func TestCountryTag(t *testing.T) {
	for in, want := range map[string][2]string{
		"The Office (US)": {"The Office", "US"},
		"The Office (UK)": {"The Office", "GB"},
		"Shameless [US]":  {"Shameless", "US"},
		"The Office":      {"The Office", ""},
		"Godzilla (2014)": {"Godzilla (2014)", ""},
		"Love (Netflix)":  {"Love (Netflix)", ""},
	} {
		title, country := SplitCountry(in)
		if title != want[0] || country != want[1] {
			t.Errorf("SplitCountry(%q) = %q, %q; want %q, %q", in, title, country, want[0], want[1])
		}
	}

	offices := fakeSearch{
		{TMDBID: 2316, Title: "The Office", Year: 2005, Popularity: 188, Countries: []string{"US"}},
		{TMDBID: 2996, Title: "The Office", Year: 2001, Popularity: 15, Countries: []string{"GB"}},
	}
	for title, want := range map[string]int{"The Office (US)": 2316, "The Office (UK)": 2996} {
		d, err := Run(context.Background(), offices, provider.Show, title, 0)
		if err != nil {
			t.Fatal(err)
		}
		if d.State != Matched || d.Best.TMDBID != want {
			t.Errorf("%s: %s %v, want matched %d", title, d.State, d.Best, want)
		}
	}
	// Without the tag, it's still a close call.
	if d, _ := Run(context.Background(), offices, provider.Show, "The Office", 0); d.State != NeedsReview {
		t.Errorf("The Office: %s, want needs_review", d.State)
	}
}
