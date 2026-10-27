//go:build !linux && !darwin

package library

// fsType is unknown on this platform: folders are watched.
func fsType(string) string { return "" }

// inotifyLimit is Linux only.
func inotifyLimit() int { return 0 }
