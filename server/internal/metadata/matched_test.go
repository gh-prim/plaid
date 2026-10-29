package metadata

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
)

// TestMatchedItemsArePublished: a matched movie reaches the library's
// subscribers with its title, year and artwork; a show's seasons and
// episodes don't (F007 R27).
func TestMatchedItemsArePublished(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true, withArtwork)
	got, stop := e.svc.SubscribeMatched(e.lib.ID)
	defer stop()
	e.write("Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mkv", "x")
	e.scan()

	select {
	case m := <-got:
		if m.Kind != "movie" || m.Title != "Alien" || m.Year != 1979 || m.Poster == nil || m.Poster.ID == "" {
			t.Errorf("matched = %+v", m)
		}
	default:
		t.Fatal("nothing published")
	}
	select {
	case m := <-got:
		t.Errorf("published twice: %+v", m)
	default:
	}
}

func TestMatchedShowOnly(t *testing.T) {
	e := newApplyEnv(t, library.TypeShows, "en", "US", true)
	got, stop := e.svc.SubscribeMatched(e.lib.ID)
	defer stop()
	e.write("Breaking Bad/Season 01/Breaking Bad - S01E01.mkv", "x")
	e.write("Breaking Bad/Season 01/Breaking Bad - S01E02.mkv", "y")
	e.scan()
	e.logErrs()

	var kinds []string
	for len(got) > 0 {
		kinds = append(kinds, (<-got).Kind)
	}
	if len(kinds) != 1 || kinds[0] != "show" {
		t.Errorf("published kinds = %v, want [show]", kinds)
	}
}

func TestMatchedSlowSubscriberDropsOldest(t *testing.T) {
	s := &Service{}
	ch, stop := s.SubscribeMatched("lib")
	defer stop()
	for i := range matchedBuffer + 3 {
		s.matched.publish("lib", Matched{ID: string(rune('a' + i%26)), Year: i})
	}
	if len(ch) != matchedBuffer {
		t.Fatalf("buffered %d, want %d", len(ch), matchedBuffer)
	}
	if first := <-ch; first.Year != 3 {
		t.Errorf("oldest kept = %d, want 3", first.Year)
	}
}

// refusingProvider answers like TMDB after a 401 while refuse is set.
type refusingProvider struct {
	provider.Provider
	refuse atomic.Bool
}

func (p *refusingProvider) Search(ctx context.Context, kind, q string, year int, loc provider.Locale) ([]provider.Candidate, error) {
	if p.refuse.Load() {
		return nil, httpx.ErrInvalidKey
	}
	return p.Provider.Search(ctx, kind, q, year, loc)
}

func (p *refusingProvider) Movie(ctx context.Context, id int, loc provider.Locale) (*provider.Details, error) {
	if p.refuse.Load() {
		return nil, httpx.ErrInvalidKey
	}
	return p.Provider.Movie(ctx, id, loc)
}

type recordedProblems struct{ calls []string }

func (r *recordedProblems) Report(code string, p map[string]string) {
	r.calls = append(r.calls, "report "+code+" "+p["provider"])
}

func (r *recordedProblems) Resolve(code string, p map[string]string) {
	r.calls = append(r.calls, "resolve "+code+" "+p["provider"])
}

// TestRefusedKeyIsAProblem: a refused TMDB key is reported once and
// resolved at the next success (F007 R29).
func TestRefusedKeyIsAProblem(t *testing.T) {
	var prov *refusingProvider
	rec := &recordedProblems{}
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true, func(_ *applyEnv, o *Options) {
		prov = &refusingProvider{Provider: o.Provider}
		o.Provider, o.Problems = prov, rec
	})
	prov.refuse.Store(true)
	e.write("Heat (1995)/Heat (1995).mkv", "x")
	e.write("Alien (1979)/Alien (1979).mkv", "y")
	e.scan()
	prov.refuse.Store(false)
	e.now = e.now.Add(2 * time.Hour) // paused jobs come back after an hour
	e.drain()
	want := []string{"report provider_key_refused tmdb", "resolve provider_key_refused tmdb"}
	if !slices.Equal(rec.calls, want) {
		t.Errorf("problems = %q, want %q", rec.calls, want)
	}
}
