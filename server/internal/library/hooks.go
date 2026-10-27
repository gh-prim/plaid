package library

import "time"

// Hooks let F005 (metadata) follow what the scanner does without the
// library package knowing about metadata. They're called from the scanning
// goroutine after the scan's writes are committed, and must return quickly.
// A nil hook is skipped.
type Hooks struct {
	// ItemsCreated receives the ids of the items (any kind) a scan created.
	// It's a hint: F005 also sweeps for items without metadata at startup,
	// so items from a scan that failed half-way aren't lost (F005 R1).
	ItemsCreated func(libraryID string, itemIDs []string)
	// Sidecars receives, for each folder a scan walked, the .nfo and image
	// files found in it. Only folders that were available are reported, so
	// a file absent from the list is really gone (F005 R14).
	Sidecars func(libraryID, folderPath string, files []Sidecar)
}

// Sidecar is a metadata file next to the media: .nfo or an image.
type Sidecar struct {
	Path    string // absolute
	Rel     string // slash-separated, relative to the library folder
	Size    int64
	ModTime time.Time
}

// sidecarExts are the extensions reported as sidecars.
var sidecarExts = map[string]bool{".nfo": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// IsSidecar reports whether name is a metadata file F005 reads.
func IsSidecar(name string) bool { return sidecarExts[lowerExt(name)] }
