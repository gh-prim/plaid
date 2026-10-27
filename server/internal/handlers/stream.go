package handlers

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/playback"
)

// StreamPrefix is where playback streams are served (F006 R25).
const StreamPrefix = "/api/v1/stream/"

// StreamHandler serves /api/v1/stream/{token}/… outside the generated
// server, which can't do Range, HEAD or long polling. The operations are
// still in api/openapi.yaml for clients. The token in the path is the only
// authentication; paths on disk always come from the session, never from
// the URL.
func StreamHandler(svc *playback.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			apierr.Write(w, apierr.New(http.StatusMethodNotAllowed, "method_not_allowed", "streams are read-only"))
			return
		}
		token, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, StreamPrefix), "/")
		if token == "" || svc == nil {
			apierr.Write(w, apierr.PlaybackSessionNotFound())
			return
		}
		switch rest {
		case "file":
			serveFile(w, r, svc, token)
		default:
			apierr.Write(w, apierr.NotFound("no such stream resource"))
		}
	})
}

// serveFile serves the original file for Direct Play (R10).
func serveFile(w http.ResponseWriter, r *http.Request, svc *playback.Service, token string) {
	sess, err := svc.StreamSession(r.Context(), token, true)
	switch {
	case errors.Is(err, playback.ErrSessionNotFound):
		apierr.Write(w, apierr.PlaybackSessionNotFound())
		return
	case err != nil:
		apierr.Write(w, apierr.Internal())
		return
	}
	if sess.Decision.Method != playback.DirectPlay {
		apierr.Write(w, apierr.NotFound("this session isn't Direct Play"))
		return
	}
	m := sess.Decision.Media
	f, err := os.Open(m.Path)
	if err != nil {
		apierr.Write(w, apierr.FileUnavailable())
		return
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		apierr.Write(w, apierr.FileUnavailable())
		return
	}
	h := w.Header()
	h.Set("Content-Type", m.ContentType())
	h.Set("ETag", `"`+strconv.FormatInt(st.Size(), 36)+"-"+strconv.FormatInt(st.ModTime().UnixNano(), 36)+`"`)
	h.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filepath.Base(m.Path)}))
	h.Set("Cache-Control", "private")
	http.ServeContent(w, r, "", st.ModTime(), f)
}
