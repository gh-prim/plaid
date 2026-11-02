package handlers

import (
	"context"
	"time"

	"github.com/oapi-codegen/nullable"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/auth"
)

// ListGroups lists the sharing groups (F009).
func (h *Handlers) ListGroups(ctx context.Context, _ api.ListGroupsRequestObject) (api.ListGroupsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	groups, err := h.accounts.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListGroups200JSONResponse, 0, len(groups))
	for _, g := range groups {
		out = append(out, toGroup(g))
	}
	return out, nil
}

// CreateGroup creates a sharing group.
func (h *Handlers) CreateGroup(ctx context.Context, req api.CreateGroupRequestObject) (api.CreateGroupResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	in := accounts.NewGroup{Name: b.Name, Household: value(b.Household)}
	if b.LibraryIds != nil {
		in.LibraryIDs = *b.LibraryIds
	}
	if b.Rights != nil {
		r := fromRights(*b.Rights)
		in.Rights = &r
	}
	g, err := h.accounts.CreateGroup(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.CreateGroup201JSONResponse(toGroup(g)), nil
}

// UpdateGroup changes a group; playbacks of titles its members may no
// longer see end.
func (h *Handlers) UpdateGroup(ctx context.Context, req api.UpdateGroupRequestObject) (api.UpdateGroupResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	patch := accounts.GroupPatch{Name: b.Name, Household: b.Household, LibraryIDs: b.LibraryIds}
	if b.Rights != nil {
		r := fromRights(*b.Rights)
		patch.Rights = &r
	}
	g, err := h.accounts.UpdateGroup(ctx, p, req.GroupId, patch)
	if err != nil {
		return nil, err
	}
	h.recheckPlayback(ctx)
	return api.UpdateGroup200JSONResponse(toGroup(g)), nil
}

// DeleteGroup deletes a group.
func (h *Handlers) DeleteGroup(ctx context.Context, req api.DeleteGroupRequestObject) (api.DeleteGroupResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.DeleteGroup(ctx, p, req.GroupId); err != nil {
		return nil, err
	}
	h.recheckPlayback(ctx)
	return api.DeleteGroup204Response{}, nil
}

// recheckPlayback ends playbacks their accounts may no longer see, at once
// (F009: revoking is immediate, active streams included).
func (h *Handlers) recheckPlayback(ctx context.Context) {
	if h.playback != nil {
		h.playback.Recheck(context.WithoutCancel(ctx))
	}
}

// ListInvitations lists pending invitations.
func (h *Handlers) ListInvitations(ctx context.Context, _ api.ListInvitationsRequestObject) (api.ListInvitationsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	invs, err := h.accounts.ListInvitations(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListInvitations200JSONResponse, 0, len(invs))
	for _, inv := range invs {
		out = append(out, toInvitation(inv))
	}
	return out, nil
}

// CreateInvitation creates a single-use invitation link.
func (h *Handlers) CreateInvitation(ctx context.Context, req api.CreateInvitationRequestObject) (api.CreateInvitationResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	inv, token, err := h.accounts.CreateInvitation(ctx, p, accounts.NewInvitation{
		Name: b.Name, GroupIDs: b.GroupIds, Days: value(b.ExpiresInDays),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateInvitation201JSONResponse{Invitation: toInvitation(inv), Token: token}, nil
}

// RevokeInvitation cancels an invitation.
func (h *Handlers) RevokeInvitation(ctx context.Context, req api.RevokeInvitationRequestObject) (api.RevokeInvitationResponseObject, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.RevokeInvitation(ctx, p, req.InvitationId); err != nil {
		return nil, err
	}
	return api.RevokeInvitation204Response{}, nil
}

// CheckInvitation says what an invitation link is for.
func (h *Handlers) CheckInvitation(ctx context.Context, req api.CheckInvitationRequestObject) (api.CheckInvitationResponseObject, error) {
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	prev, err := h.accounts.CheckInvitation(ctx, b.Token)
	if err != nil {
		return nil, err
	}
	return api.CheckInvitation200JSONResponse{ServerName: h.serverName, Name: prev.Name, InvitedBy: prev.InvitedBy,
		ExpiresAt: prev.ExpiresAt}, nil
}

// AcceptInvitation creates the invited account and signs it in.
func (h *Handlers) AcceptInvitation(ctx context.Context, req api.AcceptInvitationRequestObject) (api.AcceptInvitationResponseObject, error) {
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	in := accounts.AcceptInput{
		Token:   b.Token,
		NewUser: accounts.NewUser{DisplayName: b.DisplayName, Username: b.Username, Password: b.Password, Locale: deref(b.Locale)},
		PIN:     b.Pin,
		Device:  accounts.Device{Name: "Web browser", Type: "web"},
	}
	if b.Device != nil {
		in.Device = toDevice(*b.Device)
	}
	res, err := h.accounts.AcceptInvitation(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.AcceptInvitation201JSONResponse(toAuthResult(ctx, res, auth.IsWebClient(ctx))), nil
}

// adminUsers returns accounts as the People screen shows them.
func (h *Handlers) adminUsers(ctx context.Context, users []accounts.User) ([]api.AdminUser, error) {
	sharing, err := h.accounts.Sharing(ctx)
	if err != nil {
		return nil, err
	}
	seen, err := h.accounts.LastSeen(ctx)
	if err != nil {
		return nil, err
	}
	defaults, err := h.accounts.DefaultProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.AdminUser, 0, len(users))
	for _, u := range users {
		a := toAdminUser(u)
		sh, ok := sharing[u.ID]
		if !ok {
			sh = accounts.UserSharing{GroupIDs: []string{}, ExtraLibraryIDs: []string{}}
		}
		a.GroupIds, a.ExtraLibraryIds, a.ExtraRights = sh.GroupIDs, sh.ExtraLibraryIDs, toOverride(sh.ExtraRights)
		a.LastSeenAt = nullable.NewNullNullable[time.Time]()
		if at, ok := seen[u.ID]; ok {
			if t, err := time.Parse("2006-01-02T15:04:05.000Z", at); err == nil {
				a.LastSeenAt.Set(t)
			}
		}
		if pr, ok := defaults[u.ID]; ok {
			a.AvatarColor, a.Avatar = pr.AvatarColor, toAvatar(pr)
		} else {
			a.Avatar = api.Avatar{Kind: api.AvatarKindInitial}
		}
		acc, err := h.accounts.Access(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		a.Access = toAccess(acc)
		out = append(out, a)
	}
	return out, nil
}

func (h *Handlers) adminUser(ctx context.Context, u accounts.User) (api.AdminUser, error) {
	out, err := h.adminUsers(ctx, []accounts.User{u})
	if err != nil {
		return api.AdminUser{}, err
	}
	return out[0], nil
}

// withAccess adds what the account may see and do to me.
func (h *Handlers) withAccess(ctx context.Context, userID string, me *api.Me) error {
	acc, err := h.accounts.Access(ctx, userID)
	if err != nil {
		return err
	}
	me.Access = toAccess(acc)
	return nil
}

func toRights(r accounts.Rights) api.Rights {
	out := api.Rights{MaxQuality: api.MaxQuality(r.MaxQuality), Downloads: r.Downloads, Party: api.PartyRight(r.Party),
		CanCreateProfiles: r.CanCreateProfiles, MaxStreams: nullable.NewNullNullable[int]()}
	if r.MaxStreams != nil {
		out.MaxStreams.Set(*r.MaxStreams)
	}
	return out
}

func fromRights(r api.Rights) accounts.Rights {
	out := accounts.Rights{MaxQuality: string(r.MaxQuality), Downloads: r.Downloads, Party: string(r.Party),
		CanCreateProfiles: r.CanCreateProfiles}
	if r.MaxStreams.IsSpecified() && !r.MaxStreams.IsNull() {
		n := r.MaxStreams.MustGet()
		out.MaxStreams = &n
	}
	return out
}

// nullableValue reads a nullable field: nil when absent or null.
func nullableValue[T any](n nullable.Nullable[T]) *T {
	if !n.IsSpecified() || n.IsNull() {
		return nil
	}
	v := n.MustGet()
	return &v
}

func fromOverride(o api.RightsOverride) accounts.RightsOverride {
	out := accounts.RightsOverride{MaxStreams: nullableValue(o.MaxStreams), UnlimitedStreams: nullableValue(o.UnlimitedStreams),
		Downloads: nullableValue(o.Downloads), CanCreateProfiles: nullableValue(o.CanCreateProfiles)}
	if q := nullableValue(o.MaxQuality); q != nil {
		s := string(*q)
		out.MaxQuality = &s
	}
	if p := nullableValue(o.Party); p != nil {
		s := string(*p)
		out.Party = &s
	}
	return out
}

func toOverride(o accounts.RightsOverride) api.RightsOverride {
	out := api.RightsOverride{MaxQuality: nullable.NewNullNullable[api.MaxQuality](), MaxStreams: nullable.NewNullNullable[int](),
		UnlimitedStreams: nullable.NewNullNullable[bool](), Downloads: nullable.NewNullNullable[bool](),
		Party: nullable.NewNullNullable[api.PartyRight](), CanCreateProfiles: nullable.NewNullNullable[bool]()}
	if o.MaxQuality != nil {
		out.MaxQuality.Set(api.MaxQuality(*o.MaxQuality))
	}
	if o.MaxStreams != nil {
		out.MaxStreams.Set(*o.MaxStreams)
	}
	if o.UnlimitedStreams != nil {
		out.UnlimitedStreams.Set(*o.UnlimitedStreams)
	}
	if o.Downloads != nil {
		out.Downloads.Set(*o.Downloads)
	}
	if o.Party != nil {
		out.Party.Set(api.PartyRight(*o.Party))
	}
	if o.CanCreateProfiles != nil {
		out.CanCreateProfiles.Set(*o.CanCreateProfiles)
	}
	return out
}

func toAccess(a accounts.Access) api.EffectiveAccess {
	out := api.EffectiveAccess{Rights: toRights(a.Rights), LibraryIds: nullable.NewNullNullable[[]string]()}
	if a.LibraryIDs != nil {
		out.LibraryIds.Set(a.LibraryIDs)
	}
	return out
}

func toGroup(g accounts.Group) api.Group {
	return api.Group{Id: g.ID, Name: g.Name, Household: g.Household, LibraryIds: g.LibraryIDs, MemberIds: g.MemberIDs,
		Rights: toRights(g.Rights)}
}

func toInvitation(inv accounts.Invitation) api.Invitation {
	return api.Invitation{Id: inv.ID, Name: inv.Name, GroupIds: inv.GroupIDs, CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt}
}
