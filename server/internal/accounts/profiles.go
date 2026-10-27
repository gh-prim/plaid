package accounts

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
)

const profileColumns = `id, user_id, name, avatar_color, pin_hash, max_age, allow_unrated, locale, is_default`

func scanProfile(row interface{ Scan(...any) error }) (Profile, string, error) {
	var (
		pr                   Profile
		pinHash, locale      sql.NullString
		maxAge               sql.NullInt64
		allowUnrated, isDflt int
	)
	err := row.Scan(&pr.ID, &pr.UserID, &pr.Name, &pr.AvatarColor, &pinHash, &maxAge, &allowUnrated, &locale, &isDflt)
	if err != nil {
		return Profile{}, "", err
	}
	pr.HasPIN = pinHash.Valid
	if maxAge.Valid {
		age := int(maxAge.Int64)
		pr.MaxAge = &age
	}
	if locale.Valid {
		pr.Locale = &locale.String
	}
	pr.AllowUnrated = allowUnrated == 1
	pr.IsDefault = isDflt == 1
	return pr, pinHash.String, nil
}

// ListProfiles returns the profiles of userID, default first, then by age.
func (s *Service) ListProfiles(ctx context.Context, userID string) ([]Profile, error) {
	return s.listProfiles(ctx, s.db, userID)
}

func (s *Service) listProfiles(ctx context.Context, q querier, userID string) ([]Profile, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+profileColumns+` FROM profiles
		WHERE user_id = ? ORDER BY is_default DESC, created_at, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		pr, _, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

// getProfile returns a profile of userID and its PIN hash. Profiles of other
// accounts are 404.
func (s *Service) getProfile(ctx context.Context, q querier, userID, id string) (Profile, string, error) {
	pr, pin, err := scanProfile(q.QueryRowContext(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, "", apierr.NotFound("no such profile")
	}
	return pr, pin, err
}

// AccessLimit returns what the session's active profile may see (F003
// R18, enforced by F005 R25). With no profile picked yet, the account's
// default profile applies, so skipping the profile picker lifts nothing.
func (s *Service) AccessLimit(ctx context.Context, p auth.Principal) (access.Limit, error) {
	var (
		pr  Profile
		err error
	)
	if p.ProfileID != "" {
		pr, _, err = s.getProfile(ctx, s.db, p.UserID, p.ProfileID)
	} else {
		pr, _, err = scanProfile(s.db.QueryRowContext(ctx,
			`SELECT `+profileColumns+` FROM profiles WHERE user_id = ? AND is_default = 1`, p.UserID))
	}
	if err != nil {
		return access.Limit{}, err
	}
	return access.Limit{MaxAge: pr.MaxAge, AllowUnrated: pr.AllowUnrated}, nil
}

// requireProfileManager lets only an unrestricted profile (or a session with
// no profile picked yet) create, change or delete profiles, so a kid on a
// restricted profile can't lift their own limit (spec revision 6).
func (s *Service) requireProfileManager(ctx context.Context, p auth.Principal) error {
	if p.ProfileID == "" {
		return nil
	}
	active, _, err := s.getProfile(ctx, s.db, p.UserID, p.ProfileID)
	if err != nil {
		return err
	}
	if active.MaxAge != nil {
		return apierr.Forbidden()
	}
	return nil
}

// NewProfile is the input of CreateProfile.
type NewProfile struct {
	Name         string
	AvatarColor  int
	PIN          string
	MaxAge       *int
	AllowUnrated *bool
	Locale       string
}

// CreateProfile adds a profile to the caller's account (F003-R16).
func (s *Service) CreateProfile(ctx context.Context, p auth.Principal, in NewProfile) (Profile, error) {
	if err := s.requireProfileManager(ctx, p); err != nil {
		return Profile{}, err
	}
	var pr problems
	in.Name = checkLength("name", in.Name, 32, &pr)
	checkAvatarColor(in.AvatarColor, &pr)
	if in.PIN != "" {
		checkPIN("pin", in.PIN, &pr)
	}
	if in.MaxAge != nil {
		checkMaxAge(*in.MaxAge, &pr)
	}
	if in.Locale != "" {
		checkLocale(in.Locale, &pr)
	}
	if err := pr.err(); err != nil {
		return Profile{}, err
	}
	// Unrated content is refused by default once a limit is set (F003-R18).
	allowUnrated := in.MaxAge == nil
	if in.AllowUnrated != nil {
		allowUnrated = *in.AllowUnrated
	}
	pinHash := sql.NullString{}
	if in.PIN != "" {
		h, err := s.hasher.Hash(ctx, in.PIN)
		if err != nil {
			return Profile{}, err
		}
		pinHash = sql.NullString{String: h, Valid: true}
	}

	profile := Profile{
		ID: newID(), UserID: p.UserID, Name: in.Name, AvatarColor: in.AvatarColor, HasPIN: pinHash.Valid,
		MaxAge: in.MaxAge, AllowUnrated: allowUnrated,
	}
	if in.Locale != "" {
		profile.Locale = &in.Locale
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id = ?`, p.UserID).Scan(&count); err != nil {
			return err
		}
		if count >= maxProfiles {
			return apierr.ValidationFailed(apierr.Detail{Field: "profiles", Code: "too_many"})
		}
		now := ts(s.now())
		_, err := tx.ExecContext(ctx, `INSERT INTO profiles
			(id, user_id, name, avatar_color, pin_hash, max_age, allow_unrated, locale, is_default, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
			profile.ID, p.UserID, profile.Name, profile.AvatarColor, pinHash, profile.MaxAge,
			boolInt(allowUnrated), profile.Locale, now, now)
		return err
	})
	return profile, err
}

// ProfilePatch lists the fields of a profile update. For the Set* fields,
// Set false means unchanged; Set true with a nil value clears the field.
type ProfilePatch struct {
	Name         *string
	AvatarColor  *int
	SetPIN       bool
	PIN          *string
	SetMaxAge    bool
	MaxAge       *int
	AllowUnrated *bool
	SetLocale    bool
	Locale       *string
	MakeDefault  bool
	// Proof for sensitive changes: the profile's current PIN or the account password.
	CurrentPIN string
	Password   string
}

func (pp ProfilePatch) sensitive() bool { return pp.SetPIN || pp.SetMaxAge || pp.AllowUnrated != nil }

// UpdateProfile changes a profile of the caller's account.
func (s *Service) UpdateProfile(ctx context.Context, p auth.Principal, id string, patch ProfilePatch) (Profile, error) {
	if err := s.requireProfileManager(ctx, p); err != nil {
		return Profile{}, err
	}
	profile, pinHash, err := s.getProfile(ctx, s.db, p.UserID, id)
	if err != nil {
		return Profile{}, err
	}

	var pr problems
	if patch.Name != nil {
		v := checkLength("name", *patch.Name, 32, &pr)
		patch.Name = &v
	}
	if patch.AvatarColor != nil {
		checkAvatarColor(*patch.AvatarColor, &pr)
	}
	if patch.SetPIN && patch.PIN != nil {
		checkPIN("pin", *patch.PIN, &pr)
	}
	if patch.SetMaxAge && patch.MaxAge != nil {
		checkMaxAge(*patch.MaxAge, &pr)
	}
	if patch.SetLocale && patch.Locale != nil {
		checkLocale(*patch.Locale, &pr)
	}
	if err := pr.err(); err != nil {
		return Profile{}, err
	}
	if patch.sensitive() {
		if err := s.checkProfileProof(ctx, p, profile, pinHash, patch.CurrentPIN, patch.Password); err != nil {
			return Profile{}, err
		}
	}

	newPIN := sql.NullString{String: pinHash, Valid: profile.HasPIN}
	if patch.SetPIN {
		newPIN = sql.NullString{}
		if patch.PIN != nil {
			h, err := s.hasher.Hash(ctx, *patch.PIN)
			if err != nil {
				return Profile{}, err
			}
			newPIN = sql.NullString{String: h, Valid: true}
		}
	}

	if patch.Name != nil {
		profile.Name = *patch.Name
	}
	if patch.AvatarColor != nil {
		profile.AvatarColor = *patch.AvatarColor
	}
	profile.HasPIN = newPIN.Valid
	if patch.SetMaxAge {
		profile.MaxAge = patch.MaxAge
	}
	if patch.AllowUnrated != nil {
		profile.AllowUnrated = *patch.AllowUnrated
	}
	if patch.SetLocale {
		profile.Locale = patch.Locale
	}

	err = s.inTx(ctx, func(tx *sql.Tx) error {
		now := ts(s.now())
		if patch.MakeDefault && !profile.IsDefault {
			if _, err := tx.ExecContext(ctx, `UPDATE profiles SET is_default = 0, updated_at = ? WHERE user_id = ? AND is_default = 1`,
				now, p.UserID); err != nil {
				return err
			}
			profile.IsDefault = true
		}
		_, err := tx.ExecContext(ctx, `UPDATE profiles SET name = ?, avatar_color = ?, pin_hash = ?, max_age = ?,
			allow_unrated = ?, locale = ?, is_default = ?, updated_at = ? WHERE id = ?`,
			profile.Name, profile.AvatarColor, newPIN, profile.MaxAge, boolInt(profile.AllowUnrated),
			profile.Locale, boolInt(profile.IsDefault), now, id)
		return err
	})
	return profile, err
}

// checkProfileProof accepts the profile's current PIN or the account password.
func (s *Service) checkProfileProof(ctx context.Context, p auth.Principal, profile Profile, pinHash, pin, password string) error {
	if pin != "" && profile.HasPIN {
		return s.checkPIN(ctx, p.SessionID+":"+profile.ID, pinHash, pin)
	}
	if password != "" {
		user, hash, err := s.getUser(ctx, s.db, "id = ?", p.UserID)
		if err != nil {
			return err
		}
		return s.checkAccountPassword(ctx, user, hash, password)
	}
	if profile.HasPIN {
		return apierr.InvalidPIN()
	}
	return apierr.InvalidCredentials(403)
}

// checkPIN verifies pin against pinHash with the per-key PIN lockout (F003-R17).
func (s *Service) checkPIN(ctx context.Context, key, pinHash, pin string) error {
	if wait := s.pins.Wait(key); wait > 0 {
		return apierr.ProfileLocked(wait)
	}
	ok, err := s.hasher.Verify(ctx, pin, pinHash)
	if err != nil {
		return err
	}
	if !ok {
		s.pins.Fail(key)
		return apierr.InvalidPIN()
	}
	s.pins.Reset(key)
	return nil
}

// DeleteProfile deletes a profile; the last one can't go (F003-R16). If it
// was the default, the oldest remaining profile becomes the default.
func (s *Service) DeleteProfile(ctx context.Context, p auth.Principal, id string) error {
	if err := s.requireProfileManager(ctx, p); err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		profile, _, err := s.getProfile(ctx, tx, p.UserID, id)
		if err != nil {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id = ?`, p.UserID).Scan(&count); err != nil {
			return err
		}
		if count <= 1 {
			return apierr.LastProfile()
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id); err != nil {
			return err
		}
		if profile.IsDefault {
			_, err = tx.ExecContext(ctx, `UPDATE profiles SET is_default = 1, updated_at = ?
				WHERE id = (SELECT id FROM profiles WHERE user_id = ? ORDER BY created_at, id LIMIT 1)`,
				ts(s.now()), p.UserID)
		}
		return err
	})
}

// SelectProfile makes profileID the active profile of the current session.
// A PIN-protected profile needs its PIN every time (F003-R17).
func (s *Service) SelectProfile(ctx context.Context, p auth.Principal, profileID, pin string) (Me, error) {
	profile, pinHash, err := s.getProfile(ctx, s.db, p.UserID, profileID)
	if err != nil {
		return Me{}, err
	}
	if profile.HasPIN {
		if err := s.checkPIN(ctx, p.SessionID+":"+profile.ID, pinHash, pin); err != nil {
			return Me{}, err
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sessions SET profile_id = ? WHERE id = ?`, profileID, p.SessionID); err != nil {
		return Me{}, err
	}
	p.ProfileID = profileID
	return s.Me(ctx, p)
}

// autoProfile is the profile a new session starts on: the only one, if it has
// no PIN. Otherwise the client shows "Who's watching?".
func autoProfile(profiles []Profile) string {
	if len(profiles) == 1 && !profiles[0].HasPIN {
		return profiles[0].ID
	}
	return ""
}
