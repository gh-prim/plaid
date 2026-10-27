package library

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// fsMagic maps statfs(2) f_type values to names (see statfs(2), linux/magic.h).
var fsMagic = map[int64]string{
	0xEF53:     "ext4", // also ext2/ext3
	0x9123683E: "btrfs",
	0x58465342: "xfs",
	0x2FC12FC1: "zfs",
	0x01021994: "tmpfs",
	0x794C7630: "overlay",
	0xF2F52010: "f2fs",
	0x4D44:     "vfat",
	0x2011BAB0: "exfat",
	0x5346544E: "ntfs",
	0x7366746E: "ntfs3",
	0x6969:     "nfs",
	0x517B:     "smb",
	0xFF534D42: "cifs",
	0xFE534D42: "smb2",
	0x65735546: "fuse",
	0x01021997: "9p",
}

// fsType returns the file system type of path, "" when unknown.
func fsType(path string) string {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return ""
	}
	if name, ok := fsMagic[int64(st.Type)]; ok { //nolint:unconvert // int32 on some architectures
		return name
	}
	return "0x" + strconv.FormatInt(int64(st.Type), 16) //nolint:unconvert // same
}

// inotifyLimit reads the per-user inotify watch limit, 0 when unknown.
func inotifyLimit() int {
	b, err := os.ReadFile("/proc/sys/fs/inotify/max_user_watches")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}
