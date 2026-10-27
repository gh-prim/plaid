// Package settings stores server-wide key/value settings in the settings
// table (F005: the admin's metadata provider keys).
package settings

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Keys used today. They name settings; they aren't credentials.
//
//nolint:gosec // G101 matches the "api_key" in the names
const (
	TMDBAPIKey        = "metadata.tmdb.api_key"
	FanartAPIKey      = "metadata.fanart.api_key"
	FanartPersonalKey = "metadata.fanart.personal_key"
)

// Store reads and writes settings.
type Store struct {
	db *sql.DB
	// Now is the clock; tests replace it.
	Now func() time.Time
}

// New returns a store over an open database.
func New(db *sql.DB) *Store { return &Store{db: db, Now: time.Now} }

// Get returns a setting's value; ok is false when it isn't set.
func (s *Store) Get(ctx context.Context, key string) (value string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

// Set stores a setting, replacing any previous value.
func (s *Store) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, s.Now().UTC().Format(time.RFC3339))
	return err
}

// Delete removes a setting. Deleting a missing key is not an error.
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
}
