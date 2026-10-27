package playback

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestFindFFmpeg(t *testing.T) {
	if _, err := FindFFmpeg(context.Background(), "/nonexistent/ffmpeg"); !errors.Is(err, ErrFFmpegNotFound) {
		t.Errorf("missing binary: %v", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	f, err := FindFFmpeg(context.Background(), "")
	if err != nil || f.Version == "" {
		t.Errorf("ffmpeg = %+v (%v)", f, err)
	}
}
