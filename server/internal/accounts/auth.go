package accounts

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
)

// setupCodeLen is the length of the one-time first-run code printed in the log.
const setupCodeLen = 8

// SetupComplete reports whether any account exists (F003-R2).
func (s *Service) SetupComplete(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists)
	return exists, err
}

// PrepareSetupCode returns a fresh one-time setup code while the server has
// no account, for main to print in the log; "" once setup is complete.
func (s *Service) PrepareSetupCode(ctx context.Context) (string, error) {
	done, err := s.SetupComplete(ctx)
	if err != nil || done {
		return "", err
	}
	code := auth.NewCode(setupCodeLen)
	s.mu.Lock()
	s.setupCode = code
	s.mu.Unlock()
	return code, nil
}

// RequiresSetupCode reports whether a client must give the setup code: it
// isn't on a private or loopback address, or it came through a proxy we
// don't trust (F003-R3, spec revision 2).
func RequiresSetupCode(info clientinfo.Info) bool {
	return !auth.IsLocal(info.IP) || info.UntrustedForward
}

// SetupInput is the body of first-run setup.
type SetupInput struct {
	NewUser
	SetupCode string
	Device    Device
}

// CreateFirstAdmin creates the first admin and signs them in (F003-R2, R3).
func (s *Service) CreateFirstAdmin(ctx context.Context, in SetupInput) (AuthResult, error) {
	info := clientinfo.FromContext(ctx)
	if done, err := s.SetupComplete(ctx); err != nil || done {
		if err == nil {
			err = apierr.SetupComplete()
		}
		return AuthResult{}, err
	}
	if RequiresSetupCode(info) {
		key := "setup:" + info.IP.String()
		if wait := s.logins.Wait(key); wait > 0 {
			return AuthResult{}, apierr.TooManyAttempts(wait)
		}
		s.mu.Lock()
		want := s.setupCode
		s.mu.Unlock()
		given := strings.ToUpper(strings.TrimSpace(in.SetupCode))
		if want == "" || !auth.EqualSecret(given, want) {
			s.logins.Fail(key)
			s.logger.Info("setup refused", "reason", "setup code required", "ip", info.IP.String())
			return AuthResult{}, apierr.SetupCodeRequired()
		}
	}

	in.Role = auth.RoleAdmin
	if err := in.validate("password"); err != nil {
		return AuthResult{}, err
	}
	var p problems
	in.Device = checkDevice(in.Device, &p)
	if err := p.err(); err != nil {
		return AuthResult{}, err
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return AuthResult{}, err
	}

	var res AuthResult
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		// Checked again under the write lock: two concurrent setups can both
		// pass the check above, only one gets here first.
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return apierr.SetupComplete()
		}
		user, profile, err := s.insertUser(ctx, tx, in.NewUser, hash, false)
		if err != nil {
			return err
		}
		token, sess, err := s.createSession(ctx, tx, user.ID, profile.ID, in.Device, ipString(info))
		if err != nil {
			return err
		}
		res = AuthResult{Token: token, Session: sess, Me: Me{User: user, Profiles: []Profile{profile}}}
		res.Active = &res.Profiles[0]
		return nil
	})
	if err != nil {
		return AuthResult{}, err
	}
	s.mu.Lock()
	s.setupCode = ""
	s.mu.Unlock()
	s.logger.Info("first admin created", "user_id", res.User.ID, "session_id", res.Session.ID, "ip", ipString(info))
	return res, nil
}

// Login signs in with username and password (F003-R6, R7).
func (s *Service) Login(ctx context.Context, username, password string, d Device) (AuthResult, error) {
	info := clientinfo.FromContext(ctx)
	var p problems
	d = checkDevice(d, &p)
	if err := p.err(); err != nil {
		return AuthResult{}, err
	}
	username = strings.ToLower(strings.TrimSpace(username))
	userKey, ipKey := "u:"+username, "ip:"+ipString(info)
	if wait := max(s.logins.Wait(userKey), s.logins.Wait(ipKey)); wait > 0 {
		return AuthResult{}, apierr.TooManyAttempts(wait)
	}

	fail := func(userID, reason string) (AuthResult, error) {
		s.logins.Fail(userKey)
		s.logins.Fail(ipKey)
		s.logger.Info("sign-in failed", "user_id", userID, "reason", reason, "ip", ipString(info))
		return AuthResult{}, apierr.InvalidCredentials(http.StatusUnauthorized)
	}

	user, hash, err := s.getUser(ctx, s.db, "username = ?", username)
	if _, notFound := errors.AsType[*apierr.Error](err); notFound {
		// Same hashing work as a real check, so timing doesn't tell (F003-R6).
		if err := s.hasher.VerifyDummy(ctx, password); err != nil {
			return AuthResult{}, err
		}
		return fail("", "unknown username")
	}
	if err != nil {
		return AuthResult{}, err
	}
	ok, err := s.hasher.Verify(ctx, password, hash)
	if err != nil {
		return AuthResult{}, err
	}
	if !ok {
		return fail(user.ID, "wrong password")
	}
	if user.Disabled {
		return fail(user.ID, "account disabled")
	}
	s.logins.Reset(userKey)
	s.logins.Reset(ipKey)

	if s.hasher.NeedsRehash(hash) {
		if newHash, err := s.hasher.Hash(ctx, password); err == nil {
			_, _ = s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, newHash, user.ID)
		}
	}

	profiles, err := s.ListProfiles(ctx, user.ID)
	if err != nil {
		return AuthResult{}, err
	}
	token, sess, err := s.createSession(ctx, s.db, user.ID, autoProfile(profiles), d, ipString(info))
	if err != nil {
		return AuthResult{}, err
	}
	s.logger.Info("signed in", "user_id", user.ID, "session_id", sess.ID, "device_type", d.Type, "ip", ipString(info))
	return authResult(token, sess, user, profiles), nil
}

func authResult(token string, sess Session, user User, profiles []Profile) AuthResult {
	res := AuthResult{Token: token, Session: sess, Me: Me{User: user, Profiles: profiles}}
	for i := range res.Profiles {
		if res.Profiles[i].ID == sess.ProfileID {
			res.Active = &res.Profiles[i]
		}
	}
	return res
}

func ipString(info clientinfo.Info) string {
	if !info.IP.IsValid() {
		return ""
	}
	return info.IP.String()
}
