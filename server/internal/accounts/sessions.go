package accounts

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
)

const (
	// sessionTTL is the sliding lifetime of a session (F003-R11).
	sessionTTL = 90 * 24 * time.Hour
	// touchEvery limits last_seen_at writes per session (F003-R11).
	touchEvery = 5 * time.Minute
)

var _ auth.SessionLookup = (*Service)(nil)

// createSession stores a new session and returns its clear token, once.
func (s *Service) createSession(ctx context.Context, q querier, userID, profileID string, d Device, ip string) (string, Session, error) {
	token := auth.NewToken()
	now := s.now()
	sess := Session{
		ID:            newID(),
		UserID:        userID,
		ProfileID:     profileID,
		DeviceName:    d.Name,
		DeviceType:    d.Type,
		ClientVersion: d.ClientVersion,
		LastIP:        ip,
		CreatedAt:     now,
		LastSeenAt:    now,
		ExpiresAt:     now.Add(sessionTTL),
	}
	_, err := q.ExecContext(ctx, `INSERT INTO sessions
		(id, user_id, profile_id, token_hash, device_name, device_type, client_version, last_ip, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, userID, nullString(profileID), auth.HashToken(token), d.Name, d.Type,
		nullString(d.ClientVersion), nullString(ip), ts(now), ts(now), ts(sess.ExpiresAt))
	if err != nil {
		return "", Session{}, err
	}
	return token, sess, nil
}

// Lookup resolves a session token for the auth middleware. Expired, revoked
// and disabled-account sessions don't exist as far as callers can tell.
func (s *Service) Lookup(ctx context.Context, token string) (auth.Principal, error) {
	now := s.now()
	var (
		p          auth.Principal
		profileID  sql.NullString
		lastSeen   string
		mustChange int
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.profile_id, s.last_seen_at, u.role, u.must_change_password
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.disabled_at IS NULL`,
		auth.HashToken(token), ts(now),
	).Scan(&p.SessionID, &p.UserID, &profileID, &lastSeen, &p.Role, &mustChange)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Principal{}, auth.ErrNoSession
	}
	if err != nil {
		return auth.Principal{}, err
	}
	p.ProfileID = profileID.String
	p.MustChangePassword = mustChange == 1

	if now.Sub(parseTS(lastSeen)) >= touchEvery {
		ip := ""
		if info := clientinfo.FromContext(ctx); info.IP.IsValid() {
			ip = info.IP.String()
		}
		// Best effort: a failed touch must not fail the request.
		if _, err := s.db.ExecContext(ctx,
			`UPDATE sessions SET last_seen_at = ?, expires_at = ?, last_ip = COALESCE(?, last_ip) WHERE id = ?`,
			ts(now), ts(now.Add(sessionTTL)), nullString(ip), p.SessionID); err != nil {
			s.logger.Warn("session touch failed", "session_id", p.SessionID, "err", err)
		}
	}
	return p, nil
}

const sessionColumns = `id, user_id, profile_id, device_name, device_type, client_version, last_ip, created_at, last_seen_at, expires_at`

func scanSession(row interface{ Scan(...any) error }) (Session, error) {
	var (
		sess                             Session
		profileID, clientVersion, lastIP sql.NullString
		createdAt, lastSeenAt, expiresAt string
	)
	err := row.Scan(&sess.ID, &sess.UserID, &profileID, &sess.DeviceName, &sess.DeviceType,
		&clientVersion, &lastIP, &createdAt, &lastSeenAt, &expiresAt)
	if err != nil {
		return Session{}, err
	}
	sess.ProfileID, sess.ClientVersion, sess.LastIP = profileID.String, clientVersion.String, lastIP.String
	sess.CreatedAt, sess.LastSeenAt, sess.ExpiresAt = parseTS(createdAt), parseTS(lastSeenAt), parseTS(expiresAt)
	return sess, nil
}

func (s *Service) getSession(ctx context.Context, q querier, id string) (Session, error) {
	sess, err := scanSession(q.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, apierr.NotFound("no such session")
	}
	return sess, err
}

// ListSessions returns the live sessions of userID, most recently seen first.
func (s *Service) ListSessions(ctx context.Context, userID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+sessionColumns+` FROM sessions
		WHERE user_id = ? AND revoked_at IS NULL AND expires_at > ?
		ORDER BY last_seen_at DESC, id DESC`, userID, ts(s.now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// RevokeSession ends one live session of userID. Unknown or foreign ids are
// 404, so nobody can probe other accounts' session ids.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`,
		ts(s.now()), sessionID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apierr.NotFound("no such session")
	}
	s.logger.Info("session revoked", "user_id", userID, "session_id", sessionID)
	return nil
}

// RevokeOtherSessions ends every session of userID except keepID.
func (s *Service) RevokeOtherSessions(ctx context.Context, userID, keepID string) error {
	n, err := s.revokeSessions(ctx, s.db, userID, keepID)
	if err == nil && n > 0 {
		s.logger.Info("other sessions revoked", "user_id", userID, "kept_session_id", keepID, "count", n)
	}
	return err
}

// revokeSessions ends all live sessions of userID except keepID ("" = none).
func (s *Service) revokeSessions(ctx context.Context, q querier, userID, keepID string) (int64, error) {
	res, err := q.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND id != ? AND revoked_at IS NULL`,
		ts(s.now()), userID, keepID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Logout ends the session sessionID.
func (s *Service) Logout(ctx context.Context, p auth.Principal) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, ts(s.now()), p.SessionID)
	if err == nil {
		s.logger.Info("signed out", "user_id", p.UserID, "session_id", p.SessionID)
	}
	return err
}

// SessionActive reports whether a session can still be used: not revoked,
// not expired, its account not disabled. F006 ends the playback of a
// session that isn't, whichever way it ended (R7). Unknown ids are false.
func (s *Service) SessionActive(ctx context.Context, sessionID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.disabled_at IS NULL`,
		sessionID, ts(s.now())).Scan(&n)
	return n == 1, err
}

// SessionOwner names who is behind a session, for the admin's list of
// playback sessions (F006 R29).
type SessionOwner struct {
	Username    string
	ProfileName string // "" with no profile picked
	DeviceName  string
}

// SessionOwner returns the names behind p's session.
func (s *Service) SessionOwner(ctx context.Context, p auth.Principal) (SessionOwner, error) {
	var (
		o       SessionOwner
		profile sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT u.username, s.device_name, (SELECT name FROM profiles WHERE id = ? AND user_id = u.id)
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.id = ?`,
		p.ProfileID, p.SessionID).Scan(&o.Username, &o.DeviceName, &profile)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionOwner{}, apierr.NotFound("no such session")
	}
	o.ProfileName = profile.String
	return o, err
}
