package handlers

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
	"github.com/gh-prim/plaid/server/internal/playback"
)

// StreamPrefix is where playback streams are served (F006 R25).
const StreamPrefix = "/api/v1/stream/"

// Content types of HLS resources.
const (
	typePlaylist = "application/vnd.apple.mpegurl"
	typeSegment  = "video/mp4"
)

// StreamHandler serves /api/v1/stream/{token}/… outside the generated
// server, which can't do Range, HEAD or long polling. The operations are
// still in api/openapi.yaml for clients. The token in the path is the only
// authentication; paths on disk always come from the session, never from
// the URL. meta and lib serve the playing item's artwork (R40); without
// them, artwork is a 404.
func StreamHandler(svc *playback.Service, meta *metadata.Service, lib *library.Store) http.Handler {
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
		dir, name := "", rest
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			dir, name = rest[:i], rest[i+1:]
		}
		base, ext, _ := strings.Cut(name, ".")
		switch {
		case dir == "" && name == "file":
			serveFile(w, r, svc, token)
		case dir == "" && name == "master.m3u8":
			servePlaylist(w, r, svc, token, func(s playback.Session) (string, error) { return playback.MasterPlaylist(s), nil })
		case dir == "video" && name == "index.m3u8":
			servePlaylist(w, r, svc, token, svc.MediaPlaylist)
		case dir == "video" && name == "init.mp4":
			serveInit(w, r, svc, token)
		case dir == "video" && ext == "m4s":
			if n, ok := streamIndex(base); ok {
				serveSegment(w, r, svc, token, n)
				return
			}
			apierr.Write(w, apierr.NotFound("no such stream resource"))
		case dir == "artwork" && streamArtworkKinds[name]:
			serveArtwork(w, r, svc, meta, lib, token, name)
		case dir == "subtitles" && (ext == "m3u8" || ext == playback.SubtitleVTT || ext == playback.SubtitleASS || ext == playback.SubtitleSUP):
			if n, ok := streamIndex(base); ok {
				serveSubtitle(w, r, svc, token, n, ext)
				return
			}
			apierr.Write(w, apierr.NotFound("no such stream resource"))
		default:
			apierr.Write(w, apierr.NotFound("no such stream resource"))
		}
	})
}

// streamIndex reads a segment or stream number: digits only, bounded.
func streamIndex(s string) (int, bool) {
	if s == "" || len(s) > 7 || strings.TrimLeft(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// session resolves the token; checkAuth also checks its F003 session
// (playlists and files, not every segment).
func session(w http.ResponseWriter, r *http.Request, svc *playback.Service, token string, checkAuth bool) (playback.Session, bool) {
	sess, err := svc.StreamSession(r.Context(), token, checkAuth)
	switch {
	case errors.Is(err, playback.ErrSessionNotFound):
		apierr.Write(w, apierr.PlaybackSessionNotFound())
		return sess, false
	case err != nil:
		apierr.Write(w, apierr.Internal())
		return sess, false
	}
	return sess, true
}

// streamError writes a playback error as an API error.
func streamError(w http.ResponseWriter, err error) {
	var e *apierr.Error
	if errors.As(playbackErr(err), &e) {
		apierr.Write(w, e)
		return
	}
	apierr.Write(w, apierr.Internal())
}

// serveFile serves the original file for Direct Play (R10).
func serveFile(w http.ResponseWriter, r *http.Request, svc *playback.Service, token string) {
	sess, ok := session(w, r, svc, token, true)
	if !ok {
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

// servePlaylist serves an HLS playlist, never cached (R25).
func servePlaylist(w http.ResponseWriter, r *http.Request, svc *playback.Service, token string, build func(playback.Session) (string, error)) {
	sess, ok := session(w, r, svc, token, true)
	if !ok {
		return
	}
	if sess.Decision.Method == playback.DirectPlay {
		apierr.Write(w, apierr.NotFound("this session is Direct Play"))
		return
	}
	body, err := build(sess)
	if err != nil {
		streamError(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", typePlaylist)
	h.Set("Cache-Control", "private, no-store")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, body)
	}
}

func serveInit(w http.ResponseWriter, r *http.Request, svc *playback.Service, token string) {
	sess, ok := session(w, r, svc, token, true)
	if !ok {
		return
	}
	path, err := svc.Init(r.Context(), sess)
	if err != nil {
		streamError(w, err)
		return
	}
	serveFiles(w, r, typeSegment, []string{path})
}

// serveSegment serves segment n (R14). Segment requests skip the F003
// check: playlists and the init segment already did it.
func serveSegment(w http.ResponseWriter, r *http.Request, svc *playback.Service, token string, n int) {
	sess, ok := session(w, r, svc, token, false)
	if !ok {
		return
	}
	files, err := svc.Segment(r.Context(), sess, n)
	if err != nil {
		streamError(w, err)
		return
	}
	serveFiles(w, r, typeSegment, files)
}

// serveSubtitle serves a subtitle rendition playlist, WebVTT or ASS (R16).
func serveSubtitle(w http.ResponseWriter, r *http.Request, svc *playback.Service, token string, index int, ext string) {
	if ext == "m3u8" {
		servePlaylist(w, r, svc, token, func(s playback.Session) (string, error) {
			if !playback.IsTextSubtitle(playback.Subtitle{Index: index, Codec: codecOf(s, index)}) {
				return "", playback.ErrNoSuchSubtitle
			}
			return playback.SubtitlePlaylist(s.Decision.Media.DurationMs, index), nil
		})
		return
	}
	sess, ok := session(w, r, svc, token, true)
	if !ok {
		return
	}
	// ?from=<ms>: answer at once with the part around the position while
	// the whole track is extracted (V2); the header says where it ends.
	var (
		path  string
		until int64
		err   error
	)
	if from, ferr := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64); ferr == nil && from >= 0 {
		path, until, err = svc.SubtitleQuick(r.Context(), sess, index, ext, from)
	} else {
		path, err = svc.SubtitleFile(r.Context(), sess, index, ext)
	}
	if err != nil {
		streamError(w, err)
		return
	}
	if until > 0 {
		w.Header().Set("X-Subtitle-Until", strconv.FormatInt(until, 10))
		w.Header().Set("Cache-Control", "no-store")
	}
	ctype := "text/vtt; charset=utf-8"
	switch ext {
	case playback.SubtitleASS:
		ctype = "text/x-ssa; charset=utf-8"
	case playback.SubtitleSUP:
		ctype = "application/octet-stream"
	}
	serveFiles(w, r, ctype, []string{path})
}

// streamArtworkKinds are the kinds served through a token (R40).
var streamArtworkKinds = map[string]bool{"poster": true, "backdrop": true, "still": true}

// serveArtwork serves the playing item's artwork with the session's age
// limit (R40). An episode without an image of that kind uses its
// season's, then its show's.
func serveArtwork(w http.ResponseWriter, r *http.Request, svc *playback.Service, meta *metadata.Service, lib *library.Store, token, kind string) {
	sess, ok := session(w, r, svc, token, true)
	if !ok {
		return
	}
	want := 0
	if v := r.URL.Query().Get("w"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 3840 {
			apierr.Write(w, apierr.ValidationFailed(apierr.Detail{Field: "w", Code: "out_of_range"}))
			return
		}
		want = n
	}
	if meta == nil || lib == nil {
		apierr.Write(w, apierr.NotFound("no such image"))
		return
	}
	ctx := r.Context()
	imageID, err := streamArtworkID(ctx, meta, lib, sess.ItemID, kind)
	if err != nil {
		apierr.Write(w, apierr.Internal())
		return
	}
	if imageID == "" {
		apierr.Write(w, apierr.NotFound("no such image"))
		return
	}
	limit, err := svc.AccessLimit(ctx, sess)
	if err != nil {
		apierr.Write(w, apierr.Internal())
		return
	}
	v, err := meta.Image(ctx, imageID, want, limit)
	switch {
	case errors.Is(err, metadata.ErrImageNotFound):
		apierr.Write(w, apierr.NotFound("no such image"))
		return
	case err != nil && metadata.IsProviderError(err):
		apierr.Write(w, apierr.ProviderUnavailable())
		return
	case err != nil:
		apierr.Write(w, apierr.Internal())
		return
	}
	f, err := os.Open(v.Path)
	if err != nil {
		apierr.Write(w, apierr.NotFound("no such image"))
		return
	}
	defer func() { _ = f.Close() }()
	h := w.Header()
	h.Set("Content-Type", v.ContentType)
	h.Set("ETag", v.ETag)
	h.Set("Cache-Control", "private, max-age=3600")
	h.Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", time.Time{}, f)
}

// streamArtworkID is the selected image of a kind for an item, else for
// its parent (season), else its grandparent (show); "" when none has one.
func streamArtworkID(ctx context.Context, meta *metadata.Service, lib *library.Store, itemID, kind string) (string, error) {
	var ids []string
	for id := itemID; id != "" && len(ids) < 3; {
		ids = append(ids, id)
		item, err := lib.GetItem(ctx, id)
		if errors.Is(err, library.ErrNotFound) {
			break
		}
		if err != nil {
			return "", err
		}
		id = item.ParentID
	}
	sums, err := meta.Store().Summaries(ctx, ids)
	if err != nil {
		return "", err
	}
	for _, id := range ids {
		sm := sums[id]
		if art := map[string]*metadata.Art{"poster": sm.Poster, "backdrop": sm.Backdrop, "still": sm.Still}[kind]; art != nil {
			return art.ID, nil
		}
	}
	return "", nil
}

func codecOf(s playback.Session, index int) string {
	for _, sub := range s.Decision.Media.Subtitles {
		if sub.Index == index {
			return sub.Codec
		}
	}
	return ""
}

// serveFiles serves files as one body, with ranges. They're opened before
// anything is written, so a cleanup deleting them afterwards is harmless.
func serveFiles(w http.ResponseWriter, r *http.Request, ctype string, paths []string) {
	body, err := openAll(paths)
	if err != nil {
		apierr.Write(w, apierr.NotFound("no such stream resource"))
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, "", time.Time{}, body)
}

// multiFile reads files one after the other as a single seekable body.
type multiFile struct {
	files []*os.File
	sizes []int64
	total int64
	off   int64
}

func openAll(paths []string) (*multiFile, error) {
	m := &multiFile{}
	for _, p := range paths {
		f, err := os.Open(p) //nolint:gosec // paths come from the session's folder, never the URL
		if err != nil {
			m.Close()
			return nil, err
		}
		st, err := f.Stat()
		if err != nil {
			_ = f.Close()
			m.Close()
			return nil, err
		}
		m.files, m.sizes, m.total = append(m.files, f), append(m.sizes, st.Size()), m.total+st.Size()
	}
	return m, nil
}

func (m *multiFile) Read(p []byte) (int, error) {
	if m.off >= m.total {
		return 0, io.EOF
	}
	start := int64(0)
	for i, f := range m.files {
		if m.off < start+m.sizes[i] {
			n, err := f.ReadAt(p[:min(int64(len(p)), start+m.sizes[i]-m.off)], m.off-start)
			m.off += int64(n)
			if errors.Is(err, io.EOF) && n > 0 {
				err = nil
			}
			return n, err
		}
		start += m.sizes[i]
	}
	return 0, io.EOF
}

func (m *multiFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += m.off
	case io.SeekEnd:
		offset += m.total
	}
	if offset < 0 {
		return 0, errors.New("negative position")
	}
	m.off = offset
	return offset, nil
}

func (m *multiFile) Close() {
	for _, f := range m.files {
		_ = f.Close()
	}
}
