package playback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/library"
)

// Previews (V2 hero): a short loop of a few scenes from the title
// itself, cut by FFmpeg in the background and cached. A movie's scenes
// come from its first third, after the opening credits; a show's from its
// first episode, so they spoil nothing.
const (
	previewScenes      = 4
	previewSceneLength = 6 * time.Second
	// previewFade fades each scene's sound in and out.
	previewFade = 400 * time.Millisecond
	// previewFrom and previewTo bound where the scenes are taken, as a
	// share of the runtime.
	previewFrom = 0.08
	previewTo   = 0.35
	// previewMinRuntime: shorter files (trailers, extras) get no preview.
	previewMinRuntime = 5 * time.Minute
	previewWidth      = 1280
	previewTimeout    = 5 * time.Minute
	// previewRetry is how long a failed preview stays unavailable.
	previewRetry = time.Hour
	// previewCacheMax caps the cached previews; each is a few MB, so this
	// holds a few thousand titles. Past 90%, nothing more is cut ahead.
	previewCacheMax = 10 << 30
	// previewVersion changes the cache key when the recipe changes.
	previewVersion = "v2"
)

// PreviewState says whether an item's preview can be played.
type PreviewState int

const (
	// PreviewUnavailable means no file, no FFmpeg, too short, or it failed.
	PreviewUnavailable PreviewState = iota
	// PreviewPreparing means it's being cut; ask again in a few seconds.
	PreviewPreparing
	// PreviewReady means the path is the MP4.
	PreviewReady
)

// previews is the queue of cuts, one at a time: a title someone opens
// goes first, at full speed; otherwise every movie and show is cut ahead
// of time on two threads, newest first, and only while nobody transcodes,
// so a hero's scenes are ready before anyone asks.
type previews struct {
	cache *fileCache
	mu    sync.Mutex
	// urgent holds the titles someone is looking at; queued, their keys.
	urgent []previewSource
	queued map[string]bool
	failed map[string]time.Time
	// backlog is the ids still to look at ahead of time; filled says when.
	backlog []string
	filled  time.Time
	wake    chan struct{}
	// stopAhead interrupts the cut ahead of time running, if any, when
	// someone opens a title; it's cut again on a later pass.
	stopAhead context.CancelFunc
}

func newPreviews(dir string) *previews {
	return &previews{
		cache:  &fileCache{Dir: dir, Max: previewCacheMax},
		queued: map[string]bool{},
		failed: map[string]time.Time{},
		wake:   make(chan struct{}, 1),
	}
}

type previewSource struct {
	fileID   string
	path     string
	duration time.Duration
	hdr      bool
	// audio is the absolute index of the sound track, -1 when there's none.
	audio int
	key   string
}

// Preview returns the item's preview when it's ready, and otherwise
// starts cutting it. An item the profile may not see is ErrItemNotFound.
func (s *Service) Preview(ctx context.Context, p auth.Principal, itemID string) (string, PreviewState, error) {
	item, err := s.visibleItem(ctx, p, itemID)
	if err != nil {
		return "", PreviewUnavailable, err
	}
	if s.o.FFmpeg == nil || s.previews == nil {
		return "", PreviewUnavailable, nil
	}
	src, ok, err := s.previewSource(ctx, item)
	if err != nil || !ok {
		return "", PreviewUnavailable, err
	}
	name := src.key + ".mp4"
	pv := s.previews
	if path := filepath.Join(pv.cache.Dir, name); fileExists(path) {
		now := time.Now()
		_ = os.Chtimes(path, now, now) // recently used, kept by the pruning
		return path, PreviewReady, nil
	}
	pv.mu.Lock()
	defer pv.mu.Unlock()
	if t, ok := pv.failed[src.key]; ok && s.now().Sub(t) < previewRetry {
		return "", PreviewUnavailable, nil
	}
	if !pv.queued[src.key] {
		pv.queued[src.key] = true
		pv.urgent = append(pv.urgent, src)
		if pv.stopAhead != nil {
			pv.stopAhead()
		}
		select {
		case pv.wake <- struct{}{}:
		default:
		}
	}
	return "", PreviewPreparing, nil
}

func (s *Service) now() time.Time {
	if s.o.Now != nil {
		return s.o.Now()
	}
	return time.Now()
}

// previewSource picks the file: the movie's or episode's own, a show's
// first episode's.
func (s *Service) previewSource(ctx context.Context, item library.Item) (previewSource, bool, error) {
	id := item.ID
	switch item.Kind {
	case "movie", "episode":
	case "show":
		first, err := s.o.Library.FirstEpisode(ctx, item.ID)
		if errors.Is(err, library.ErrNotFound) {
			return previewSource{}, false, nil
		}
		if err != nil {
			return previewSource{}, false, err
		}
		id = first
	default:
		return previewSource{}, false, nil
	}
	files, err := s.o.Library.ItemFiles(ctx, id)
	if err != nil {
		return previewSource{}, false, err
	}
	for _, f := range files {
		d := time.Duration(f.DurationMs) * time.Millisecond
		if f.MissingSince != nil || d < previewMinRuntime || f.Width == 0 {
			continue
		}
		h := sha256.New()
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00%d\x00%s", f.ID, f.Path, f.ModTime.UnixNano(), previewVersion)
		return previewSource{
			fileID: f.ID, path: f.Path, duration: d, hdr: f.HDRFormat != "",
			audio: previewAudio(f.Streams), key: hex.EncodeToString(h.Sum(nil))[:32],
		}, true, nil
	}
	return previewSource{}, false, nil
}

// Waits between the cuts ahead of time.
const (
	// previewRescan: the backlog is done; look for new titles this often.
	previewRescan = 5 * time.Minute
	// previewBusy: someone transcodes; look again this often.
	previewBusy = 30 * time.Second
)

func (s *Service) previewRescan() time.Duration {
	if s.t.PreviewRescan > 0 {
		return s.t.PreviewRescan
	}
	return previewRescan
}

// runPreviews cuts the previews until ctx is done.
func (s *Service) runPreviews(ctx context.Context) {
	pv := s.previews
	if n := dropOldPreviews(pv.cache.Dir); n > 0 {
		s.o.Log.Info("previews of an older recipe deleted", "count", n)
	}
	for {
		wait := s.previewRescan()
		if src, ok := pv.next(); ok {
			s.cutPreview(ctx, src, true)
			continue
		}
		if !s.o.PreviewsAhead {
			wait = time.Hour
		} else if transcodes, _, _ := s.Load(); transcodes > 0 {
			wait = previewBusy
		} else if src, ok := s.nextAhead(ctx); ok {
			s.cutPreview(ctx, src, false)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-pv.wake:
		case <-time.After(wait):
		}
	}
}

// next pops the oldest urgent title.
func (pv *previews) next() (previewSource, bool) {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	if len(pv.urgent) == 0 {
		return previewSource{}, false
	}
	src := pv.urgent[0]
	pv.urgent = pv.urgent[1:]
	return src, true
}

// nextAhead finds the next title without a preview, refilling the backlog
// from the library at most every previewRescan.
func (s *Service) nextAhead(ctx context.Context) (previewSource, bool) {
	pv := s.previews
	pv.mu.Lock()
	if len(pv.backlog) == 0 && s.now().Sub(pv.filled) >= s.previewRescan() {
		pv.mu.Unlock()
		ids, err := s.o.Library.TopLevelIDs(ctx)
		if err != nil {
			s.o.Log.Warn("previews: listing titles", "err", err)
			return previewSource{}, false
		}
		pv.mu.Lock()
		pv.backlog, pv.filled = ids, s.now()
	}
	pv.mu.Unlock()
	if pv.cache.size() > previewCacheMax/10*9 {
		return previewSource{}, false
	}
	for ctx.Err() == nil {
		pv.mu.Lock()
		if len(pv.backlog) == 0 {
			pv.mu.Unlock()
			return previewSource{}, false
		}
		id := pv.backlog[0]
		pv.backlog = pv.backlog[1:]
		pv.mu.Unlock()
		item, err := s.o.Library.GetItem(ctx, id)
		if err != nil {
			continue
		}
		src, ok, err := s.previewSource(ctx, item)
		if err != nil || !ok || fileExists(filepath.Join(pv.cache.Dir, src.key+".mp4")) {
			continue
		}
		pv.mu.Lock()
		t, failed := pv.failed[src.key]
		skip := pv.queued[src.key] || failed && s.now().Sub(t) < previewRetry
		if !skip {
			pv.queued[src.key] = true
		}
		pv.mu.Unlock()
		if !skip {
			return src, true
		}
	}
	return previewSource{}, false
}

// cutPreview cuts and caches one preview: an urgent one on every core, one
// ahead of time on two threads.
func (s *Service) cutPreview(ctx context.Context, src previewSource, urgent bool) {
	pv := s.previews
	start := time.Now()
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	if !urgent {
		pv.mu.Lock()
		pv.stopAhead = stop
		pv.mu.Unlock()
	}
	_, err := pv.cache.Get(ctx, src.key+".mp4", func(ctx context.Context, dst string) error {
		ctx, cancel := context.WithTimeout(ctx, previewTimeout)
		defer cancel()
		args := previewArgs(src, s.o.FFmpeg, dst, urgent)
		out, err := exec.CommandContext(ctx, s.o.FFmpeg.Path, args...).CombinedOutput() //nolint:gosec // argument list
		if err != nil {
			return fmt.Errorf("cutting preview: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	})
	pv.mu.Lock()
	delete(pv.queued, src.key)
	if !urgent {
		pv.stopAhead = nil
	}
	interrupted := ctx.Err() != nil
	if err != nil && !interrupted {
		pv.failed[src.key] = s.now()
	}
	pv.mu.Unlock()
	if err != nil && interrupted {
		s.o.Log.Info("preview ahead of time interrupted", "file_id", src.fileID)
		return
	}
	if err != nil {
		s.o.Log.Warn("preview not cut", "file_id", src.fileID, "err", err)
		return
	}
	s.o.Log.Info("preview cut", "file_id", src.fileID, "urgent", urgent, "duration_ms", time.Since(start).Milliseconds())
}

// previewScenesAt spreads the scenes evenly over [previewFrom, previewTo]
// of the runtime.
func previewScenesAt(d time.Duration) []time.Duration {
	from, to := time.Duration(float64(d)*previewFrom), time.Duration(float64(d)*previewTo)
	step := (to - from) / previewScenes
	out := make([]time.Duration, previewScenes)
	for i := range out {
		out[i] = from + time.Duration(i)*step
	}
	return out
}

// previewAudio picks the default sound track, else the first one; -1
// when the file has none.
func previewAudio(streams []library.MediaStream) int {
	first := -1
	for _, st := range streams {
		if st.Type != "audio" || st.External {
			continue
		}
		if st.Default {
			return st.Index
		}
		if first < 0 {
			first = st.Index
		}
	}
	return first
}

// previewArgs builds the FFmpeg command: one fast-seeked input per scene,
// each scaled (and tone mapped from HDR when the build can), its sound in
// stereo faded in and out so the cuts don't click, joined into one H.264
// and AAC MP4 that starts playing before it's fully loaded.
func previewArgs(src previewSource, ff *FFmpeg, dst string, urgent bool) []string {
	secs := func(d time.Duration) string { return strconv.FormatFloat(d.Seconds(), 'f', 3, 64) }
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y"}
	scenes := previewScenesAt(src.duration)
	threads := []string{}
	if !urgent {
		threads = []string{"-threads", "2"}
	}
	for _, at := range scenes {
		args = append(args, threads...)
		args = append(args, "-ss", secs(at), "-t", secs(previewSceneLength), "-i", src.path)
	}
	filter := "scale=" + strconv.Itoa(previewWidth) + ":-2"
	if src.hdr && ff.HasFilter("zscale") && ff.HasFilter("tonemap") {
		filter += "," + swTonemap
	}
	filter += ",setsar=1,fps=24,format=yuv420p"
	var graph strings.Builder
	sound := src.audio >= 0
	fadeOut := previewSceneLength - previewFade
	afilter := fmt.Sprintf("aformat=sample_rates=48000:channel_layouts=stereo,asetpts=PTS-STARTPTS,"+
		"afade=t=in:d=%s,afade=t=out:st=%s:d=%s", secs(previewFade), secs(fadeOut), secs(previewFade))
	for i := range scenes {
		fmt.Fprintf(&graph, "[%d:v:0]%s[v%d];", i, filter, i)
		if sound {
			fmt.Fprintf(&graph, "[%d:%d]%s[a%d];", i, src.audio, afilter, i)
		}
	}
	for i := range scenes {
		fmt.Fprintf(&graph, "[v%d]", i)
		if sound {
			fmt.Fprintf(&graph, "[a%d]", i)
		}
	}
	if sound {
		fmt.Fprintf(&graph, "concat=n=%d:v=1:a=1[out][aout]", len(scenes))
		args = append(args, "-filter_complex", graph.String(), "-map", "[out]",
			"-map", "[aout]", "-c:a", "aac", "-b:a", "128k")
	} else {
		fmt.Fprintf(&graph, "concat=n=%d:v=1:a=0[out]", len(scenes))
		args = append(args, "-filter_complex", graph.String(), "-map", "[out]", "-an")
	}
	args = append(args, threads...)
	return append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "26", "-profile:v", "high",
		"-movflags", "+faststart", "-f", "mp4", dst)
}

// previewDir is where previews are cached, under the cache folder, one
// folder per recipe version.
func previewDir(cacheDir string) string {
	return filepath.Join(cacheDir, "previews", previewVersion)
}

// dropOldPreviews deletes what an older recipe left next to dir, so it
// doesn't hold the cache's room until it ages out; it returns how many
// entries went.
func dropOldPreviews(dir string) int {
	parent := filepath.Dir(dir)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.Name() == filepath.Base(dir) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(parent, e.Name())); err == nil {
			n++
		}
	}
	return n
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CacheDir is the folder of converted subtitles and previews.
func (s *Service) CacheDir() string { return s.o.CacheDir }
