package handlers

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
	"github.com/gh-prim/plaid/server/internal/metadata/match"
)

// The F005 admin and read routes beyond items: search, fix match,
// identify, edits, artwork choice, keys, status, collections and people.

// metadataService returns the service, or the error routes give when F005
// isn't wired (tests that only need F004).
func (h *Handlers) metadataService() (*metadata.Service, error) {
	if h.metadata == nil {
		return nil, apierr.MetadataDisabled()
	}
	return h.metadata, nil
}

// metadataErr maps service errors to API errors.
func metadataErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, metadata.ErrNotFound), errors.Is(err, library.ErrNotFound):
		return apierr.NotFound("not found")
	case errors.Is(err, metadata.ErrImageNotFound):
		return apierr.NotFound("no such image")
	case errors.Is(err, metadata.ErrDisabled):
		return apierr.MetadataDisabled()
	case errors.Is(err, metadata.ErrMatchNotFound):
		return apierr.MatchNotFound()
	case errors.Is(err, metadata.ErrKindMismatch):
		return apierr.ItemKindMismatch()
	case errors.Is(err, metadata.ErrAlreadyIdentified):
		return apierr.AlreadyIdentified()
	case errors.Is(err, metadata.ErrFieldNotEditable):
		return apierr.FieldNotEditable()
	case errors.Is(err, metadata.ErrBadImagePath):
		return apierr.BadRequest("not a TMDB image file name")
	case metadata.IsProviderError(err):
		return apierr.ProviderUnavailable()
	}
	return err
}

func toCandidates(cands []match.Scored, withConfidence bool) []api.MatchCandidate {
	out := make([]api.MatchCandidate, 0, len(cands))
	for _, c := range cands {
		mc := api.MatchCandidate{TmdbId: c.TMDBID, Title: c.Title, OriginalTitle: optional(c.OriginalTitle),
			Year: optionalInt(c.Year), Overview: optional(c.Overview)}
		if c.PosterPath != "" {
			p := c.PosterPath
			if p[0] == '/' {
				p = p[1:]
			}
			mc.PosterPath = &p
		}
		if withConfidence {
			conf := float32(c.Confidence)
			mc.Confidence = &conf
		}
		out = append(out, mc)
	}
	return out
}

// SearchMetadata searches TMDB for a match (R27).
func (h *Handlers) SearchMetadata(ctx context.Context, req api.SearchMetadataRequestObject) (api.SearchMetadataResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	year := 0
	if req.Params.Year != nil {
		year = *req.Params.Year
	}
	itemID := deref(req.Params.ItemId)
	cands, err := svc.Search(ctx, string(req.Params.Kind), req.Params.Query, year, itemID)
	if err != nil {
		return nil, metadataErr(err)
	}
	return api.SearchMetadata200JSONResponse{Candidates: toCandidates(cands, itemID != "")}, nil
}

// GetCandidateImage proxies a candidate's TMDB poster (R27).
func (h *Handlers) GetCandidateImage(ctx context.Context, req api.GetCandidateImageRequestObject) (api.GetCandidateImageResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	body, err := svc.CandidateImage(ctx, req.Path)
	if err != nil {
		return nil, metadataErr(err)
	}
	return api.GetCandidateImage200ImagejpegResponse{Body: bytes.NewReader(body), ContentLength: int64(len(body))}, nil
}

// ListLibraryReview lists the items that need an admin (R5, R38).
func (h *Handlers) ListLibraryReview(ctx context.Context, req api.ListLibraryReviewRequestObject) (api.ListLibraryReviewResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	if _, err := h.library.Store().GetLibrary(ctx, req.LibraryId); err != nil {
		return nil, libraryErr(err, nil)
	}
	var p problems
	limit := checkLimit(req.Params.Limit, &p)
	if err := p.err(); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = 50
	}
	entries, err := svc.Store().Review(ctx, req.LibraryId, deref(req.Params.Cursor), limit+1)
	if err != nil {
		return nil, err
	}
	out := api.ReviewPage{Items: []api.ReviewItem{}}
	if len(entries) > limit {
		entries = entries[:limit]
		out.NextCursor = &entries[limit-1].ItemID
	}
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ItemID
	}
	sums, err := h.summariesOf(ctx, ids, access.Limit{})
	if err != nil {
		return nil, err
	}
	byID := map[string]api.ItemSummary{}
	for _, s := range sums {
		byID[s.Id] = s
	}
	for _, e := range entries {
		s, ok := byID[e.ItemID]
		if !ok {
			continue
		}
		out.Items = append(out.Items, api.ReviewItem{Item: s, Reason: api.ReviewItemReason(e.Reason),
			DuplicateOf: optional(e.DuplicateOf), Candidates: toCandidates(e.Candidates, true)})
	}
	return api.ListLibraryReview200JSONResponse(out), nil
}

// RefreshLibraryMetadata refetches a library's metadata (R31).
func (h *Handlers) RefreshLibraryMetadata(ctx context.Context, req api.RefreshLibraryMetadataRequestObject) (api.RefreshLibraryMetadataResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	if _, err := h.library.Store().GetLibrary(ctx, req.LibraryId); err != nil {
		return nil, libraryErr(err, nil)
	}
	images := req.Body != nil && req.Body.Images != nil && *req.Body.Images
	if err := svc.RefreshLibrary(ctx, req.LibraryId, images); err != nil {
		return nil, err
	}
	return api.RefreshLibraryMetadata202Response{}, nil
}

// MatchItem fixes an item's match (R26, R39).
func (h *Handlers) MatchItem(ctx context.Context, req api.MatchItemRequestObject) (api.MatchItemResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	tmdbID := 0
	if b.TmdbId != nil {
		tmdbID = *b.TmdbId
	}
	// An episode moved to another one no longer exists: the answer is the
	// item its files now belong to.
	id, err := svc.Match(ctx, req.ItemId, tmdbID, b.SeasonNumber, b.EpisodeNumber)
	if err != nil {
		return nil, metadataErr(err)
	}
	out, err := h.itemDetail(ctx, id, access.Limit{})
	if err != nil {
		return nil, err
	}
	return api.MatchItem200JSONResponse(out), nil
}

// UnmatchItem forgets a match (R26).
func (h *Handlers) UnmatchItem(ctx context.Context, req api.UnmatchItemRequestObject) (api.UnmatchItemResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	if err := svc.Unmatch(ctx, req.ItemId); err != nil {
		return nil, metadataErr(err)
	}
	return api.UnmatchItem204Response{}, nil
}

// RefreshItem refetches an item (R31).
func (h *Handlers) RefreshItem(ctx context.Context, req api.RefreshItemRequestObject) (api.RefreshItemResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	images := req.Body != nil && req.Body.Images != nil && *req.Body.Images
	if err := svc.Refresh(ctx, req.ItemId, images); err != nil {
		return nil, metadataErr(err)
	}
	return api.RefreshItem202Response{}, nil
}

// UpdateItemMetadata edits and locks fields (R29).
func (h *Handlers) UpdateItemMetadata(ctx context.Context, req api.UpdateItemMetadataRequestObject) (api.UpdateItemMetadataResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	p := metadata.Patch{Title: b.Title, SortTitle: b.SortTitle, OriginalTitle: b.OriginalTitle, Overview: b.Overview,
		Tagline: b.Tagline, Certification: b.Certification, Genres: b.Genres}
	if b.ReleaseDate != nil {
		d := b.ReleaseDate.Format(time.DateOnly)
		p.ReleaseDate = &d
	}
	if b.Unlock != nil {
		p.Unlock = *b.Unlock
	}
	if err := svc.PatchMetadata(ctx, req.ItemId, p); err != nil {
		return nil, metadataErr(err)
	}
	out, err := h.itemDetail(ctx, req.ItemId, access.Limit{})
	if err != nil {
		return nil, err
	}
	return api.UpdateItemMetadata200JSONResponse(out), nil
}

func toArtworkCandidates(list []metadata.ArtworkCandidate) api.ArtworkCandidates {
	out := api.ArtworkCandidates{Candidates: make([]api.ArtworkCandidate, 0, len(list))}
	for _, c := range list {
		ac := api.ArtworkCandidate{Id: c.ID, Kind: api.ArtworkKind(c.Kind), Source: api.ArtworkCandidateSource(c.Source),
			Language: optional(c.Language), Width: optionalInt(c.Width), Height: optionalInt(c.Height),
			Selected: c.Selected, Locked: c.Locked}
		if c.Votes > 0 {
			v := float32(c.Votes)
			ac.Votes = &v
		}
		out.Candidates = append(out.Candidates, ac)
	}
	return out
}

// ListItemArtwork lists artwork candidates (R30).
func (h *Handlers) ListItemArtwork(ctx context.Context, req api.ListItemArtworkRequestObject) (api.ListItemArtworkResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	list, err := svc.ArtworkCandidates(ctx, req.ItemId, string(req.Params.Kind))
	if err != nil {
		return nil, metadataErr(err)
	}
	return api.ListItemArtwork200JSONResponse(toArtworkCandidates(list)), nil
}

// SelectItemArtwork picks an image and locks its kind (R30).
func (h *Handlers) SelectItemArtwork(ctx context.Context, req api.SelectItemArtworkRequestObject) (api.SelectItemArtworkResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	if err := svc.SelectArtwork(ctx, req.ItemId, string(req.Kind), b.CandidateId); err != nil {
		return nil, metadataErr(err)
	}
	list, err := svc.ArtworkCandidates(ctx, req.ItemId, string(req.Kind))
	if err != nil {
		return nil, metadataErr(err)
	}
	return api.SelectItemArtwork200JSONResponse(toArtworkCandidates(list)), nil
}

// UnlockItemArtwork gives a kind back to the automatic choice (R30).
func (h *Handlers) UnlockItemArtwork(ctx context.Context, req api.UnlockItemArtworkRequestObject) (api.UnlockItemArtworkResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	if err := svc.UnlockArtwork(ctx, req.ItemId, string(req.Kind)); err != nil {
		return nil, metadataErr(err)
	}
	return api.UnlockItemArtwork204Response{}, nil
}

// IdentifyFile says what an unrecognized file is (R28).
func (h *Handlers) IdentifyFile(ctx context.Context, req api.IdentifyFileRequestObject) (api.IdentifyFileResponseObject, error) {
	pr, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	var episodes []int
	if b.EpisodeNumbers != nil {
		episodes = *b.EpisodeNumbers
	}
	id, err := svc.Identify(ctx, req.FileId, pr.UserID, b.TmdbId, b.SeasonNumber, episodes)
	if err != nil {
		return nil, metadataErr(err)
	}
	out, err := h.itemDetail(ctx, id, access.Limit{})
	if err != nil {
		return nil, err
	}
	return api.IdentifyFile200JSONResponse(out), nil
}

func keySource(s string) api.ProviderStatusKeySource {
	if s == "" {
		return api.ProviderStatusKeySourceNone
	}
	return api.ProviderStatusKeySource(s)
}

func (h *Handlers) metadataSettings(ctx context.Context, svc *metadata.Service) (api.MetadataSettings, error) {
	ks, err := svc.Settings(ctx)
	if err != nil {
		return api.MetadataSettings{}, err
	}
	var out api.MetadataSettings
	out.Tmdb.KeySource = api.MetadataSettingsTmdbKeySource(keySource(ks.TMDBSource))
	out.Fanart.KeySource = api.MetadataSettingsFanartKeySource(keySource(ks.FanartSource))
	out.Fanart.PersonalKeySet = ks.FanartPersonalKeySet
	return out, nil
}

// GetMetadataSettings says where each key comes from, never the key (R34).
func (h *Handlers) GetMetadataSettings(ctx context.Context, _ api.GetMetadataSettingsRequestObject) (api.GetMetadataSettingsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	out, err := h.metadataSettings(ctx, svc)
	if err != nil {
		return nil, err
	}
	return api.GetMetadataSettings200JSONResponse(out), nil
}

// UpdateMetadataSettings checks and saves keys (R34).
func (h *Handlers) UpdateMetadataSettings(ctx context.Context, req api.UpdateMetadataSettingsRequestObject) (api.UpdateMetadataSettingsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	var ch metadata.KeyChange
	empty := ""
	pick := func(v interface {
		IsSpecified() bool
		IsNull() bool
		MustGet() string
	}) *string {
		switch {
		case !v.IsSpecified():
			return nil
		case v.IsNull():
			return &empty
		}
		s := v.MustGet()
		return &s
	}
	ch.TMDB, ch.Fanart, ch.FanartPersonal = pick(b.TmdbApiKey), pick(b.FanartApiKey), pick(b.FanartPersonalKey)
	if err := svc.UpdateKeys(ctx, ch); err != nil {
		if errors.Is(err, metadata.ErrInvalidAPIKey) {
			field := "tmdbApiKey"
			if ch.TMDB == nil || *ch.TMDB == "" {
				field = "fanartApiKey"
			}
			return nil, apierr.InvalidAPIKey(field)
		}
		return nil, metadataErr(err)
	}
	out, err := h.metadataSettings(ctx, svc)
	if err != nil {
		return nil, err
	}
	return api.UpdateMetadataSettings200JSONResponse(out), nil
}

// GetMetadataStatus reports providers and per-library counters (R35).
func (h *Handlers) GetMetadataStatus(ctx context.Context, _ api.GetMetadataStatusRequestObject) (api.GetMetadataStatusResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	states, err := svc.Status(ctx)
	if err != nil {
		return nil, err
	}
	out := api.MetadataStatus{Providers: []api.ProviderStatus{}, Libraries: []api.LibraryMetadataCounts{}}
	for _, st := range states {
		ps := api.ProviderStatus{Provider: api.ProviderStatusProvider(st.Provider), State: api.ProviderStatusState(st.State),
			KeySource: keySource(st.KeySource), LastError: optional(st.LastError)}
		if !st.LastErrorAt.IsZero() {
			t := st.LastErrorAt
			ps.LastErrorAt = &t
		}
		out.Providers = append(out.Providers, ps)
	}
	libs, err := h.library.Store().ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range libs {
		c, err := svc.Store().Counts(ctx, l.ID)
		if err != nil {
			return nil, err
		}
		out.Libraries = append(out.Libraries, api.LibraryMetadataCounts{LibraryId: l.ID, Pending: c.Pending, Matched: c.Matched,
			NeedsReview: c.NeedsReview, Unmatched: c.Unmatched, Refreshing: &c.Refreshing})
	}
	return api.GetMetadataStatus200JSONResponse(out), nil
}

func collectionSummary(c metadata.Collection) api.CollectionSummary {
	return api.CollectionSummary{Id: c.ID, Name: c.Name, Overview: optional(c.Overview), MovieCount: len(c.ItemIDs),
		Poster: toArtwork(c.Poster), Backdrop: toArtwork(c.Backdrop)}
}

// ListCollections lists a library's collections (R23).
func (h *Handlers) ListCollections(ctx context.Context, req api.ListCollectionsRequestObject) (api.ListCollectionsResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	if _, err := h.library.Store().GetLibrary(ctx, req.Params.LibraryId); err != nil {
		return nil, libraryErr(err, nil)
	}
	cols, err := svc.Store().Collections(ctx, req.Params.LibraryId, limit)
	if err != nil {
		return nil, err
	}
	out := api.CollectionList{Collections: make([]api.CollectionSummary, 0, len(cols))}
	for _, c := range cols {
		out.Collections = append(out.Collections, collectionSummary(c))
	}
	return api.ListCollections200JSONResponse(out), nil
}

// GetCollection returns a collection with its movies (R23).
func (h *Handlers) GetCollection(ctx context.Context, req api.GetCollectionRequestObject) (api.GetCollectionResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	c, err := svc.Store().Collection(ctx, req.CollectionId, limit)
	if err != nil {
		return nil, metadataErr(err)
	}
	if len(c.ItemIDs) == 0 {
		return nil, apierr.NotFound("not found")
	}
	items, err := h.summariesOf(ctx, c.ItemIDs, limit)
	if err != nil {
		return nil, err
	}
	if err := h.withUserStates(ctx, pr, items); err != nil {
		return nil, err
	}
	s := collectionSummary(c)
	return api.GetCollection200JSONResponse(api.CollectionDetail{Id: s.Id, Name: s.Name, Overview: s.Overview,
		MovieCount: s.MovieCount, Poster: s.Poster, Backdrop: s.Backdrop, Items: items}), nil
}

// GetPerson returns a person and what they're in (R8, R25).
func (h *Handlers) GetPerson(ctx context.Context, req api.GetPersonRequestObject) (api.GetPersonResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := h.metadataService()
	if err != nil {
		return nil, err
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	p, err := svc.Store().Person(ctx, req.PersonId, limit)
	if err != nil {
		return nil, metadataErr(err)
	}
	ids := make([]string, len(p.Credits))
	for i, c := range p.Credits {
		ids[i] = c.ItemID
	}
	sums, err := h.summariesOf(ctx, ids, limit)
	if err != nil {
		return nil, err
	}
	if err := h.withUserStates(ctx, pr, sums); err != nil {
		return nil, err
	}
	byID := map[string]api.ItemSummary{}
	for _, s := range sums {
		byID[s.Id] = s
	}
	out := api.Person{Id: p.ID, Name: p.Name, Portrait: toArtwork(p.Portrait), Credits: []api.PersonCredit{}}
	for _, c := range p.Credits {
		if s, ok := byID[c.ItemID]; ok {
			out.Credits = append(out.Credits, api.PersonCredit{Item: s, Role: api.PersonCreditRole(c.Role), Character: optional(c.Character)})
		}
	}
	return api.GetPerson200JSONResponse(out), nil
}
