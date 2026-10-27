package handlers

import (
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
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
	sums, err := h.summaries(ctx, items)
	if err != nil {
		return nil, err
	}
	out := api.ItemPage{Items: make([]api.ItemSummary, 0, len(items)), NextCursor: optional(next)}
	for _, it := range items {
		s := toItemSummary(it)
		withSummary(&s, sums[it.ID])
		out.Items = append(out.Items, s)
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
	var detail *metadata.Detail
	if h.metadata != nil {
		d, ok, err := h.metadata.Store().Detail(ctx, it.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			detail = &d
			withSummary(&s, d.Summary)
		}
	}
	out := api.ItemDetail{
		Id: s.Id, Kind: s.Kind, ParentId: s.ParentId, Title: s.Title, Year: s.Year,
		SeasonNumber: s.SeasonNumber, EpisodeNumber: s.EpisodeNumber, AirDate: s.AirDate, Hints: s.Hints,
		FileCount: s.FileCount, ChildCount: s.ChildCount, Technical: s.Technical,
		DisplayTitle: s.DisplayTitle, MatchStatus: s.MatchStatus, ReleaseDate: s.ReleaseDate, Rating: s.Rating,
		Certification: s.Certification, AgeRating: s.AgeRating, Genres: s.Genres,
		Poster: s.Poster, Backdrop: s.Backdrop, Still: s.Still,
		Files: make([]api.MediaFile, 0, len(files)),
	}
	if detail != nil {
		out.Metadata = toItemMetadata(*detail)
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

// summaries loads the F005 summaries of a page of items.
func (h *Handlers) summaries(ctx context.Context, items []library.Item) (map[string]metadata.Summary, error) {
	if h.metadata == nil {
		return map[string]metadata.Summary{}, nil
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return h.metadata.Store().Summaries(ctx, ids)
}

// withSummary adds F005's fields to a summary (R36). Without a metadata
// row the item is pending and shows its parsed title.
func withSummary(s *api.ItemSummary, m metadata.Summary) {
	if m.State == "" {
		return
	}
	s.MatchStatus = api.MatchStatus(m.State)
	if m.Title != "" {
		s.DisplayTitle = m.Title
	}
	if t, err := time.Parse(time.DateOnly, m.ReleaseDate); err == nil {
		s.ReleaseDate = &openapi_types.Date{Time: t}
	}
	if m.Rating > 0 {
		r := float32(m.Rating)
		s.Rating = &r
	}
	s.Certification = optional(m.Certification)
	s.AgeRating = m.AgeRating
	s.Genres = m.Genres
	s.Poster, s.Backdrop, s.Still = toArtwork(m.Poster), toArtwork(m.Backdrop), toArtwork(m.Still)
}

func toArtwork(a *metadata.Art) *api.Artwork {
	if a == nil {
		return nil
	}
	src := api.ArtworkSource(a.Source)
	out := &api.Artwork{ImageId: a.ID, Width: optionalInt(a.Width), Height: optionalInt(a.Height), Source: &src}
	if c := a.Colors; c != nil {
		out.Colors = &api.AmbientColors{Dominant: c.Dominant, Vibrant: c.Vibrant, Muted: c.Muted, Dark: c.Dark,
			OnDominant: api.AmbientColorsOnDominant(c.OnDominant)}
	}
	return out
}

func toItemMetadata(d metadata.Detail) *api.ItemMetadata {
	out := &api.ItemMetadata{
		OriginalTitle: optional(d.OriginalTitle), Overview: optional(d.Overview), Tagline: optional(d.Tagline),
		RuntimeMinutes: optionalInt(d.Runtime), RatingCount: optionalInt(d.RatingCount),
		CertificationRegion: optional(d.CertRegion), Studios: d.Studios, FieldLanguages: d.FieldLanguages,
		LockedFields: d.LockedFields, LocalFields: d.LocalFields, DuplicateOf: optional(d.DuplicateOf),
		Trailers: make([]api.Trailer, 0, len(d.Trailers)), Credits: make([]api.Credit, 0, len(d.Credits)),
		Logo: toArtwork(d.Logo), Clearart: toArtwork(d.Clearart), Backdrops: make([]api.Artwork, 0, len(d.Backdrops)),
		ProviderIds: api.ProviderIds{Tmdb: optionalInt(d.TMDBID), Imdb: optional(d.IMDbID), Tvdb: optionalInt(d.TVDBID)},
	}
	if d.MatchSource != "" {
		src := api.ItemMetadataMatchSource(d.MatchSource)
		out.MatchSource = &src
		c := float32(d.Confidence)
		out.Confidence = &c
	}
	if d.ShowStatus != "" {
		st := api.ItemMetadataShowStatus(d.ShowStatus)
		out.ShowStatus = &st
	}
	if t, err := time.Parse(time.DateOnly, d.EndDate); err == nil {
		out.EndDate = &openapi_types.Date{Time: t}
	}
	if d.CollectionID != "" {
		out.Collection = &api.CollectionRef{Id: d.CollectionID, Name: d.CollectionName}
	}
	for _, v := range d.Trailers {
		out.Trailers = append(out.Trailers, api.Trailer{Site: api.TrailerSite(v.Site), Key: v.Key, Name: optional(v.Name),
			Language: v.Language, Type: api.TrailerType(v.Type), Official: v.Official})
	}
	for _, c := range d.Credits {
		out.Credits = append(out.Credits, api.Credit{PersonId: c.PersonID, Name: c.Name, Role: api.CreditRole(c.Role),
			Character: optional(c.Character), Portrait: toArtwork(c.Portrait)})
	}
	for i := range d.Backdrops {
		out.Backdrops = append(out.Backdrops, *toArtwork(&d.Backdrops[i]))
	}
	return out
}

func toItemSummary(it library.Item) api.ItemSummary {
	out := api.ItemSummary{
		Id: it.ID, Kind: api.ItemKind(it.Kind), ParentId: optional(it.ParentID), Title: optional(it.Title),
		DisplayTitle: it.Title, MatchStatus: api.MatchStatusPending, Genres: []string{},
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
