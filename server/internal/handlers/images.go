package handlers

import (
	"context"
	"errors"
	"os"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/metadata"
)

// immutable: an imageId's bytes never change (R19).
const imageCacheControl = "private, max-age=31536000, immutable"

// GetImage serves an artwork image at the width the client asks for
// (F005 R19), hiding images of items the profile may not see (R25).
func (h *Handlers) GetImage(ctx context.Context, req api.GetImageRequestObject) (api.GetImageResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.metadata == nil {
		return nil, apierr.NotFound("no such image")
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	want := 0
	if req.Params.W != nil {
		want = *req.Params.W
	}
	v, err := h.metadata.Image(ctx, req.ImageId, want, limit)
	switch {
	case errors.Is(err, metadata.ErrImageNotFound):
		return nil, apierr.NotFound("no such image")
	case err != nil && metadata.IsProviderError(err):
		return nil, apierr.ProviderUnavailable()
	case err != nil:
		return nil, err
	}
	etag := v.ETag
	if m := req.Params.IfNoneMatch; m != nil && *m == etag {
		return api.GetImage304Response{Headers: api.GetImage304ResponseHeaders{ETag: &etag}}, nil
	}
	f, err := os.Open(v.Path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	cc, nosniff := imageCacheControl, "nosniff"
	headers := api.GetImage200ResponseHeaders{ETag: &etag, CacheControl: &cc, XContentTypeOptions: &nosniff}
	body := closingReader{f}
	if v.ContentType == "image/png" {
		return api.GetImage200ImagepngResponse{Body: body, Headers: headers, ContentLength: st.Size()}, nil
	}
	return api.GetImage200ImagejpegResponse{Body: body, Headers: headers, ContentLength: st.Size()}, nil
}

// closingReader closes the file once it's been read to the end.
type closingReader struct{ f *os.File }

func (c closingReader) Read(p []byte) (int, error) {
	n, err := c.f.Read(p)
	if err != nil {
		_ = c.f.Close()
	}
	return n, err
}
