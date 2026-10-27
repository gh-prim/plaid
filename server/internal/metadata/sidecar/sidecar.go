// Package sidecar classifies the .nfo and image files found next to the
// media (F005 R12, R13): what kind of artwork a file is, and whether it
// belongs to the folder's item, to one video, or to a season.
package sidecar

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Kinds, as the artwork table stores them, plus Nfo.
const (
	Nfo      = "nfo"
	Poster   = "poster"
	Backdrop = "backdrop"
	Logo     = "logo"
	Clearart = "clearart"
	Banner   = "banner"
	Thumb    = "thumb" // a movie's landscape, or an episode's still
)

// Scopes: what the file belongs to.
const (
	// ScopeFolder: the movie or show whose folder holds the file. In a
	// season folder, a poster is the season's.
	ScopeFolder = "folder"
	// ScopeVideo: the video named Base in the same folder.
	ScopeVideo = "video"
	// ScopeSeason: season Season of the show whose folder holds the file.
	ScopeSeason = "season"
	// ScopeShow: tvshow.nfo.
	ScopeShow = "show"
)

// Class is what a sidecar file is. Kind is "" for a file PLAID ignores.
type Class struct {
	Kind   string
	Scope  string
	Base   string // ScopeVideo: the video's name without extension
	Season int    // ScopeSeason: 0 for specials
}

var folderImages = map[string]string{
	"poster": Poster, "folder": Poster, "cover": Poster,
	"fanart": Backdrop, "backdrop": Backdrop, "background": Backdrop,
	"logo": Logo, "clearlogo": Logo,
	"clearart":  Clearart,
	"banner":    Banner,
	"landscape": Thumb, "thumb": Thumb,
}

var videoSuffixes = map[string]string{
	"poster": Poster, "fanart": Backdrop, "thumb": Thumb, "landscape": Thumb,
	"logo": Logo, "clearlogo": Logo, "clearart": Clearart, "banner": Banner,
}

var seasonPoster = regexp.MustCompile(`^season(\d{1,3}|-specials)-poster$`)

// Classify looks at a file name (any directories are ignored).
func Classify(file string) Class {
	name := path.Base(strings.ReplaceAll(file, "\\", "/"))
	ext := strings.ToLower(path.Ext(name))
	base := strings.TrimSuffix(name, path.Ext(name))
	lower := strings.ToLower(base)

	if ext == ".nfo" {
		switch lower {
		case "movie":
			return Class{Kind: Nfo, Scope: ScopeFolder}
		case "tvshow":
			return Class{Kind: Nfo, Scope: ScopeShow}
		}
		return Class{Kind: Nfo, Scope: ScopeVideo, Base: base}
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return Class{}
	}
	if kind, ok := folderImages[lower]; ok {
		return Class{Kind: kind, Scope: ScopeFolder}
	}
	if m := seasonPoster.FindStringSubmatch(lower); m != nil {
		n := 0
		if m[1] != "-specials" {
			n, _ = strconv.Atoi(m[1])
		}
		return Class{Kind: Poster, Scope: ScopeSeason, Season: n}
	}
	if i := strings.LastIndexByte(base, '-'); i > 0 {
		if kind, ok := videoSuffixes[strings.ToLower(base[i+1:])]; ok {
			return Class{Kind: kind, Scope: ScopeVideo, Base: base[:i]}
		}
	}
	return Class{}
}
