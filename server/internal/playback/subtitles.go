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
)

// ErrNoSuchSubtitle is a subtitle index or format the session's file
// doesn't have (or that can't be served as text).
var ErrNoSuchSubtitle = errors.New("no such subtitle")

// textFormats are the subtitle formats FFmpeg converts to WebVTT.
var textFormats = map[string]bool{"srt": true, "ass": true, "vtt": true, "tx3g": true}

// subtitleBuildTimeout bounds one conversion: an embedded track means
// reading the whole file once.
const subtitleBuildTimeout = 5 * time.Minute

// SubtitleFile returns a session's text subtitle track as WebVTT, or an
// ASS/SSA track as is, converted once with FFmpeg and cached by file,
// mtime and track (R16).
func (s *Service) SubtitleFile(ctx context.Context, sess Session, index int, format string) (string, error) {
	m := sess.Decision.Media
	sub := m.subtitle(index)
	switch {
	case sub == nil, format == SubtitleVTT && !textFormats[sub.Format()], format == SubtitleASS && sub.Format() != "ass",
		format != SubtitleVTT && format != SubtitleASS:
		return "", ErrNoSuchSubtitle
	case s.o.FFmpeg == nil:
		return "", ErrFFmpegMissing
	}
	src, mtime, mapping := m.Path, m.ModTime, stream(sub.Index)
	if sub.ExternalPath != "" {
		st, err := os.Stat(sub.ExternalPath)
		if err != nil {
			return "", ErrFileUnavailable
		}
		src, mtime, mapping = sub.ExternalPath, st.ModTime(), "0:s:0"
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%d\x00%s", m.FileID, src, mtime.UnixNano(), mapping)
	name := hex.EncodeToString(h.Sum(nil))[:32] + "." + format
	return s.subtitles.Get(ctx, name, func(ctx context.Context, dst string) error {
		ctx, cancel := context.WithTimeout(ctx, subtitleBuildTimeout)
		defer cancel()
		codec, muxer := "webvtt", "webvtt"
		if format == SubtitleASS {
			codec, muxer = "copy", "ass"
		}
		args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y", "-i", src,
			"-map", mapping, "-c:s", codec, "-f", muxer, dst}
		start := time.Now()
		out, err := exec.CommandContext(ctx, s.o.FFmpeg.Path, args...).CombinedOutput() //nolint:gosec // argument list
		if err != nil {
			return fmt.Errorf("converting subtitles: %w: %s", err, strings.TrimSpace(string(out)))
		}
		s.o.Log.Info("subtitles converted", "file_id", m.FileID, "stream", sub.Index, "format", format,
			"duration_ms", time.Since(start).Milliseconds())
		return nil
	})
}

// SubtitleURL is where a subtitle delivered separately is fetched: the
// WebVTT route, for every text track (R16).
func SubtitleURL(s Session, sub Subtitle) string {
	if !textFormats[sub.Format()] {
		return ""
	}
	return StreamPrefix + s.token + "/subtitles/" + strconv.Itoa(sub.Index) + ".vtt"
}
