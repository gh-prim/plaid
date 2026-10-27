package accounts

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
)

const (
	pairingTTL     = 10 * time.Minute
	pairingCodeLen = 6
)

// Pairing is a started TV pairing, as returned to the TV once.
type Pairing struct {
	ID         string
	Code       string // XXX-XXX
	PollSecret string
	ExpiresAt  time.Time
}

// FormatCode shows a 6-character code as XXX-XXX.
func FormatCode(code string) string { return code[:3] + "-" + code[3:] }

// normalizeCode accepts "k7p-4qx", "K7P 4QX" or "K7P4QX".
func normalizeCode(raw string) string {
	r := strings.NewReplacer("-", "", " ", "")
	return strings.ToUpper(r.Replace(strings.TrimSpace(raw)))
}

// StartPairing creates a pairing code for a TV (F003-R20).
func (s *Service) StartPairing(ctx context.Context, d Device) (Pairing, error) {
	var p problems
	d = checkDevice(d, &p)
	if err := p.err(); err != nil {
		return Pairing{}, err
	}
	key := "ip:" + ipString(clientinfo.FromContext(ctx))
	if wait := s.pairings.Wait(key); wait > 0 {
		return Pairing{}, apierr.TooManyAttempts(wait)
	}
	s.pairings.Fail(key) // counts every creation, not only failures

	now := s.now()
	pr := Pairing{ID: newID(), PollSecret: auth.NewToken(), ExpiresAt: now.Add(pairingTTL)}
	for range 5 {
		code := auth.NewCode(pairingCodeLen)
		// An expired row may still hold the code until the janitor runs.
		_, err := s.db.ExecContext(ctx, `DELETE FROM pairing_codes WHERE code = ? AND expires_at <= ? AND claimed_at IS NULL`,
			code, ts(now))
		if err != nil {
			return Pairing{}, err
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO pairing_codes
			(id, code, poll_secret_hash, device_name, device_type, client_version, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			pr.ID, code, auth.HashToken(pr.PollSecret), d.Name, d.Type, nullString(d.ClientVersion), ts(now), ts(pr.ExpiresAt))
		if isUniqueViolation(err, "pairing_codes.code") {
			continue
		}
		if err != nil {
			return Pairing{}, err
		}
		pr.Code = FormatCode(code)
		return pr, nil
	}
	return Pairing{}, errors.New("no free pairing code after 5 tries")
}

// ClaimPairing signs the TV showing code in, on profileID of the caller's
// account. The TV gets the session on its next poll.
func (s *Service) ClaimPairing(ctx context.Context, p auth.Principal, rawCode, profileID, pin string) error {
	key := "u:" + p.UserID
	if wait := s.claims.Wait(key); wait > 0 {
		return apierr.TooManyAttempts(wait)
	}
	code := normalizeCode(rawCode)
	var (
		id, deviceName, deviceType, expiresAt string
		clientVersion, claimedAt              sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, device_name, device_type, client_version, expires_at, claimed_at
		FROM pairing_codes WHERE code = ?`, code).
		Scan(&id, &deviceName, &deviceType, &clientVersion, &expiresAt, &claimedAt)
	if errors.Is(err, sql.ErrNoRows) {
		s.claims.Fail(key)
		return apierr.PairingNotFound()
	}
	if err != nil {
		return err
	}
	now := s.now()
	if claimedAt.Valid || !now.Before(parseTS(expiresAt)) {
		return apierr.PairingExpired()
	}

	profile, pinHash, err := s.getProfile(ctx, s.db, p.UserID, profileID)
	if err != nil {
		return apierr.ValidationFailed(apierr.Detail{Field: "profileId", Code: "not_found"})
	}
	if profile.HasPIN {
		if err := s.checkPIN(ctx, "pair:"+p.UserID+":"+profile.ID, pinHash, pin); err != nil {
			return err
		}
	}

	d := Device{Name: deviceName, Type: deviceType, ClientVersion: clientVersion.String}
	var token, sessionID string
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		t, sess, err := s.createSession(ctx, tx, p.UserID, profile.ID, d, "")
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE pairing_codes SET claimed_at = ?, claimed_session_id = ?
			WHERE id = ? AND claimed_at IS NULL`, ts(now), sess.ID, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apierr.PairingExpired() // claimed concurrently
		}
		token, sessionID = t, sess.ID
		return nil
	})
	if err != nil {
		return err
	}
	s.claims.Reset(key)
	s.mu.Lock()
	s.pairTokens[id] = pendingToken{token: token, expires: parseTS(expiresAt).Add(pairingGrace)}
	s.mu.Unlock()
	s.logger.Info("pairing claimed", "user_id", p.UserID, "session_id", sessionID, "device_type", deviceType)
	return nil
}

// pairingGrace lets a TV that was claimed just before expiry still poll.
const pairingGrace = 10 * time.Minute

// PollPairing is the TV waiting for a claim. pending is true while nobody
// has claimed it; after the claim the result carries the token, exactly once.
func (s *Service) PollPairing(ctx context.Context, id, secret string) (res AuthResult, pending bool, err error) {
	var (
		secretHash                  []byte
		expiresAt                   string
		claimedAt, sessionID, given sql.NullString
	)
	err = s.db.QueryRowContext(ctx, `SELECT poll_secret_hash, expires_at, claimed_at, claimed_session_id, token_delivered_at
		FROM pairing_codes WHERE id = ?`, id).Scan(&secretHash, &expiresAt, &claimedAt, &sessionID, &given)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthResult{}, false, apierr.PairingNotFound()
	}
	if err != nil {
		return AuthResult{}, false, err
	}
	if subtle.ConstantTimeCompare(secretHash, auth.HashToken(secret)) != 1 || given.Valid {
		return AuthResult{}, false, apierr.PairingNotFound()
	}
	if !claimedAt.Valid {
		if !s.now().Before(parseTS(expiresAt)) {
			return AuthResult{}, false, apierr.PairingExpired()
		}
		return AuthResult{}, true, nil
	}

	s.mu.Lock()
	pt, ok := s.pairTokens[id]
	delete(s.pairTokens, id)
	s.mu.Unlock()
	if !ok {
		// The server restarted between claim and poll: the TV starts over.
		return AuthResult{}, false, apierr.PairingNotFound()
	}
	res, err = s.sessionResult(ctx, pt.token, sessionID.String)
	if err != nil {
		return AuthResult{}, false, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE pairing_codes SET token_delivered_at = ? WHERE id = ?`, ts(s.now()), id); err != nil {
		return AuthResult{}, false, err
	}
	return res, false, nil
}

// sessionResult builds the AuthResult of an existing session.
func (s *Service) sessionResult(ctx context.Context, token, sessionID string) (AuthResult, error) {
	sess, err := s.getSession(ctx, s.db, sessionID)
	if err != nil {
		return AuthResult{}, err
	}
	user, err := s.GetUser(ctx, sess.UserID)
	if err != nil {
		return AuthResult{}, err
	}
	profiles, err := s.ListProfiles(ctx, sess.UserID)
	if err != nil {
		return AuthResult{}, err
	}
	return authResult(token, sess, user, profiles), nil
}
