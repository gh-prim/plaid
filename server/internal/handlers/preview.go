package handlers

import (
	"errors"
	"net/http"
	"os"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/playback"
)

// previewRetryAfter is how long a client waits before asking again for a
// preview being cut, in seconds.
const previewRetryAfter = "2"

// PreviewHandler serves GET /api/v1/items/{itemId}/preview (V2 hero)
// outside the generated server, which can't do Range: a 200 MP4 when the
// preview is ready, a 202 while it's being cut, a 404 when there's none
// or the profile may not see the item. The operation is still in
// api/openapi.yaml for clients.
func PreviewHandler(svc *playback.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := principal(r.Context())
		if err != nil {
			apierr.Write(w, apierr.Unauthorized())
			return
		}
		id := r.PathValue("itemId")
		if svc == nil || id == "" {
			apierr.Write(w, apierr.NotFound("no preview"))
			return
		}
		path, state, err := svc.Preview(r.Context(), p, id, r.URL.Query().Get("audio"))
		switch {
		case errors.Is(err, playback.ErrItemNotFound):
			apierr.Write(w, apierr.NotFound("no such item"))
			return
		case err != nil:
			apierr.Write(w, apierr.Internal())
			return
		}
		switch state {
		case playback.PreviewPreparing:
			w.Header().Set("Retry-After", previewRetryAfter)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			if r.Method != http.MethodHead {
				_, _ = w.Write([]byte(`{"status":"preparing"}`))
			}
		case playback.PreviewReady:
			f, err := os.Open(path) //nolint:gosec // the path comes from the cache, never the URL
			if err != nil {
				// Pruned in between: it'll be cut again.
				w.Header().Set("Retry-After", previewRetryAfter)
				w.WriteHeader(http.StatusAccepted)
				return
			}
			defer func() { _ = f.Close() }()
			st, err := f.Stat()
			if err != nil {
				apierr.Write(w, apierr.NotFound("no preview"))
				return
			}
			w.Header().Set("Content-Type", "video/mp4")
			// Revalidated: the same URL gets a new file when the recipe
			// changes or the file is cut again.
			w.Header().Set("Cache-Control", "private, no-cache")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeContent(w, r, "", st.ModTime(), f)
		default:
			apierr.Write(w, apierr.NotFound("no preview"))
		}
	})
}
