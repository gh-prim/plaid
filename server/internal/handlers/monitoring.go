package handlers

import (
	"context"
	"sort"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/library"
)

// GetMonitoring returns the data sent per day and per person, where remote
// plays came from and each device's health (admin monitoring).
func (h *Handlers) GetMonitoring(ctx context.Context, req api.GetMonitoringRequestObject) (api.GetMonitoringResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	days, offset := 30, 0
	if d := req.Params.Days; d != nil && *d >= 1 && *d <= 366 {
		days = *d
	}
	if o := req.Params.TzOffsetMinutes; o != nil && *o >= -840 && *o <= 840 {
		offset = *o
	}
	out := api.Monitoring{Days: []api.MonitoringDay{}, People: []api.MonitoringPerson{}, Places: []api.MonitoringPlace{},
		Devices: []api.DeviceHealth{}}
	if h.playback == nil {
		return api.GetMonitoring200JSONResponse(out), nil
	}
	m, err := h.playback.Monitor(ctx, days, offset)
	if err != nil {
		return nil, err
	}
	out.Local, out.Remote, out.Unplaced, out.GeoipReady = m.Local, m.Remote, m.Unplaced, h.playback.GeoReady()
	for _, d := range m.Days {
		date, _ := time.Parse("2006-01-02", d.Date)
		out.Days = append(out.Days, api.MonitoringDay{Date: openapi_types.Date{Time: date}, Bytes: d.Bytes})
	}
	for _, p := range m.People {
		person := api.MonitoringPerson{DisplayName: p.DisplayName, Bytes: p.Bytes, Plays: p.Plays, WatchedMs: p.WatchedMs}
		if p.UserID != "" {
			person.UserId = &p.UserID
		}
		out.People = append(out.People, person)
	}
	for _, p := range m.Places {
		place := api.MonitoringPlace{Country: p.Country, Lat: p.Lat, Lon: p.Lon, Plays: p.Plays, People: p.People, LastAt: p.Last}
		if p.City != "" {
			place.City = &p.City
		}
		out.Places = append(out.Places, place)
	}
	for _, d := range m.Devices {
		out.Devices = append(out.Devices, api.DeviceHealth{DeviceName: d.DeviceName, DisplayName: d.DisplayName, Plays: d.Plays,
			DirectPlay: d.DirectPlay, DirectStream: d.DirectStream, Transcoded: d.Transcoded, CodecIssues: d.CodecIssues,
			Score: d.Score, LastAt: d.Last})
	}
	return api.GetMonitoring200JSONResponse(out), nil
}

// Report filters: stale is added more than 6 months ago and not played for
// a year.
const (
	staleAdded  = 182 * 24 * time.Hour
	stalePlayed = 365 * 24 * time.Hour
)

func stale(it library.ReportItem, now time.Time) bool {
	return now.Sub(it.AddedAt) > staleAdded && (it.LastPlayed == nil || now.Sub(*it.LastPlayed) > stalePlayed)
}

// qualityOf classes a picture by its lines.
func qualityOf(height int) string {
	switch {
	case height >= 1600:
		return "uhd"
	case height >= 900:
		return "fhd"
	case height >= 600:
		return "hd"
	case height > 0:
		return "sd"
	}
	return ""
}

// GetLibraryReport lists movies and shows with their size, best picture,
// files and plays, filtered and sorted, a page at a time (admin).
func (h *Handlers) GetLibraryReport(ctx context.Context, req api.GetLibraryReportRequestObject) (api.GetLibraryReportResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := req.Params
	items, genres, err := h.library.Store().Report(ctx, value(q.LibraryId))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := api.LibraryReport{Genres: []api.ReportGenre{}, Items: []api.LibraryReportItem{}}
	for _, g := range genres {
		out.Genres = append(out.Genres, api.ReportGenre{Id: g.ID, Name: g.Name})
	}
	var keep []library.ReportItem
	for _, it := range items {
		t := &out.Totals
		t.Titles++
		t.Bytes += it.Bytes
		if it.Plays == 0 {
			t.Unwatched++
			t.UnwatchedBytes += it.Bytes
		}
		if stale(it, now) {
			t.Stale++
			t.StaleBytes += it.Bytes
		}
		if it.Duplicate {
			t.Duplicates++
			t.DuplicateBytes += it.Bytes
		}
		switch value(q.Show) {
		case api.GetLibraryReportParamsShowUnwatched:
			if it.Plays > 0 {
				continue
			}
		case api.GetLibraryReportParamsShowStale:
			if !stale(it, now) {
				continue
			}
		case api.GetLibraryReportParamsShowDuplicates:
			if !it.Duplicate {
				continue
			}
		}
		if q.Quality != nil && qualityOf(it.Height) != string(*q.Quality) {
			continue
		}
		if q.GenreId != nil && !containsInt(it.GenreIDs, *q.GenreId) {
			continue
		}
		keep = append(keep, it)
	}
	sortReport(keep, value(q.Sort))
	out.Total = len(keep)
	offset, limit := value(q.Offset), 60
	if q.Limit != nil {
		limit = *q.Limit
	}
	if offset > len(keep) {
		offset = len(keep)
	}
	page := keep[offset:min(offset+limit, len(keep))]
	ids := make([]string, len(page))
	for i, it := range page {
		ids[i] = it.ID
	}
	sums, err := h.summariesOf(ctx, ids, access.Limit{})
	if err != nil {
		return nil, err
	}
	byID := map[string]api.ItemSummary{}
	for _, s := range sums {
		byID[s.Id] = s
	}
	for _, it := range page {
		sum, ok := byID[it.ID]
		if !ok {
			continue
		}
		out.Items = append(out.Items, api.LibraryReportItem{Item: sum, Bytes: it.Bytes, Files: it.Files, Height: it.Height,
			Duplicate: it.Duplicate, Plays: it.Plays, LastPlayedAt: it.LastPlayed, AddedAt: it.AddedAt})
	}
	return api.GetLibraryReport200JSONResponse(out), nil
}

func sortReport(items []library.ReportItem, by api.GetLibraryReportParamsSort) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch by {
		case api.GetLibraryReportParamsSortAdded:
			return a.AddedAt.After(b.AddedAt)
		case api.GetLibraryReportParamsSortTitle:
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		case api.GetLibraryReportParamsSortPlays:
			return a.Plays > b.Plays || a.Plays == b.Plays && a.Bytes > b.Bytes
		}
		return a.Bytes > b.Bytes
	})
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
