package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/api"
)

// StartPairing gives a TV a code to show.
func (h *Handlers) StartPairing(ctx context.Context, req api.StartPairingRequestObject) (api.StartPairingResponseObject, error) {
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	pr, err := h.accounts.StartPairing(ctx, toDevice(b.Device))
	if err != nil {
		return nil, err
	}
	return api.StartPairing201JSONResponse{Id: pr.ID, Code: pr.Code, PollSecret: pr.PollSecret, ExpiresAt: pr.ExpiresAt}, nil
}

// PollPairing hands the TV its session once someone claimed the code.
func (h *Handlers) PollPairing(ctx context.Context, req api.PollPairingRequestObject) (api.PollPairingResponseObject, error) {
	res, pending, err := h.accounts.PollPairing(ctx, req.PairingId, req.Params.XPairingSecret)
	if err != nil {
		return nil, err
	}
	if pending {
		return api.PollPairing202JSONResponse{Status: api.PairingPendingStatusPending}, nil
	}
	return api.PollPairing200JSONResponse(toAuthResult(ctx, res, false)), nil
}

// ClaimPairing signs the TV showing a code in.
func (h *Handlers) ClaimPairing(ctx context.Context, req api.ClaimPairingRequestObject) (api.ClaimPairingResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	sess, err := h.accounts.ClaimPairing(ctx, p, b.Code, b.ProfileId, deref(b.Pin))
	if err != nil {
		return nil, err
	}
	return api.ClaimPairing200JSONResponse{
		Id: sess.ID, DeviceName: sess.DeviceName, DeviceType: api.DeviceType(sess.DeviceType),
	}, nil
}
