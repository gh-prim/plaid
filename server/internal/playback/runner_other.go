//go:build !linux

package playback

import (
	"os/exec"
	"syscall"
)

// procAttr puts FFmpeg in its own process group. Outside Linux there's no
// Pdeathsig: if PLAID is killed, FFmpeg runs on until its output ends or
// the next start empties the transcode folder (documented in the plan).
func procAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

func spawn(cmd *exec.Cmd) error { return cmd.Start() }
