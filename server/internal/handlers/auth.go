package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/buildinfo"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
)

// GetServerInfo lets clients check they reached a PLAID server (F003-R21).
func (h *Handlers) GetServerInfo(ctx context.Context, _ api.GetServerInfoRequestObject) (api.GetServerInfoResponseObject, error) {
	done, err := h.accounts.SetupComplete(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetServerInfo200JSONResponse{Name: h.serverName, Version: buildinfo.Version, SetupComplete: done}, nil
}

// GetSetup reports whether first-run setup is still open (F003-R2, R3).
func (h *Handlers) GetSetup(ctx context.Context, _ api.GetSetupRequestObject) (api.GetSetupResponseObject, error) {
	done, err := h.accounts.SetupComplete(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetSetup200JSONResponse{
		SetupComplete:     done,
		RequiresSetupCode: !done && accounts.RequiresSetupCode(clientinfo.FromContext(ctx)),
	}, nil
}

// CreateFirstAdmin creates the first admin and signs them in.
func (h *Handlers) CreateFirstAdmin(ctx context.Context, req api.CreateFirstAdminRequestObject) (api.CreateFirstAdminResponseObject, error) {
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	in := accounts.SetupInput{
		NewUser:   accounts.NewUser{DisplayName: b.DisplayName, Username: b.Username, Password: b.Password, Locale: deref(b.Locale)},
		SetupCode: deref(b.SetupCode),
		Device:    accounts.Device{Name: "Web browser", Type: "web"},
	}
	if b.Device != nil {
		in.Device = toDevice(*b.Device)
	}
	res, err := h.accounts.CreateFirstAdmin(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.CreateFirstAdmin201JSONResponse(toAuthResult(ctx, res, auth.IsWebClient(ctx))), nil
}

// Login signs in with username and password.
func (h *Handlers) Login(ctx context.Context, req api.LoginRequestObject) (api.LoginResponseObject, error) {
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	res, err := h.accounts.Login(ctx, b.Username, b.Password, toDevice(b.Device))
	if err != nil {
		return nil, err
	}
	return api.Login200JSONResponse(toAuthResult(ctx, res, auth.IsWebClient(ctx))), nil
}

// Logout ends the current session.
func (h *Handlers) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.accounts.Logout(ctx, p); err != nil {
		return nil, err
	}
	if auth.ViaCookie(ctx) {
		auth.ClearSessionCookie(ctx)
	}
	return api.Logout204Response{}, nil
}
