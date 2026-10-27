package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/api"
)

// GetServerCapabilities reports ffprobe, ffmpeg and file watching
// (F004-R18, R25, F006 R31).
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
	if h.playback != nil {
		if f := h.playback.FFmpeg(); f != nil {
			out.Ffmpeg.Found, out.Ffmpeg.Version = true, optional(f.Version)
		}
	}
	return api.GetServerCapabilities200JSONResponse(out), nil
}
