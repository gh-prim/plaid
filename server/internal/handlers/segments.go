package handlers

import (
	"context"

	"github.com/oapi-codegen/nullable"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
)

// GetItemSegments returns a movie's or an episode's intro, recap, credits
// and preview, from TheIntroDB (cached by the server).
func (h *Handlers) GetItemSegments(ctx context.Context, req api.GetItemSegmentsRequestObject) (api.GetItemSegmentsResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.catalog == nil {
		return nil, notImplemented()
	}
	// Same visibility as every item read (F005 R25).
	page, err := h.itemPage(ctx, pr, []string{req.ItemId})
	if err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, apierr.NotFound("no such item")
	}
	out := api.ItemSegments{Segments: []api.Segment{}}
	if h.segments == nil {
		return api.GetItemSegments200JSONResponse(out), nil
	}
	segs, err := h.segments.For(ctx, req.ItemId)
	if err != nil {
		return nil, err
	}
	for _, s := range segs {
		seg := api.Segment{Kind: api.SegmentKind(s.Kind)}
		if s.StartMs != nil {
			seg.StartMs = nullable.NewNullableWithValue(*s.StartMs)
		}
		if s.EndMs != nil {
			seg.EndMs = nullable.NewNullableWithValue(*s.EndMs)
		}
		out.Segments = append(out.Segments, seg)
	}
	return api.GetItemSegments200JSONResponse(out), nil
}
