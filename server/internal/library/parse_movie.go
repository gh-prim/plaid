package library

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Movie is what a movie file name says (R7).
type Movie struct {
	Title   string
	Year    int // 0 when unknown
	Edition string
	// Hints are external ids kept for F005: "tmdb", "imdb", "tvdb".
	Hints map[string]string
	// Version is the free label after " - " ("Movie (2019) - 1080p").
	Version string
}

var (
	braceTag   = regexp.MustCompile(`(?i)\{(edition|tmdb|imdb|tvdb)-([^}]*)\}`)
	bracketTag = regexp.MustCompile(`(?i)\[(tmdbid|imdbid|tvdbid)[-=]([^\]]*)\]`)
	leadingTag = regexp.MustCompile(`^\[[^\]]*\]\s*`)
	editionRe  = regexp.MustCompile(`(?i)(?:^|[^\pL])(director'?s[ ._-]cut|extended(?:[ ._-](?:edition|cut))?|unrated|theatrical(?:[ ._-](?:cut|edition))?|remastered|imax)(?:[^\pL]|$)`)
)

// editionLabel maps the edition words to their canonical label.
func editionLabel(word string) string {
	w := strings.ToLower(word)
	switch {
	case strings.HasPrefix(w, "director"):
		return "Director's Cut"
	case strings.HasPrefix(w, "extended"):
		return "Extended"
	case strings.HasPrefix(w, "theatrical"):
		return "Theatrical"
	case w == "imax":
		return "IMAX"
	default:
		return strings.ToUpper(w[:1]) + w[1:]
	}
}

// ParseMovie parses a path relative to a Movies library folder. The file
// name is tried first; the parent folder fills in a missing title or year.
// ok is false when no title could be found.
func ParseMovie(relPath string) (m Movie, ok bool) {
	relPath = filepath.ToSlash(relPath)
	base := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	m = parseMovieName(base)

	if dir := filepath.Dir(relPath); dir != "." && dir != "/" {
		f := parseMovieName(filepath.Base(dir))
		if m.Title == "" || m.Year == 0 && f.Year != 0 && f.Title != "" {
			m.Title, m.Year = f.Title, f.Year
		}
		if m.Edition == "" {
			m.Edition = f.Edition
		}
		for k, v := range f.Hints {
			if _, set := m.Hints[k]; !set {
				if m.Hints == nil {
					m.Hints = map[string]string{}
				}
				m.Hints[k] = v
			}
		}
	}
	return m, m.Title != ""
}

func parseMovieName(s string) Movie {
	var m Movie
	addHint := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if m.Hints == nil {
			m.Hints = map[string]string{}
		}
		m.Hints[k] = v
	}
	for _, sm := range braceTag.FindAllStringSubmatch(s, -1) {
		switch k := strings.ToLower(sm[1]); k {
		case "edition":
			m.Edition = strings.TrimSpace(sm[2])
		default:
			addHint(k, sm[2])
		}
	}
	for _, sm := range bracketTag.FindAllStringSubmatch(s, -1) {
		addHint(strings.TrimSuffix(strings.ToLower(sm[1]), "id"), sm[2])
	}
	s = braceTag.ReplaceAllString(s, " ")
	s = bracketTag.ReplaceAllString(s, " ")
	s = strings.TrimSpace(leadingTag.ReplaceAllString(strings.TrimSpace(s), ""))

	if m.Edition == "" {
		if sm := editionRe.FindStringSubmatch(s); sm != nil {
			m.Edition = editionLabel(sm[1])
		}
	}

	title := ""
	// The last plausible year is the year, so a title that is itself a
	// year ("2001 A Space Odyssey (1968)", "1917 (2019)") keeps it.
	if years := findYears(s); len(years) > 0 {
		y := years[len(years)-1]
		t := cleanTitle(stripEdition(s[:y.start]))
		if t == "" && y.end-y.start > 4 {
			return m // only a bracketed year: no title
		}
		if t != "" {
			title, m.Year = t, y.year
			if rest := strings.TrimSpace(s[y.end:]); strings.HasPrefix(rest, "-") {
				m.Version = strings.TrimSpace(strings.TrimLeft(rest, "- "))
			}
		}
	}
	if title == "" {
		cut := firstReleaseTag(s)
		if cut < 0 {
			cut = len(s)
		}
		title = cleanTitle(stripEdition(s[:cut]))
	}
	m.Title = title
	return m
}

func stripEdition(s string) string {
	return editionRe.ReplaceAllString(s, " ")
}
