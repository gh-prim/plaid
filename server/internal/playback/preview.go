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
	// previewCopyMaxBitrate: an H.264 file up to 1080p, 8-bit SDR, at most
	// this many bits per second, has its video copied, not encoded: the
	// cut is instant and needs no decoder, and a preview stays about 30 MB.
	previewCopyMaxBitrate = 12_000_000
	// previewCopyCacheMax caps the copied previews on their own: they're
	// bigger but cost nothing to cut again, so they never push the encoded
	// ones out.
	previewCopyCacheMax = 4 << 30
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
	// cache holds the encoded previews; copies, the copied ones.
	cache  *fileCache
	copies *fileCache
	mu     sync.Mutex
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
		cache:  &fileCache{Dir: filepath.Join(dir, "encoded"), Max: previewCacheMax},
		copies: &fileCache{Dir: filepath.Join(dir, "copied"), Max: previewCopyCacheMax},
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
	// copy: the video is copied as it is (previewCopy).
	copy bool
	key  string
}

// cacheFor is the cache a source's preview goes to.
func (pv *previews) cacheFor(src previewSource) *fileCache {
	if src.copy {
		return pv.copies
	}
	return pv.cache
}

// Preview returns the item's preview when it's ready, and otherwise
// starts cutting it. audio is the language the viewer wants to hear, if
// known. An item the profile may not see is ErrItemNotFound.
func (s *Service) Preview(ctx context.Context, p auth.Principal, itemID, audio string) (string, PreviewState, error) {
	item, err := s.visibleItem(ctx, p, itemID)
	if err != nil {
		return "", PreviewUnavailable, err
	}
	if s.o.FFmpeg == nil || s.previews == nil {
		return "", PreviewUnavailable, nil
	}
	src, ok, err := s.previewSource(ctx, item, audio)
	if err != nil || !ok {
		return "", PreviewUnavailable, err
	}
	name := src.key + ".mp4"
	pv := s.previews
	if path := filepath.Join(pv.cacheFor(src).Dir, name); fileExists(path) {
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
// first episode's; and its sound track: in audio when it has one, else in
// the library's metadata language, else the default.
func (s *Service) previewSource(ctx context.Context, item library.Item, audio string) (previewSource, bool, error) {
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
	files, err := s.o.Library.PlayableFiles(ctx, id)
	if err != nil {
		return previewSource{}, false, err
	}
	langs := []string{audio}
	if lib, err := s.o.Library.GetLibrary(ctx, item.LibraryID); err == nil {
		langs = append(langs, lib.MetadataLanguage)
	}
	for _, f := range files {
		d := time.Duration(f.DurationMs) * time.Millisecond
		if f.MissingSince != nil || d < previewMinRuntime || f.Width == 0 {
			continue
		}
		track := previewAudio(f.Streams, langs...)
		h := sha256.New()
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00%d\x00%s\x00%d", f.ID, f.Path, f.ModTime.UnixNano(), previewVersion, track)
		return previewSource{
			fileID: f.ID, path: f.Path, duration: d, hdr: f.HDRFormat != "",
			audio: track, copy: previewCopy(f, s.previewCopyMax()), key: hex.EncodeToString(h.Sum(nil))[:32],
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
	if n := dropOldPreviews(filepath.Dir(pv.cache.Dir)); n > 0 {
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
		src, ok, err := s.previewSource(ctx, item, "")
		// A copied preview is cut in a second or two when someone asks.
		if err != nil || !ok || src.copy || fileExists(filepath.Join(pv.cache.Dir, src.key+".mp4")) {
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
	_, err := pv.cacheFor(src).Get(ctx, src.key+".mp4", func(ctx context.Context, dst string) error {
		ctx, cancel := context.WithTimeout(ctx, previewTimeout)
		defer cancel()
		return cutScenes(ctx, s.o.FFmpeg, src, dst, urgent)
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

// previewAudio picks the first sound track in the first of langs the file
// has (commentaries last), else the default track, else the first one; -1
// when the file has none.
func previewAudio(streams []library.MediaStream, langs ...string) int {
	for _, l := range langs {
		code, ok := library.NormalizeLanguage(l)
		if !ok {
			continue
		}
		found := -1
		for _, st := range streams {
			if st.Type != "audio" || st.External || st.Language != code {
				continue
			}
			if !strings.Contains(strings.ToLower(st.Title), "comment") {
				return st.Index
			}
			if found < 0 {
				found = st.Index
			}
		}
		if found >= 0 {
			return found
		}
	}
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

// Threads per FFmpeg run: the decoder, filters and encoder each get this
// many, so a 4K HEVC source can't fill the memory of a small server.
const (
	previewThreadsUrgent = "4"
	previewThreadsAhead  = "2"
)

// cutScenes cuts the scenes one at a time, each into its own file (one
// decoder in memory at once: four 4K HEVC decoders side by side took
// 2.8 GB and got killed), then joins them without encoding again.
func cutScenes(ctx context.Context, ff *FFmpeg, src previewSource, dst string, urgent bool) error {
	tmp, err := os.MkdirTemp("", "plaid-preview-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	threads := previewThreadsAhead
	if urgent {
		threads = previewThreadsUrgent
	}
	run := func(args []string) error {
		out, err := exec.CommandContext(ctx, ff.Path, args...).CombinedOutput() //nolint:gosec // argument list
		if err != nil {
			return fmt.Errorf("cutting preview: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	var list strings.Builder
	for i, at := range previewScenesAt(src.duration) {
		scene := filepath.Join(tmp, fmt.Sprintf("scene%d.mp4", i))
		if err := run(previewSceneArgs(src, ff, at, scene, threads)); err != nil {
			return err
		}
		fmt.Fprintf(&list, "file '%s'\n", scene)
	}
	listPath := filepath.Join(tmp, "scenes.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o600); err != nil {
		return err
	}
	return run(previewJoinArgs(listPath, dst))
}

// previewCopyMax is Timings.PreviewCopyMaxBitrate, or its default.
func (s *Service) previewCopyMax() int64 {
	if s.t.PreviewCopyMaxBitrate != 0 {
		return s.t.PreviewCopyMaxBitrate
	}
	return previewCopyMaxBitrate
}

// previewCopy says whether a file's video can go out as it is: H.264 any
// browser plays (8-bit 4:2:0, SDR, up to 1080p) at a bitrate up to max,
// which keeps the preview light.
func previewCopy(f library.PlayableFile, maxBitrate int64) bool {
	if f.Probe == nil {
		return false
	}
	v := f.Probe.Video()
	return v != nil && v.Codec == "h264" && v.HDR == "" && v.Width <= 1920 && v.Height <= 1080 &&
		(v.PixFmt == "yuv420p" || v.PixFmt == "yuvj420p") && f.Bitrate > 0 && f.Bitrate <= maxBitrate
}

// previewSceneArgs cuts one scene from at: fast-seeked, its video copied
// (previewCopy) or scaled (and tone mapped from HDR when the build can),
// its sound in stereo faded in and out so the joins don't click; H.264
// and AAC, or no sound when the file has none. Every scene gets the same
// parameters, so they join as they are.
func previewSceneArgs(src previewSource, ff *FFmpeg, at time.Duration, dst, threads string) []string {
	secs := func(d time.Duration) string { return strconv.FormatFloat(d.Seconds(), 'f', 3, 64) }
	filter := "scale=" + strconv.Itoa(previewWidth) + ":-2"
	if src.hdr && ff.HasFilter("zscale") && ff.HasFilter("tonemap") {
		filter += "," + swTonemap
	}
	filter += ",setsar=1,fps=24,format=yuv420p"
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y", "-filter_threads", threads,
		"-threads", threads}
	if src.copy {
		// The video starts on the keyframe before at; so does the sound.
		args = append(args, "-noaccurate_seek")
	}
	args = append(args, "-ss", secs(at), "-t", secs(previewSceneLength), "-i", src.path, "-map", "0:v:0")
	if src.copy {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-vf", filter)
	}
	if src.audio >= 0 {
		fadeOut := previewSceneLength - previewFade
		args = append(args, "-map", "0:"+strconv.Itoa(src.audio), "-af",
			fmt.Sprintf("aformat=sample_rates=48000:channel_layouts=stereo,asetpts=PTS-STARTPTS,"+
				"afade=t=in:d=%s,afade=t=out:st=%s:d=%s", secs(previewFade), secs(fadeOut), secs(previewFade)),
			"-c:a", "aac", "-b:a", "128k")
	} else {
		args = append(args, "-an")
	}
	if src.copy {
		return append(args, "-avoid_negative_ts", "make_zero", "-f", "mp4", dst)
	}
	return append(args, "-threads", threads, "-c:v", "libx264", "-preset", "veryfast", "-crf", "26",
		"-profile:v", "high", "-f", "mp4", dst)
}

// previewJoinArgs joins the scenes listed in list without encoding them
// again, into an MP4 that starts playing before it's fully loaded.
func previewJoinArgs(list, dst string) []string {
	return []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y", "-f", "concat", "-safe", "0",
		"-i", list, "-c", "copy", "-movflags", "+faststart", "-f", "mp4", dst}
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
