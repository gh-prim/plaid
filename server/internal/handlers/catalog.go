package handlers

import (
	"context"
	"net/http"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/catalog"
	"github.com/gh-prim/plaid/server/internal/library"
)

// notImplemented answers the F008 catalog routes until their plan step lands.
func notImplemented() error {
	return apierr.New(http.StatusNotImplemented, "not_implemented", "not implemented yet")
}

// GetLibraryFacets lists what a library's grid can be filtered by (F008-R27).
func (h *Handlers) GetLibraryFacets(_ context.Context, _ api.GetLibraryFacetsRequestObject) (api.GetLibraryFacetsResponseObject, error) {
	return nil, notImplemented()
}

// Search finds movies, shows, episodes and people by word prefix (F008-R28).
func (h *Handlers) Search(_ context.Context, _ api.SearchRequestObject) (api.SearchResponseObject, error) {
	return nil, notImplemented()
}

// GetHome returns the active profile's home (F008-R31).
func (h *Handlers) GetHome(_ context.Context, _ api.GetHomeRequestObject) (api.GetHomeResponseObject, error) {
	return nil, notImplemented()
}

// GetHomeRow pages one home row (F008-R31).
func (h *Handlers) GetHomeRow(_ context.Context, _ api.GetHomeRowRequestObject) (api.GetHomeRowResponseObject, error) {
	return nil, notImplemented()
}

// GetUpNext returns the episode to play next (F008-R32).
func (h *Handlers) GetUpNext(_ context.Context, _ api.GetUpNextRequestObject) (api.GetUpNextResponseObject, error) {
	return nil, notImplemented()
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
		LibraryID: lib.ID, Kind: library.KindMovie, ProfileID: pr.ProfileID, Access: lim,
		Sort: catalog.SortTitle, Seed: value(prm.Seed), WithTotal: value(prm.Total),
		YearFrom: value(prm.YearFrom), YearTo: value(prm.YearTo), HDR: prm.Hdr,
	}
	if lib.Type == library.TypeShows {
		q.Kind = library.KindShow
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
	return out, h.withUserStates(ctx, pr, out.Items)
}
