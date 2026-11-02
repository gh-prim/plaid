package access

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gh-prim/plaid/server/internal/db"
)

func age(n int) *int { return &n }

func TestAllows(t *testing.T) {
	for _, c := range []struct {
		name    string
		limit   Limit
		library string
		rating  *int
		want    bool
	}{
		{"no limit, rated", Limit{}, "a", age(18), true},
		{"no limit, unrated", Limit{}, "a", nil, true},
		{"under the limit", Limit{MaxAge: age(12)}, "a", age(10), true},
		{"at the limit", Limit{MaxAge: age(12)}, "a", age(12), true},
		{"over the limit", Limit{MaxAge: age(12)}, "a", age(13), false},
		{"unrated, not allowed", Limit{MaxAge: age(16)}, "a", nil, false},
		{"unrated, allowed", Limit{MaxAge: age(16), AllowUnrated: true}, "a", nil, true},
		{"all ages", Limit{MaxAge: age(0)}, "a", age(0), true},
		{"shared library", Limit{Libraries: []string{"a", "b"}}, "b", nil, true},
		{"library not shared", Limit{Libraries: []string{"a"}}, "b", nil, false},
		{"no library shared", Limit{Libraries: []string{}}, "a", nil, false},
		{"shared, over the age limit", Limit{Libraries: []string{"a"}, MaxAge: age(12)}, "a", age(16), false},
	} {
		if got := c.limit.Allows(c.library, c.rating); got != c.want {
			t.Errorf("%s: Allows = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSQLMatchesAllows runs the SQL condition on real rows and checks it
// agrees with Allows for every limit, library and rating.
func TestSQLMatchesAllows(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, library_id TEXT, age_rating INTEGER)`); err != nil {
		t.Fatal(err)
	}
	type row struct {
		library string
		rating  *int
	}
	var rows []row
	for _, lib := range []string{"a", "b", "c"} {
		for _, r := range []*int{nil, age(0), age(7), age(12), age(13), age(16), age(18)} {
			rows = append(rows, row{lib, r})
		}
	}
	for i, r := range rows {
		var v sql.NullInt64
		if r.rating != nil {
			v = sql.NullInt64{Int64: int64(*r.rating), Valid: true}
		}
		if _, err := d.Exec(`INSERT INTO t (id, library_id, age_rating) VALUES (?, ?, ?)`, i, r.library, v); err != nil {
			t.Fatal(err)
		}
	}
	limits := []Limit{{}, {MaxAge: age(12)}, {MaxAge: age(12), AllowUnrated: true}, {MaxAge: age(0)}, {MaxAge: age(18)},
		{Libraries: []string{}}, {Libraries: []string{"a"}}, {Libraries: []string{"a", "c"}, MaxAge: age(12)}}
	for _, l := range limits {
		cond, args := l.SQL("m", "m")
		q := `SELECT id FROM t m`
		if cond != "" {
			q += ` WHERE ` + cond
		}
		res, err := d.Query(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		got := map[int]bool{}
		for res.Next() {
			var id int
			if err := res.Scan(&id); err != nil {
				t.Fatal(err)
			}
			got[id] = true
		}
		res.Close()
		for i, r := range rows {
			if want := l.Allows(r.library, r.rating); got[i] != want {
				t.Errorf("limit %+v, row %v: SQL = %v, Allows = %v", l, r, got[i], want)
			}
		}
	}
}
