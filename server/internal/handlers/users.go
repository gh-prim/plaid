package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/auth"
)

// admin returns the caller when they're an admin (F003-R13).
func admin(ctx context.Context) (auth.Principal, error) {
	if err := auth.RequireAdmin(ctx); err != nil {
		return auth.Principal{}, err
	}
	return principal(ctx)
}

// ListUsers lists every account.
func (h *Handlers) ListUsers(ctx context.Context, _ api.ListUsersRequestObject) (api.ListUsersResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	users, err := h.accounts.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListUsers200JSONResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toAdminUser(u))
	}
	return out, nil
}

// CreateUser creates an account with a temporary password.
func (h *Handlers) CreateUser(ctx context.Context, req api.CreateUserRequestObject) (api.CreateUserResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	u, err := h.accounts.CreateUser(ctx, p, accounts.NewUser{
		DisplayName: b.DisplayName, Username: b.Username, Password: b.TemporaryPassword,
		Role: auth.Role(b.Role), Locale: deref(b.Locale),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateUser201JSONResponse(toAdminUser(u)), nil
}

// UpdateUser changes an account.
func (h *Handlers) UpdateUser(ctx context.Context, req api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	patch := accounts.UserPatch{DisplayName: b.DisplayName, Locale: b.Locale, Disabled: b.Disabled}
	if b.Role != nil {
		role := auth.Role(*b.Role)
		patch.Role = &role
	}
	u, err := h.accounts.UpdateUser(ctx, p, req.UserId, patch)
	if err != nil {
		return nil, err
	}
	return api.UpdateUser200JSONResponse(toAdminUser(u)), nil
}

// DeleteUser deletes an account.
func (h *Handlers) DeleteUser(ctx context.Context, req api.DeleteUserRequestObject) (api.DeleteUserResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.DeleteUser(ctx, p, req.UserId); err != nil {
		return nil, err
	}
	return api.DeleteUser204Response{}, nil
}

// ResetUserPassword sets a temporary password.
func (h *Handlers) ResetUserPassword(ctx context.Context, req api.ResetUserPasswordRequestObject) (api.ResetUserPasswordResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.ResetPassword(ctx, p, req.UserId, b.TemporaryPassword); err != nil {
		return nil, err
	}
	return api.ResetUserPassword204Response{}, nil
}

// ListUserSessions lists an account's signed-in devices.
func (h *Handlers) ListUserSessions(ctx context.Context, req api.ListUserSessionsRequestObject) (api.ListUserSessionsResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.accounts.GetUser(ctx, req.UserId); err != nil {
		return nil, err
	}
	sessions, err := h.accounts.ListSessions(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return api.ListUserSessions200JSONResponse(toSessions(sessions, p.SessionID)), nil
}

// RevokeUserSession signs out one device of an account.
func (h *Handlers) RevokeUserSession(ctx context.Context, req api.RevokeUserSessionRequestObject) (api.RevokeUserSessionResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := h.accounts.RevokeSession(ctx, req.UserId, req.SessionId); err != nil {
		return nil, err
	}
	return api.RevokeUserSession204Response{}, nil
}
