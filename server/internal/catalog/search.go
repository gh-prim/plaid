package catalog

import (
	"cmp"
	"context"
	"database/sql"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/gh-prim/plaid/server/internal/access"
)

// searchCandidates bounds the rows each group reads from the index before
// they are ranked; the best of them are kept.
const searchCandidates = 200

// Results are search hits by group, best first (F008-R28).
type Results struct {
	Movies, Shows, Episodes []string // item ids
	People                  []PersonHit
}

// PersonHit is a person found by name.
type PersonHit struct{ ID, Name string }

type hit struct {
	id, title, original string
	rank                float64
	votes               int
}

// Search finds movies, shows and episodes by title and original title,
// and people by name, on word prefixes, case and accents ignored. Items
// hidden or above the age limit are left out, and people are found only
// through a credit the profile may see. Each group keeps at most limit
// hits: exact titles first, then titles starting with the query, then by
// BM25 rank, then by vote count.
func (c *Catalog) Search(ctx context.Context, q string, lim access.Limit, limit int) (Results, error) {
	res := Results{Movies: []string{}, Shows: []string{}, Episodes: []string{}, People: []PersonHit{}}
	match := matchQuery(q)
	if match == "" {
		return res, nil
	}
	cond, condArgs := lim.SQL("m")
	if cond != "" {
		cond = " AND " + cond
	}
	for _, g := range []struct {
		kind string
		out  *[]string
	}{{"movie", &res.Movies}, {"show", &res.Shows}, {"episode", &res.Episodes}} {
		query := `SELECT i.id, search_index.title, COALESCE(search_index.original_title, ''), bm25(search_index, 10, 5),
			COALESCE(m.rating_count, 0)
			FROM search_index JOIN items i ON i.id = search_index.ref_id LEFT JOIN item_metadata m ON m.item_id = i.id
			WHERE search_index MATCH ? AND search_index.kind = ? AND i.hidden = 0` + cond + `
			ORDER BY bm25(search_index, 10, 5) LIMIT ?` //nolint:gosec // cond is the access predicate
		hits, err := c.hits(ctx, query, append(append([]any{match, g.kind}, condArgs...), searchCandidates))
		if err != nil {
			return res, err
		}
		for _, h := range best(hits, q, limit) {
			*g.out = append(*g.out, h.id)
		}
	}

	query := `SELECT p.id, p.name, '', bm25(search_index, 10, 5), 0
		FROM search_index JOIN people p ON p.id = search_index.ref_id
		WHERE search_index MATCH ? AND search_index.kind = 'person'
		AND EXISTS (SELECT 1 FROM item_credits cr JOIN items i ON i.id = cr.item_id LEFT JOIN item_metadata m ON m.item_id = i.id
			WHERE cr.person_id = p.id AND i.hidden = 0` + cond + `)
		ORDER BY bm25(search_index, 10, 5) LIMIT ?` //nolint:gosec // as above
	hits, err := c.hits(ctx, query, append(append([]any{match}, condArgs...), searchCandidates))
	if err != nil {
		return res, err
	}
	for _, h := range best(hits, q, limit) {
		res.People = append(res.People, PersonHit{ID: h.id, Name: h.title})
	}
	return res, nil
}

func (c *Catalog) hits(ctx context.Context, query string, args []any) ([]hit, error) {
	var out []hit
	err := c.scan(ctx, query, args, func(r *sql.Rows) error {
		var h hit
		if err := r.Scan(&h.id, &h.title, &h.original, &h.rank, &h.votes); err != nil {
			return err
		}
		out = append(out, h)
		return nil
	})
	return out, err
}

// best orders hits by how the title matches q, then rank and votes, and
// keeps the first limit.
func best(hits []hit, q string, limit int) []hit {
	fq := fold(q)
	class := func(h hit) int {
		t, o := fold(h.title), fold(h.original)
		switch {
		case t == fq || o == fq:
			return 0
		case strings.HasPrefix(t, fq) || strings.HasPrefix(o, fq):
			return 1
		}
		return 2
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(class(a), class(b)), cmp.Compare(a.rank, b.rank), cmp.Compare(b.votes, a.votes))
	})
	return hits[:min(limit, len(hits))]
}

// matchQuery turns what the user typed into an FTS5 query: every word a
// quoted prefix, all of them required. Only letters and digits make words,
// so FTS5 syntax (quotes, *, :, ^, AND, NEAR…) can't get through.
func matchQuery(q string) string {
	words := strings.FieldsFunc(q, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for i, w := range words {
		words[i] = `"` + w + `"*`
	}
	return strings.Join(words, " ")
}

// fold lowercases s, strips accents and keeps letters and digits separated
// by single spaces, the way the index's tokenizer sees words.
func fold(s string) string {
	var b strings.Builder
	space := false
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		default:
			space = true
		}
	}
	return b.String()
}
