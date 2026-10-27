package accounts

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
)

const userColumns = `id, username, display_name, role, locale, must_change_password, disabled_at, created_at`

func scanUser(row interface{ Scan(...any) error }) (User, string, error) {
	var (
		u          User
		mustChange int
		disabledAt sql.NullString
		createdAt  string
		hash       string
	)
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Locale, &mustChange, &disabledAt, &createdAt, &hash)
	if err != nil {
		return User{}, "", err
	}
	u.MustChangePassword = mustChange == 1
	u.Disabled = disabledAt.Valid
	u.CreatedAt = parseTS(createdAt)
	return u, hash, nil
}

// getUser returns a user and its password hash.
func (s *Service) getUser(ctx context.Context, q querier, where string, arg any) (User, string, error) {
	u, hash, err := scanUser(q.QueryRowContext(ctx,
		`SELECT `+userColumns+`, password_hash FROM users WHERE `+where, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", apierr.NotFound("no such user")
	}
	return u, hash, err
}

// GetUser returns the account id.
func (s *Service) GetUser(ctx context.Context, id string) (User, error) {
	u, _, err := s.getUser(ctx, s.db, "id = ?", id)
	return u, err
}

// ListUsers returns every account, oldest first.
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userColumns+`, '' FROM users ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, _, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// NewUser is the input of CreateUser and of first-run setup.
type NewUser struct {
	DisplayName string
	Username    string
	Password    string
	Role        auth.Role
	Locale      string
}

// validate normalizes u and returns a validation or weak_password error.
func (u *NewUser) validate(passwordField string) error {
	var p problems
	u.Username = normalizeUsername(u.Username, &p)
	u.DisplayName = checkDisplayName(u.DisplayName, &p)
	if u.Locale == "" {
		u.Locale = "en"
	}
	u.Locale = checkLocale(u.Locale, &p)
	if u.Role != auth.RoleAdmin && u.Role != auth.RoleMember {
		p.add("role", "invalid")
	}
	if err := p.err(); err != nil {
		return err
	}
	if problem := auth.PasswordProblem(u.Password); problem != "" {
		return apierr.WeakPassword(passwordField, problem)
	}
	return nil
}

// insertUser stores u with hash and its default profile, named after it.
func (s *Service) insertUser(ctx context.Context, q querier, u NewUser, hash string, mustChange bool) (User, Profile, error) {
	now := s.now()
	user := User{
		ID: newID(), Username: u.Username, DisplayName: u.DisplayName, Role: u.Role,
		Locale: u.Locale, MustChangePassword: mustChange, CreatedAt: now,
	}
	_, err := q.ExecContext(ctx, `INSERT INTO users
		(id, username, display_name, password_hash, role, locale, must_change_password, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.DisplayName, hash, user.Role, user.Locale, boolInt(mustChange), ts(now), ts(now))
	if isUniqueViolation(err, "users.username") {
		return User{}, Profile{}, apierr.UsernameTaken()
	}
	if err != nil {
		return User{}, Profile{}, err
	}
	profile := Profile{ID: newID(), UserID: user.ID, Name: truncateRunes(u.DisplayName, 32), AllowUnrated: true, IsDefault: true}
	_, err = q.ExecContext(ctx, `INSERT INTO profiles
		(id, user_id, name, avatar_color, allow_unrated, is_default, created_at, updated_at)
		VALUES (?, ?, ?, 0, 1, 1, ?, ?)`, profile.ID, user.ID, profile.Name, ts(now), ts(now))
	return user, profile, err
}

// CreateUser creates an account with a temporary password (F003-R14).
func (s *Service) CreateUser(ctx context.Context, actor auth.Principal, u NewUser) (User, error) {
	if err := u.validate("temporaryPassword"); err != nil {
		return User{}, err
	}
	hash, err := s.hasher.Hash(ctx, u.Password)
	if err != nil {
		return User{}, err
	}
	var user User
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		user, _, err = s.insertUser(ctx, tx, u, hash, true)
		return err
	})
	if err == nil {
		s.logger.Info("user created", "user_id", user.ID, "role", user.Role, "by", actor.UserID)
	}
	return user, err
}

// UserPatch lists the fields an admin can change; nil means unchanged.
type UserPatch struct {
	DisplayName *string
	Role        *auth.Role
	Locale      *string
	Disabled    *bool
}

// UpdateUser applies patch to account id (F003-R1, R14).
func (s *Service) UpdateUser(ctx context.Context, actor auth.Principal, id string, patch UserPatch) (User, error) {
	var p problems
	if patch.DisplayName != nil {
		v := checkDisplayName(*patch.DisplayName, &p)
		patch.DisplayName = &v
	}
	if patch.Locale != nil {
		checkLocale(*patch.Locale, &p)
	}
	if patch.Role != nil && *patch.Role != auth.RoleAdmin && *patch.Role != auth.RoleMember {
		p.add("role", "invalid")
	}
	if err := p.err(); err != nil {
		return User{}, err
	}

	var user User
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		if user, _, err = s.getUser(ctx, tx, "id = ?", id); err != nil {
			return err
		}
		losesAdmin := (patch.Role != nil && *patch.Role != auth.RoleAdmin) || (patch.Disabled != nil && *patch.Disabled)
		if losesAdmin {
			if err := s.guardLastAdmin(ctx, tx, user); err != nil {
				return err
			}
		}
		now := ts(s.now())
		if patch.DisplayName != nil {
			user.DisplayName = *patch.DisplayName
		}
		if patch.Locale != nil {
			user.Locale = *patch.Locale
		}
		if patch.Role != nil {
			user.Role = *patch.Role
		}
		if patch.Disabled != nil {
			user.Disabled = *patch.Disabled
		}
		// COALESCE keeps the original date when it was already disabled.
		_, err = tx.ExecContext(ctx, `UPDATE users SET display_name = ?, locale = ?, role = ?,
			disabled_at = CASE WHEN ? THEN COALESCE(disabled_at, ?) ELSE NULL END, updated_at = ? WHERE id = ?`,
			user.DisplayName, user.Locale, user.Role, boolInt(user.Disabled), now, now, id)
		if err != nil {
			return err
		}
		if patch.Disabled != nil && *patch.Disabled {
			_, err = s.revokeSessions(ctx, tx, id, "")
		}
		return err
	})
	if err == nil {
		s.logger.Info("user updated", "user_id", id, "role", user.Role, "disabled", user.Disabled, "by", actor.UserID)
	}
	return user, err
}

// DeleteUser deletes an account with its profiles and sessions.
func (s *Service) DeleteUser(ctx context.Context, actor auth.Principal, id string) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		user, _, err := s.getUser(ctx, tx, "id = ?", id)
		if err != nil {
			return err
		}
		if err := s.guardLastAdmin(ctx, tx, user); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
		return err
	})
	if err == nil {
		s.logger.Info("user deleted", "user_id", id, "by", actor.UserID)
	}
	return err
}

// ResetPassword sets a temporary password and ends all of the account's sessions.
func (s *Service) ResetPassword(ctx context.Context, actor auth.Principal, id, temporary string) error {
	if problem := auth.PasswordProblem(temporary); problem != "" {
		return apierr.WeakPassword("temporaryPassword", problem)
	}
	hash, err := s.hasher.Hash(ctx, temporary)
	if err != nil {
		return err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, must_change_password = 1, updated_at = ? WHERE id = ?`,
			hash, ts(s.now()), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apierr.NotFound("no such user")
		}
		_, err = s.revokeSessions(ctx, tx, id, "")
		return err
	})
	if err == nil {
		s.logger.Info("password reset", "user_id", id, "by", actor.UserID)
	}
	return err
}

// guardLastAdmin refuses to take away the admin rights of the last active
// admin (F003-R1). Runs inside the caller's transaction.
func (s *Service) guardLastAdmin(ctx context.Context, tx *sql.Tx, target User) error {
	if target.Role != auth.RoleAdmin || target.Disabled {
		return nil
	}
	var others int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND disabled_at IS NULL AND id != ?`, target.ID).Scan(&others)
	if err != nil {
		return err
	}
	if others == 0 {
		return apierr.LastAdmin()
	}
	return nil
}

// Me returns the account behind p with its profiles and active profile.
func (s *Service) Me(ctx context.Context, p auth.Principal) (Me, error) {
	user, err := s.GetUser(ctx, p.UserID)
	if err != nil {
		return Me{}, err
	}
	profiles, err := s.ListProfiles(ctx, p.UserID)
	if err != nil {
		return Me{}, err
	}
	me := Me{User: user, Profiles: profiles}
	for i := range profiles {
		if profiles[i].ID == p.ProfileID {
			me.Active = &profiles[i]
		}
	}
	return me, nil
}

// MePatch lists the fields a user can change on their own account.
type MePatch struct {
	DisplayName *string
	Locale      *string
}

// UpdateMe changes the caller's display name or locale (F003-R15).
func (s *Service) UpdateMe(ctx context.Context, p auth.Principal, patch MePatch) (Me, error) {
	var pr problems
	if patch.DisplayName != nil {
		v := checkDisplayName(*patch.DisplayName, &pr)
		patch.DisplayName = &v
	}
	if patch.Locale != nil {
		checkLocale(*patch.Locale, &pr)
	}
	if err := pr.err(); err != nil {
		return Me{}, err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = COALESCE(?, display_name),
		locale = COALESCE(?, locale), updated_at = ? WHERE id = ?`,
		patch.DisplayName, patch.Locale, ts(s.now()), p.UserID)
	if err != nil {
		return Me{}, err
	}
	return s.Me(ctx, p)
}

// ChangePassword changes the caller's password and ends their other sessions
// (F003-R11, R15). Wrong current passwords count toward the sign-in backoff.
func (s *Service) ChangePassword(ctx context.Context, p auth.Principal, current, next string) error {
	user, hash, err := s.getUser(ctx, s.db, "id = ?", p.UserID)
	if err != nil {
		return err
	}
	if err := s.checkAccountPassword(ctx, user, hash, current); err != nil {
		return err
	}
	if problem := auth.PasswordProblem(next); problem != "" {
		return apierr.WeakPassword("newPassword", problem)
	}
	newHash, err := s.hasher.Hash(ctx, next)
	if err != nil {
		return err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, must_change_password = 0, updated_at = ? WHERE id = ?`,
			newHash, ts(s.now()), p.UserID); err != nil {
			return err
		}
		_, err := s.revokeSessions(ctx, tx, p.UserID, p.SessionID)
		return err
	})
	if err == nil {
		s.logger.Info("password changed", "user_id", p.UserID, "session_id", p.SessionID)
	}
	return err
}

// checkAccountPassword re-checks the password of a signed-in user, with the
// same backoff as sign-in so a stolen session can't brute-force it.
func (s *Service) checkAccountPassword(ctx context.Context, user User, hash, password string) error {
	key := "u:" + user.Username
	if wait := s.logins.Wait(key); wait > 0 {
		return apierr.TooManyAttempts(wait)
	}
	ok, err := s.hasher.Verify(ctx, password, hash)
	if err != nil {
		return err
	}
	if !ok {
		s.logins.Fail(key)
		s.logger.Info("password check failed", "user_id", user.ID)
		return apierr.InvalidCredentials(http.StatusForbidden)
	}
	return nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
