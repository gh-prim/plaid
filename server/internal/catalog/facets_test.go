package catalog

import (
	"context"
	"reflect"
	"testing"

	"github.com/gh-prim/plaid/server/internal/access"
)

func TestFacets(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	f.movie(movie{id: "eve", title: "Ève", added: now, released: "2019-01-01", age: age(16), genres: `[{"id":18,"name":"Drama"}]`})
	f.movie(movie{id: "1917", title: "1917", added: now, released: "2019-12-25", age: age(13), genres: `[{"id":18,"name":"Drama"}]`, width: 3840, height: 2160, hdr: "hdr10"})
	f.movie(movie{id: "gone", title: "Gone", added: now, genres: `[{"id":18,"name":"Drama"}]`})
	f.exec(`UPDATE items SET hidden = 1 WHERE id = 'gone'`)

	got, err := f.c.Facets(context.Background(), "lm", "movie", access.Limit{})
	if err != nil {
		t.Fatal(err)
	}
	want := Facets{
		Total: 7,
		// By count, then name; ids kept, the hidden item not counted.
		Genres:  []GenreCount{{35, "Comedy", 2}, {18, "Drama", 2}, {878, "Science Fiction", 2}, {16, "Animation", 1}, {27, "Horror", 1}},
		Decades: []DecadeCount{{1970, 1}, {1990, 1}, {2000, 1}, {2010, 2}, {2020, 1}},
		// Home Video's 640×480 is sd; Ève has no file.
		Resolutions: []ResolutionCount{{Res2160, 2}, {Res1080, 2}, {Res720, 1}, {ResSD, 1}},
		HDR:         2,
		Letters:     []LetterCount{{"#", 1}, {"A", 2}, {"D", 1}, {"E", 1}, {"H", 1}, {"T", 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("facets:\n got %+v\nwant %+v", got, want)
	}

	kid, err := f.c.Facets(context.Background(), "lm", "movie", access.Limit{MaxAge: age(12)})
	if err != nil {
		t.Fatal(err)
	}
	if kid.Total != 2 || len(kid.Genres) != 2 || kid.HDR != 0 || !reflect.DeepEqual(kid.Letters, []LetterCount{{"A", 1}, {"T", 1}}) {
		t.Errorf("max age 12: %+v", kid)
	}
}

func TestInitial(t *testing.T) {
	for in, want := range map[string]string{"amelie": "A", "élise": "E", "1917": "#", "千と千尋": "#", "": "#", "  zorro": "Z", "ølstykke": "#"} {
		if got := initial(in); got != want {
			t.Errorf("initial(%q) = %q, want %q", in, got, want)
		}
	}
}
