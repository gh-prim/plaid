package handlers

import (
	"context"
	"errors"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/playback"
)

// GetServerCapabilities reports ffprobe, ffmpeg and file watching
// (F004-R18, R25, F006 R24, R31).
func (h *Handlers) GetServerCapabilities(ctx context.Context, _ api.GetServerCapabilitiesRequestObject) (api.GetServerCapabilitiesResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	c, err := h.library.Capabilities(ctx)
	if err != nil {
		return nil, err
	}
	out := api.Capabilities{InotifyLimit: c.InotifyLimit, InotifyUsed: c.InotifyUsed, WatchLimitReached: c.OverLimit}
	if out.WatchLimitReached == nil {
		out.WatchLimitReached = []string{}
	}
	out.Ffprobe.Found = c.FFprobeFound
	out.Ffprobe.Version = optional(c.FFprobeVersion)
	out.HwAccel = toHardwareStatus(playback.HWStatus{Configured: playback.HWNone})
	if h.playback != nil {
		if f := h.playback.FFmpeg(); f != nil {
			out.Ffmpeg.Found, out.Ffmpeg.Version = true, optional(f.Version)
		}
		out.HwAccel = toHardwareStatus(h.playback.Hardware())
	}
	return api.GetServerCapabilities200JSONResponse(out), nil
}

// TestServerHardware runs the hardware test again (F007 R28).
func (h *Handlers) TestServerHardware(ctx context.Context, _ api.TestServerHardwareRequestObject) (api.TestServerHardwareResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if h.playback == nil {
		return api.TestServerHardware200JSONResponse(toHardwareStatus(playback.HWStatus{Configured: playback.HWNone})), nil
	}
	st, err := h.playback.RetestHardware(ctx)
	if errors.Is(err, playback.ErrHWTestRunning) {
		return nil, apierr.HWTestRunning()
	}
	if err != nil {
		return nil, err
	}
	return api.TestServerHardware200JSONResponse(toHardwareStatus(st)), nil
}
