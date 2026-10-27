// Package match decides which provider result a parsed title is (F005 R4,
// R5): it normalizes titles, scores candidates on title similarity and
// year, and only accepts a clear winner. Close calls go to review.
package match

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/gh-prim/plaid/server/internal/metadata/provider"
)

// Tuning, from the spec (R4, R5) and the cases in testdata/cases.json.
const (
	TitleWeight = 0.7
	YearWeight  = 0.3
	// Threshold is the lowest confidence applied without review.
	Threshold = 0.85
	// Margin is how far the best candidate must lead the second.
	Margin = 0.05
	// PopularityLead breaks a tie within Margin: the favorite still wins
	// when it's this many times more popular than every close candidate
	// (Alien 1979 vs an obscure "The Alien" 1979: 83×). Real remakes stay
	// below it (Dune 2.9×, The Office US/UK 12.5×) and go to review, and
	// so do some obvious ones popularity can't tell apart from them
	// (Arrival 2016: 12.5×, It 2017: 15.6×).
	PopularityLead = 20
	// kept is how many candidates a needs_review item stores.
	kept = 5
)

// Decision states, as item_metadata.state stores them.
const (
	Matched     = "matched"
	NeedsReview = "needs_review"
	Unmatched   = "unmatched"
)

// articles are leading articles dropped before comparing, in the languages
// PLAID ships or that are common in release names.
var articles = map[string]bool{
	"the": true, "a": true, "an": true,
	"le": true, "la": true, "les": true, "l": true, "un": true, "une": true,
	"der": true, "die": true, "das": true, "ein": true, "eine": true,
	"el": true, "los": true, "las": true, "il": true, "lo": true, "gli": true,
	"de": true, "het": true, "een": true,
}

var (
	elision = regexp.MustCompile(`\b([ld])['’]`)
	punct   = regexp.MustCompile(`[^\p{L}\p{N}]+`)
)

// Normalize makes titles comparable: accents removed, lower case, "&" as
// "and", apostrophes dropped ("Philosopher's" = "Philosophers"), elisions
// split ("L'Alliance" = "alliance"), other punctuation as spaces, and a
// leading article removed.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	s = strings.ReplaceAll(b.String(), "&", " and ")
	s = elision.ReplaceAllString(s, "$1 ")
	s = strings.NewReplacer("'", "", "’", "").Replace(s)
	words := strings.Fields(punct.ReplaceAllString(s, " "))
	if len(words) > 1 && articles[words[0]] {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

// Similarity is 1 − normalized Levenshtein distance between normalized
// titles, in [0, 1].
func Similarity(a, b string) float64 {
	ra, rb := []rune(Normalize(a)), []rune(Normalize(b))
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	return 1 - float64(levenshtein(ra, rb))/float64(max(len(ra), len(rb)))
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// yearScore compares a parsed year (0 = unknown) with a candidate's.
func yearScore(parsed, candidate int) float64 {
	switch {
	case parsed == 0:
		return 0.5
	case candidate == parsed:
		return 1
	case candidate != 0 && (candidate == parsed-1 || candidate == parsed+1):
		return 0.7
	}
	return 0
}

// Scored is a candidate with its confidence.
type Scored struct {
	provider.Candidate
	Confidence float64
}

// Score is a candidate's confidence for a parsed title and year (R4).
func Score(title string, year int, c provider.Candidate) float64 {
	sim := max(Similarity(title, c.Title), Similarity(title, c.OriginalTitle))
	return TitleWeight*sim + YearWeight*yearScore(year, c.Year)
}

// Decision is the outcome of matching one title.
type Decision struct {
	State string
	// Best is the applied candidate when State is Matched.
	Best *Scored
	// Candidates are the top ones, best first (kept for review).
	Candidates []Scored
}

// Decide scores candidates and applies R5: the best one wins when it's at
// least Threshold and leads the second by more than Margin, or, on a tie
// within Margin, is PopularityLead times more popular than every candidate
// in that tie.
func Decide(title string, year int, cands []provider.Candidate) Decision {
	scored := make([]Scored, 0, len(cands))
	for _, c := range cands {
		scored = append(scored, Scored{Candidate: c, Confidence: Score(title, year, c)})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Confidence != scored[j].Confidence {
			return scored[i].Confidence > scored[j].Confidence
		}
		return scored[i].Popularity > scored[j].Popularity
	})
	if len(scored) == 0 {
		return Decision{State: Unmatched}
	}
	d := Decision{State: NeedsReview, Candidates: scored[:min(len(scored), kept)]}
	best := scored[0]
	if best.Confidence >= Threshold && (clearLead(scored) || popularLead(scored)) {
		d.State = Matched
		d.Best = &best
	}
	return d
}

func clearLead(s []Scored) bool { return len(s) == 1 || s[0].Confidence-s[1].Confidence > Margin }

// popularLead: within the tie, the best candidate is by far the most popular.
func popularLead(s []Scored) bool {
	for _, c := range s[1:] {
		if s[0].Confidence-c.Confidence > Margin {
			break
		}
		if c.Popularity*PopularityLead > s[0].Popularity {
			return false
		}
	}
	return true
}

// Searcher is the part of a provider matching needs.
type Searcher interface {
	Search(ctx context.Context, kind, query string, year int, loc provider.Locale) ([]provider.Candidate, error)
}

// Run searches for a parsed title, again without the year when nothing
// comes back (R4), and decides. Searches use English, so titles compare
// against what release names usually carry; the original title covers the
// rest.
func Run(ctx context.Context, p Searcher, kind, title string, year int) (Decision, error) {
	loc := provider.Locale{Language: "en", Region: "US"}
	cands, err := p.Search(ctx, kind, title, year, loc)
	if err != nil {
		return Decision{}, err
	}
	if len(cands) == 0 && year != 0 {
		if cands, err = p.Search(ctx, kind, title, 0, loc); err != nil {
			return Decision{}, err
		}
	}
	d := Decide(title, year, cands)
	// "Les Intouchables" finds nothing close; its TMDB title is
	// "Intouchables". When no candidate reaches the threshold, try once
	// more without a leading article.
	words := strings.Fields(title)
	weak := len(d.Candidates) == 0 || d.Candidates[0].Confidence < Threshold
	if weak && len(words) > 1 && articles[strings.ToLower(words[0])] {
		more, err := p.Search(ctx, kind, strings.Join(words[1:], " "), year, loc)
		if err != nil {
			return Decision{}, err
		}
		d = Decide(title, year, merge(cands, more))
	}
	return d, nil
}

func merge(a, b []provider.Candidate) []provider.Candidate {
	out := append([]provider.Candidate(nil), a...)
	for _, c := range b {
		dup := false
		for _, x := range a {
			dup = dup || x.TMDBID == c.TMDBID
		}
		if !dup {
			out = append(out, c)
		}
	}
	return out
}
