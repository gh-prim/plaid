package handlers

import (
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/library"
)

var itemKinds = map[api.ItemKind]bool{api.Movie: true, api.Show: true, api.Season: true, api.Episode: true}

func checkLimit(limit *int, p *problems) int {
	if limit == nil {
		return 0
	}
	if *limit < 1 || *limit > library.MaxPageSize {
		p.add("limit", "out_of_range")
	}
	return *limit
}

// ListLibraryItems lists a library's items by kind and parent (F004-R22).
func (h *Handlers) ListLibraryItems(ctx context.Context, req api.ListLibraryItemsRequestObject) (api.ListLibraryItemsResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	var p problems
	q := library.ItemQuery{
		LibraryID: req.LibraryId, ParentID: deref(req.Params.ParentId),
		Cursor: deref(req.Params.Cursor), Limit: checkLimit(req.Params.Limit, &p),
		Access: limit,
	}
	if k := req.Params.Kind; k != nil {
		if !itemKinds[*k] {
			p.add("kind", "invalid")
		}
		q.Kind = string(*k)
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	items, next, err := h.library.Store().ListItems(ctx, q)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	out := api.ItemPage{Items: make([]api.ItemSummary, 0, len(items)), NextCursor: optional(next)}
	for _, it := range items {
		out.Items = append(out.Items, toItemSummary(it))
	}
	return api.ListLibraryItems200JSONResponse(out), nil
}

// GetItem returns an item with its files and streams.
func (h *Handlers) GetItem(ctx context.Context, req api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	st := h.library.Store()
	it, err := st.GetItem(ctx, req.ItemId)
	// An item the profile may not see doesn't exist for it (F005 R25).
	if errors.Is(err, library.ErrNotFound) || err == nil && !limit.Allows(it.AgeRating) {
		return nil, apierr.NotFound("no such item")
	}
	if err != nil {
		return nil, err
	}
	files, err := st.ItemFiles(ctx, it.ID)
	if err != nil {
		return nil, err
	}
	s := toItemSummary(it)
	out := api.ItemDetail{
		Id: s.Id, Kind: s.Kind, ParentId: s.ParentId, Title: s.Title, Year: s.Year,
		SeasonNumber: s.SeasonNumber, EpisodeNumber: s.EpisodeNumber, AirDate: s.AirDate, Hints: s.Hints,
		FileCount: s.FileCount, ChildCount: s.ChildCount, Technical: s.Technical,
		Files: make([]api.MediaFile, 0, len(files)),
	}
	for _, f := range files {
		out.Files = append(out.Files, toMediaFile(f))
	}
	return api.GetItem200JSONResponse(out), nil
}

// ListUnrecognizedFiles lists files whose name couldn't be parsed (F004-R11).
func (h *Handlers) ListUnrecognizedFiles(ctx context.Context, req api.ListUnrecognizedFilesRequestObject) (api.ListUnrecognizedFilesResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var p problems
	limit := checkLimit(req.Params.Limit, &p)
	if err := p.err(); err != nil {
		return nil, err
	}
	files, next, err := h.library.Store().ListUnrecognized(ctx, req.LibraryId, deref(req.Params.Cursor), limit)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	out := api.UnrecognizedPage{Files: make([]api.UnrecognizedFile, 0, len(files)), NextCursor: optional(next)}
	for _, f := range files {
		out.Files = append(out.Files, api.UnrecognizedFile{
			Id: f.ID, Path: f.Path, Size: f.Size, ProbeStatus: api.UnrecognizedFileProbeStatus(f.ProbeStatus),
		})
	}
	return api.ListUnrecognizedFiles200JSONResponse(out), nil
}

func optionalInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

func optionalInt64(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}

func toItemSummary(it library.Item) api.ItemSummary {
	out := api.ItemSummary{
		Id: it.ID, Kind: api.ItemKind(it.Kind), ParentId: optional(it.ParentID), Title: optional(it.Title),
		Year: optionalInt(it.Year), SeasonNumber: it.Season, EpisodeNumber: it.Episode,
		FileCount: it.FileCount, ChildCount: it.ChildCount,
		Technical: api.TechnicalSummary{
			HdrFormats:     make([]api.TechnicalSummaryHdrFormats, 0, len(it.Technical.HDRFormats)),
			AudioLanguages: it.Technical.AudioLanguages,
		},
	}
	if it.AirDate != "" {
		if t, err := time.Parse(time.DateOnly, it.AirDate); err == nil {
			out.AirDate = &openapi_types.Date{Time: t}
		}
	}
	if len(it.Hints) > 0 {
		h := api.ItemHints{}
		if v, ok := it.Hints["tmdb"]; ok {
			h.Tmdb = &v
		}
		if v, ok := it.Hints["imdb"]; ok {
			h.Imdb = &v
		}
		out.Hints = &h
	}
	if t := it.Technical; t.Width > 0 && t.Height > 0 {
		out.Technical.Resolution = &api.Resolution{Width: t.Width, Height: t.Height}
	}
	for _, f := range it.Technical.HDRFormats {
		out.Technical.HdrFormats = append(out.Technical.HdrFormats, api.TechnicalSummaryHdrFormats(f))
	}
	return out
}

func toMediaFile(f library.MediaFile) api.MediaFile {
	out := api.MediaFile{
		Id: f.ID, Path: f.Path, Size: f.Size, ModifiedAt: f.ModTime, Edition: optional(f.Edition),
		ParseStatus: api.MediaFileParseStatus(f.ParseStatus), ProbeStatus: api.MediaFileProbeStatus(f.ProbeStatus),
		Container: optional(f.Container), DurationMs: optionalInt64(f.DurationMs), Bitrate: optionalInt64(f.Bitrate),
		Width: optionalInt(f.Width), Height: optionalInt(f.Height), VideoCodec: optional(f.VideoCodec),
		MissingSince: f.MissingSince, Streams: make([]api.MediaStream, 0, len(f.Streams)),
	}
	if f.HDRFormat != "" {
		hdr := api.MediaFileHdrFormat(f.HDRFormat)
		out.HdrFormat = &hdr
	}
	if f.DVProfile > 0 {
		out.DolbyVision = &api.DolbyVision{Profile: f.DVProfile, Compatibility: f.DVCompat}
	}
	for _, s := range f.Streams {
		out.Streams = append(out.Streams, api.MediaStream{
			Index: s.Index, Type: api.MediaStreamType(s.Type), Codec: optional(s.Codec), Language: s.Language,
			Title: optional(s.Title), Channels: optionalInt(s.Channels), Default: s.Default, Forced: s.Forced,
			Sdh: s.SDH, ImageBased: s.Image, External: s.External,
		})
	}
	return out
}
