// Package sysstats reads what the admin dashboard shows about the machine
// (V2): processor load, temperature, model, and the space on the drives
// holding the libraries. Every reading is best effort: what the system
// doesn't expose (another OS, a container) is left out, never an error.
package sysstats

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	started = time.Now()

	cpuMu   sync.Mutex
	cpuLast [2]uint64 // busy, total jiffies at the last reading
)

// Uptime is how long PLAID has been running.
func Uptime() time.Duration { return time.Since(started) }

// CPUPercent is the processor load since the last call (the dashboard
// polls), 0–100; ok is false where /proc/stat isn't there. The first call
// samples 200 ms.
func CPUPercent() (float64, bool) {
	busy, total, ok := readCPU()
	if !ok {
		return 0, false
	}
	cpuMu.Lock()
	last := cpuLast
	cpuLast = [2]uint64{busy, total}
	cpuMu.Unlock()
	if last[1] == 0 || total <= last[1] {
		time.Sleep(200 * time.Millisecond)
		b2, t2, ok := readCPU()
		if !ok || t2 <= total {
			return 0, false
		}
		cpuMu.Lock()
		cpuLast = [2]uint64{b2, t2}
		cpuMu.Unlock()
		return 100 * float64(b2-busy) / float64(t2-total), true
	}
	return 100 * float64(busy-last[0]) / float64(total-last[1]), true
}

func readCPU() (busy, total uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	line, _, _ := strings.Cut(string(b), "\n")
	f := strings.Fields(line)
	if len(f) < 5 || f[0] != "cpu" {
		return 0, 0, false
	}
	var idle uint64
	for i, s := range f[1:] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		total += v
		if i == 3 || i == 4 { // idle, iowait
			idle += v
		}
	}
	return total - idle, total, true
}

// TemperatureC is the hottest thermal zone, in °C.
func TemperatureC() (float64, bool) {
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	best, found := 0.0, false
	for _, z := range zones {
		b, err := os.ReadFile(z) //nolint:gosec // fixed system path
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil || v <= 0 {
			continue
		}
		if c := v / 1000; !found || c > best {
			best, found = c, true
		}
	}
	return best, found
}

// Model is the machine's name (a Raspberry Pi's board, a PC's product), or "".
func Model() string {
	for _, p := range []string{"/proc/device-tree/model", "/sys/devices/virtual/dmi/id/product_name"} {
		if b, err := os.ReadFile(p); err == nil { //nolint:gosec // fixed system paths
			if s := strings.TrimSpace(strings.TrimRight(string(b), "\x00")); s != "" && s != "System Product Name" {
				return s
			}
		}
	}
	return ""
}

// Drive is the space on the device holding a folder.
type Drive struct {
	Device     uint64
	TotalBytes int64
	FreeBytes  int64
}

// DriveOf reads the drive a folder is on; false when it can't be reached.
func DriveOf(path string) (Drive, bool) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return Drive{}, false
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return Drive{}, false
	}
	bs := int64(fs.Bsize) //nolint:unconvert // uint32 on darwin, int64 on linux
	return Drive{
		Device:     uint64(st.Dev),        //nolint:unconvert,gosec // int32 on darwin; only compared
		TotalBytes: int64(fs.Blocks) * bs, //nolint:gosec // block counts fit
		FreeBytes:  int64(fs.Bavail) * bs, //nolint:gosec // block counts fit
	}, true
}
