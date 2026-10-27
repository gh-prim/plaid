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
}

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
	f := &FFmpeg{Path: path}
	// "ffmpeg version 7.1.1-1 Copyright (c) ..."
	if fields := strings.Fields(string(out)); len(fields) >= 3 && fields[1] == "version" {
		f.Version = fields[2]
	}
	return f, nil
}
