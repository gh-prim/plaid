package handlers

import (
	"context"
	"slices"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/sysstats"
)

// GetServerStats returns what the machine is doing for the admin
// dashboard (V2): load, temperature, playback and the libraries' drives.
func (h *Handlers) GetServerStats(ctx context.Context, _ api.GetServerStatsRequestObject) (api.GetServerStatsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	out := api.ServerStats{
		UptimeSeconds: int64(sysstats.Uptime().Seconds()),
		Drives:        []api.DriveStats{},
	}
	if m := sysstats.Model(); m != "" {
		out.Model = &m
	}
	if v, ok := sysstats.CPUPercent(); ok {
		v32 := float32(v)
		out.CpuPercent = &v32
	}
	if v, ok := sysstats.TemperatureC(); ok {
		v32 := float32(v)
		out.TemperatureC = &v32
	}
	if used, total, ok := sysstats.Memory(); ok {
		out.MemoryUsedBytes, out.MemoryTotalBytes = &used, &total
	}
	if h.playback != nil {
		out.Transcodes, out.MaxTranscodes, out.UploadBps = h.playback.Load()
	}
	libs, err := h.library.Store().ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	byDevice := map[uint64]int{}
	for _, l := range libs {
		for _, f := range l.Folders {
			d, ok := sysstats.DriveOf(f.Path)
			if !ok {
				out.Drives = append(out.Drives, api.DriveStats{Path: f.Path, Libraries: []string{l.Name}, Available: false})
				continue
			}
			if i, seen := byDevice[d.Device]; seen {
				if !slices.Contains(out.Drives[i].Libraries, l.Name) {
					out.Drives[i].Libraries = append(out.Drives[i].Libraries, l.Name)
				}
				continue
			}
			byDevice[d.Device] = len(out.Drives)
			total, free := d.TotalBytes, d.FreeBytes
			out.Drives = append(out.Drives, api.DriveStats{
				Path: f.Path, Libraries: []string{l.Name}, Available: true, TotalBytes: &total, FreeBytes: &free,
			})
		}
	}
	return api.GetServerStats200JSONResponse(out), nil
}

// GetServerStatsHistory returns the last day of playback samples (step 21).
func (h *Handlers) GetServerStatsHistory(ctx context.Context, _ api.GetServerStatsHistoryRequestObject) (api.GetServerStatsHistoryResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	out := api.StatsHistory{Samples: []api.StatsSample{}}
	if h.playback != nil {
		for _, s := range h.playback.History() {
			out.Samples = append(out.Samples, api.StatsSample{At: s.At, Streams: s.Streams, Transcodes: s.Transcodes, UploadBps: s.UploadBps})
		}
	}
	return api.GetServerStatsHistory200JSONResponse(out), nil
}

// GetActivity returns the hours watched per day, the last plays and the
// last titles added (step 21).
func (h *Handlers) GetActivity(ctx context.Context, req api.GetActivityRequestObject) (api.GetActivityResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	days, offset := 7, 0
	if d := req.Params.Days; d != nil && *d >= 1 && *d <= 90 {
		days = *d
	}
	if o := req.Params.TzOffsetMinutes; o != nil && *o >= -840 && *o <= 840 {
		offset = *o
	}
	out := api.Activity{Days: []api.ActivityDay{}, Recent: []api.PlayRecord{}, RecentlyAdded: []api.ItemSummary{}}
	if h.playback != nil {
		perDay, plays, err := h.playback.Activity(ctx, days, offset)
		if err != nil {
			return nil, err
		}
		for _, d := range perDay {
			date, _ := time.Parse("2006-01-02", d.Date)
			out.Days = append(out.Days, api.ActivityDay{Date: openapi_types.Date{Time: date}, Hours: float32(d.Hours), Plays: d.Plays})
		}
		for _, p := range plays {
			out.Recent = append(out.Recent, api.PlayRecord{ItemId: p.ItemID, Title: p.Title, UserId: p.UserID,
				DisplayName: p.DisplayName, ProfileName: p.ProfileName, DeviceName: p.DeviceName,
				Method: api.PlayRecordMethod(p.Method), Remote: p.Remote, StartedAt: p.StartedAt, WatchedMs: p.WatchedMs})
		}
	}
	seen, err := h.accounts.LastSeen(ctx)
	if err != nil {
		return nil, err
	}
	since := time.Now().AddDate(0, 0, -days).UTC().Format("2006-01-02T15:04:05.000Z")
	for _, v := range seen {
		if v.At >= since {
			out.ActivePeople++
		}
	}
	ids, err := h.library.Store().RecentlyAdded(ctx, 10)
	if err != nil {
		return nil, err
	}
	sums, err := h.summariesOf(ctx, ids, access.Limit{})
	if err != nil {
		return nil, err
	}
	out.RecentlyAdded = sums
	return api.GetActivity200JSONResponse(out), nil
}
