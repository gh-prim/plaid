package nfo

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func parseFile(t *testing.T, name string) *NFO {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n, err := Parse(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return n
}

func TestRadarrMovie(t *testing.T) {
	n := parseFile(t, "radarr-movie.nfo")
	if n.Kind != Movie || n.Title != "Le Fabuleux Destin d'Amélie Poulain" || n.SortTitle != "Fabuleux Destin d'Amélie Poulain" ||
		n.Year != 2001 || n.Premiered != "2001-04-25" || n.Runtime != 122 || n.MPAA != "FR:U" ||
		n.TMDBID != 194 || n.IMDbID != "tt0211915" || n.Set != "Jeunet collection" ||
		n.Tagline != "Elle va changer votre vie." || !strings.HasPrefix(n.Plot, "Amélie") {
		t.Errorf("movie = %+v", n)
	}
	if !slices.Equal(n.Genres, []string{"Comédie", "Romance"}) || !slices.Equal(n.Studios, []string{"Claudie Ossard Productions"}) {
		t.Errorf("genres %v studios %v", n.Genres, n.Studios)
	}
	if len(n.Actors) != 2 || n.Actors[1] != (Actor{Name: "Mathieu Kassovitz", Role: "Nino Quincampoix", Order: 1}) {
		t.Errorf("actors = %+v", n.Actors)
	}
}

func TestLegacyKodiMovieWithTrailingURL(t *testing.T) {
	n := parseFile(t, "kodi-legacy-movie.nfo")
	if n.Kind != Movie || n.Title != "The Matrix" || n.IMDbID != "tt0133093" || n.TMDBID != 603 ||
		n.Plot != "A computer hacker learns the truth." || n.Set != "The Matrix Collection" || n.MPAA != "Rated R" {
		t.Errorf("movie = %+v", n)
	}
	if n.Runtime != 0 {
		t.Errorf("runtime %q isn't a number, got %d", "136 min", n.Runtime)
	}
	if len(n.Studios) != 1 || !strings.HasPrefix(n.Studios[0], "Warner Bros.") {
		t.Errorf("HTML entity in studio: %q", n.Studios)
	}
}

func TestSonarrShowAndEpisode(t *testing.T) {
	s := parseFile(t, "sonarr-tvshow.nfo")
	if s.Kind != Show || s.TVDBID != 334824 || s.TMDBID != 70523 || s.IMDbID != "tt5753856" || s.Year != 2017 || s.MPAA != "DE:FSK 16" {
		t.Errorf("show = %+v", s)
	}
	e := parseFile(t, "sonarr-episode.nfo")
	if e.Kind != Episode || e.Season != 1 || e.Episode != 1 || e.Premiered != "2017-12-01" || e.Title != "Secrets" || e.TVDBID != 6337834 {
		t.Errorf("episode = %+v", e)
	}
	if s.Season != -1 || s.Episode != -1 {
		t.Errorf("a show has no season/episode, got %d/%d", s.Season, s.Episode)
	}
}

func TestURLOnly(t *testing.T) {
	n := parseFile(t, "url-only.nfo")
	if n.Kind != URLOnly || n.IMDbID != "tt0094291" {
		t.Errorf("url-only = %+v", n)
	}
	n, err := Parse(strings.NewReader("https://www.themoviedb.org/tv/1396-breaking-bad\n"))
	if err != nil || n.TMDBID != 1396 {
		t.Errorf("tmdb link = %+v, %v", n, err)
	}
}

func TestBrokenAndHostile(t *testing.T) {
	for name, in := range map[string]string{
		"empty":        "",
		"garbage":      "this is not an nfo",
		"unknown root": "<musicvideo><title>x</title></musicvideo>",
		"broken xml":   "<movie><title>x</title",
	} {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	big := strings.NewReader("<movie><plot>" + strings.Repeat("x", MaxSize) + "</plot></movie>")
	if _, err := Parse(big); !errors.Is(err, ErrTooLarge) {
		t.Errorf("over 1 MB: %v", err)
	}
	// encoding/xml never expands external entities.
	xxe := `<?xml version="1.0"?><!DOCTYPE movie [<!ENTITY x SYSTEM "file:///etc/passwd">]><movie><title>&x;</title></movie>`
	if n, err := Parse(strings.NewReader(xxe)); err == nil && strings.Contains(n.Title, "root:") {
		t.Error("external entity expanded")
	}
}
