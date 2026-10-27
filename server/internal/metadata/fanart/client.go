// Package fanart is the fanart.tv artwork provider (F005 R16): logos,
// clearart, extra backdrops and posters, and season posters. Movies are
// looked up by TMDB id, shows by TVDB id.
package fanart

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
)

// Hosts fanart.tv requests and images may go to.
var Hosts = []string{"webservice.fanart.tv", "assets.fanart.tv"}

// BaseURL is fanart.tv's API root.
const BaseURL = "https://webservice.fanart.tv/v3"

// Artwork kinds, as the artwork table stores them.
const (
	Poster   = "poster"
	Backdrop = "backdrop"
	Logo     = "logo"
	Clearart = "clearart"
	Thumb    = "thumb"
)

// maxPerKind bounds the images kept per kind.
const maxPerKind = 10

// Image is one fanart.tv image.
type Image struct {
	Kind     string
	URL      string
	Language string // "" for no text
	Likes    int
	Season   int // season posters; -1 otherwise (0 = specials)
}

// Client talks to fanart.tv with a project key and an optional personal key.
type Client struct {
	http        *httpx.Client
	base        string
	key         string
	personalKey string
}

// New returns a client. base is BaseURL outside tests.
func New(h *httpx.Client, base, key, personalKey string) *Client {
	return &Client{http: h, base: strings.TrimSuffix(base, "/"), key: key, personalKey: personalKey}
}

var movieKinds = map[string]string{
	"hdmovielogo": Logo, "hdmovieclearart": Clearart, "moviebackground": Backdrop, "movieposter": Poster, "moviethumb": Thumb,
}

var showKinds = map[string]string{
	"hdtvlogo": Logo, "hdclearart": Clearart, "showbackground": Backdrop, "tvposter": Poster, "tvthumb": Thumb, "seasonposter": Poster,
}

// Movie returns a movie's images by TMDB id. A movie fanart.tv doesn't
// know has none.
func (c *Client) Movie(ctx context.Context, tmdbID int) ([]Image, error) {
	return c.images(ctx, "/movies/"+strconv.Itoa(tmdbID), movieKinds)
}

// Show returns a show's images by TVDB id.
func (c *Client) Show(ctx context.Context, tvdbID int) ([]Image, error) {
	if tvdbID == 0 {
		return nil, nil
	}
	return c.images(ctx, "/tv/"+strconv.Itoa(tvdbID), showKinds)
}

type entry struct {
	URL    string `json:"url"`
	Lang   string `json:"lang"`
	Likes  string `json:"likes"`
	Season string `json:"season"`
}

func (c *Client) images(ctx context.Context, path string, kinds map[string]string) ([]Image, error) {
	q := url.Values{"api_key": {c.key}}
	if c.personalKey != "" {
		q.Set("client_key", c.personalKey)
	}
	body, err := c.http.Get(ctx, c.base+path+"?"+q.Encode(), nil)
	if errors.Is(err, httpx.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(kinds))
	for name := range kinds {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []Image
	for _, name := range names {
		var entries []entry
		if v, ok := raw[name]; !ok || json.Unmarshal(v, &entries) != nil {
			continue
		}
		for _, e := range entries[:min(len(entries), maxPerKind)] {
			u, err := url.Parse(e.URL)
			if err != nil || u.Scheme != "https" || u.Hostname() != "assets.fanart.tv" {
				continue // only images from fanart.tv's own host (NFR security)
			}
			img := Image{Kind: kinds[name], URL: e.URL, Language: lang(e.Lang), Season: -1}
			img.Likes, _ = strconv.Atoi(e.Likes)
			if name == "seasonposter" {
				if e.Season == "all" || e.Season == "" {
					continue
				}
				img.Season, _ = strconv.Atoi(e.Season)
			}
			out = append(out, img)
		}
	}
	return out, nil
}

// lang maps fanart.tv's language ("", "00" for no text) to ISO 639-1.
func lang(l string) string {
	if l == "00" {
		return ""
	}
	return strings.ToLower(l)
}
