package accounts

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
)

// Avatar kinds (F009).
const (
	AvatarInitial = "initial"
	AvatarBuilder = "builder"
	AvatarPhoto   = "photo"
)

// SetAvatar shows the initial (options "") or a built avatar (options, the
// builder's choices as JSON, already validated by the caller) on a profile
// of the caller's account.
func (s *Service) SetAvatar(ctx context.Context, p auth.Principal, profileID, kind, options string) (Profile, error) {
	if kind != AvatarInitial && kind != AvatarBuilder {
		return Profile{}, apierr.ValidationFailed(apierr.Detail{Field: "kind", Code: "invalid"})
	}
	var out Profile
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, _, err := s.getProfile(ctx, tx, p.UserID, profileID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE profiles SET avatar_kind = ?, avatar_options = ?, avatar_version = avatar_version + 1,
			updated_at = ? WHERE id = ?`, kind, nullString(options), ts(s.now()), profileID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM profile_photos WHERE profile_id = ?`, profileID); err != nil {
			return err
		}
		out, _, err = s.getProfile(ctx, tx, p.UserID, profileID)
		return err
	})
	return out, err
}

// SetPhoto shows a photo on a profile of the caller's account. jpeg is the
// normalized picture.
func (s *Service) SetPhoto(ctx context.Context, p auth.Principal, profileID string, jpeg []byte) (Profile, error) {
	var out Profile
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, _, err := s.getProfile(ctx, tx, p.UserID, profileID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO profile_photos (profile_id, jpeg) VALUES (?, ?)
			ON CONFLICT (profile_id) DO UPDATE SET jpeg = excluded.jpeg`, profileID, jpeg)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE profiles SET avatar_kind = 'photo', avatar_options = NULL,
			avatar_version = avatar_version + 1, updated_at = ? WHERE id = ?`, ts(s.now()), profileID)
		if err != nil {
			return err
		}
		out, _, err = s.getProfile(ctx, tx, p.UserID, profileID)
		return err
	})
	return out, err
}

// ProfilePicture is what GET /avatars/{profileId} draws.
type ProfilePicture struct {
	Avatar Avatar
	Color  int
	// JPEG is the photo, for a photo avatar.
	JPEG []byte
}

// Picture returns any profile's avatar: every signed-in account may see it
// (a party shows everyone's).
func (s *Service) Picture(ctx context.Context, profileID string) (ProfilePicture, error) {
	var (
		pic  ProfilePicture
		opts sql.NullString
		jpeg []byte
	)
	err := s.db.QueryRowContext(ctx, `SELECT p.avatar_kind, p.avatar_options, p.avatar_version, p.avatar_color, ph.jpeg
		FROM profiles p LEFT JOIN profile_photos ph ON ph.profile_id = p.id WHERE p.id = ?`, profileID).
		Scan(&pic.Avatar.Kind, &opts, &pic.Avatar.Version, &pic.Color, &jpeg)
	if errors.Is(err, sql.ErrNoRows) {
		return ProfilePicture{}, apierr.NotFound("no such profile")
	}
	if err != nil {
		return ProfilePicture{}, err
	}
	pic.Avatar.Options, pic.JPEG = opts.String, jpeg
	return pic, nil
}

// DefaultProfiles returns each account's default profile, by user id, for
// the People list.
func (s *Service) DefaultProfiles(ctx context.Context) (map[string]Profile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+profileColumns+` FROM profiles WHERE is_default = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Profile{}
	for rows.Next() {
		pr, _, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out[pr.UserID] = pr
	}
	return out, rows.Err()
}
