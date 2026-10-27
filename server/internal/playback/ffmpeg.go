package playback

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrFFmpegNotFound is returned by FindFFmpeg when there is no ffmpeg.
var ErrFFmpegNotFound = errors.New("ffmpeg not found")

// FFmpeg is the ffmpeg binary playback runs (ADR 0006).
type FFmpeg struct {
	Path    string
	Version string
	// Filters lists the filters this build has: distro builds differ
	// (Homebrew's has no libass "subtitles" filter).
	Filters map[string]bool
}

// HasFilter reports whether the build has a filter.
func (f *FFmpeg) HasFilter(name string) bool { return f != nil && f.Filters[name] }

// FindFFmpeg locates ffmpeg: the configured path (PLAID_FFMPEG_PATH) when
// set, else PATH, and reads its version.
func FindFFmpeg(ctx context.Context, configured string) (*FFmpeg, error) {
	name := "ffmpeg"
	if configured != "" {
		name = configured
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFFmpegNotFound, err)
	}
	vctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(vctx, path, "-hide_banner", "-version").Output() //nolint:gosec // admin-configured ffmpeg binary
	if err != nil {
		return nil, fmt.Errorf("%w: %s -version: %w", ErrFFmpegNotFound, path, err)
	}
	f := &FFmpeg{Path: path, Filters: map[string]bool{}}
	// "ffmpeg version 7.1.1-1 Copyright (c) ..."
	if fields := strings.Fields(string(out)); len(fields) >= 3 && fields[1] == "version" {
		f.Version = fields[2]
	}
	// " TS overlay          VV->V      Overlay a video source on top of the input."
	if out, err := exec.CommandContext(vctx, path, "-hide_banner", "-filters").Output(); err == nil { //nolint:gosec // same binary
		for _, line := range strings.Split(string(out), "\n") {
			if fields := strings.Fields(line); len(fields) >= 3 && strings.Contains(fields[2], "->") {
				f.Filters[fields[1]] = true
			}
		}
	}
	return f, nil
}
