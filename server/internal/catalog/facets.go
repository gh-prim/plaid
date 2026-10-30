package catalog

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/gh-prim/plaid/server/internal/access"
)

// Facets is what a library's grid can be filtered by, with counts over
// the items the profile may see (F008-R27).
type Facets struct {
	Total       int
	Genres      []GenreCount // most first
	Decades     []DecadeCount
	Resolutions []ResolutionCount // 2160p, 1080p, 720p, sd
	HDR         int
	Letters     []LetterCount // "#" first, then A–Z
}

// GenreCount is a TMDB genre with its name in the library's language.
type GenreCount struct {
	ID    int
	Name  string
	Count int
}

// DecadeCount counts items of the decade starting in From.
type DecadeCount struct{ From, Count int }

// ResolutionCount counts items by the bucket of their largest file.
type ResolutionCount struct {
	Value string
	Count int
}

// LetterCount counts items whose title starts with Letter.
type LetterCount struct {
	Letter string
	Count  int
}

// Facets returns the facets of a library's top-level items of kind.
func (c *Catalog) Facets(ctx context.Context, libraryID, kind string, lim access.Limit) (Facets, error) {
	where := "i.library_id = ? AND i.hidden = 0 AND i.kind = ?"
	args := []any{libraryID, kind}
	if cond, a := lim.SQL("m"); cond != "" {
		where += " AND " + cond
		args = append(args, a...)
	}
	// v: the visible items with their sort key and year.
	v := `v AS (SELECT i.id AS id, COALESCE(m.sort_title, i.sort_title, '') AS sort_key, ` + yearExpr + ` AS year
		FROM items i LEFT JOIN item_metadata m ON m.item_id = i.id WHERE ` + where + `)`
	f := Facets{Genres: []GenreCount{}, Decades: []DecadeCount{}, Resolutions: []ResolutionCount{}, Letters: []LetterCount{}}

	var keys []string
	if err := c.scan(ctx, `WITH `+v+` SELECT sort_key FROM v ORDER BY sort_key`, args, func(r *sql.Rows) error {
		var k string
		if err := r.Scan(&k); err != nil {
			return err
		}
		keys = append(keys, k)
		return nil
	}); err != nil {
		return f, err
	}
	f.Total = len(keys)
	f.Letters = letters(keys)

	if err := c.scan(ctx, `WITH `+v+` SELECT g.genre_id, MAX(g.name), COUNT(*) AS n FROM v JOIN item_genres g ON g.item_id = v.id
		GROUP BY g.genre_id ORDER BY n DESC, MAX(g.name)`, args, func(r *sql.Rows) error {
		var g GenreCount
		if err := r.Scan(&g.ID, &g.Name, &g.Count); err != nil {
			return err
		}
		f.Genres = append(f.Genres, g)
		return nil
	}); err != nil {
		return f, err
	}

	if err := c.scan(ctx, `WITH `+v+` SELECT year / 10 * 10 AS d, COUNT(*) FROM v WHERE year > 0 GROUP BY d ORDER BY d`, args, func(r *sql.Rows) error {
		var d DecadeCount
		if err := r.Scan(&d.From, &d.Count); err != nil {
			return err
		}
		f.Decades = append(f.Decades, d)
		return nil
	}); err != nil {
		return f, err
	}

	targs := append([]any{libraryID, kind, libraryID}, args...)
	byRes := map[string]int{}
	if err := c.scan(ctx, `WITH `+techCTE+`, `+v+` SELECT `+resolutionBucket("COALESCE(t.w, 0)", "COALESCE(t.h, 0)")+` AS b,
		COUNT(*), COALESCE(SUM(t.hdr), 0) FROM v LEFT JOIN t ON t.root = v.id GROUP BY b`, targs, func(r *sql.Rows) error {
		var (
			b       sql.NullString
			n, hdrs int
		)
		if err := r.Scan(&b, &n, &hdrs); err != nil {
			return err
		}
		if b.Valid {
			byRes[b.String] = n
		}
		f.HDR += hdrs
		return nil
	}); err != nil {
		return f, err
	}
	for _, b := range []string{Res2160, Res1080, Res720, ResSD} {
		if n := byRes[b]; n > 0 {
			f.Resolutions = append(f.Resolutions, ResolutionCount{b, n})
		}
	}
	return f, nil
}

// letters groups sort keys by first letter, accents folded ("é" is E);
// digits, symbols and other scripts are "#".
func letters(keys []string) []LetterCount {
	counts := map[string]int{}
	for _, k := range keys {
		counts[initial(k)]++
	}
	out := make([]LetterCount, 0, len(counts))
	for l, n := range counts {
		out = append(out, LetterCount{l, n})
	}
	slices.SortFunc(out, func(a, b LetterCount) int { return strings.Compare(a.Letter, b.Letter) })
	return out
}

func initial(key string) string {
	for _, r := range norm.NFD.String(strings.TrimSpace(key)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		r = unicode.ToUpper(r)
		if r >= 'A' && r <= 'Z' {
			return string(r)
		}
		return "#"
	}
	return "#"
}

func (c *Catalog) scan(ctx context.Context, query string, args []any, row func(*sql.Rows) error) error {
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := row(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
