package fanart_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gh-prim/plaid/server/internal/metadata/fanart"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

func count(imgs []fanart.Image, kind string) int {
	n := 0
	for _, i := range imgs {
		if i.Kind == kind {
			n++
		}
	}
	return n
}

func TestMovie(t *testing.T) {
	c := tmdbtest.New(t).Fanart()
	imgs, err := c.Movie(context.Background(), 603)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{fanart.Logo, fanart.Clearart, fanart.Backdrop, fanart.Poster} {
		if n := count(imgs, kind); n == 0 || n > 10 {
			t.Errorf("%s: %d images", kind, n)
		}
	}
	for _, i := range imgs {
		if !strings.HasPrefix(i.URL, "https://assets.fanart.tv/") || i.Season != -1 {
			t.Errorf("image %+v", i)
		}
		if i.Kind == fanart.Backdrop && i.Language != "" {
			t.Errorf("backdrops have no text: %+v", i)
		}
	}
	if none, err := c.Movie(context.Background(), 1); err != nil || len(none) != 0 {
		t.Errorf("unknown movie = %v, %v; want no images", none, err)
	}
}

func TestShowSeasonPosters(t *testing.T) {
	c := tmdbtest.New(t).Fanart()
	imgs, err := c.Show(context.Background(), 81189)
	if err != nil {
		t.Fatal(err)
	}
	seasons := 0
	for _, i := range imgs {
		if i.Season >= 0 {
			seasons++
			if i.Kind != fanart.Poster {
				t.Errorf("season image %+v isn't a poster", i)
			}
		}
	}
	if seasons == 0 || count(imgs, fanart.Logo) == 0 {
		t.Errorf("%d season posters, %d logos", seasons, count(imgs, fanart.Logo))
	}
	if imgs, err := c.Show(context.Background(), 0); err != nil || imgs != nil {
		t.Errorf("no TVDB id: %v, %v", imgs, err)
	}
}
