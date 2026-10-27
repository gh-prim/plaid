package handlers

import (
	"context"

	"github.com/oapi-codegen/nullable"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
)

// principal returns the caller of a protected route. The auth middleware
// guarantees one; its absence is a wiring bug, answered with 401.
func principal(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.FromContext(ctx)
	if !ok {
		return auth.Principal{}, apierr.Unauthorized()
	}
	return p, nil
}

func body[T any](b *T) (T, error) {
	if b == nil {
		var zero T
		return zero, apierr.BadRequest("request body is required")
	}
	return *b, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toDevice(d api.Device) accounts.Device {
	return accounts.Device{Name: d.Name, Type: string(d.Type), ClientVersion: deref(d.ClientVersion)}
}

func toUser(u accounts.User) api.User {
	return api.User{
		Id: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: api.Role(u.Role),
		Locale: u.Locale, MustChangePassword: u.MustChangePassword,
	}
}

func toAdminUser(u accounts.User) api.AdminUser {
	return api.AdminUser{
		Id: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: api.Role(u.Role),
		Locale: u.Locale, MustChangePassword: u.MustChangePassword, Disabled: u.Disabled, CreatedAt: u.CreatedAt,
	}
}

func toProfile(p accounts.Profile) api.Profile {
	out := api.Profile{
		Id: p.ID, Name: p.Name, AvatarColor: p.AvatarColor, HasPin: p.HasPIN,
		AllowUnrated: p.AllowUnrated, IsDefault: p.IsDefault,
		// Nullable fields are always set: unset ones would marshal as zero values.
		MaxAge: nullable.NewNullNullable[int](),
		Locale: nullable.NewNullNullable[string](),
	}
	if p.MaxAge != nil {
		out.MaxAge.Set(*p.MaxAge)
	}
	if p.Locale != nil {
		out.Locale.Set(*p.Locale)
	}
	return out
}

func toProfiles(ps []accounts.Profile) []api.Profile {
	out := make([]api.Profile, 0, len(ps))
	for _, p := range ps {
		out = append(out, toProfile(p))
	}
	return out
}

func toMe(me accounts.Me) api.Me {
	out := api.Me{User: toUser(me.User), Profiles: toProfiles(me.Profiles)}
	if me.Active != nil {
		active := toProfile(*me.Active)
		out.ActiveProfile = &active
	}
	return out
}

// toAuthResult returns the session to the client: as a cookie for the web
// app, as a token in the body for everyone else (F003-R9).
func toAuthResult(ctx context.Context, res accounts.AuthResult, asCookie bool) api.AuthResult {
	me := toMe(res.Me)
	out := api.AuthResult{
		ExpiresAt:     res.Session.ExpiresAt,
		Session:       api.SessionInfo{Id: res.Session.ID, DeviceName: res.Session.DeviceName, DeviceType: api.DeviceType(res.Session.DeviceType)},
		User:          me.User,
		Profiles:      me.Profiles,
		ActiveProfile: me.ActiveProfile,
	}
	if asCookie {
		auth.SetSessionCookie(ctx, res.Token)
	} else {
		out.Token = &res.Token
	}
	return out
}

func toSessions(sessions []accounts.Session, currentID string) []api.SessionListItem {
	out := make([]api.SessionListItem, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, api.SessionListItem{
			Id: s.ID, DeviceName: s.DeviceName, DeviceType: api.DeviceType(s.DeviceType),
			ClientVersion: optional(s.ClientVersion), LastIp: optional(s.LastIP),
			CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt, Current: s.ID == currentID,
		})
	}
	return out
}

// fromNullable splits a PATCH field: set reports whether it was in the body,
// value is nil when it was null.
func fromNullable[T any](n nullable.Nullable[T]) (set bool, value *T) {
	if !n.IsSpecified() {
		return false, nil
	}
	if n.IsNull() {
		return true, nil
	}
	v := n.MustGet()
	return true, &v
}
