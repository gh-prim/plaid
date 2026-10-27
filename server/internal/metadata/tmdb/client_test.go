package tmdb_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdb"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

var (
	en = provider.Locale{Language: "en", Region: "US"}
	fr = provider.Locale{Language: "fr", Region: "FR"}
)

func TestSearch(t *testing.T) {
	srv := tmdbtest.New(t)
	c := srv.Client()
	ctx := context.Background()
	got, err := c.Search(ctx, provider.Movie, "The Matrix", 1999, en)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0].TMDBID != 603 || got[0].Year != 1999 || got[0].Title != "The Matrix" {
		t.Errorf("The Matrix (1999) = %+v", got[:min(len(got), 2)])
	}
	shows, err := c.Search(ctx, provider.Show, "The Office", 0, en)
	if err != nil {
		t.Fatal(err)
	}
	ids := []int{}
	for _, s := range shows {
		ids = append(ids, s.TMDBID)
	}
	if !slices.Contains(ids, 2316) || !slices.Contains(ids, 2996) || len(shows) > 10 {
		t.Errorf("The Office: ids %v, want both 2316 (US) and 2996 (UK), at most 10", ids)
	}
	paths := srv.Paths()
	if !strings.Contains(paths[0], "year=1999") {
		t.Errorf("movie search lacks the year: %s", paths[0])
	}
	if strings.Contains(paths[1], "year") {
		t.Errorf("a show search without a year sent one: %s", paths[1])
	}
	if _, err := c.Search(ctx, "music", "x", 0, en); err == nil {
		t.Error("unknown kind accepted")
	}
}

func TestFindIMDb(t *testing.T) {
	c := tmdbtest.New(t).Client()
	id, err := c.FindIMDb(context.Background(), provider.Movie, "tt0133093")
	if err != nil || id != 603 {
		t.Errorf("tt0133093 = %d, %v; want 603", id, err)
	}
	if id, err := c.FindIMDb(context.Background(), provider.Show, "tt0133093"); err != nil || id != 0 {
		t.Errorf("as a show = %d, %v; want 0", id, err)
	}
}

func TestMovie(t *testing.T) {
	srv := tmdbtest.New(t)
	m, err := srv.Client().Movie(context.Background(), 603, en)
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "The Matrix" || m.IMDbID != "tt0133093" || m.Runtime != 136 || m.ReleaseDate != "1999-03-31" {
		t.Errorf("movie = %+v", m)
	}
	if m.Certifications["US"] != "R" || m.Certifications["GB"] != "15" || m.Certifications["FR"] != "TP" || m.Certifications["DE"] != "16" {
		t.Errorf("certifications = %v", m.Certifications)
	}
	if m.Collection == nil || m.Collection.TMDBID != 2344 {
		t.Errorf("collection = %+v", m.Collection)
	}
	if len(m.Cast) == 0 || len(m.Cast) > 20 || m.Cast[0].Name != "Keanu Reeves" || m.Cast[0].Character == "" || m.Cast[0].Role != "cast" {
		t.Errorf("cast = %d, first %+v", len(m.Cast), m.Cast[0])
	}
	var directors, writers int
	for _, p := range m.Crew {
		switch p.Role {
		case "director":
			directors++
		case "writer":
			writers++
		}
	}
	if directors != 2 || writers != 2 {
		t.Errorf("crew: %d directors, %d writers (Wachowskis, no duplicates)", directors, writers)
	}
	if len(m.Genres) == 0 || len(m.Studios) == 0 || m.Rating == 0 || m.RatingCount == 0 {
		t.Errorf("genres %v, studios %v, rating %v/%d", m.Genres, m.Studios, m.Rating, m.RatingCount)
	}
	if len(m.Images.Posters) == 0 || len(m.Images.Backdrops) == 0 || m.Images.Posters[0].Path == "" {
		t.Errorf("images = %+v", m.Images)
	}
	if len(m.FieldLanguages) != 0 {
		t.Errorf("English movie fell back: %v", m.FieldLanguages)
	}
	q := srv.Paths()[0]
	for _, want := range []string{"append_to_response=credits%2Cvideos%2Cimages%2Crelease_dates%2Cexternal_ids%2Ctranslations", "language=en-US", "include_image_language=en%2Cen%2Cnull"} {
		if !strings.Contains(q, want) {
			t.Errorf("request %s lacks %s", q, want)
		}
	}
	if srv.Requests() != 1 {
		t.Errorf("a movie took %d requests, want 1 (R9)", srv.Requests())
	}
}

func TestTrailers(t *testing.T) {
	c := tmdbtest.New(t).Client()
	m, err := c.Movie(context.Background(), 438631, fr)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Trailers) == 0 || len(m.Trailers) > 3 {
		t.Fatalf("trailers = %+v", m.Trailers)
	}
	first := m.Trailers[0]
	if first.Language != "fr" || first.Type != "Trailer" || !first.Official || first.Site != "YouTube" || first.Key == "" {
		t.Errorf("first trailer of a French library = %+v, want an official French trailer", first)
	}
	for _, v := range m.Trailers {
		if v.Site != "YouTube" || (v.Type != "Trailer" && v.Type != "Teaser") || (v.Language != "fr" && v.Language != "en") {
			t.Errorf("trailer %+v", v)
		}
	}
}

func TestFrenchFallsBackToEnglish(t *testing.T) {
	c := tmdbtest.New(t).Client()
	office, err := c.Show(context.Background(), 2316, fr)
	if err != nil {
		t.Fatal(err)
	}
	if office.Tagline == "" || office.FieldLanguages["tagline"] != "en" {
		t.Errorf("tagline %q (%v): the empty French tagline should come from English", office.Tagline, office.FieldLanguages)
	}
	if office.FieldLanguages["overview"] != "" || office.Overview == "" {
		t.Errorf("the French overview exists and shouldn't fall back: %v", office.FieldLanguages)
	}
	movie, err := c.Movie(context.Background(), 603, fr)
	if err != nil {
		t.Fatal(err)
	}
	if movie.Title != "Matrix" || len(movie.FieldLanguages) != 0 {
		t.Errorf("Matrix in French = %q, fallbacks %v", movie.Title, movie.FieldLanguages)
	}
}

func TestShow(t *testing.T) {
	c := tmdbtest.New(t).Client()
	s, err := c.Show(context.Background(), 1396, en)
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != provider.Show || s.Title != "Breaking Bad" || s.Status != "ended" || s.TVDBID != 81189 || s.IMDbID != "tt0903747" {
		t.Errorf("show = %+v", s)
	}
	if s.Certifications["US"] != "TV-MA" || s.Certifications["FR"] != "16" {
		t.Errorf("certifications = %v", s.Certifications)
	}
	if len(s.Crew) == 0 || s.Crew[0].Role != "creator" || len(s.Cast) == 0 || s.Cast[0].Character == "" {
		t.Errorf("crew %+v, cast %+v", s.Crew, s.Cast[:1])
	}
	numbers := []int{}
	for _, sn := range s.Seasons {
		numbers = append(numbers, sn.Number)
	}
	if !slices.Contains(numbers, 0) || !slices.Contains(numbers, 5) || len(s.Studios) == 0 {
		t.Errorf("seasons %v, networks %v", numbers, s.Studios)
	}
}

func TestSeason(t *testing.T) {
	srv := tmdbtest.New(t)
	c := srv.Client()
	ctx := context.Background()
	s, err := c.Season(ctx, 1396, 1, fr)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Episodes) != 7 || s.Episodes[0].Name != "Chute libre" || s.Episodes[0].StillPath == "" {
		t.Errorf("Breaking Bad S1 in French = %d episodes, first %+v", len(s.Episodes), s.Episodes[0])
	}
	if srv.Requests() != 1 {
		t.Errorf("a complete French season took %d requests, want 1", srv.Requests())
	}

	// The Daily Show 2024 in French: placeholder names and no overviews,
	// filled from English with one more request (R11).
	ds, err := c.Season(ctx, 2224, 29, fr)
	if err != nil {
		t.Fatal(err)
	}
	e := ds.Episodes[0]
	if strings.HasPrefix(e.Name, "Épisode") || e.Name == "" || e.Overview == "" ||
		e.FieldLanguages["name"] != "en" || e.FieldLanguages["overview"] != "en" {
		t.Errorf("first Daily Show episode = %+v", e)
	}
	if srv.Requests() != 3 {
		t.Errorf("requests = %d, want 3 (Breaking Bad, then Daily Show fr + en)", srv.Requests())
	}
}

func TestCollection(t *testing.T) {
	c := tmdbtest.New(t).Client()
	col, err := c.Collection(context.Background(), 8091, fr)
	if err != nil {
		t.Fatal(err)
	}
	ids := []int{}
	for _, p := range col.Parts {
		ids = append(ids, p.TMDBID)
	}
	if col.Name == "" || !slices.Contains(ids, 348) {
		t.Errorf("collection %q parts %v", col.Name, ids)
	}
}

func TestErrors(t *testing.T) {
	srv := tmdbtest.New(t)
	ctx := context.Background()
	if _, err := srv.Client().Movie(ctx, 999999, en); !errors.Is(err, httpx.ErrNotFound) {
		t.Errorf("unknown movie: %v", err)
	}
	h := httpx.New(httpx.Options{Name: "tmdb", Hosts: []string{"127.0.0.1"}, Transport: srv.Server.Client().Transport})
	if _, err := tmdb.New(h, srv.URL, "wrong-key").Movie(ctx, 603, en); !errors.Is(err, httpx.ErrInvalidKey) {
		t.Errorf("wrong key: %v", err)
	}
}
