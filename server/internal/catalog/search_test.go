package catalog

import (
	"context"
	"slices"
	"testing"

	"github.com/gh-prim/plaid/server/internal/access"
)

func (f *fixture) search(q string, lim access.Limit) Results {
	f.t.Helper()
	r, err := f.c.Search(context.Background(), q, lim, 8)
	if err != nil {
		f.t.Fatalf("Search(%q): %v", q, err)
	}
	return r
}

func TestSearch(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	f.movie(movie{id: "aliens", title: "Aliens", added: now, votes: 300, age: age(16)})
	f.movie(movie{id: "alice", title: "Alice in Wonderland", added: now, votes: 50, age: age(0)})
	f.movie(movie{id: "lotr", title: "The Lord of the Rings", added: now, votes: 1000, age: age(12)})
	f.exec(`UPDATE item_metadata SET original_title = 'Le Fabuleux Destin d''Amélie Poulain' WHERE item_id = 'amelie'`)
	f.movie(movie{id: "parasite", title: "Parasite", added: now, age: age(16)})
	f.exec(`UPDATE item_metadata SET original_title = '기생충' WHERE item_id = 'parasite'`)

	for _, c := range []struct {
		q    string
		want []string
	}{
		{"ali", []string{"alien", "aliens", "alice"}}, // titles starting with it, by BM25 (shorter first), then votes
		{"ALIEN", []string{"alien", "aliens"}},        // the exact title first
		{"rings", []string{"lotr"}},                   // a word inside the title
		{"lord ri", []string{"lotr"}},                 // every word a prefix
		{"amelie", []string{"amelie"}},                // accents folded
		{"fabuleux", []string{"amelie"}},              // original title
		{"기생", []string{"parasite"}},                  // other scripts
		{`dune: "part`, []string{"dune"}},             // FTS5 syntax is just separators
		{"*", nil},
		{"zzz", nil},
	} {
		got := f.search(c.q, access.Limit{}).Movies
		if c.want == nil {
			c.want = []string{}
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%q: %v, want %v", c.q, got, c.want)
		}
	}

	f.exec(`UPDATE items SET hidden = 1 WHERE id = 'aliens'`)
	if got := f.search("aliens", access.Limit{}).Movies; len(got) != 0 {
		t.Errorf("hidden item found: %v", got)
	}
	if got := f.search("ali", access.Limit{MaxAge: age(12)}).Movies; !slices.Equal(got, []string{"alice"}) {
		t.Errorf("max age 12: %v", got)
	}

	// A new movie is found as soon as its metadata is written.
	f.movie(movie{id: "arrival", title: "Arrival", added: now, age: age(12)})
	if got := f.search("arriv", access.Limit{}).Movies; !slices.Equal(got, []string{"arrival"}) {
		t.Errorf("new movie: %v", got)
	}
}

func TestSearchGroupsAndPeople(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	f.exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('dark', 'ls', 'show', 'Dark', ?, ?)`, now, now)
	f.exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES ('dark1', 'ls', 'season', 'dark', 1, ?, ?)`, now, now)
	f.exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, episode_number, title, created_at, updated_at) VALUES ('ep', 'ls', 'episode', 'dark1', 1, 1, 'Dark Matter', ?, ?)`, now, now)
	f.exec(`INSERT INTO item_metadata (item_id, state, age_rating, updated_at) VALUES ('dark', 'matched', 16, ?), ('ep', 'matched', 16, ?)`, now, now)
	f.exec(`INSERT INTO people (id, name, updated_at) VALUES ('ridley', 'Ridley Scott', ?), ('nobody', 'Ridley Nobody', ?), ('jean', 'Jean-Pierre Jeunet', ?)`, now, now, now)
	f.exec(`INSERT INTO item_credits (item_id, person_id, role, ord) VALUES ('alien', 'ridley', 'director', 0), ('amelie', 'jean', 'director', 0)`)

	r := f.search("dark", access.Limit{})
	if !slices.Equal(r.Shows, []string{"dark"}) || !slices.Equal(r.Episodes, []string{"ep"}) || len(r.Movies) != 0 {
		t.Errorf("dark: %+v", r)
	}
	r = f.search("ridley", access.Limit{})
	if len(r.People) != 1 || r.People[0] != (PersonHit{"ridley", "Ridley Scott"}) {
		t.Errorf("people with a credit only: %+v", r.People)
	}
	if r := f.search("jean pierre", access.Limit{}); len(r.People) != 1 {
		t.Errorf("hyphenated name: %+v", r.People)
	}
	kid := access.Limit{MaxAge: age(12)}
	if r := f.search("ridley", kid); len(r.People) != 0 {
		t.Errorf("a person only in a 16+ movie, for a kid: %+v", r.People)
	}
	if r := f.search("dark", kid); len(r.Shows)+len(r.Episodes) != 0 {
		t.Errorf("a 16+ show for a kid: %+v", r)
	}
	if r := f.search("jean", kid); len(r.People) != 1 {
		t.Errorf("Jeunet through Amélie (12): %+v", r.People)
	}
}

func TestFold(t *testing.T) {
	for in, want := range map[string]string{"Amélie": "amelie", "  Dune:  Part Two ": "dune part two", "Jean-Pierre": "jean pierre", "L'Été": "l ete"} {
		if got := fold(in); got != want {
			t.Errorf("fold(%q) = %q, want %q", in, got, want)
		}
	}
}
