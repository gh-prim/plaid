package library

import (
	"path/filepath"
	"sort"
	"strings"
)

// externalSubtitleBase is the first media_streams.idx used for external
// subtitle files; ffprobe indexes stay below it.
const externalSubtitleBase = 1000

// ExternalSubtitle is a subtitle file next to a video (R13).
type ExternalSubtitle struct {
	Path     string
	Codec    string
	Language string // ISO 639-2/B, "und" when unknown
	Forced   bool
	SDH      bool
	Image    bool
}

// matchSubtitles links the subtitle files of one directory to its videos.
// A subtitle belongs to the video whose base name is the longest prefix of
// its own ("Movie (2019).fr.forced.srt" → "Movie (2019).mkv"). A ".sub"
// counts only with a ".idx" of the same name. Names are file names inside
// dir; the result is keyed by video name.
func matchSubtitles(dir string, videos, others []string) map[string][]ExternalSubtitle {
	idx := map[string]bool{}
	var subs []string
	for _, n := range others {
		switch ext := lowerExt(n); {
		case ext == ".idx":
			idx[strings.TrimSuffix(n, filepath.Ext(n))] = true
		case isSubtitle(n):
			subs = append(subs, n)
		}
	}
	sort.Strings(subs)

	out := map[string][]ExternalSubtitle{}
	for _, s := range subs {
		ext := lowerExt(s)
		sbase := strings.TrimSuffix(s, filepath.Ext(s))
		if ext == ".sub" && !idx[sbase] {
			continue
		}
		owner, ownerBase := "", ""
		for _, v := range videos {
			vbase := strings.TrimSuffix(v, filepath.Ext(v))
			if (sbase == vbase || strings.HasPrefix(sbase, vbase+".")) && len(vbase) > len(ownerBase) {
				owner, ownerBase = v, vbase
			}
		}
		if owner == "" {
			continue
		}
		sub := parseSubtitleSuffix(sbase[len(ownerBase):])
		sub.Path = filepath.Join(dir, s)
		sub.Codec = subtitleCodecs[ext]
		sub.Image = ext == ".sub"
		out[owner] = append(out[owner], sub)
	}
	return out
}

// parseSubtitleSuffix reads ".fr.forced", ".en.sdh", ".eng" right to left.
// "hi" means hearing impaired only when a language comes before it;
// alone it is Hindi.
func parseSubtitleSuffix(suffix string) ExternalSubtitle {
	sub := ExternalSubtitle{Language: "und"}
	var toks []string
	for _, t := range strings.Split(suffix, ".") {
		if t = strings.TrimSpace(t); t != "" {
			toks = append(toks, strings.ToLower(t))
		}
	}
	langFound := false
	for i := len(toks) - 1; i >= 0; i-- {
		t := toks[i]
		switch {
		case t == "forced":
			sub.Forced = true
		case t == "sdh" || t == "cc":
			sub.SDH = true
		case t == "hi" && i > 0 && hasLanguage(toks[:i]):
			sub.SDH = true
		case !langFound:
			if code, ok := normalizeLanguage(t); ok {
				sub.Language, langFound = code, true
			}
		}
	}
	return sub
}

func hasLanguage(toks []string) bool {
	for _, t := range toks {
		if _, ok := normalizeLanguage(t); ok {
			return true
		}
	}
	return false
}
