package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/remoteaccess"
)

// remoteAccessOff answers when remote access isn't wired in (tests) or
// has nothing to test yet.
func remoteAccessOff() *apierr.Error {
	return apierr.New(http.StatusConflict, "remote_access_off", "remote access is off or not registered yet")
}

// GetRemoteAccess returns remote access settings and status (admin, F012).
func (h *Handlers) GetRemoteAccess(ctx context.Context, _ api.GetRemoteAccessRequestObject) (api.GetRemoteAccessResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if h.remote == nil {
		return nil, remoteAccessOff()
	}
	return api.GetRemoteAccess200JSONResponse(toRemoteAccess(h.remote.Status())), nil
}

// UpdateRemoteAccess turns remote access on or off (admin, F012).
func (h *Handlers) UpdateRemoteAccess(ctx context.Context, req api.UpdateRemoteAccessRequestObject) (api.UpdateRemoteAccessResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if h.remote == nil {
		return nil, remoteAccessOff()
	}
	u := remoteaccess.Update{Enabled: req.Body.Enabled, ManualPort: req.Body.ManualPort}
	if req.Body.PortMode != nil {
		m := string(*req.Body.PortMode)
		u.PortMode = &m
	}
	st, err := h.remote.Update(ctx, u)
	if errors.Is(err, remoteaccess.ErrInvalidManualPort) {
		return nil, apierr.ValidationFailed(apierr.Detail{Field: "manualPort", Code: "required"})
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateRemoteAccess200JSONResponse(toRemoteAccess(st)), nil
}

// TestRemoteAccess checks the server from outside (admin, F012).
func (h *Handlers) TestRemoteAccess(ctx context.Context, _ api.TestRemoteAccessRequestObject) (api.TestRemoteAccessResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if h.remote == nil {
		return nil, remoteAccessOff()
	}
	st, err := h.remote.Test(ctx)
	if errors.Is(err, remoteaccess.ErrOff) {
		return nil, remoteAccessOff()
	}
	if err != nil {
		return nil, err
	}
	return api.TestRemoteAccess200JSONResponse(toRemoteAccess(st)), nil
}

func toRemoteAccess(s remoteaccess.Status) api.RemoteAccess {
	out := api.RemoteAccess{
		Enabled: s.Enabled, PortMode: api.RemotePortMode(s.PortMode), InternalPort: s.InternalPort,
		State: api.RemoteAccessState(s.State), PortMapping: toRemoteStep(s.PortMapping),
		Certificate: toRemoteStep(s.Certificate), Reachability: toRemoteStep(s.Reachability),
		PublicIp: optional(s.PublicIP), Hostname: optional(s.Hostname), Url: optional(s.URL), ShortUrl: optional(s.ShortURL),
	}
	if s.ManualPort != 0 {
		out.ManualPort = &s.ManualPort
	}
	if s.ExternalPort != 0 {
		out.ExternalPort = &s.ExternalPort
	}
	return out
}

func toRemoteStep(s remoteaccess.Step) api.RemoteStep {
	out := api.RemoteStep{State: api.RemoteStepState(s.State), Method: optional(s.Method), Error: optional(s.Error)}
	if !s.At.IsZero() {
		at := s.At.UTC().Truncate(time.Second)
		out.At = &at
	}
	return out
}
