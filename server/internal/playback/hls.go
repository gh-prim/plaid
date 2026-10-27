package playback

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Errors of HLS streams. The handlers turn them into API errors.
var (
	ErrNoSuchSegment    = errors.New("no such segment")
	ErrSegmentNotReady  = errors.New("segment_not_ready")
	ErrTranscodeLimit   = errors.New("transcode_limit_reached")
	ErrTranscodeStorage = errors.New("transcode_storage_full")
)

// Timings of HLS streams (R14, R15, R26–R28). Tests shorten them.
type Timings struct {
	WaitAhead     int           // segments a request waits for instead of restarting
	SegmentWait   time.Duration // how long a request waits for its segment
	ThrottleAhead int64         // ms produced ahead of the client before pausing FFmpeg
	ResumeAhead   int64         // ms ahead under which a paused run continues
	IdleStop      time.Duration // without segment requests, the run stops
	SlotIdle      time.Duration // without any activity, the transcode slot is released
	KeepBehind    int64         // ms of segments kept behind the last requested one
	Tick          time.Duration // housekeeping period
}

// DefaultTimings are the spec's values.
var DefaultTimings = Timings{
	WaitAhead: 3, SegmentWait: 20 * time.Second, ThrottleAhead: 120_000, ResumeAhead: 60_000,
	IdleStop: 60 * time.Second, SlotIdle: 10 * time.Minute, KeepBehind: 300_000, Tick: time.Second,
}

// hlsStream is the HLS output of one session for one decision: its folder,
// its segment layout and its current FFmpeg run. A new decision (R6) gets
// a new stream, in a new folder.
type hlsStream struct {
	mu        sync.Mutex
	sessionID string
	dir       string
	layout    Layout
	decision  Decision
	preset    string
	run       *Run
	runs      int
	lastSeg   int
	lastSegAt time.Time
	keptFrom  int  // segments before it were deleted
	slot      bool // holds a transcode slot (R26)
	hw        *HWConfig
	slowRun   int // the run last logged as slower than real time
}

func (st *hlsStream) piece(n int) string { return filepath.Join(st.dir, strconv.Itoa(n)+".m4s") }

func (st *hlsStream) initPath() string { return filepath.Join(st.dir, "init.mp4") }

// onDisk lists a segment's pieces when they're all there.
func (st *hlsStream) onDisk(seg Segment) ([]string, bool) {
	files := make([]string, 0, seg.Last-seg.First)
	for p := seg.First; p < seg.Last; p++ {
		if _, err := os.Stat(st.piece(p)); err != nil {
			return nil, false
		}
		files = append(files, st.piece(p))
	}
	return files, true
}

// alive is the current run when it's still running. Callers hold mu.
func (st *hlsStream) alive() *Run {
	if st.run == nil {
		return nil
	}
	select {
	case <-st.run.Done():
		return nil
	default:
		return st.run
	}
}

// layoutOf is a decision's segment list.
func layoutOf(d Decision) Layout {
	if d.Method == DirectStream {
		return StreamLayout(d.Media.Keyframes, d.Media.DurationMs)
	}
	return TranscodeLayout(d.Media.DurationMs)
}

// newStream registers the HLS stream of a session's current decision.
func (s *Service) newStream(sess Session, gen int, slot bool) {
	st := &hlsStream{
		sessionID: sess.ID, dir: s.storage.SessionDir(sess.ID, gen), layout: layoutOf(sess.Decision),
		decision: sess.Decision, preset: s.config().EncoderPreset, slot: slot, lastSegAt: s.o.Now(),
		lastSeg: layoutOf(sess.Decision).SegmentAt(sess.StartPositionMs),
	}
	if hw := s.hwConfig(); hw != nil && sess.Decision.HWAccel == hw.Accel {
		st.hw = hw
	}
	s.mu.Lock()
	s.streams[sess.ID] = st
	s.mu.Unlock()
}

func (s *Service) stream(sessionID string) *hlsStream {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streams[sessionID]
}

// closeStream stops a session's run, deletes its folder and frees its
// slot unless keepSlot.
func (s *Service) closeStream(sessionID string, keepSlot bool) {
	s.mu.Lock()
	st := s.streams[sessionID]
	delete(s.streams, sessionID)
	s.mu.Unlock()
	if st == nil {
		return
	}
	st.mu.Lock()
	run := st.run
	st.run = nil
	hadSlot := st.slot
	st.slot = false
	st.mu.Unlock()
	if run != nil {
		run.Stop()
	}
	_ = os.RemoveAll(st.dir)
	if hadSlot && !keepSlot {
		s.releaseSlot()
	}
}

// startRun starts FFmpeg at a piece, after stopping the previous run.
// Callers hold st.mu.
func (s *Service) startRun(st *hlsStream, piece int) (*Run, error) {
	if old := st.run; old != nil {
		old.Stop()
		st.run = nil
	}
	if st.decision.Method == Transcode && !st.slot {
		if !s.acquireSlot() {
			return nil, ErrTranscodeLimit
		}
		st.slot = true
	}
	if s.storageFull() {
		return nil, ErrTranscodeStorage
	}
	st.runs++
	spec := RunSpec{Decision: st.decision, Layout: st.layout, StartPiece: piece, Dir: st.dir, RunID: st.runs, Preset: st.preset, HW: st.hw, CPUs: usableCPUs()}
	run, err := StartRun(s.o.FFmpeg.Path, spec)
	if err != nil {
		return nil, err
	}
	st.run = run
	s.o.Log.Info("ffmpeg run started", "playback_id", st.sessionID, "run", st.runs, "method", st.decision.Method,
		"segment", st.layout.SegmentOf(piece), "start_ms", st.layout.PieceStartMs[piece],
		"command", s.o.FFmpeg.Path+" "+strings.Join(spec.Args(), " "))
	go s.watchRun(st, run)
	return run, nil
}

// watchRun logs how a run ended, and restarts a failed hardware run in
// software (R24).
func (s *Service) watchRun(st *hlsStream, run *Run) {
	<-run.Done()
	if err := run.Err(); err != nil {
		if s.fallBack(st, run) == nil {
			s.o.Log.Error("ffmpeg run failed", "playback_id", st.sessionID, "run", run.Spec.RunID, "err", err)
		}
		return
	}
	s.o.Log.Info("ffmpeg run ended", "playback_id", st.sessionID, "run", run.Spec.RunID, "produced", run.Produced())
}

// fallBack replaces a failed run: a hardware run that failed in its first
// 10 seconds restarts in software where it stopped (R24). It returns the
// stream's current run when another caller already replaced it, and nil
// when the failure stands. Both the run's watcher and the requests
// waiting on it call it; the first one does the work.
func (s *Service) fallBack(st *hlsStream, failed *Run) *Run {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.run != failed {
		return st.alive()
	}
	if failed.Spec.HW == nil || failed.Err() == nil || failed.Ran() > hwEarlyFailure {
		return nil
	}
	accel := failed.Spec.HW.Accel
	s.o.Log.Warn("hardware transcode failed; restarting in software", "code", "hw_fallback", "playback_id", st.sessionID,
		"run", failed.Spec.RunID, "accel", accel, "err", failed.Err())
	s.hwFailed(accel)
	st.hw = nil
	s.manager.SetHWAccel(st.sessionID, "")
	run, err := s.startRun(st, max(failed.Produced()+1, failed.Spec.StartPiece))
	if err != nil {
		s.o.Log.Error("software restart failed", "playback_id", st.sessionID, "err", err)
		return nil
	}
	return run
}

// waitPiece waits for piece n of a run, following it to its software
// replacement when a hardware run fails (R24).
func (s *Service) waitPiece(ctx context.Context, st *hlsStream, run *Run, n int) error {
	for {
		if run.WaitPiece(ctx, n) {
			return nil
		}
		if run.Err() == nil {
			return ErrSegmentNotReady
		}
		next := s.fallBack(st, run)
		if next == nil {
			return run.Err()
		}
		run = next
	}
}

// Segment returns the files of HLS segment n, in order, waiting for them
// or restarting FFmpeg as R14 says.
func (s *Service) Segment(ctx context.Context, sess Session, n int) ([]string, error) {
	st := s.stream(sess.ID)
	if st == nil || s.o.FFmpeg == nil {
		return nil, ErrNoSuchSegment
	}
	st.mu.Lock()
	if n < 0 || n >= len(st.layout.Segments) {
		st.mu.Unlock()
		return nil, ErrNoSuchSegment
	}
	seg := st.layout.Segments[n]
	st.lastSeg, st.lastSegAt = n, s.o.Now()
	s.manager.SegmentPosition(sess.ID, seg.StartMs)
	s.dropBehind(st)
	if files, ok := st.onDisk(seg); ok {
		st.mu.Unlock()
		return files, nil
	}
	run := st.alive()
	if run != nil && !s.reaches(st, run, seg, n) {
		run = nil
	}
	if run == nil {
		var err error
		if run, err = s.startRun(st, seg.First); err != nil {
			st.mu.Unlock()
			return nil, err
		}
	}
	run.Resume()
	st.mu.Unlock()

	wctx, cancel := context.WithTimeout(ctx, s.t.SegmentWait)
	defer cancel()
	if err := s.waitPiece(wctx, st, run, seg.Last-1); err != nil {
		return nil, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if files, ok := st.onDisk(seg); ok {
		return files, nil
	}
	return nil, ErrSegmentNotReady // deleted under the cap in the meantime
}

// reaches reports whether a running run will produce segment n soon: it
// started at or before it, and is at most WaitAhead segments short of
// it (R14). Callers hold st.mu.
func (s *Service) reaches(st *hlsStream, run *Run, seg Segment, n int) bool {
	if run.Spec.StartPiece > seg.First {
		return false
	}
	next := run.Produced() + 1
	if next >= seg.Last {
		return false // produced already but no longer on disk
	}
	return n-st.layout.SegmentOf(next) <= s.t.WaitAhead
}

// Init returns the session's init segment, starting FFmpeg at the
// session's position when nothing has written it yet.
func (s *Service) Init(ctx context.Context, sess Session) (string, error) {
	st := s.stream(sess.ID)
	if st == nil || s.o.FFmpeg == nil {
		return "", ErrNoSuchSegment
	}
	st.mu.Lock()
	if _, err := os.Stat(st.initPath()); err == nil {
		st.mu.Unlock()
		return st.initPath(), nil
	}
	run := st.alive()
	if run == nil {
		var err error
		if run, err = s.startRun(st, st.layout.Segments[st.lastSeg].First); err != nil {
			st.mu.Unlock()
			return "", err
		}
	}
	run.Resume()
	st.mu.Unlock()
	wctx, cancel := context.WithTimeout(ctx, s.t.SegmentWait)
	defer cancel()
	if err := s.waitPiece(wctx, st, run, run.Spec.StartPiece); err != nil {
		return "", err
	}
	if _, err := os.Stat(st.initPath()); err != nil {
		return "", ErrSegmentNotReady
	}
	return st.initPath(), nil
}

// MediaPlaylist is the session's HLS media playlist.
func (s *Service) MediaPlaylist(sess Session) (string, error) {
	st := s.stream(sess.ID)
	if st == nil {
		return "", ErrNoSuchSegment
	}
	return st.layout.MediaPlaylist(), nil
}

// dropBehind deletes segments more than KeepBehind before the last
// requested one (R27). Callers hold st.mu.
func (s *Service) dropBehind(st *hlsStream) {
	cutoff := st.layout.Segments[st.lastSeg].StartMs - s.t.KeepBehind
	for st.keptFrom < st.lastSeg && st.layout.Segments[st.keptFrom].EndMs <= cutoff {
		removeSegment(st, st.layout.Segments[st.keptFrom])
		st.keptFrom++
	}
}

func removeSegment(st *hlsStream, seg Segment) (freed int64) {
	for p := seg.First; p < seg.Last; p++ {
		if info, err := os.Stat(st.piece(p)); err == nil && os.Remove(st.piece(p)) == nil {
			freed += info.Size()
		}
	}
	return freed
}

// housekeep runs every tick: throttling (R15), idle stops (R28), slot
// release (R26) and the storage cap (R27).
func (s *Service) housekeep() {
	now := s.o.Now()
	s.mu.Lock()
	streams := make([]*hlsStream, 0, len(s.streams))
	for _, st := range s.streams {
		streams = append(streams, st)
	}
	s.mu.Unlock()

	var total int64
	for _, st := range streams {
		lastActive, ok := s.manager.Activity(st.sessionID)
		st.mu.Lock()
		if run := st.alive(); run != nil {
			switch ahead := st.layout.PieceStartMs[max(run.Produced(), run.Spec.StartPiece)] - st.layout.Segments[st.lastSeg].EndMs; {
			case now.Sub(st.lastSegAt) >= s.t.IdleStop:
				st.run = nil
				s.o.Log.Info("ffmpeg run idle, stopping", "playback_id", st.sessionID, "run", run.Spec.RunID)
				go run.Stop()
			case ahead > s.t.ThrottleAhead:
				run.Pause()
			case ahead < s.t.ResumeAhead:
				run.Resume()
			}
			s.checkSpeed(st, run, now)
		}
		if st.slot && st.alive() == nil && (!ok || now.Sub(lastActive) >= s.t.SlotIdle) {
			st.slot = false
			s.releaseSlot()
			s.o.Log.Info("transcode slot released after inactivity", "playback_id", st.sessionID)
		}
		st.mu.Unlock()
		total += dirSize(st.dir)
	}
	s.enforceCap(streams, total)
}

// slowSpeed is the transcode speed under which a run is flagged (NFR).
const slowSpeed = 1.2

// checkSpeed logs, once per run, a transcode that doesn't keep up with
// real time after its first 15 seconds. A throttled run is fast enough
// by definition. Callers hold st.mu.
func (s *Service) checkSpeed(st *hlsStream, run *Run, now time.Time) {
	if st.decision.Method != Transcode || st.slowRun == run.Spec.RunID || run.Paused() || now.Sub(run.Started) < 15*time.Second {
		return
	}
	if speed := run.Speed(); speed > 0 && speed < slowSpeed {
		st.slowRun = run.Spec.RunID
		accel := "software"
		if run.Spec.HW != nil {
			accel = run.Spec.HW.Accel
		}
		s.o.Log.Warn("transcode slower than real time", "code", "transcode_too_slow", "playback_id", st.sessionID,
			"run", run.Spec.RunID, "speed", speed, "accel", accel, "width", st.decision.Width, "height", st.decision.Height)
	}
}

// Speed is the transcode speed of a session's running FFmpeg (1.0 is
// real time), for the admin list (R29); false when nothing runs.
func (s *Service) Speed(sessionID string) (float64, bool) {
	st := s.stream(sessionID)
	if st == nil {
		return 0, false
	}
	st.mu.Lock()
	run := st.alive()
	st.mu.Unlock()
	if run == nil {
		return 0, false
	}
	speed := run.Speed()
	return speed, speed > 0
}

// enforceCap deletes the segments furthest behind each session's position
// until the folder fits its cap; if that isn't enough, new transcodes are
// refused until it does (R27).
func (s *Service) enforceCap(streams []*hlsStream, total int64) {
	limit := s.config().TranscodeDirMaxBytes
	if limit == 0 {
		limit = s.storage.DefaultCap()
	}
	if total > limit {
		type behind struct {
			st       *hlsStream
			seg, gap int
		}
		var old []behind
		for _, st := range streams {
			st.mu.Lock()
			for k := st.keptFrom; k < st.lastSeg; k++ {
				old = append(old, behind{st, k, st.lastSeg - k})
			}
			st.mu.Unlock()
		}
		slices.SortFunc(old, func(a, b behind) int { return b.gap - a.gap })
		for _, b := range old {
			if total <= limit {
				break
			}
			b.st.mu.Lock()
			if b.seg < b.st.lastSeg {
				total -= removeSegment(b.st, b.st.layout.Segments[b.seg])
				if b.seg == b.st.keptFrom {
					b.st.keptFrom++
				}
			}
			b.st.mu.Unlock()
		}
	}
	full := total > limit
	s.mu.Lock()
	if full != s.full {
		if full {
			s.o.Log.Warn("transcode folder full: new transcodes are refused", "code", "transcode_storage_full",
				"size", formatBytes(total), "cap", formatBytes(limit))
		} else {
			s.o.Log.Info("transcode folder back under its cap", "size", formatBytes(total), "cap", formatBytes(limit))
		}
	}
	s.full = full
	s.mu.Unlock()
}

func (s *Service) storageFull() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.full
}

// maxTranscodes is the setting, or its default.
func (s *Service) maxTranscodes() int {
	if n := s.config().MaxTranscodes; n > 0 {
		return n
	}
	return DefaultMaxTranscodes(cpuCount(), s.Hardware().Active != "")
}

func (s *Service) acquireSlot() bool {
	limit := s.maxTranscodes()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.slots >= limit {
		return false
	}
	s.slots++
	return true
}

func (s *Service) releaseSlot() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slots = max(0, s.slots-1)
}

// MasterPlaylist is the session's HLS master playlist: one variant, and
// the text subtitles as renditions (R11, R16).
func MasterPlaylist(sess Session) string {
	d := sess.Decision
	m := d.Media
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-INDEPENDENT-SEGMENTS\n")
	names := map[string]bool{}
	subs := false
	for _, sub := range m.Subtitles {
		if !textFormats[sub.Format()] {
			continue
		}
		subs = true
		name := sub.Title
		if name == "" {
			name = sub.Language
		}
		if name == "" || name == "und" {
			name = "Subtitles"
		}
		if sub.Forced {
			name += " (forced)"
		}
		if names[name] {
			name += " " + strconv.Itoa(sub.Index)
		}
		names[name] = true
		selected := d.Subtitle != nil && d.Subtitle.Index == sub.Index && d.SubtitleDelivery != DeliveryBurnIn
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=%q,DEFAULT=%s,AUTOSELECT=YES,FORCED=%s,",
			strings.ReplaceAll(name, `"`, "'"), yesNo(selected), yesNo(sub.Forced))
		if lang := hlsLanguage(sub.Language); lang != "" {
			fmt.Fprintf(&b, "LANGUAGE=%q,", lang)
		}
		fmt.Fprintf(&b, "URI=\"subtitles/%d.m3u8\"\n", sub.Index)
	}
	width, height, bandwidth := m.Width(), m.Height(), m.Bitrate
	if d.Method == Transcode {
		width, height, bandwidth = d.Width, d.Height, d.VideoBitrate+audioBitrate(d)
	}
	if bandwidth <= 0 {
		bandwidth = 20_000_000
	}
	fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d", bandwidth)
	if width > 0 && height > 0 {
		fmt.Fprintf(&b, ",RESOLUTION=%dx%d", width, height)
	}
	if codecs := codecsOf(d); codecs != "" {
		fmt.Fprintf(&b, ",CODECS=%q", codecs)
	}
	if subs {
		b.WriteString(",SUBTITLES=\"subs\"")
	}
	b.WriteString("\nvideo/index.m3u8\n")
	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "YES"
	}
	return "NO"
}

// Width and Height are the source video's size.
func (m Media) Width() int {
	if m.Video == nil {
		return 0
	}
	return m.Video.Width
}

// Height is the source video's height.
func (m Media) Height() int {
	if m.Video == nil {
		return 0
	}
	return m.Video.Height
}

func audioBitrate(d Decision) int64 {
	switch {
	case d.Audio == nil:
		return 0
	case d.AudioCopy:
		return 640_000
	case d.AudioCodec == "ac3" || d.AudioCodec == "eac3":
		return 448_000
	case d.AudioChannels > 2:
		return 384_000
	}
	return 192_000
}

// codecsOf is the variant's CODECS attribute (RFC 6381), "" when a codec
// can't be named, which players accept.
func codecsOf(d Decision) string {
	var v string
	if d.Method == Transcode {
		v = "avc1.640029"
		if d.Width > 1920 || d.Height > 1080 {
			v = "avc1.640033"
		}
	} else if v = sourceVideoCodec(d.Media); v == "" {
		return ""
	}
	if d.Audio == nil {
		return v
	}
	codec := d.AudioCodec
	if d.AudioCopy {
		codec = d.Audio.Codec
	}
	a, ok := map[string]string{"aac": "mp4a.40.2", "ac3": "ac-3", "eac3": "ec-3", "mp3": "mp4a.40.34",
		"opus": "Opus", "flac": "fLaC", "alac": "alac"}[codec]
	if !ok {
		return ""
	}
	return v + "," + a
}

func sourceVideoCodec(m Media) string {
	if m.Video == nil || m.Video.Level <= 0 {
		return ""
	}
	v := m.Video
	switch v.Codec {
	case "h264":
		p, ok := map[string]string{"High": "6400", "Main": "4D40", "Baseline": "4200", "Constrained Baseline": "42E0", "High 10": "6E00"}[v.Profile]
		if !ok {
			return ""
		}
		return fmt.Sprintf("avc1.%s%02X", p, v.Level)
	case "hevc":
		switch v.Profile {
		case "Main":
			return fmt.Sprintf("hvc1.1.6.L%d.B0", v.Level)
		case "Main 10":
			return fmt.Sprintf("hvc1.2.4.L%d.B0", v.Level)
		}
	case "av1":
		depth := max(v.BitDepth, 8)
		return fmt.Sprintf("av01.0.%02dM.%02d", v.Level, depth)
	}
	return ""
}

// hlsLanguage turns ISO 639-2 codes into the RFC 5646 tags HLS wants,
// for the common ones; others stay as they are.
func hlsLanguage(code string) string {
	if code == "" || code == "und" {
		return ""
	}
	if two, ok := map[string]string{
		"eng": "en", "fre": "fr", "fra": "fr", "ger": "de", "deu": "de", "spa": "es", "ita": "it", "por": "pt",
		"jpn": "ja", "chi": "zh", "zho": "zh", "kor": "ko", "rus": "ru", "dut": "nl", "nld": "nl", "swe": "sv",
		"nor": "no", "dan": "da", "fin": "fi", "pol": "pl", "ara": "ar", "heb": "he", "hin": "hi", "tur": "tr",
	}[code]; ok {
		return two
	}
	return code
}
