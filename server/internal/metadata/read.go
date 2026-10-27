package metadata

import (
	"context"
	"database/sql"
	"errors"
)

// Summary is what item lists show from F005 (R36).
type Summary struct {
	State         string
	Title         string
	ReleaseDate   string
	Rating        float64
	Certification string
	AgeRating     *int
	Genres        []string
}

// Summaries returns the summaries of the given items; items without a
// metadata row are absent (pending).
func (s *Store) Summaries(ctx context.Context, itemIDs []string) (map[string]Summary, error) {
	out := make(map[string]Summary, len(itemIDs))
	if len(itemIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(itemIDs))
	for i, id := range itemIDs {
		args[i] = id
	}
	q := `SELECT item_id, ` + rowColumns + ` FROM item_metadata WHERE item_id IN (` + placeholders(len(itemIDs)) + `)` //nolint:gosec // only placeholders
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		r, err := scanRow(prefixed{rows, &id})
		if err != nil {
			return nil, err
		}
		out[id] = r.summary()
	}
	return out, rows.Err()
}

// prefixed scans a leading column before a row's columns.
type prefixed struct {
	rows  *sql.Rows
	first any
}

func (p prefixed) Scan(dest ...any) error { return p.rows.Scan(append([]any{p.first}, dest...)...) }

func (r row) summary() Summary {
	sm := Summary{State: r.State, Title: r.Title, ReleaseDate: r.ReleaseDate, Rating: r.Rating,
		Certification: r.Certification, AgeRating: r.AgeRating, Genres: []string{}}
	for _, g := range r.Genres {
		sm.Genres = append(sm.Genres, g.Name)
	}
	return sm
}

// Credit is a cast or crew credit of an item.
type Credit struct {
	PersonID  string
	Name      string
	Role      string
	Character string
}

// Detail is everything F005 stores about an item (R36).
type Detail struct {
	Summary
	MatchSource    string
	Confidence     float64
	OriginalTitle  string
	Overview       string
	Tagline        string
	EndDate        string
	ShowStatus     string
	Runtime        int
	RatingCount    int
	CertRegion     string
	Studios        []string
	Trailers       []Trailer
	CollectionID   string
	CollectionName string
	Credits        []Credit
	FieldLanguages map[string]string
	LockedFields   []string
	LocalFields    []string
	TMDBID         int
	IMDbID         string
	TVDBID         int
	DuplicateOf    string
}

// Trailer is a stored trailer (R22).
type Trailer struct {
	Site, Key, Name, Language, Type string
	Official                        bool
}

// Detail returns an item's metadata; ok is false when it has none yet.
func (s *Store) Detail(ctx context.Context, itemID string) (d Detail, ok bool, err error) {
	r, err := scanRow(s.db.QueryRowContext(ctx, `SELECT `+rowColumns+` FROM item_metadata WHERE item_id = ?`, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return d, false, nil
	}
	if err != nil {
		return d, false, err
	}
	d = Detail{
		Summary: r.summary(), MatchSource: r.MatchSource, Confidence: r.Confidence, OriginalTitle: r.OriginalTitle,
		Overview: r.Overview, Tagline: r.Tagline, EndDate: r.EndDate, ShowStatus: r.ShowStatus, Runtime: r.Runtime,
		RatingCount: r.RatingCount, CertRegion: r.CertRegion, Studios: nonNil(r.Studios), CollectionID: r.CollectionID,
		FieldLanguages: r.FieldLanguages, LockedFields: nonNil(r.LockedFields), LocalFields: nonNil(r.LocalFields),
		TMDBID: r.TMDBID, IMDbID: r.IMDbID, TVDBID: r.TVDBID, DuplicateOf: r.DuplicateOf, Trailers: []Trailer{},
	}
	if d.FieldLanguages == nil {
		d.FieldLanguages = map[string]string{}
	}
	for _, v := range r.Trailers {
		d.Trailers = append(d.Trailers, Trailer{Site: v.Site, Key: v.Key, Name: v.Name, Language: v.Language, Type: v.Type, Official: v.Official})
	}
	if d.CollectionID != "" {
		if err := s.db.QueryRowContext(ctx, `SELECT name FROM collections WHERE id = ?`, d.CollectionID).Scan(&d.CollectionName); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return d, false, err
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.name, c.role, COALESCE(c.character, '') FROM item_credits c
		JOIN people p ON p.id = c.person_id WHERE c.item_id = ?
		ORDER BY CASE c.role WHEN 'creator' THEN 0 WHEN 'director' THEN 1 WHEN 'writer' THEN 2 ELSE 3 END, c.ord`, itemID)
	if err != nil {
		return d, false, err
	}
	defer rows.Close()
	d.Credits = []Credit{}
	for rows.Next() {
		var c Credit
		if err := rows.Scan(&c.PersonID, &c.Name, &c.Role, &c.Character); err != nil {
			return d, false, err
		}
		d.Credits = append(d.Credits, c)
	}
	return d, true, rows.Err()
}

// RefreshLibrary schedules a refresh of every movie and show of a library,
// after its language or region changed (R10). Matches are kept.
func (s *Store) RefreshLibrary(ctx context.Context, libraryID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE item_metadata SET refresh_at = ? WHERE state <> 'pending'
		AND item_id IN (SELECT id FROM items WHERE library_id = ? AND kind IN ('movie', 'show'))`, ts(s.now()), libraryID)
	return err
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
