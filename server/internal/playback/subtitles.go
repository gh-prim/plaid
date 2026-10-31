package playback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Subtitle formats served on their own (R16).
const (
	SubtitleVTT = "vtt"
	SubtitleASS = "ass"
	// SubtitleSUP is a PGS track as is, for clients that draw it (V2).
	SubtitleSUP = "sup"
)

// ErrNoSuchSubtitle is a subtitle index or format the session's file
// doesn't have (or that can't be served as text).
var ErrNoSuchSubtitle = errors.New("no such subtitle")

// textFormats are the subtitle formats FFmpeg converts to WebVTT.
var textFormats = map[string]bool{"srt": true, "ass": true, "vtt": true, "tx3g": true}

// subtitleBuildTimeout bounds one conversion: an embedded track means
// reading the whole file once.
const subtitleBuildTimeout = 5 * time.Minute

// subtitleWindow is how much of a track a quick extraction covers, from a
// little before the position (V2): seconds of reading instead of the
// whole file.
const (
	subtitleWindow  = 15 * time.Minute
	subtitleLead    = time.Minute
	subtitleBuckets = 5 * time.Minute
)

// SubtitleFile returns a session's text subtitle track as WebVTT, an
// ASS/SSA track as is, or a PGS track as is (.sup), converted once with
// FFmpeg and cached by file, mtime and track (R16). An embedded track
// means reading the whole file: tens of seconds over a NAS.
func (s *Service) SubtitleFile(ctx context.Context, sess Session, index int, format string) (string, error) {
	src, err := s.subtitleSource(sess, index, format)
	if err != nil {
		return "", err
	}
	return s.subtitles.Get(ctx, src.name(-1), func(ctx context.Context, dst string) error {
		return s.extractSubtitle(ctx, sess, src, -1, dst)
	})
}

// SubtitleQuick answers at once with what's around fromMs (V2, like
// Plex): the whole track when it's cached, else a 15-minute window that
// takes seconds, while the whole track is extracted in the background.
// partialUntil is the end of the window, 0 for the whole track.
func (s *Service) SubtitleQuick(ctx context.Context, sess Session, index int, format string, fromMs int64) (path string, partialUntil int64, err error) {
	src, err := s.subtitleSource(sess, index, format)
	if err != nil {
		return "", 0, err
	}
	full := src.name(-1)
	// A file next to the video is small: read it whole.
	if src.external || s.subtitles.Has(full) {
		path, err := s.SubtitleFile(ctx, sess, index, format)
		return path, 0, err
	}
	bg := context.WithoutCancel(ctx)
	go func() {
		if _, err := s.SubtitleFile(bg, sess, index, format); err != nil {
			s.o.Log.Warn("subtitles not prepared", "file_id", sess.Decision.Media.FileID, "stream", index, "err", err)
		}
	}()
	bucket := int64(subtitleBuckets / time.Millisecond)
	start := max(0, fromMs/bucket*bucket-int64(subtitleLead/time.Millisecond))
	path, err = s.subtitles.Get(ctx, src.name(start), func(ctx context.Context, dst string) error {
		return s.extractSubtitle(ctx, sess, src, start, dst)
	})
	return path, start + int64(subtitleWindow/time.Millisecond), err
}

type subtitleSource struct {
	sub      *Subtitle
	format   string
	path     string
	mapping  string
	key      string
	external bool
}

// name is the cache file: the whole track, or the window from startMs.
func (src subtitleSource) name(startMs int64) string {
	if startMs < 0 {
		return src.key + "." + src.format
	}
	return src.key + "-" + strconv.FormatInt(startMs, 10) + "." + src.format
}

func (s *Service) subtitleSource(sess Session, index int, format string) (subtitleSource, error) {
	m := sess.Decision.Media
	sub := m.subtitle(index)
	switch {
	case sub == nil, format == SubtitleVTT && !textFormats[sub.Format()], format == SubtitleASS && sub.Format() != "ass",
		format == SubtitleSUP && sub.Format() != "pgs",
		format != SubtitleVTT && format != SubtitleASS && format != SubtitleSUP:
		return subtitleSource{}, ErrNoSuchSubtitle
	case s.o.FFmpeg == nil:
		return subtitleSource{}, ErrFFmpegMissing
	}
	src := subtitleSource{sub: sub, format: format, path: m.Path, mapping: stream(sub.Index)}
	mtime := m.ModTime
	if sub.ExternalPath != "" {
		st, err := os.Stat(sub.ExternalPath)
		if err != nil {
			return subtitleSource{}, ErrFileUnavailable
		}
		src.path, src.mapping, src.external = sub.ExternalPath, "0:s:0", true
		mtime = st.ModTime()
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%d\x00%s", m.FileID, src.path, mtime.UnixNano(), src.mapping)
	src.key = hex.EncodeToString(h.Sum(nil))[:32]
	return src, nil
}

// extractSubtitle runs FFmpeg for the whole track (startMs < 0) or for a
// window from startMs, keeping the file's own timestamps.
func (s *Service) extractSubtitle(ctx context.Context, sess Session, src subtitleSource, startMs int64, dst string) error {
	ctx, cancel := context.WithTimeout(ctx, subtitleBuildTimeout)
	defer cancel()
	codec, muxer := "webvtt", "webvtt"
	switch src.format {
	case SubtitleASS:
		codec, muxer = "copy", "ass"
	case SubtitleSUP:
		codec, muxer = "copy", "sup"
	}
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y"}
	if startMs >= 0 {
		args = append(args, "-ss", strconv.FormatFloat(float64(startMs)/1000, 'f', 3, 64), "-copyts")
	}
	args = append(args, "-i", src.path, "-map", src.mapping)
	if startMs >= 0 {
		args = append(args, "-t", strconv.FormatFloat(subtitleWindow.Seconds(), 'f', 0, 64))
	}
	args = append(args, "-c:s", codec, "-f", muxer, dst)
	start := time.Now()
	out, err := exec.CommandContext(ctx, s.o.FFmpeg.Path, args...).CombinedOutput() //nolint:gosec // argument list
	if err != nil {
		return fmt.Errorf("converting subtitles: %w: %s", err, strings.TrimSpace(string(out)))
	}
	s.o.Log.Info("subtitles converted", "file_id", sess.Decision.Media.FileID, "stream", src.sub.Index, "format", src.format,
		"window_start_ms", startMs, "duration_ms", time.Since(start).Milliseconds())
	return nil
}

// SubtitleURL is where a subtitle delivered separately is fetched: the
// WebVTT route for every text track (R16), the raw PGS for a client that
// draws it (V2).
func SubtitleURL(s Session, sub Subtitle) string {
	base := StreamPrefix + s.token + "/subtitles/" + strconv.Itoa(sub.Index)
	switch {
	case textFormats[sub.Format()]:
		return base + ".vtt"
	case sub.Image && s.Profile.drawsSeparately(sub.Format()):
		return base + "." + SubtitleSUP
	}
	return ""
}

// IsTextSubtitle reports a track served as WebVTT (an HLS rendition).
func IsTextSubtitle(sub Subtitle) bool { return textFormats[sub.Format()] }
