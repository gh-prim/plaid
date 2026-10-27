package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
)

// GetMe returns the signed-in account.
func (h *Handlers) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	me, err := h.accounts.Me(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetMe200JSONResponse(toMe(me)), nil
}

// UpdateMe changes the caller's display name or locale.
func (h *Handlers) UpdateMe(ctx context.Context, req api.UpdateMeRequestObject) (api.UpdateMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	me, err := h.accounts.UpdateMe(ctx, p, accounts.MePatch{DisplayName: b.DisplayName, Locale: b.Locale})
	if err != nil {
		return nil, err
	}
	return api.UpdateMe200JSONResponse(toMe(me)), nil
}

// ChangeMyPassword changes the caller's password.
func (h *Handlers) ChangeMyPassword(ctx context.Context, req api.ChangeMyPasswordRequestObject) (api.ChangeMyPasswordResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.ChangePassword(ctx, p, b.CurrentPassword, b.NewPassword); err != nil {
		return nil, err
	}
	return api.ChangeMyPassword204Response{}, nil
}

// ListMyProfiles lists the caller's profiles.
func (h *Handlers) ListMyProfiles(ctx context.Context, _ api.ListMyProfilesRequestObject) (api.ListMyProfilesResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	profiles, err := h.accounts.ListProfiles(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return api.ListMyProfiles200JSONResponse(toProfiles(profiles)), nil
}

// CreateMyProfile adds a profile.
func (h *Handlers) CreateMyProfile(ctx context.Context, req api.CreateMyProfileRequestObject) (api.CreateMyProfileResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	in := accounts.NewProfile{
		Name: b.Name, PIN: deref(b.Pin), MaxAge: b.MaxAge, AllowUnrated: b.AllowUnrated, Locale: deref(b.Locale),
	}
	if b.AvatarColor != nil {
		in.AvatarColor = *b.AvatarColor
	}
	profile, err := h.accounts.CreateProfile(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.CreateMyProfile201JSONResponse(toProfile(profile)), nil
}

// UpdateMyProfile changes a profile.
func (h *Handlers) UpdateMyProfile(ctx context.Context, req api.UpdateMyProfileRequestObject) (api.UpdateMyProfileResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	patch := accounts.ProfilePatch{
		Name: b.Name, AvatarColor: b.AvatarColor, AllowUnrated: b.AllowUnrated,
		MakeDefault: b.IsDefault != nil && *b.IsDefault,
		CurrentPIN:  deref(b.CurrentPin), Password: deref(b.Password),
	}
	patch.SetPIN, patch.PIN = fromNullable(b.Pin)
	patch.SetMaxAge, patch.MaxAge = fromNullable(b.MaxAge)
	patch.SetLocale, patch.Locale = fromNullable(b.Locale)
	profile, err := h.accounts.UpdateProfile(ctx, p, req.ProfileId, patch)
	if err != nil {
		return nil, err
	}
	return api.UpdateMyProfile200JSONResponse(toProfile(profile)), nil
}

// DeleteMyProfile deletes a profile.
func (h *Handlers) DeleteMyProfile(ctx context.Context, req api.DeleteMyProfileRequestObject) (api.DeleteMyProfileResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.DeleteProfile(ctx, p, req.ProfileId); err != nil {
		return nil, err
	}
	return api.DeleteMyProfile204Response{}, nil
}

// SelectProfile sets the active profile of the current session.
func (h *Handlers) SelectProfile(ctx context.Context, req api.SelectProfileRequestObject) (api.SelectProfileResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	me, err := h.accounts.SelectProfile(ctx, p, b.ProfileId, deref(b.Pin))
	if err != nil {
		return nil, err
	}
	return api.SelectProfile200JSONResponse(toMe(me)), nil
}

// ListMySessions lists the caller's signed-in devices.
func (h *Handlers) ListMySessions(ctx context.Context, _ api.ListMySessionsRequestObject) (api.ListMySessionsResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := h.accounts.ListSessions(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return api.ListMySessions200JSONResponse(toSessions(sessions, p.SessionID)), nil
}

// RevokeMySession signs out one of the caller's devices.
func (h *Handlers) RevokeMySession(ctx context.Context, req api.RevokeMySessionRequestObject) (api.RevokeMySessionResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.RevokeSession(ctx, p.UserID, req.SessionId); err != nil {
		return nil, err
	}
	return api.RevokeMySession204Response{}, nil
}

// RevokeMyOtherSessions signs out every other device of the caller.
func (h *Handlers) RevokeMyOtherSessions(ctx context.Context, _ api.RevokeMyOtherSessionsRequestObject) (api.RevokeMyOtherSessionsResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.RevokeOtherSessions(ctx, p.UserID, p.SessionID); err != nil {
		return nil, err
	}
	return api.RevokeMyOtherSessions204Response{}, nil
}
