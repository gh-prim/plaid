package library

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Episode is what an episode path says (R9).
type Episode struct {
	Show     string
	ShowYear int // 0 when unknown
	// Season is the season number; 0 is Specials. For date-based episodes
	// without a season folder, it's the air date's year.
	Season int
	// Episodes lists every episode the file contains (R10); empty for
	// date-based episodes.
	Episodes []int
	AirDate  string // YYYY-MM-DD for date-based episodes
}

var (
	// S01E02, s1e2, S01E01E02, S01E01-E03, S01.E02.
	sxeRe   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,4})[ ._]?e(\d{1,4})((?:[ ._]?-?[ ._]?e\d{1,4})*)(?:[^0-9]|$)`)
	sxeTail = regexp.MustCompile(`(?i)(-?)[ ._]?e(\d{1,4})`)
	// 1x02
	xRe = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])(\d{1,2})x(\d{1,3})(?:[^0-9a-z]|$)`)
	// 2024-10-27, 2024.10.27
	dateRe = regexp.MustCompile(`(?:^|[^0-9])(\d{4})[-.](\d{2})[-.](\d{2})(?:[^0-9]|$)`)
)

// maxEpisodeRange bounds "S01E01-E50" style ranges.
const maxEpisodeRange = 50

// ParseEpisode parses a path relative to a Shows library folder. The show is
// the first path segment; a file directly in the library folder takes the
// show from the text before its episode number. ok is false without a show
// or an episode number.
func ParseEpisode(relPath string) (e Episode, ok bool) {
	segs := strings.Split(filepath.ToSlash(relPath), "/")
	file := segs[len(segs)-1]
	base := strings.TrimSuffix(file, filepath.Ext(file))

	e.Season = -1
	for _, seg := range segs[1:max(len(segs)-1, 1)] {
		if n := parseSeasonFolder(seg); n >= 0 {
			e.Season = n
		}
	}

	matchStart := -1
	if m := sxeRe.FindStringSubmatchIndex(base); m != nil {
		e.Season = atoi(base[m[2]:m[3]])
		first := atoi(base[m[4]:m[5]])
		e.Episodes = []int{first}
		last := first
		for _, t := range sxeTail.FindAllStringSubmatch(base[m[6]:m[7]], -1) {
			n := atoi(t[2])
			if t[1] == "-" && n > last && n-last <= maxEpisodeRange {
				for i := last + 1; i <= n; i++ {
					e.Episodes = append(e.Episodes, i)
				}
			} else if n != last {
				e.Episodes = append(e.Episodes, n)
			}
			last = n
		}
		matchStart = m[0]
	} else if m := xRe.FindStringSubmatchIndex(base); m != nil {
		e.Season = atoi(base[m[2]:m[3]])
		e.Episodes = []int{atoi(base[m[4]:m[5]])}
		matchStart = m[0]
	} else if m := dateRe.FindStringSubmatchIndex(base); m != nil {
		d := base[m[2]:m[3]] + "-" + base[m[4]:m[5]] + "-" + base[m[6]:m[7]]
		if t, err := time.Parse(time.DateOnly, d); err == nil {
			e.AirDate = d
			if e.Season < 0 {
				e.Season = t.Year()
			}
			matchStart = m[0]
		}
	}

	if len(segs) >= 2 {
		show := parseMovieName(segs[0])
		e.Show, e.ShowYear = show.Title, show.Year
	} else if matchStart > 0 {
		show := parseMovieName(base[:matchStart])
		e.Show, e.ShowYear = show.Title, show.Year
	}

	if e.Season < 0 {
		e.Season = 0
	}
	return e, e.Show != "" && (len(e.Episodes) > 0 || e.AirDate != "")
}
