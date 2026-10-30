package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/catalog"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
)

// notImplemented answers the F008 catalog routes until their plan step lands.
func notImplemented() error {
	return apierr.New(http.StatusNotImplemented, "not_implemented", "not implemented yet")
}

// GetLibraryFacets lists what a library's grid can be filtered by (F008-R27).
func (h *Handlers) GetLibraryFacets(ctx context.Context, req api.GetLibraryFacetsRequestObject) (api.GetLibraryFacetsResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.catalog == nil {
		return nil, notImplemented()
	}
	lim, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	lib, err := h.library.Store().GetLibrary(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	f, err := h.catalog.Facets(ctx, lib.ID, topKind(lib), lim)
	if err != nil {
		return nil, err
	}
	out := api.LibraryFacets{
		Total: f.Total, Hdr: f.HDR,
		Genres:      make([]api.GenreFacet, 0, len(f.Genres)),
		Decades:     make([]api.DecadeFacet, 0, len(f.Decades)),
		Resolutions: make([]api.ResolutionFacet, 0, len(f.Resolutions)),
		Letters:     make([]api.LetterFacet, 0, len(f.Letters)),
	}
	for _, g := range f.Genres {
		out.Genres = append(out.Genres, api.GenreFacet{Id: g.ID, Name: g.Name, Count: g.Count})
	}
	for _, d := range f.Decades {
		out.Decades = append(out.Decades, api.DecadeFacet{From: d.From, Count: d.Count})
	}
	for _, r := range f.Resolutions {
		out.Resolutions = append(out.Resolutions, api.ResolutionFacet{Value: api.ResolutionFilter(r.Value), Count: r.Count})
	}
	for _, l := range f.Letters {
		out.Letters = append(out.Letters, api.LetterFacet{Letter: l.Letter, Count: l.Count})
	}
	return api.GetLibraryFacets200JSONResponse(out), nil
}

// topKind is a library's top-level kind: movie or show.
func topKind(lib library.Library) string {
	if lib.Type == library.TypeShows {
		return library.KindShow
	}
	return library.KindMovie
}

// Search finds movies, shows, episodes and people by word prefix (F008-R28).
func (h *Handlers) Search(ctx context.Context, req api.SearchRequestObject) (api.SearchResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.catalog == nil {
		return nil, notImplemented()
	}
	var p problems
	if n := utf8.RuneCountInString(strings.TrimSpace(req.Params.Q)); n < 1 || n > 100 {
		p.add("q", "out_of_range")
	}
	limit := value(req.Params.Limit)
	switch {
	case req.Params.Limit == nil:
		limit = 8
	case limit < 1 || limit > 50:
		p.add("limit", "out_of_range")
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	lim, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	r, err := h.catalog.Search(ctx, req.Params.Q, lim, limit)
	if err != nil {
		return nil, err
	}
	out := api.SearchResults{People: make([]api.PersonRef, 0, len(r.People))}
	for _, g := range []struct {
		ids []string
		out *[]api.ItemSummary
	}{{r.Movies, &out.Movies}, {r.Shows, &out.Shows}, {r.Episodes, &out.Episodes}} {
		page, err := h.itemPage(ctx, pr, g.ids)
		if err != nil {
			return nil, err
		}
		*g.out = page.Items
	}
	portraits := map[string]metadata.Art{}
	if h.metadata != nil && len(r.People) > 0 {
		ids := make([]string, len(r.People))
		for i, ph := range r.People {
			ids[i] = ph.ID
		}
		if portraits, err = h.metadata.Store().Portraits(ctx, ids); err != nil {
			return nil, err
		}
	}
	for _, ph := range r.People {
		ref := api.PersonRef{Id: ph.ID, Name: ph.Name}
		if a, ok := portraits[ph.ID]; ok {
			ref.Portrait = toArtwork(&a)
		}
		out.People = append(out.People, ref)
	}
	return api.Search200JSONResponse(out), nil
}

// GetHome returns the active profile's home (F008-R31).
func (h *Handlers) GetHome(ctx context.Context, _ api.GetHomeRequestObject) (api.GetHomeResponseObject, error) {
	pr, q, err := h.homeQuery(ctx)
	if err != nil {
		return nil, err
	}
	ambient, rows, err := h.catalog.Home(ctx, q)
	if err != nil {
		return nil, err
	}
	// Every row's items load together.
	ids := []string{}
	if ambient != "" {
		ids = append(ids, ambient)
	}
	for _, r := range rows {
		ids = append(ids, r.IDs...)
	}
	byID, err := h.summariesByID(ctx, pr, ids)
	if err != nil {
		return nil, err
	}
	out := api.Home{Rows: make([]api.HomeRow, 0, len(rows))}
	if s, ok := byID[ambient]; ok {
		out.Ambient = &s
	}
	for _, r := range rows {
		if row := toHomeRow(r, byID); len(row.Items) > 0 {
			out.Rows = append(out.Rows, row)
		}
	}
	return api.GetHome200JSONResponse(out), nil
}

// GetHomeRow pages one home row (F008-R31).
func (h *Handlers) GetHomeRow(ctx context.Context, req api.GetHomeRowRequestObject) (api.GetHomeRowResponseObject, error) {
	pr, q, err := h.homeQuery(ctx)
	if err != nil {
		return nil, err
	}
	var p problems
	limit := checkLimit(req.Params.Limit, &p)
	if limit == 0 {
		limit = catalog.HomeRowSize
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	r, err := h.catalog.HomeRow(ctx, q, req.RowId, value(req.Params.Cursor), limit)
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		return nil, apierr.NotFound("no such row")
	case errors.Is(err, catalog.ErrBadCursor):
		return nil, apierr.ValidationFailed(apierr.Detail{Field: "cursor", Code: "invalid"})
	case err != nil:
		return nil, err
	}
	byID, err := h.summariesByID(ctx, pr, r.IDs)
	if err != nil {
		return nil, err
	}
	row := toHomeRow(r, byID)
	return api.GetHomeRow200JSONResponse(api.ItemPage{Items: row.Items, NextCursor: row.NextCursor}), nil
}

// homeCWSize bounds the continue-watching list a home reads.
const homeCWSize = 200

// homeQuery is the active profile's home query, with its continue
// watching list from F006.
func (h *Handlers) homeQuery(ctx context.Context) (auth.Principal, catalog.HomeQuery, error) {
	pr, err := principal(ctx)
	if err != nil {
		return pr, catalog.HomeQuery{}, err
	}
	if h.catalog == nil {
		return pr, catalog.HomeQuery{}, notImplemented()
	}
	if pr.ProfileID == "" {
		return pr, catalog.HomeQuery{}, apierr.ProfileRequired()
	}
	lim, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return pr, catalog.HomeQuery{}, err
	}
	q := catalog.HomeQuery{ProfileID: pr.ProfileID, Access: lim}
	if h.playback != nil {
		if q.ContinueWatching, _, err = h.playback.Watch().ContinueWatching(ctx, pr.ProfileID, lim, "", homeCWSize); err != nil {
			return pr, q, err
		}
	}
	return pr, q, nil
}

// summariesByID loads items into summaries, keyed by id.
func (h *Handlers) summariesByID(ctx context.Context, pr auth.Principal, ids []string) (map[string]api.ItemSummary, error) {
	seen := map[string]bool{}
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	page, err := h.itemPage(ctx, pr, uniq)
	if err != nil {
		return nil, err
	}
	out := make(map[string]api.ItemSummary, len(page.Items))
	for _, s := range page.Items {
		out[s.Id] = s
	}
	return out, nil
}

func toHomeRow(r catalog.Row, byID map[string]api.ItemSummary) api.HomeRow {
	out := api.HomeRow{Id: r.ID, Type: api.HomeRowType(r.Type), LibraryId: optional(r.LibraryID),
		LibraryName: optional(r.LibraryName), NextCursor: optional(r.Next), Items: make([]api.ItemSummary, 0, len(r.IDs))}
	if r.Genre != nil {
		out.Genre = &api.GenreRef{Id: r.Genre.ID, Name: r.Genre.Name}
	}
	if a := r.SeeAll; a != nil {
		order := api.Asc
		if a.Desc {
			order = api.Desc
		}
		out.SeeAll = &api.SeeAll{LibraryId: a.LibraryID, Sort: api.ItemSort(a.Sort), Order: order}
		if a.Genre != 0 {
			g := a.Genre
			out.SeeAll.Genre = &g
		}
	}
	for _, id := range r.IDs {
		if s, ok := byID[id]; ok {
			out.Items = append(out.Items, s)
		}
	}
	return out
}

// GetUpNext returns the episode to play next (F008-R32).
func (h *Handlers) GetUpNext(ctx context.Context, req api.GetUpNextRequestObject) (api.GetUpNextResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.catalog == nil {
		return nil, notImplemented()
	}
	lim, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	id, err := h.catalog.UpNext(ctx, req.ItemId, pr.ProfileID, lim)
	if errors.Is(err, catalog.ErrNotFound) {
		return nil, apierr.NotFound("no such item")
	}
	if err != nil {
		return nil, err
	}
	if id == "" {
		return api.GetUpNext204Response{}, nil
	}
	page, err := h.itemPage(ctx, pr, []string{id})
	if err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return api.GetUpNext204Response{}, nil
	}
	return api.GetUpNext200JSONResponse(api.UpNext{Item: page.Items[0]}), nil
}

// value returns what p points to, or T's zero value.
func value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// catalogQuery reports whether a library listing uses F008's sort, filters
// or offset; without them it keeps F004's keyset listing.
func catalogQuery(q api.ListLibraryItemsParams) bool {
	return q.Sort != nil || q.Order != nil || q.Seed != nil || q.Watch != nil || q.Genre != nil ||
		q.YearFrom != nil || q.YearTo != nil || q.Resolution != nil || q.Hdr != nil || q.Offset != nil || q.Total != nil
}

// listCatalog lists a library's top-level items sorted and filtered
// (F008-R26).
func (h *Handlers) listCatalog(ctx context.Context, req api.ListLibraryItemsRequestObject, pr auth.Principal, lim access.Limit) (api.ListLibraryItemsResponseObject, error) {
	if h.catalog == nil {
		return nil, notImplemented()
	}
	prm := req.Params
	lib, err := h.library.Store().GetLibrary(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	q := catalog.Query{
		LibraryID: lib.ID, Kind: topKind(lib), ProfileID: pr.ProfileID, Access: lim,
		Sort: catalog.SortTitle, Seed: value(prm.Seed), WithTotal: value(prm.Total),
		YearFrom: value(prm.YearFrom), YearTo: value(prm.YearTo), HDR: prm.Hdr,
	}
	var p problems
	if prm.ParentId != nil {
		p.add("parentId", "not_with_sort_or_filters")
	}
	if prm.Cursor != nil {
		p.add("cursor", "use_offset")
	}
	if prm.Kind != nil && string(*prm.Kind) != q.Kind {
		p.add("kind", "invalid")
	}
	q.Limit = checkLimit(prm.Limit, &p)
	if q.Limit == 0 {
		q.Limit = 50
	}
	if o := value(prm.Offset); o < 0 {
		p.add("offset", "out_of_range")
	} else {
		q.Offset = o
	}
	if s := prm.Sort; s != nil {
		if !s.Valid() {
			p.add("sort", "invalid")
		}
		q.Sort = string(*s)
	}
	q.Desc = q.Sort != catalog.SortTitle
	if o := prm.Order; o != nil {
		if !o.Valid() {
			p.add("order", "invalid")
		}
		q.Desc = *o == api.Desc
	}
	if w := prm.Watch; w != nil {
		if !w.Valid() {
			p.add("watch", "invalid")
		}
		q.Watch = string(*w)
	}
	if prm.Genre != nil {
		q.Genres = *prm.Genre
	}
	if prm.Resolution != nil {
		for _, r := range *prm.Resolution {
			if !r.Valid() {
				p.add("resolution", "invalid")
			}
			q.Resolutions = append(q.Resolutions, string(r))
		}
	}
	if q.YearFrom > 0 && q.YearTo > 0 && q.YearFrom > q.YearTo {
		p.add("yearTo", "before_year_from")
	}
	if err := p.err(); err != nil {
		return nil, err
	}

	page, err := h.catalog.List(ctx, q)
	if err != nil {
		return nil, err
	}
	out, err := h.itemPage(ctx, pr, page.IDs)
	if err != nil {
		return nil, err
	}
	if page.Total >= 0 {
		out.Total = &page.Total
	}
	return api.ListLibraryItems200JSONResponse(out), nil
}

// itemPage loads items by id into summaries with metadata and the
// profile's watch state, in the given order.
func (h *Handlers) itemPage(ctx context.Context, pr auth.Principal, ids []string) (api.ItemPage, error) {
	items, err := h.library.Store().Items(ctx, ids)
	if err != nil {
		return api.ItemPage{}, err
	}
	sums, err := h.summaries(ctx, items)
	if err != nil {
		return api.ItemPage{}, err
	}
	out := api.ItemPage{Items: make([]api.ItemSummary, 0, len(items))}
	for _, it := range items {
		s := toItemSummary(it)
		withSummary(&s, sums[it.ID])
		out.Items = append(out.Items, s)
	}
	if err := h.withShows(ctx, out.Items); err != nil {
		return api.ItemPage{}, err
	}
	return out, h.withUserStates(ctx, pr, out.Items)
}
