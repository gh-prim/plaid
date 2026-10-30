package catalog

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/gh-prim/plaid/server/internal/access"
)

// Home row types and limits (F008-R31).
const (
	RowContinueWatching = "continue_watching"
	RowRecentlyAdded    = "recently_added"
	RowGenre            = "genre"

	// HomeRowSize is how many items each row brings with the home.
	HomeRowSize = 20
	// genreRows and genreMinItems decide which genres get a row.
	genreRows     = 4
	genreMinItems = 10
)

// ErrBadCursor is a row cursor this server didn't make.
var ErrBadCursor = errors.New("invalid cursor")

// HomeQuery is whose home it is.
type HomeQuery struct {
	ProfileID string
	Access    access.Limit
	// ContinueWatching is F006's list for the profile, in order (R37).
	ContinueWatching []string
}

// Row is a home row and one page of its items.
type Row struct {
	ID          string
	Type        string
	LibraryID   string
	LibraryName string
	Genre       *GenreRef
	IDs         []string
	// Next is the cursor of the next page, "" on the last one.
	Next   string
	SeeAll *SeeAll
}

// GenreRef is a TMDB genre and the name the row shows.
type GenreRef struct {
	ID   int
	Name string
}

// SeeAll is the library grid a row's "See all" opens.
type SeeAll struct {
	LibraryID string
	Sort      string
	Desc      bool
	Genre     int // 0: none
}

type lib struct{ id, name, kind, language string }

// Home returns the item whose backdrop tints the top of the home ("" when
// none) and the rows, in order, each with its first page; rows with no
// items are left out.
func (c *Catalog) Home(ctx context.Context, q HomeQuery) (string, []Row, error) {
	ambient, err := c.ambient(ctx, q)
	if err != nil {
		return "", nil, err
	}
	var rows []Row
	if r := page(Row{ID: RowContinueWatching, Type: RowContinueWatching}, q.ContinueWatching, 0, HomeRowSize); len(r.IDs) > 0 {
		rows = append(rows, r)
	}
	libs, err := c.libraries(ctx)
	if err != nil {
		return "", nil, err
	}
	for _, l := range libs {
		r, err := c.recentlyAdded(ctx, q, l, 0, HomeRowSize)
		if err != nil {
			return "", nil, err
		}
		if len(r.IDs) > 0 {
			rows = append(rows, r)
		}
	}
	genres, err := c.homeGenres(ctx, q)
	if err != nil {
		return "", nil, err
	}
	for _, g := range genres {
		r, err := c.genreRow(ctx, q, g, 0, HomeRowSize)
		if err != nil {
			return "", nil, err
		}
		rows = append(rows, r)
	}
	return ambient, rows, nil
}

// HomeRow returns one page of a home row, from a row id and cursor the
// home gave. An unknown row is ErrNotFound.
func (c *Catalog) HomeRow(ctx context.Context, q HomeQuery, rowID, cursor string, limit int) (Row, error) {
	offset, err := decodeOffset(cursor)
	if err != nil {
		return Row{}, err
	}
	kind, arg, _ := strings.Cut(rowID, ".")
	switch kind {
	case RowContinueWatching:
		if arg != "" {
			return Row{}, ErrNotFound
		}
		return page(Row{ID: rowID, Type: kind}, q.ContinueWatching, offset, limit), nil
	case RowRecentlyAdded:
		libs, err := c.libraries(ctx)
		if err != nil {
			return Row{}, err
		}
		i := slices.IndexFunc(libs, func(l lib) bool { return l.id == arg })
		if i < 0 {
			return Row{}, ErrNotFound
		}
		return c.recentlyAdded(ctx, q, libs[i], offset, limit)
	case RowGenre:
		id, err := strconv.Atoi(arg)
		if err != nil {
			return Row{}, ErrNotFound
		}
		name, err := c.genreName(ctx, id, q)
		if errors.Is(err, sql.ErrNoRows) {
			return Row{}, ErrNotFound
		}
		if err != nil {
			return Row{}, err
		}
		return c.genreRow(ctx, q, GenreRef{id, name}, offset, limit)
	}
	return Row{}, ErrNotFound
}

// page slices ids into r from offset.
func page(r Row, ids []string, offset, limit int) Row {
	lo := min(offset, len(ids))
	hi := min(lo+limit, len(ids))
	r.IDs = ids[lo:hi]
	if hi < len(ids) {
		r.Next = encodeOffset(hi)
	}
	return r
}

func (c *Catalog) recentlyAdded(ctx context.Context, q HomeQuery, l lib, offset, limit int) (Row, error) {
	p, err := c.List(ctx, Query{LibraryID: l.id, Kind: l.kind, ProfileID: q.ProfileID, Access: q.Access,
		Sort: SortAdded, Desc: true, Offset: offset, Limit: limit + 1})
	if err != nil {
		return Row{}, err
	}
	r := Row{ID: RowRecentlyAdded + "." + l.id, Type: RowRecentlyAdded, LibraryID: l.id, LibraryName: l.name,
		IDs: p.IDs, SeeAll: &SeeAll{LibraryID: l.id, Sort: SortAdded, Desc: true}}
	if len(r.IDs) > limit {
		r.IDs, r.Next = r.IDs[:limit], encodeOffset(offset+limit)
	}
	return r, nil
}

// genreRow lists every visible movie and show of the genre, across
// libraries, in an order that changes once a day per profile.
func (c *Catalog) genreRow(ctx context.Context, q HomeQuery, g GenreRef, offset, limit int) (Row, error) {
	where, args := visibleTop(q.Access)
	query := `SELECT i.id, i.library_id FROM items i JOIN item_genres g ON g.item_id = i.id AND g.genre_id = ?
		LEFT JOIN item_metadata m ON m.item_id = i.id WHERE ` + where + ` ORDER BY i.id`
	var ids []string
	perLib := map[string]int{}
	if err := c.scan(ctx, query, append([]any{g.ID}, args...), func(r *sql.Rows) error {
		var id, libID string
		if err := r.Scan(&id, &libID); err != nil {
			return err
		}
		ids = append(ids, id)
		perLib[libID]++
		return nil
	}); err != nil {
		return Row{}, err
	}
	rnd := rand.New(rand.NewPCG(c.daySeed(q.ProfileID, strconv.Itoa(g.ID)))) //nolint:gosec // an order, not a secret
	rnd.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })

	// "See all" opens the library holding most of the genre.
	best := ""
	for id, n := range perLib {
		if best == "" || n > perLib[best] || n == perLib[best] && id < best {
			best = id
		}
	}
	r := Row{ID: RowGenre + "." + strconv.Itoa(g.ID), Type: RowGenre, Genre: &g}
	if best != "" {
		r.SeeAll = &SeeAll{LibraryID: best, Sort: SortTitle, Genre: g.ID}
	}
	return page(r, ids, offset, limit), nil
}

// homeGenres picks the genre rows: genres with at least genreMinItems
// visible movies and shows, the ones the profile watches most first, then
// the biggest.
func (c *Catalog) homeGenres(ctx context.Context, q HomeQuery) ([]GenreRef, error) {
	where, args := visibleTop(q.Access)
	query := `WITH seen AS (
			SELECT w.item_id AS id FROM watch_state w JOIN items i ON i.id = w.item_id AND i.kind = 'movie'
			WHERE w.profile_id = ? AND (w.watched = 1 OR w.position_ms > 0)
			UNION
			SELECT s.parent_id FROM watch_state w JOIN items e ON e.id = w.item_id AND e.kind = 'episode'
			JOIN items s ON s.id = e.parent_id
			WHERE w.profile_id = ? AND (w.watched = 1 OR w.position_ms > 0))
		SELECT g.genre_id, COUNT(*) AS n, COUNT(seen.id) AS watched
		FROM items i JOIN item_genres g ON g.item_id = i.id LEFT JOIN item_metadata m ON m.item_id = i.id
		LEFT JOIN seen ON seen.id = i.id
		WHERE ` + where + ` GROUP BY g.genre_id HAVING n >= ?`
	type genre struct{ id, n, watched int }
	var gs []genre
	if err := c.scan(ctx, query, append(append([]any{q.ProfileID, q.ProfileID}, args...), genreMinItems), func(r *sql.Rows) error {
		var g genre
		if err := r.Scan(&g.id, &g.n, &g.watched); err != nil {
			return err
		}
		gs = append(gs, g)
		return nil
	}); err != nil {
		return nil, err
	}
	slices.SortFunc(gs, func(a, b genre) int {
		return cmp.Or(cmp.Compare(b.watched, a.watched), cmp.Compare(b.n, a.n), cmp.Compare(a.id, b.id))
	})
	out := make([]GenreRef, 0, genreRows)
	for _, g := range gs[:min(genreRows, len(gs))] {
		name, err := c.genreName(ctx, g.id, q)
		if err != nil {
			return nil, err
		}
		out = append(out, GenreRef{g.id, name})
	}
	return out, nil
}

// genreName names a genre from a library in the profile's UI language,
// else from the first library (by name) that has it.
func (c *Catalog) genreName(ctx context.Context, id int, q HomeQuery) (string, error) {
	var name string
	err := c.db.QueryRowContext(ctx, `SELECT g.name FROM item_genres g JOIN items i ON i.id = g.item_id
		JOIN libraries l ON l.id = i.library_id
		WHERE g.genre_id = ? AND i.hidden = 0
		ORDER BY l.metadata_language = (
			SELECT substr(COALESCE(p.locale, u.locale), 1, 2) FROM profiles p JOIN users u ON u.id = p.user_id WHERE p.id = ?
		) DESC, l.name COLLATE NOCASE, l.id LIMIT 1`, id, q.ProfileID).Scan(&name)
	return name, err
}

// ambient is the latest continue-watching item with a backdrop (an
// episode lends its show's), else the most recently added matched item
// with one.
func (c *Catalog) ambient(ctx context.Context, q HomeQuery) (string, error) {
	const hasBackdrop = `EXISTS (SELECT 1 FROM artwork a WHERE a.owner_type = 'item' AND a.owner_id = %s AND a.kind = 'backdrop' AND a.selected = 1)`
	for _, id := range q.ContinueWatching {
		var got string
		err := c.db.QueryRowContext(ctx, `SELECT CASE WHEN `+strings.ReplaceAll(hasBackdrop, "%s", "i.id")+` THEN i.id ELSE show.id END
			FROM items i LEFT JOIN items s ON s.id = i.parent_id AND i.kind = 'episode' LEFT JOIN items show ON show.id = s.parent_id
			WHERE i.id = ? AND (`+strings.ReplaceAll(hasBackdrop, "%s", "i.id")+` OR (show.id IS NOT NULL AND `+strings.ReplaceAll(hasBackdrop, "%s", "show.id")+`))`, id).Scan(&got)
		if err == nil {
			return got, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	where, args := visibleTop(q.Access)
	var id string
	err := c.db.QueryRowContext(ctx, `SELECT i.id FROM items i LEFT JOIN item_metadata m ON m.item_id = i.id
		WHERE `+where+` AND m.state = 'matched' AND `+strings.ReplaceAll(hasBackdrop, "%s", "i.id")+`
		ORDER BY COALESCE(i.newest_child_added_at, i.created_at) DESC, i.id LIMIT 1`, args...).Scan(&id) //nolint:gosec // fixed fragments
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (c *Catalog) libraries(ctx context.Context) ([]lib, error) {
	var out []lib
	err := c.scan(ctx, `SELECT id, name, type, metadata_language FROM libraries ORDER BY name COLLATE NOCASE, id`, nil, func(r *sql.Rows) error {
		var (
			l   lib
			typ string
		)
		if err := r.Scan(&l.id, &l.name, &typ, &l.language); err != nil {
			return err
		}
		l.kind = "movie"
		if typ == "shows" {
			l.kind = "show"
		}
		out = append(out, l)
		return nil
	})
	return out, err
}

// visibleTop is the WHERE of visible movies and shows, with i and m.
func visibleTop(lim access.Limit) (string, []any) {
	where := "i.hidden = 0 AND i.kind IN ('movie', 'show')"
	cond, args := lim.SQL("m")
	if cond != "" {
		where += " AND " + cond
	}
	return where, args
}

// daySeed seeds an order that stays the same for a profile all day.
func (c *Catalog) daySeed(profileID, salt string) (uint64, uint64) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(profileID + "|" + c.now().Format("2006-01-02") + "|" + salt))
	s := h.Sum64()
	return s, s ^ 0x9e3779b97f4a7c15
}

func encodeOffset(n int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(n)))
}

func decodeOffset(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return 0, ErrBadCursor
	}
	n, err := strconv.Atoi(string(b))
	if err != nil || n < 0 {
		return 0, ErrBadCursor
	}
	return n, nil
}
