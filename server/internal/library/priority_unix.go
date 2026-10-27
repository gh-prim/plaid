//go:build linux || darwin

package library

import (
	"errors"
	"log/slog"
	"sync"

	"golang.org/x/sys/unix"
)

var priorityWarnOnce sync.Once

// lowerPriority renices a probe so playback keeps the CPU. The hardened
// systemd unit forbids setpriority (~@resources): EPERM is logged once at
// debug and ignored; the worker cap is the real protection.
func lowerPriority(pid int, log *slog.Logger) {
	if err := unix.Setpriority(unix.PRIO_PROCESS, pid, 10); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			priorityWarnOnce.Do(func() { log.Debug("can't lower ffprobe priority", "err", err) })
		}
	}
}
