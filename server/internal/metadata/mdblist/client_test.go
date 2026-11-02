package mdblist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/mdblist"
)

// The Matrix as MDBList answers it, trimmed to the ratings.
const matrix = `{"title":"The Matrix","ratings":[
	{"source":"imdb","value":8.7,"score":87,"votes":2100000},
	{"source":"tmdb","value":82,"score":82,"votes":26000},
	{"source":"tomatoes","value":83,"score":83,"votes":160},
	{"source":"popcorn","value":85.4,"score":null,"votes":250000},
	{"source":"metacritic","value":0,"score":0,"votes":0}]}`

func client(t *testing.T) *mdblist.Client {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Query().Get("apikey") != "k":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/tmdb/movie/603":
			_, _ = w.Write([]byte(matrix))
		case r.URL.Path == "/tmdb/show/1399":
			_, _ = w.Write([]byte(`{"ratings":[{"source":"tomatoes","score":0},{"source":"popcorn","score":91}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	h := httpx.New(httpx.Options{Name: "mdblist", Hosts: []string{"127.0.0.1"}, Transport: srv.Client().Transport})
	return mdblist.New(h, srv.URL, "k")
}

func val(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

func TestRatings(t *testing.T) {
	c := client(t)
	ctx := context.Background()
	r, err := c.Ratings(ctx, mdblist.Movie, 603)
	if err != nil {
		t.Fatal(err)
	}
	if val(r.Critics) != 83 || val(r.Audience) != 85 {
		t.Errorf("movie = %d, %d; want 83, 85", val(r.Critics), val(r.Audience))
	}
	// A score of 0 is MDBList's "unknown".
	if r, err = c.Ratings(ctx, mdblist.Show, 1399); err != nil || r.Critics != nil || val(r.Audience) != 91 {
		t.Errorf("show = %d, %d, %v; want none, 91", val(r.Critics), val(r.Audience), err)
	}
	if r, err = c.Ratings(ctx, mdblist.Movie, 1); err != nil || r.Critics != nil || r.Audience != nil {
		t.Errorf("unknown movie = %+v, %v; want no scores", r, err)
	}
}
