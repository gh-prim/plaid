package handlers

import (
	"context"
	"slices"

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
