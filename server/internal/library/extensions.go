package library

import (
	"path/filepath"
	"strings"
)

// videoExts are the extensions recognized as video files (R4).
var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".wmv": true,
	".ts": true, ".m2ts": true, ".webm": true, ".mpg": true, ".mpeg": true, ".flv": true,
	".ogv": true, ".3gp": true,
}

// subtitleCodecs maps external subtitle extensions to the codec name ffprobe
// would report (R13). ".idx" is not listed: it only qualifies a ".sub".
var subtitleCodecs = map[string]string{
	".srt": "subrip",
	".ass": "ass",
	".ssa": "ssa",
	".vtt": "webvtt",
	".sub": "dvd_subtitle",
}

func lowerExt(name string) string {
	return strings.ToLower(filepath.Ext(name))
}

// IsVideo reports whether a file name has a video extension.
func IsVideo(name string) bool { return videoExts[lowerExt(name)] }

func isSubtitle(name string) bool {
	_, ok := subtitleCodecs[lowerExt(name)]
	return ok
}
