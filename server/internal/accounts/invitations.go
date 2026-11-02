package accounts

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
)

// Invitation defaults (F009).
const (
	defaultInviteDays = 7
	maxInviteDays     = 30
	maxInviteName     = 64
)

// Invitation is a single-use link to create a member account.
type Invitation struct {
	ID        string
	Name      string
	GroupIDs  []string
	CreatedAt time.Time
	ExpiresAt time.Time
	// Status is pending, accepted or expired.
	Status string
	// Token is the link's token while pending, when it was kept (step 21).
	Token string
	// UsedAt and UserID when accepted.
	UsedAt *time.Time
	UserID string
}

// Invitation statuses.
const (
	InvitePending  = "pending"
	InviteAccepted = "accepted"
	InviteExpired  = "expired"
)

// inviteHistory is how long accepted and expired invitations stay listed.
const inviteHistory = 30 * 24 * time.Hour

// InvitationPreview is what the invited person sees before signing up.
type InvitationPreview struct {
	Name      string
	InvitedBy string
	ExpiresAt time.Time
}

// NewInvitation is the input of CreateInvitation.
type NewInvitation struct {
	Name     string
	GroupIDs []string
	// Days until it expires, 0 for the default.
	Days int
}

// CreateInvitation creates an invitation and returns it with its token,
// which exists only in this value (F009).
func (s *Service) CreateInvitation(ctx context.Context, actor auth.Principal, in NewInvitation) (Invitation, string, error) {
	var p problems
	in.Name = checkLength("name", strings.TrimSpace(in.Name), maxInviteName, &p)
	if in.Days == 0 {
		in.Days = defaultInviteDays
	}
	if in.Days < 1 || in.Days > maxInviteDays {
		p.add("expiresInDays", "out_of_range")
	}
	// Without a group the new account would see nothing (F009).
	if len(dedupe(in.GroupIDs)) == 0 {
		p.add("groupIds", "required")
	}
	if err := p.err(); err != nil {
		return Invitation{}, "", err
	}
	token := auth.NewToken()
	now := s.now()
	inv := Invitation{ID: newID(), Name: in.Name, GroupIDs: dedupe(in.GroupIDs), CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(in.Days) * 24 * time.Hour), Status: InvitePending, Token: token}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO invitations (id, token_hash, token, name, created_by, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, inv.ID, auth.HashToken(token), token, inv.Name, actor.UserID, ts(now), ts(inv.ExpiresAt))
		if err != nil {
			return err
		}
		for i, g := range inv.GroupIDs {
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM groups WHERE id = ?)`, g).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return apierr.ValidationFailed(apierr.Detail{Field: "groupIds/" + strconv.Itoa(i), Code: "not_found"})
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO invitation_groups (invitation_id, group_id) VALUES (?, ?)`, inv.ID, g); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Invitation{}, "", err
	}
	s.logger.Info("invitation created", "invitation_id", inv.ID, "expires_at", ts(inv.ExpiresAt), "by", actor.UserID)
	return inv, token, nil
}

// ListInvitations returns the pending invitations, then the ones accepted
// or expired in the last 30 days, newest first. Cancelled ones are gone.
func (s *Service) ListInvitations(ctx context.Context) ([]Invitation, error) {
	now := s.now()
	rows, err := s.db.QueryContext(ctx, `SELECT i.id, i.name, i.created_at, i.expires_at, i.used_at, COALESCE(i.used_by, ''),
		COALESCE(i.token, ''), COALESCE(GROUP_CONCAT(g.group_id), '')
		FROM invitations i LEFT JOIN invitation_groups g ON g.invitation_id = i.id
		WHERE i.revoked_at IS NULL AND (i.expires_at > ? OR COALESCE(i.used_at, i.expires_at) > ?)
		GROUP BY i.id ORDER BY i.created_at DESC, i.id DESC`, ts(now), ts(now.Add(-inviteHistory)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pending, past []Invitation
	for rows.Next() {
		var (
			inv              Invitation
			created, expires string
			used             sql.NullString
			groups           string
		)
		if err := rows.Scan(&inv.ID, &inv.Name, &created, &expires, &used, &inv.UserID, &inv.Token, &groups); err != nil {
			return nil, err
		}
		inv.CreatedAt, inv.ExpiresAt = parseTS(created), parseTS(expires)
		inv.GroupIDs = []string{}
		if groups != "" {
			inv.GroupIDs = strings.Split(groups, ",")
		}
		switch {
		case used.Valid:
			at := parseTS(used.String)
			inv.Status, inv.UsedAt, inv.Token = InviteAccepted, &at, ""
		case !now.Before(inv.ExpiresAt):
			inv.Status, inv.Token = InviteExpired, ""
		default:
			inv.Status = InvitePending
		}
		if inv.Status == InvitePending {
			pending = append(pending, inv)
		} else {
			past = append(past, inv)
		}
	}
	return append(append([]Invitation{}, pending...), past...), rows.Err()
}

// RevokeInvitation cancels an invitation that wasn't used.
func (s *Service) RevokeInvitation(ctx context.Context, actor auth.Principal, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE invitations SET revoked_at = ?, token = NULL WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL`,
		ts(s.now()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apierr.NotFound("no such invitation")
	}
	s.logger.Info("invitation revoked", "invitation_id", id, "by", actor.UserID)
	return nil
}

// invitation looks a token up and checks it can still be used. Unknown
// tokens count toward a per-address backoff, so links can't be guessed.
func (s *Service) invitation(ctx context.Context, q querier, token string) (id string, prev InvitationPreview, err error) {
	key := "ip:" + ipString(clientinfo.FromContext(ctx))
	if wait := s.invites.Wait(key); wait > 0 {
		return "", prev, apierr.TooManyAttempts(wait)
	}
	var (
		expires                  string
		used, revoked, invitedBy sql.NullString
	)
	err = q.QueryRowContext(ctx, `SELECT i.id, i.name, i.expires_at, i.used_at, i.revoked_at, u.display_name
		FROM invitations i LEFT JOIN users u ON u.id = i.created_by WHERE i.token_hash = ?`, auth.HashToken(token)).
		Scan(&id, &prev.Name, &expires, &used, &revoked, &invitedBy)
	if errors.Is(err, sql.ErrNoRows) {
		s.invites.Fail(key)
		return "", prev, apierr.InvitationNotFound()
	}
	if err != nil {
		return "", prev, err
	}
	prev.ExpiresAt, prev.InvitedBy = parseTS(expires), invitedBy.String
	if used.Valid || revoked.Valid || !s.now().Before(prev.ExpiresAt) {
		return "", prev, apierr.InvitationExpired()
	}
	return id, prev, nil
}

// CheckInvitation says what an invitation is for (F009).
func (s *Service) CheckInvitation(ctx context.Context, token string) (InvitationPreview, error) {
	_, prev, err := s.invitation(ctx, s.db, token)
	return prev, err
}

// AcceptInput is the body of POST /invite/accept.
type AcceptInput struct {
	Token string
	NewUser
	PIN    string
	Device Device
}

// AcceptInvitation creates a member account in the invitation's groups,
// with a PIN on its first profile, and signs it in (F009).
func (s *Service) AcceptInvitation(ctx context.Context, in AcceptInput) (AuthResult, error) {
	info := clientinfo.FromContext(ctx)
	if _, _, err := s.invitation(ctx, s.db, in.Token); err != nil {
		return AuthResult{}, err
	}
	in.Role = auth.RoleMember
	if err := in.validate("password"); err != nil {
		return AuthResult{}, err
	}
	var p problems
	checkPIN("pin", in.PIN, &p)
	in.Device = checkDevice(in.Device, &p)
	if err := p.err(); err != nil {
		return AuthResult{}, err
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return AuthResult{}, err
	}
	pinHash, err := s.hasher.Hash(ctx, in.PIN)
	if err != nil {
		return AuthResult{}, err
	}
	var res AuthResult
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		// Checked again under the write lock: the link works once.
		id, _, err := s.invitation(ctx, tx, in.Token)
		if err != nil {
			return err
		}
		user, profile, err := s.insertUser(ctx, tx, in.NewUser, hash, false)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE profiles SET pin_hash = ? WHERE id = ?`, pinHash, profile.ID); err != nil {
			return err
		}
		profile.HasPIN = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_members (group_id, user_id)
			SELECT group_id, ? FROM invitation_groups WHERE invitation_id = ?`, user.ID, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE invitations SET used_at = ?, used_by = ?, token = NULL WHERE id = ?`,
			ts(s.now()), user.ID, id); err != nil {
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
	s.logger.Info("invitation accepted", "user_id", res.User.ID, "session_id", res.Session.ID, "ip", ipString(info))
	return res, nil
}
