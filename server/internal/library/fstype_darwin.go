package library

import "golang.org/x/sys/unix"

// fsType returns the file system type of path, "" when unknown.
func fsType(path string) string {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return ""
	}
	return unix.ByteSliceToString(st.Fstypename[:])
}

// inotifyLimit is Linux only; macOS (kqueue) has no such limit.
func inotifyLimit() int { return 0 }
