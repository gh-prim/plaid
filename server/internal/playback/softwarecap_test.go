package playback

import (
	"testing"

	"github.com/gh-prim/plaid/server/internal/library"
)

// A 4K software transcode comes down to 1080p unless the client chose a
// quality; with an accelerator it stays.
func TestSoftwareCap(t *testing.T) {
	m := Media{Video: &library.Stream{Width: 3840, Height: 2160}}
	four := func() Decision {
		return Decision{Method: Transcode, Media: m, Quality: "2160p", Width: 3840, Height: 2160, VideoBitrate: 20_000_000}
	}
	d := four()
	softwareCap(&d, Request{})
	if d.Quality != "1080p" || d.Width != 1920 || d.Height != 1080 || d.VideoBitrate != 8_000_000 {
		t.Errorf("software 4K = %+v; want 1080p", d)
	}
	d = four()
	d.HWAccel = HWVAAPI
	softwareCap(&d, Request{})
	if d.Height != 2160 {
		t.Error("an accelerated transcode was capped")
	}
	d = four()
	softwareCap(&d, Request{Quality: "2160p"})
	if d.Height != 2160 {
		t.Error("the client's own choice was capped")
	}
}

// A PGS track a client draws is fetched as .sup; for others it has no URL.
func TestPGSURL(t *testing.T) {
	pgs := Subtitle{Index: 2, Codec: "hdmv_pgs_subtitle", Image: true}
	if u := SubtitleURL(Session{token: "tok", Profile: pgsChrome}, pgs); u != StreamPrefix+"tok/subtitles/2.sup" {
		t.Errorf("url = %q", u)
	}
	if u := SubtitleURL(Session{token: "tok", Profile: chrome}, pgs); u != "" {
		t.Errorf("url for a client that can't draw PGS = %q", u)
	}
}
