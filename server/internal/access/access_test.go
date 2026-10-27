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
		name   string
		limit  Limit
		rating *int
		want   bool
	}{
		{"no limit, rated", Limit{}, age(18), true},
		{"no limit, unrated", Limit{}, nil, true},
		{"under the limit", Limit{MaxAge: age(12)}, age(10), true},
		{"at the limit", Limit{MaxAge: age(12)}, age(12), true},
		{"over the limit", Limit{MaxAge: age(12)}, age(13), false},
		{"unrated, not allowed", Limit{MaxAge: age(16)}, nil, false},
		{"unrated, allowed", Limit{MaxAge: age(16), AllowUnrated: true}, nil, true},
		{"all ages", Limit{MaxAge: age(0)}, age(0), true},
	} {
		if got := c.limit.Allows(c.rating); got != c.want {
			t.Errorf("%s: Allows = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSQLMatchesAllows runs the SQL condition on real rows and checks it
// agrees with Allows for every limit and rating.
func TestSQLMatchesAllows(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, age_rating INTEGER)`); err != nil {
		t.Fatal(err)
	}
	ratings := []*int{nil, age(0), age(7), age(12), age(13), age(16), age(18)}
	for i, r := range ratings {
		var v sql.NullInt64
		if r != nil {
			v = sql.NullInt64{Int64: int64(*r), Valid: true}
		}
		if _, err := d.Exec(`INSERT INTO t (id, age_rating) VALUES (?, ?)`, i, v); err != nil {
			t.Fatal(err)
		}
	}
	limits := []Limit{{}, {MaxAge: age(12)}, {MaxAge: age(12), AllowUnrated: true}, {MaxAge: age(0)}, {MaxAge: age(18)}}
	for _, l := range limits {
		cond, args := l.SQL("m")
		q := `SELECT id FROM t m`
		if cond != "" {
			q += ` WHERE ` + cond
		}
		rows, err := d.Query(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		got := map[int]bool{}
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			got[id] = true
		}
		rows.Close()
		for i, r := range ratings {
			if got[i] != l.Allows(r) {
				t.Errorf("limit %+v, rating %v: SQL = %v, Allows = %v", l, r, got[i], l.Allows(r))
			}
		}
	}
}
