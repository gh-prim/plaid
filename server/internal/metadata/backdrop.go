package metadata

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/metadata/artwork"
)

// BackdropSize is how many posters the sign-in wall gets at most.
const BackdropSize = 60

// backdropMaxAge keeps the public wall family-friendly: anyone who opens
// the sign-in page sees it, a child at the TV included.
const backdropMaxAge = 12

// backdropPoster selects the posters the sign-in wall may show: the
// selected poster of a visible, matched movie or show rated 12 or under,
// or unrated.
const backdropPoster = `SELECT a.id FROM artwork a
	JOIN items i ON i.id = a.owner_id
	JOIN item_metadata m ON m.item_id = i.id
	WHERE a.owner_type = 'item' AND a.kind = 'poster' AND a.selected = 1
	AND i.kind IN ('movie', 'show') AND i.hidden = 0 AND m.state = 'matched'
	AND (m.age_rating IS NULL OR m.age_rating <= ?)`

// BackdropPosters returns up to BackdropSize image ids for the public
// sign-in wall, in random order.
func (s *Service) BackdropPosters(ctx context.Context) ([]string, error) {
	rows, err := s.store.db.QueryContext(ctx, backdropPoster+` ORDER BY random() LIMIT ?`, backdropMaxAge, BackdropSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// BackdropImage is Image for the public sign-in wall: any id
// BackdropPosters could not return is ErrImageNotFound.
func (s *Service) BackdropImage(ctx context.Context, id string, want int) (artwork.Variant, error) {
	var got string
	err := s.store.db.QueryRowContext(ctx, backdropPoster+` AND a.id = ?`, backdropMaxAge, id).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return artwork.Variant{}, ErrImageNotFound
	}
	if err != nil {
		return artwork.Variant{}, err
	}
	return s.Image(ctx, id, want, access.Limit{})
}
