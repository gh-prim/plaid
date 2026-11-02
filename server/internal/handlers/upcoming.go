package handlers

import (
	"context"
	"time"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/party"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// The Upcoming tab (F010-R35–R39).

// upcomingAhead is the default range of the tab.
const upcomingAhead = 60 * 24 * time.Hour

// GetUpcoming returns the tab for the active profile.
func (h *Handlers) GetUpcoming(ctx context.Context, req api.GetUpcomingRequestObject) (api.GetUpcomingResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(upcomingAhead)
	if req.Params.From != nil {
		from = req.Params.From.Time
	}
	if req.Params.To != nil {
		to = req.Params.To.Add(24*time.Hour - time.Millisecond)
	}
	kind := "all"
	if req.Params.Kind != nil {
		kind = string(*req.Params.Kind)
	}
	v, err := svc.Upcoming(ctx, p, from, to, kind)
	if err != nil {
		return nil, err
	}
	out := api.GetUpcoming200JSONResponse{Invitations: []api.Party{}, Episodes: []api.UpcomingEpisode{}, Parties: []api.Party{}}
	for _, pt := range v.Invitations {
		x, err := h.toParty(ctx, svc, p, pt)
		if err != nil {
			return nil, err
		}
		out.Invitations = append(out.Invitations, x)
	}
	for _, pt := range v.Parties {
		x, err := h.toParty(ctx, svc, p, pt)
		if err != nil {
			return nil, err
		}
		out.Parties = append(out.Parties, x)
	}
	for _, e := range v.Episodes {
		day, _ := time.Parse("2006-01-02", e.AirDate)
		ep := api.UpcomingEpisode{ShowId: e.ShowID, ShowTitle: e.ShowTitle, Season: e.Season, Episode: e.Episode,
			AirDate: openapi_types.Date{Time: day}, Status: api.UpcomingEpisodeStatus(e.Status), Alert: e.Alert}
		if e.Title != "" {
			ep.Title = &e.Title
		}
		if e.ItemID != "" {
			ep.ItemId = &e.ItemID
		}
		out.Episodes = append(out.Episodes, ep)
	}
	return out, nil
}

func (h *Handlers) followState(ctx context.Context, svc *party.Service, showID string) (api.ShowFollow, error) {
	p, err := principal(ctx)
	if err != nil {
		return api.ShowFollow{}, err
	}
	explicit, followed, err := svc.Following(ctx, p, showID)
	return api.ShowFollow{Explicit: explicit, Following: followed}, err
}

// GetShowFollow says whether the profile follows a show.
func (h *Handlers) GetShowFollow(ctx context.Context, req api.GetShowFollowRequestObject) (api.GetShowFollowResponseObject, error) {
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	st, err := h.followState(ctx, svc, req.ShowId)
	if err != nil {
		return nil, err
	}
	return api.GetShowFollow200JSONResponse(st), nil
}

// FollowShow follows a show.
func (h *Handlers) FollowShow(ctx context.Context, req api.FollowShowRequestObject) (api.FollowShowResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	if err := svc.Follow(ctx, p, req.ShowId, true); err != nil {
		return nil, err
	}
	st, err := h.followState(ctx, svc, req.ShowId)
	if err != nil {
		return nil, err
	}
	return api.FollowShow200JSONResponse(st), nil
}

// UnfollowShow stops following a show.
func (h *Handlers) UnfollowShow(ctx context.Context, req api.UnfollowShowRequestObject) (api.UnfollowShowResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	if err := svc.Follow(ctx, p, req.ShowId, false); err != nil {
		return nil, err
	}
	st, err := h.followState(ctx, svc, req.ShowId)
	if err != nil {
		return nil, err
	}
	return api.UnfollowShow200JSONResponse(st), nil
}

// SetReleaseAlert turns on "Notify me".
func (h *Handlers) SetReleaseAlert(ctx context.Context, req api.SetReleaseAlertRequestObject) (api.SetReleaseAlertResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	if err := svc.Alert(ctx, p, req.ShowId, req.Season, req.Episode, true); err != nil {
		return nil, err
	}
	return api.SetReleaseAlert204Response{}, nil
}

// ClearReleaseAlert turns off "Notify me".
func (h *Handlers) ClearReleaseAlert(ctx context.Context, req api.ClearReleaseAlertRequestObject) (api.ClearReleaseAlertResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.partySvc()
	if err != nil {
		return nil, err
	}
	if err := svc.Alert(ctx, p, req.ShowId, req.Season, req.Episode, false); err != nil {
		return nil, err
	}
	return api.ClearReleaseAlert204Response{}, nil
}
