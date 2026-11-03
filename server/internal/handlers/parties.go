package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
	"github.com/gh-prim/plaid/server/internal/party"
)

// Plaid Party (F010): rooms, invitations, guests and votes. The live part
// runs over the WebSocket (api/events.md).

func (h *Handlers) partySvc() (*party.Service, error) {
	if h.party == nil {
		return nil, apierr.New(501, "not_implemented", "Plaid Party isn't available on this server")
	}
	return h.party, nil
}

// partyLink is <public base>/p/<code> (R2): the getplaid.direct address
// when remote access is on, else the address the caller is using.
func (h *Handlers) partyLink(ctx context.Context, code string) string {
	base := ""
	if h.remote != nil {
		base = strings.TrimRight(h.remote.RemoteURL(), "/")
	}
	if base == "" {
		info := clientinfo.FromContext(ctx)
		scheme := "http"
		if info.HTTPS {
			scheme = "https"
		}
		if info.Host != "" {
			base = scheme + "://" + info.Host
		}
	}
	return base + "/p/" + code
}

func toPartyItem(it party.Item) api.PartyItem {
	out := api.PartyItem{Id: it.ID, Kind: api.PartyItemKind(it.Kind), Title: it.Title}
	if it.BackdropID != "" {
		out.BackdropId = &it.BackdropID
	}
	if it.Year != 0 {
		out.Year = &it.Year
	}
	if it.RuntimeMs != 0 {
		out.RuntimeMs = &it.RuntimeMs
	}
	if it.ShowID != "" {
		out.ShowId, out.ShowTitle = &it.ShowID, &it.ShowTitle
		out.Season, out.Episode = &it.Season, &it.Episode
	}
	return out
}

func toPartyPerson(userID, name string, pr accounts.Profile) api.PartyPerson {
	return api.PartyPerson{UserId: userID, Name: name, AvatarColor: pr.AvatarColor, Avatar: toAvatar(pr)}
}

// people returns every account's display name and default profile.
func (h *Handlers) people(ctx context.Context) (map[string]string, map[string]accounts.Profile, error) {
	users, err := h.accounts.ListUsers(ctx)
	if err != nil {
		return nil, nil, err
	}
	names := map[string]string{}
	for _, u := range users {
		names[u.ID] = u.DisplayName
	}
	profiles, err := h.accounts.DefaultProfiles(ctx)
	return names, profiles, err
}

func (h *Handlers) toParty(ctx context.Context, svc *party.Service, p auth.Principal, pt party.Party) (api.Party, error) {
	names, profiles, err := h.people(ctx)
	if err != nil {
		return api.Party{}, err
	}
	out := api.Party{
		Id: pt.ID, Code: party.FormatCode(pt.Code), Link: h.partyLink(ctx, pt.Code), Status: api.PartyStatus(pt.Status),
		ControlMode: api.PartyControlMode(pt.ControlMode), CreatedAt: pt.CreatedAt,
		Organizer:   toPartyPerson(pt.OrganizerUserID, names[pt.OrganizerUserID], profiles[pt.OrganizerUserID]),
		ScheduledAt: pt.ScheduledAt, StartedAt: pt.StartedAt, EndedAt: pt.EndedAt, Invites: []api.PartyInvite{},
	}
	if pt.Note != "" {
		out.Note = &pt.Note
	}
	if pt.ScheduledAt != nil {
		on := !pt.NoReminder
		out.Reminder = &on
	}
	if pt.ItemID != "" {
		if it, err := svc.Store().Item(ctx, pt.ItemID); err == nil {
			item := toPartyItem(it)
			out.Item = &item
		}
	}
	if pt.FutureShowID != "" {
		out.FutureEpisode = &struct {
			Episode int    `json:"episode"`
			Season  int    `json:"season"`
			ShowId  string `json:"showId"` //nolint:revive // the generated type's field
		}{Episode: pt.FutureEpisode, Season: pt.FutureSeason, ShowId: pt.FutureShowID}
	}
	// A guest sees the party, not who else was invited.
	if !p.IsGuest() {
		invites, err := svc.Invites(ctx, pt.ID)
		if err != nil {
			return api.Party{}, err
		}
		for _, in := range invites {
			out.Invites = append(out.Invites, api.PartyInvite{Answer: api.PartyInviteAnswer(in.Answer),
				Person: toPartyPerson(in.UserID, names[in.UserID], profiles[in.UserID])})
			if in.UserID == p.UserID {
				a := api.PartyMyAnswer(in.Answer)
				out.MyAnswer = &a
			}
		}
	}
	if r, ok := svc.Room(pt.ID); ok {
		out.Live = &struct {
			Count      int    `json:"count"`
			Max        int    `json:"max"`
			PositionMs int64  `json:"positionMs"`
			State      string `json:"state"`
		}{Count: r.Count, Max: r.Max, PositionMs: r.Playback.PositionMs, State: r.Playback.State}
	}
	if v, err := svc.Store().Vote(ctx, pt.ID); err == nil {
		out.Vote = &struct {
			Closed         bool      `json:"closed"`
			ProposalsUntil time.Time `json:"proposalsUntil"`
			RankingUntil   time.Time `json:"rankingUntil"`
		}{Closed: v.ClosedAt != nil, ProposalsUntil: v.ProposalsUntil, RankingUntil: v.RankingUntil}
	}
	if sum, ok := svc.Summary(pt); ok {
		s := api.PartySummary{People: sum.People, TogetherMs: sum.TogetherMs, Reactions: sum.Reactions,
			PauseRequests: sum.PauseRequests}
		if sum.PeakEmoji != "" {
			s.PeakEmoji, s.PeakPositionMs = &sum.PeakEmoji, &sum.PeakMinuteMs
		}
		out.Summary = &s
	}
	return out, nil
}

// ListParties lists the caller's parties.
func (h *Handlers) ListParties(ctx context.Context, req api.ListPartiesRequestObject) (api.ListPartiesResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	all := req.Params.All != nil && *req.Params.All
	list, err := svc.List(ctx, p, all)
	if err != nil {
		return nil, err
	}
	out := api.ListParties200JSONResponse{}
	for _, pt := range list {
		x, err := h.toParty(ctx, svc, p, pt)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, nil
}

// CreateParty starts or schedules a party.
func (h *Handlers) CreateParty(ctx context.Context, req api.CreatePartyRequestObject) (api.CreatePartyResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	in := party.NewParty{ItemID: deref(b.ItemId), Note: deref(b.Note), ScheduledAt: b.ScheduledAt,
		Invite: derefSlice(b.Invite), ShowID: deref(b.ShowId), Season: derefInt(b.Season), Episode: derefInt(b.Episode)}
	if b.ControlMode != nil {
		in.ControlMode = string(*b.ControlMode)
	}
	if b.Reminder != nil {
		in.NoReminder = !*b.Reminder
	}
	if b.Vote != nil {
		in.Vote = &party.VoteDeadlines{ProposalsUntil: b.Vote.ProposalsUntil, RankingUntil: b.Vote.RankingUntil}
	}
	pt, err := svc.Create(ctx, p, in)
	if err != nil {
		return nil, err
	}
	out, err := h.toParty(ctx, svc, p, pt)
	if err != nil {
		return nil, err
	}
	return api.CreateParty201JSONResponse(out), nil
}

// ListPartyPeople lists who the caller can invite.
func (h *Handlers) ListPartyPeople(ctx context.Context, _ api.ListPartyPeopleRequestObject) (api.ListPartyPeopleResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	people, err := svc.People(ctx, p)
	if err != nil {
		return nil, err
	}
	out := api.ListPartyPeople200JSONResponse{}
	for _, x := range people {
		out = append(out, toPartyPerson(x.UserID, x.Name, x.Profile))
	}
	return out, nil
}

// LookupPartyCode is the public join page's preview.
func (h *Handlers) LookupPartyCode(ctx context.Context, req api.LookupPartyCodeRequestObject) (api.LookupPartyCodeResponseObject, error) {
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	pv, err := svc.LookupCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	out := api.LookupPartyCode200JSONResponse{Code: party.FormatCode(pv.Party.Code), Status: api.PartyPreviewStatus(pv.Party.Status),
		HostName: pv.HostName, Watching: pv.Watching, Full: pv.Full, Guests: pv.Guests,
		StartedAt: pv.StartedAt, ScheduledAt: pv.Party.ScheduledAt}
	if pv.Item.ID != "" {
		it := toPartyItem(pv.Item)
		out.Item = &it
	}
	return out, nil
}

// JoinPartyAsGuest lets someone without an account into a party.
func (h *Handlers) JoinPartyAsGuest(ctx context.Context, req api.JoinPartyAsGuestRequestObject) (api.JoinPartyAsGuestResponseObject, error) {
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	g, token, err := svc.JoinAsGuest(ctx, req.Code, b.FirstName)
	if err != nil {
		return nil, err
	}
	out := api.JoinPartyAsGuest201JSONResponse{PartyId: g.PartyID, GuestId: g.ID, FirstName: g.FirstName}
	if auth.IsWebClient(ctx) {
		auth.SetGuestCookie(ctx, token)
	} else {
		out.Token = &token
	}
	return out, nil
}

// GetParty returns one party.
func (h *Handlers) GetParty(ctx context.Context, req api.GetPartyRequestObject) (api.GetPartyResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	pt, err := svc.Get(ctx, p, req.PartyId)
	if err != nil {
		return nil, err
	}
	out, err := h.toParty(ctx, svc, p, pt)
	if err != nil {
		return nil, err
	}
	return api.GetParty200JSONResponse(out), nil
}

// UpdateParty changes a party.
func (h *Handlers) UpdateParty(ctx context.Context, req api.UpdatePartyRequestObject) (api.UpdatePartyResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	in := party.Patch{ScheduledAt: b.ScheduledAt, Note: b.Note, ItemID: b.ItemId}
	if b.ControlMode != nil {
		m := string(*b.ControlMode)
		in.ControlMode = &m
	}
	pt, err := svc.Update(ctx, p, req.PartyId, in)
	if err != nil {
		return nil, err
	}
	out, err := h.toParty(ctx, svc, p, pt)
	if err != nil {
		return nil, err
	}
	return api.UpdateParty200JSONResponse(out), nil
}

// CancelParty cancels or ends a party.
func (h *Handlers) CancelParty(ctx context.Context, req api.CancelPartyRequestObject) (api.CancelPartyResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	if err := svc.Cancel(ctx, p, req.PartyId); err != nil {
		return nil, err
	}
	return api.CancelParty204Response{}, nil
}

// InviteToParty invites more people.
func (h *Handlers) InviteToParty(ctx context.Context, req api.InviteToPartyRequestObject) (api.InviteToPartyResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	if err := svc.InviteMore(ctx, p, req.PartyId, b.UserIds); err != nil {
		return nil, err
	}
	return api.InviteToParty204Response{}, nil
}

// AnswerParty records an answer.
func (h *Handlers) AnswerParty(ctx context.Context, req api.AnswerPartyRequestObject) (api.AnswerPartyResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	if err := svc.Answer(ctx, p, req.PartyId, string(b.Answer)); err != nil {
		return nil, err
	}
	return api.AnswerParty204Response{}, nil
}

func toPartyVote(v party.VoteView) api.PartyVote {
	out := api.PartyVote{ProposalsUntil: v.Vote.ProposalsUntil, RankingUntil: v.Vote.RankingUntil,
		Phase: api.PartyVotePhase(v.Phase), MyRanking: v.Mine.Ranks}
	if out.MyRanking == nil {
		out.MyRanking = []string{}
	}
	if v.Mine.Veto != "" {
		out.MyVeto = &v.Mine.Veto
	}
	out.Proposals = []struct {
		Item       api.PartyItem `json:"item"`
		Mine       bool          `json:"mine"`
		ProposedBy string        `json:"proposedBy"`
	}{}
	for _, pr := range v.Proposals {
		out.Proposals = append(out.Proposals, struct {
			Item       api.PartyItem `json:"item"`
			Mine       bool          `json:"mine"`
			ProposedBy string        `json:"proposedBy"`
		}{Item: toPartyItem(pr.Item), Mine: pr.Mine, ProposedBy: pr.ProposedBy})
	}
	out.Voters = []struct {
		Name  string `json:"name"`
		Voted bool   `json:"voted"`
	}{}
	for _, vt := range v.Voters {
		out.Voters = append(out.Voters, struct {
			Name  string `json:"name"`
			Voted bool   `json:"voted"`
		}{Name: vt.Name, Voted: vt.Voted})
	}
	if r := v.Result; r != nil {
		res := &struct {
			Scores []struct {
				ItemId string `json:"itemId"` //nolint:revive // the generated type's field
				Points int    `json:"points"`
				Vetoed bool   `json:"vetoed"`
			} `json:"scores"`
			Seed   *string   `json:"seed,omitempty"`
			Tied   *[]string `json:"tied,omitempty"`
			Winner *string   `json:"winner,omitempty"`
		}{}
		for _, sc := range r.Scores {
			res.Scores = append(res.Scores, struct {
				ItemId string `json:"itemId"` //nolint:revive // the generated type's field
				Points int    `json:"points"`
				Vetoed bool   `json:"vetoed"`
			}{ItemId: sc.ItemID, Points: sc.Points, Vetoed: sc.Vetoed})
		}
		if len(r.Tied) > 0 {
			res.Tied, res.Seed = &r.Tied, &r.Seed
		}
		if r.Winner != "" {
			res.Winner = &r.Winner
		}
		out.Result = res
	}
	return out
}

// GetPartyVote returns a vote.
func (h *Handlers) GetPartyVote(ctx context.Context, req api.GetPartyVoteRequestObject) (api.GetPartyVoteResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	v, err := svc.VoteState(ctx, p, req.PartyId)
	if err != nil {
		return nil, err
	}
	return api.GetPartyVote200JSONResponse(toPartyVote(v)), nil
}

// ProposeForParty proposes a title.
func (h *Handlers) ProposeForParty(ctx context.Context, req api.ProposeForPartyRequestObject) (api.ProposeForPartyResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	v, err := svc.Propose(ctx, p, req.PartyId, req.ItemId)
	if err != nil {
		return nil, err
	}
	return api.ProposeForParty200JSONResponse(toPartyVote(v)), nil
}

// WithdrawPartyProposal withdraws a proposal.
func (h *Handlers) WithdrawPartyProposal(ctx context.Context, req api.WithdrawPartyProposalRequestObject) (api.WithdrawPartyProposalResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	v, err := svc.Withdraw(ctx, p, req.PartyId, req.ItemId)
	if err != nil {
		return nil, err
	}
	return api.WithdrawPartyProposal200JSONResponse(toPartyVote(v)), nil
}

// CastPartyBallot saves a ballot.
func (h *Handlers) CastPartyBallot(ctx context.Context, req api.CastPartyBallotRequestObject) (api.CastPartyBallotResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	veto := ""
	if set, v := fromNullable(b.Veto); set && v != nil {
		veto = *v
	}
	v, err := svc.Cast(ctx, p, req.PartyId, b.Ranking, veto)
	if err != nil {
		return nil, err
	}
	return api.CastPartyBallot200JSONResponse(toPartyVote(v)), nil
}

// PartyVoteSuggestions suggests titles nobody has watched.
func (h *Handlers) PartyVoteSuggestions(ctx context.Context, req api.PartyVoteSuggestionsRequestObject) (api.PartyVoteSuggestionsResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	items, err := svc.Suggestions(ctx, p, req.PartyId)
	if err != nil {
		return nil, err
	}
	out := api.PartyVoteSuggestions200JSONResponse{}
	for _, it := range items {
		out = append(out, toPartyItem(it))
	}
	return out, nil
}
