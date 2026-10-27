package library

import "strings"

// networkFS reports whether a file system type is a network or FUSE file
// system, where change notifications aren't reliable (R18).
func networkFS(fsType string) bool {
	t := strings.ToLower(fsType)
	switch t {
	case "nfs", "nfs4", "cifs", "smb", "smb2", "smbfs", "9p", "afpfs", "webdav":
		return true
	}
	return strings.HasPrefix(t, "fuse")
}
