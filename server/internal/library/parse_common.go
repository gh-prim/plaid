package library

import (
	"regexp"
	"strings"
	"time"
	"unicode"
)

// releaseTags are the tokens that end a title in scene-style names
// (resolution, source, codec, audio, release flags). Lowercase.
var releaseTags = map[string]bool{
	"2160p": true, "1080p": true, "1080i": true, "720p": true, "576p": true, "480p": true,
	"4k": true, "uhd": true,
	"bluray": true, "blu-ray": true, "bdrip": true, "brrip": true, "bdremux": true,
	"web-dl": true, "webdl": true, "webrip": true, "web-rip": true, "hdtv": true, "hdrip": true,
	"dvdrip": true, "dvdscr": true, "dvd": true, "remux": true,
	"x264": true, "x265": true, "h264": true, "h265": true, "h.264": true, "h.265": true,
	"hevc": true, "avc": true, "xvid": true, "divx": true, "10bit": true, "8bit": true,
	"hdr": true, "hdr10": true, "dv": true, "dovi": true,
	"aac": true, "ac3": true, "eac3": true, "dts": true, "dts-hd": true, "truehd": true,
	"atmos": true, "flac": true, "dd5.1": true, "ddp5.1": true,
	"proper": true, "repack": true, "internal": true, "limited": true, "multi": true,
	"vostfr": true, "truefrench": true, "subbed": true, "dubbed": true,
	"amzn": true, "nf": true, "dsnp": true, "hmax": true, "hulu": true,
}

func isSeparator(r byte) bool {
	switch r {
	case ' ', '.', '_', '-', '[', ']', '(', ')':
		return true
	}
	return false
}

type token struct {
	start, end int
}

// tokens splits s on separators, keeping byte offsets.
func tokens(s string) []token {
	var out []token
	start := -1
	for i := 0; i < len(s); i++ {
		if isSeparator(s[i]) {
			if start >= 0 {
				out = append(out, token{start, i})
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, token{start, len(s)})
	}
	return out
}

// firstReleaseTag returns the offset of the first release tag in s, or -1.
func firstReleaseTag(s string) int {
	toks := tokens(s)
	for i, t := range toks {
		if releaseTags[strings.ToLower(s[t.start:t.end])] {
			return t.start
		}
		// Tags spanning a separator: "WEB-DL", "H.264", "DD5.1".
		if i+1 < len(toks) && toks[i+1].start == t.end+1 &&
			releaseTags[strings.ToLower(s[t.start:toks[i+1].end])] {
			return t.start
		}
	}
	return -1
}

// yearMatch is a plausible year found in a name.
type yearMatch struct {
	year       int
	start, end int // offsets of the year, brackets included when present
}

// findYears returns every standalone 4-digit number between 1900 and next
// year, in order.
func findYears(s string) []yearMatch {
	maxYear := time.Now().Year() + 1
	var out []yearMatch
	for i := 0; i+4 <= len(s); i++ {
		if !isDigits(s[i : i+4]) {
			continue
		}
		if i > 0 && isAlnum(s[i-1]) || i+4 < len(s) && isAlnum(s[i+4]) {
			continue
		}
		y := atoi(s[i : i+4])
		if y < 1900 || y > maxYear {
			continue
		}
		m := yearMatch{year: y, start: i, end: i + 4}
		if i > 0 && i+4 < len(s) && (s[i-1] == '(' && s[i+4] == ')' || s[i-1] == '[' && s[i+4] == ']') {
			m.start, m.end = i-1, i+5
		}
		out = append(out, m)
		i += 3
	}
	return out
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

func isAlnum(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

var spaces = regexp.MustCompile(`\s+`)

// cleanTitle turns separators into spaces and trims leftover punctuation.
// Dots are separators only when the name has no spaces ("The.Matrix"), so
// "Mr. & Mrs. Smith" keeps its dots.
func cleanTitle(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if !strings.Contains(s, " ") {
		s = strings.ReplaceAll(s, ".", " ")
	}
	s = spaces.ReplaceAllString(s, " ")
	return strings.Trim(s, " -([{,")
}

// normTitle is the grouping key of a title: lowercase letters and digits,
// single spaces.
func normTitle(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else if r != '\'' {
			space = true
		}
	}
	return b.String()
}

// sortTitle is the default sort key until F005 brings metadata.
func sortTitle(s string) string { return normTitle(s) }

// seasonFolder matches season folder names in the languages listed by R9.
var seasonFolder = regexp.MustCompile(`(?i)^(?:season|saison|staffel|temporada|stagione|seizoen|sezon|säsong|sæson|kausi|сезон|s)[ ._-]*(\d{1,4})$`)

var specialsFolder = regexp.MustCompile(`(?i)^specials?$`)

// parseSeasonFolder returns the season number of a folder name, or -1.
func parseSeasonFolder(name string) int {
	name = strings.TrimSpace(name)
	if specialsFolder.MatchString(name) {
		return 0
	}
	if m := seasonFolder.FindStringSubmatch(name); m != nil {
		return atoi(m[1])
	}
	return -1
}

// language is one entry of the language table: ISO 639-2/B code, the 639-2/T
// code when it differs, the 639-1 code and English names.
type language struct {
	b, t, one string
	names     []string
}

var languages = []language{
	{"eng", "", "en", []string{"english"}},
	{"fre", "fra", "fr", []string{"french", "francais", "français"}},
	{"ger", "deu", "de", []string{"german", "deutsch"}},
	{"spa", "", "es", []string{"spanish", "espanol", "español", "castilian"}},
	{"ita", "", "it", []string{"italian", "italiano"}},
	{"por", "", "pt", []string{"portuguese", "português"}},
	{"dut", "nld", "nl", []string{"dutch", "flemish", "nederlands"}},
	{"swe", "", "sv", []string{"swedish", "svenska"}},
	{"nor", "", "no", []string{"norwegian", "norsk"}},
	{"nob", "", "nb", []string{"norwegian bokmal", "bokmal"}},
	{"dan", "", "da", []string{"danish", "dansk"}},
	{"fin", "", "fi", []string{"finnish", "suomi"}},
	{"ice", "isl", "is", []string{"icelandic"}},
	{"pol", "", "pl", []string{"polish", "polski"}},
	{"cze", "ces", "cs", []string{"czech"}},
	{"slo", "slk", "sk", []string{"slovak"}},
	{"slv", "", "sl", []string{"slovenian", "slovene"}},
	{"hun", "", "hu", []string{"hungarian", "magyar"}},
	{"rum", "ron", "ro", []string{"romanian"}},
	{"bul", "", "bg", []string{"bulgarian"}},
	{"hrv", "", "hr", []string{"croatian"}},
	{"srp", "", "sr", []string{"serbian"}},
	{"bos", "", "bs", []string{"bosnian"}},
	{"mac", "mkd", "mk", []string{"macedonian"}},
	{"alb", "sqi", "sq", []string{"albanian"}},
	{"gre", "ell", "el", []string{"greek"}},
	{"tur", "", "tr", []string{"turkish"}},
	{"rus", "", "ru", []string{"russian"}},
	{"ukr", "", "uk", []string{"ukrainian"}},
	{"bel", "", "be", []string{"belarusian"}},
	{"est", "", "et", []string{"estonian"}},
	{"lav", "", "lv", []string{"latvian"}},
	{"lit", "", "lt", []string{"lithuanian"}},
	{"ara", "", "ar", []string{"arabic"}},
	{"heb", "", "he", []string{"hebrew"}},
	{"per", "fas", "fa", []string{"persian", "farsi"}},
	{"hin", "", "hi", []string{"hindi"}},
	{"ben", "", "bn", []string{"bengali"}},
	{"tam", "", "ta", []string{"tamil"}},
	{"tel", "", "te", []string{"telugu"}},
	{"urd", "", "ur", []string{"urdu"}},
	{"tha", "", "th", []string{"thai"}},
	{"vie", "", "vi", []string{"vietnamese"}},
	{"ind", "", "id", []string{"indonesian"}},
	{"may", "msa", "ms", []string{"malay"}},
	{"fil", "", "", []string{"filipino"}},
	{"tgl", "", "tl", []string{"tagalog"}},
	{"chi", "zho", "zh", []string{"chinese", "mandarin", "cantonese"}},
	{"jpn", "", "ja", []string{"japanese"}},
	{"kor", "", "ko", []string{"korean"}},
	{"cat", "", "ca", []string{"catalan"}},
	{"baq", "eus", "eu", []string{"basque"}},
	{"glg", "", "gl", []string{"galician"}},
	{"wel", "cym", "cy", []string{"welsh"}},
	{"gle", "", "ga", []string{"irish"}},
	{"geo", "kat", "ka", []string{"georgian"}},
	{"arm", "hye", "hy", []string{"armenian"}},
	{"lat", "", "la", []string{"latin"}},
	{"afr", "", "af", []string{"afrikaans"}},
	{"swa", "", "sw", []string{"swahili"}},
}

var languageIndex = func() map[string]string {
	m := map[string]string{}
	for _, l := range languages {
		m[l.b] = l.b
		if l.t != "" {
			m[l.t] = l.b
		}
		if l.one != "" {
			m[l.one] = l.b
		}
		for _, n := range l.names {
			m[n] = l.b
		}
	}
	return m
}()

// normalizeLanguage maps an ISO 639-1, 639-2/T, 639-2/B code or an English
// language name to ISO 639-2/B. Region suffixes ("pt-BR", "en_US") are
// ignored. ok is false when the token isn't a known language.
func normalizeLanguage(s string) (code string, ok bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-_"); i > 0 {
		s = s[:i]
	}
	code, ok = languageIndex[s]
	return code, ok
}

// streamLanguage normalizes a probe language tag, 'und' when unknown.
func streamLanguage(s string) string {
	if code, ok := normalizeLanguage(s); ok {
		return code
	}
	return "und"
}
