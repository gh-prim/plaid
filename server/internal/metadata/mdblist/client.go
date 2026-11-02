// Package mdblist fetches Rotten Tomatoes scores through MDBList: the
// Tomatometer (critics) and the Popcornmeter (audience). Rotten Tomatoes
// has no public API; MDBList gathers both, looked up by TMDB id.
package mdblist

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
)

// Hosts MDBList requests may go to.
var Hosts = []string{"api.mdblist.com"}

// BaseURL is MDBList's API root.
const BaseURL = "https://api.mdblist.com"

// Kinds MDBList looks up by TMDB id.
const (
	Movie = "movie"
	Show  = "show"
)

// Ratings are scores out of 100; nil when Rotten Tomatoes has none.
type Ratings struct {
	Critics, Audience *int
}

// Client talks to MDBList with the admin's key.
type Client struct {
	http *httpx.Client
	base string
	key  string
}

// New returns a client. base is BaseURL outside tests.
func New(h *httpx.Client, base, key string) *Client {
	return &Client{http: h, base: strings.TrimSuffix(base, "/"), key: key}
}

type response struct {
	Ratings []struct {
		Source string   `json:"source"`
		Score  *float64 `json:"score"`
		Value  *float64 `json:"value"`
	} `json:"ratings"`
}

// Ratings returns a movie's or show's scores by TMDB id. A title MDBList
// doesn't know has none.
func (c *Client) Ratings(ctx context.Context, kind string, tmdbID int) (Ratings, error) {
	var out Ratings
	if tmdbID == 0 {
		return out, nil
	}
	body, err := c.http.Get(ctx, c.base+"/tmdb/"+kind+"/"+strconv.Itoa(tmdbID)+"?"+url.Values{"apikey": {c.key}}.Encode(), nil)
	if errors.Is(err, httpx.ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return out, err
	}
	for _, x := range r.Ratings {
		v := x.Score
		if v == nil {
			v = x.Value
		}
		// MDBList reports a score it doesn't have as 0 or null.
		if v == nil || *v <= 0 || *v > 100 {
			continue
		}
		n := int(math.Round(*v))
		switch x.Source {
		case "tomatoes":
			out.Critics = &n
		case "popcorn":
			out.Audience = &n
		}
	}
	return out, nil
}
