// Package nfo reads Kodi .nfo files (F005 R12): movie.nfo / <video>.nfo,
// tvshow.nfo and episode .nfo. It reads at most MaxSize bytes and accepts
// the "URL only" .nfo Kodi allows (a TMDB or IMDb link on one line), as
// well as an XML .nfo followed by such a link.
package nfo

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// MaxSize is the largest .nfo read.
const MaxSize = 1 << 20

// Kinds, from the root element.
const (
	Movie   = "movie"
	Show    = "tvshow"
	Episode = "episodedetails"
	// URLOnly is an .nfo that only holds a link.
	URLOnly = "url"
)

// ErrTooLarge rejects an .nfo over MaxSize.
var ErrTooLarge = errors.New("nfo: file over 1 MB")

// Actor is a cast credit.
type Actor struct {
	Name  string
	Role  string
	Order int
}

// NFO is what PLAID uses from an .nfo. Empty fields weren't in the file.
type NFO struct {
	Kind          string
	Title         string
	OriginalTitle string
	SortTitle     string
	Plot          string
	Tagline       string
	Year          int
	Premiered     string // YYYY-MM-DD
	Runtime       int    // minutes
	MPAA          string
	Genres        []string
	Studios       []string
	Set           string
	Season        int // episodes; -1 when absent
	Episode       int // episodes; -1 when absent
	TMDBID        int
	IMDbID        string
	TVDBID        int
	Actors        []Actor
}

type uniqueID struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type setXML struct {
	Name string `xml:"name"`
	Text string `xml:",chardata"`
}

type document struct {
	XMLName       xml.Name
	Title         string     `xml:"title"`
	OriginalTitle string     `xml:"originaltitle"`
	SortTitle     string     `xml:"sorttitle"`
	Plot          string     `xml:"plot"`
	Outline       string     `xml:"outline"`
	Tagline       string     `xml:"tagline"`
	Year          string     `xml:"year"`
	Premiered     string     `xml:"premiered"`
	Aired         string     `xml:"aired"`
	Runtime       string     `xml:"runtime"`
	MPAA          string     `xml:"mpaa"`
	Genres        []string   `xml:"genre"`
	Studios       []string   `xml:"studio"`
	Set           setXML     `xml:"set"`
	Season        string     `xml:"season"`
	Episode       string     `xml:"episode"`
	UniqueIDs     []uniqueID `xml:"uniqueid"`
	ID            string     `xml:"id"`
	TMDBID        string     `xml:"tmdbid"`
	IMDbID        string     `xml:"imdbid"`
	Actors        []struct {
		Name  string `xml:"name"`
		Role  string `xml:"role"`
		Order string `xml:"order"`
	} `xml:"actor"`
}

var (
	tmdbURL = regexp.MustCompile(`themoviedb\.org/(?:movie|tv)/(\d+)`)
	imdbURL = regexp.MustCompile(`imdb\.com/title/(tt\d+)`)
	imdbID  = regexp.MustCompile(`^tt\d+$`)
	tvdbURL = regexp.MustCompile(`thetvdb\.com/.*?[?&]id=(\d+)`)
)

// Parse reads an .nfo.
func Parse(r io.Reader) (*NFO, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxSize {
		return nil, ErrTooLarge
	}
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	n := &NFO{Season: -1, Episode: -1}
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '<' {
		if err := n.parseXML(trimmed); err != nil {
			return nil, err
		}
	}
	// Links: the whole file for a URL-only .nfo, or after the XML.
	text := string(raw)
	if n.TMDBID == 0 {
		if m := tmdbURL.FindStringSubmatch(text); m != nil {
			n.TMDBID, _ = strconv.Atoi(m[1])
		}
	}
	if n.IMDbID == "" {
		if m := imdbURL.FindStringSubmatch(text); m != nil {
			n.IMDbID = m[1]
		}
	}
	if n.TVDBID == 0 {
		if m := tvdbURL.FindStringSubmatch(text); m != nil {
			n.TVDBID, _ = strconv.Atoi(m[1])
		}
	}
	if n.Kind == "" {
		if n.TMDBID == 0 && n.IMDbID == "" && n.TVDBID == 0 {
			return nil, errors.New("nfo: neither XML nor a TMDB/IMDb link")
		}
		n.Kind = URLOnly
	}
	return n, nil
}

func (n *NFO) parseXML(raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	var d document
	if err := dec.Decode(&d); err != nil {
		return fmt.Errorf("nfo: %w", err)
	}
	switch d.XMLName.Local {
	case Movie, Show, Episode:
		n.Kind = d.XMLName.Local
	default:
		return fmt.Errorf("nfo: unknown root <%s>", d.XMLName.Local)
	}
	trim := strings.TrimSpace
	n.Title, n.OriginalTitle, n.SortTitle = trim(d.Title), trim(d.OriginalTitle), trim(d.SortTitle)
	n.Plot = trim(d.Plot)
	if n.Plot == "" {
		n.Plot = trim(d.Outline)
	}
	n.Tagline, n.MPAA = trim(d.Tagline), trim(d.MPAA)
	n.Premiered = date(firstOf(d.Premiered, d.Aired))
	n.Year = atoi(d.Year)
	if n.Year == 0 && len(n.Premiered) >= 4 {
		n.Year = atoi(n.Premiered[:4])
	}
	n.Runtime = atoi(d.Runtime)
	n.Genres, n.Studios = nonEmpty(d.Genres), nonEmpty(d.Studios)
	n.Set = trim(firstOf(d.Set.Name, d.Set.Text))
	if s := trim(d.Season); s != "" {
		n.Season = atoi(s)
	}
	if e := trim(d.Episode); e != "" {
		n.Episode = atoi(e)
	}
	for _, u := range d.UniqueIDs {
		v := trim(u.Value)
		switch strings.ToLower(u.Type) {
		case "tmdb":
			n.TMDBID = atoi(v)
		case "imdb":
			if imdbID.MatchString(v) {
				n.IMDbID = v
			}
		case "tvdb":
			n.TVDBID = atoi(v)
		}
	}
	// Older scrapers: <id>tt…</id>, <tmdbid>, <imdbid>.
	if n.IMDbID == "" {
		for _, v := range []string{d.IMDbID, d.ID} {
			if v = trim(v); imdbID.MatchString(v) {
				n.IMDbID = v
				break
			}
		}
	}
	if n.TMDBID == 0 {
		n.TMDBID = atoi(d.TMDBID)
	}
	for i, a := range d.Actors {
		if name := trim(a.Name); name != "" {
			order := i
			if o := trim(a.Order); o != "" {
				order = atoi(o)
			}
			n.Actors = append(n.Actors, Actor{Name: name, Role: trim(a.Role), Order: order})
		}
	}
	return nil
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func firstOf(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func date(s string) string {
	s = strings.TrimSpace(s)
	if dateRe.MatchString(s) {
		return s
	}
	return ""
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
