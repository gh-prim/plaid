// Package auth authenticates API requests and carries the caller's identity
// (Principal) through the request context for every feature.
package auth

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/apierr"
)

// Role is an account's role. The set is extensible (F009 adds "friend").
type Role string

// Roles known today.
const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// Principal is the authenticated caller of a request.
type Principal struct {
	UserID    string
	SessionID string
	// ProfileID is the active profile of the session, empty until one is picked.
	ProfileID          string
	Role               Role
	MustChangePassword bool
}

// IsAdmin reports whether p can use admin-only routes.
func (p Principal) IsAdmin() bool { return p.Role == RoleAdmin }

type ctxKey struct{}

// WithPrincipal returns a context carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the request's Principal; ok is false on public routes.
func FromContext(ctx context.Context) (p Principal, ok bool) {
	p, ok = ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// RequireAdmin returns nil for an admin, else the error a handler returns as
// is: 401 without a session, 403 for any other role.
func RequireAdmin(ctx context.Context) error {
	p, ok := FromContext(ctx)
	if !ok {
		return apierr.Unauthorized()
	}
	if !p.IsAdmin() {
		return apierr.Forbidden()
	}
	return nil
}
