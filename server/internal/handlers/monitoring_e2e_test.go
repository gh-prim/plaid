package handlers_test

import (
	"net/http"
	"testing"
	"time"
)

// Admin monitoring: data sent, places of remote plays, device health.
func TestMonitoring(t *testing.T) {
	e := newPlayEnv(t)
	s := e.start(e.admin, e.movie, map[string]any{"profile": external})
	e.clock.advance(20 * time.Minute)
	e.expect(e.do("POST", "/playback/"+s.ID+"/progress", map[string]any{"positionMs": 1_200_000, "state": "playing"},
		bearer(e.admin)), http.StatusNoContent, "")
	e.expect(e.do("DELETE", "/playback/"+s.ID, map[string]any{"positionMs": 1_200_000}, bearer(e.admin)), http.StatusNoContent, "")

	var bitrate int64
	if err := e.db.QueryRow(`SELECT COALESCE(bitrate_bps, 0) FROM play_log`).Scan(&bitrate); err != nil || bitrate <= 0 {
		t.Fatalf("logged bitrate %d, %v", bitrate, err)
	}
	// Two remote plays from Lyon on a TV that can't decode the video, one unplaced.
	at := e.clock.now().UTC().Format("2006-01-02T15:04:05.000Z")
	for i, row := range []struct {
		method, reasons, country, city string
		lat                            any
	}{
		{"transcode", "video_codec_not_supported,bitrate_too_high", "FR", "Lyon", 45.75},
		{"transcode", "video_codec_not_supported", "FR", "Lyon", 45.75},
		{"direct_play", "", "", "", nil},
	} {
		var lon any
		if row.lat != nil {
			lon = 4.85
		}
		if _, err := e.db.Exec(`INSERT INTO play_log (id, display_name, profile_name, item_id, title, device_name, method, remote,
			started_at, ended_at, watched_ms, bitrate_bps, reasons, country, city, lat, lon)
			VALUES (?, 'Bob', 'Bob', ?, 'Dune', 'Living room TV', ?, 1, ?, ?, 3600000, 8000000, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
			"r"+string(rune('0'+i)), e.dune, row.method, at, at, row.reasons, row.country, row.city, row.lat, lon); err != nil {
			t.Fatal(err)
		}
	}

	var m struct {
		Days []struct {
			Bytes int64 `json:"bytes"`
		} `json:"days"`
		People []struct {
			DisplayName string `json:"displayName"`
			Bytes       int64  `json:"bytes"`
			Plays       int    `json:"plays"`
		} `json:"people"`
		Places []struct {
			City   string  `json:"city"`
			Lat    float64 `json:"lat"`
			Plays  int     `json:"plays"`
			People int     `json:"people"`
		} `json:"places"`
		Devices []struct {
			DeviceName  string `json:"deviceName"`
			Plays       int    `json:"plays"`
			Transcoded  int    `json:"transcoded"`
			CodecIssues int    `json:"codecIssues"`
			Score       int    `json:"score"`
		} `json:"devices"`
		Local, Remote, Unplaced int
		GeoipReady              bool `json:"geoipReady"`
	}
	r := e.do("GET", "/monitoring?days=7", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&m)
	if len(m.Days) != 7 || m.Days[6].Bytes < 3*3_600_000_000 {
		t.Fatalf("days = %+v", m.Days)
	}
	// Bob sent 3 h at 8 Mbit/s: 10.8 GB, ahead of the admin's 20 minutes.
	if len(m.People) != 2 || m.People[0].DisplayName != "Bob" || m.People[0].Bytes != 10_800_000_000 || m.People[0].Plays != 3 {
		t.Errorf("people = %+v", m.People)
	}
	if len(m.Places) != 1 || m.Places[0].City != "Lyon" || m.Places[0].Plays != 2 || m.Places[0].People != 1 || m.Places[0].Lat != 45.75 {
		t.Errorf("places = %+v", m.Places)
	}
	if m.Local != 1 || m.Remote != 3 || m.Unplaced != 1 || m.GeoipReady {
		t.Errorf("local %d remote %d unplaced %d ready %v", m.Local, m.Remote, m.Unplaced, m.GeoipReady)
	}
	// The TV comes first: 1 direct play in 3, 2 transcodes, 2 codec problems.
	tv := m.Devices[0]
	if tv.DeviceName != "Living room TV" || tv.Plays != 3 || tv.Transcoded != 2 || tv.CodecIssues != 2 || tv.Score != 33 {
		t.Errorf("worst device = %+v", tv)
	}
	if last := m.Devices[len(m.Devices)-1]; last.Score < tv.Score {
		t.Errorf("devices not worst first: %+v", m.Devices)
	}
	member := e.member(e.admin, "carol")
	e.expect(e.do("GET", "/monitoring", nil, bearer(member.Token)), http.StatusForbidden, "forbidden")
}

// The library report: every title with its size, files and plays.
func TestLibraryReport(t *testing.T) {
	e := newPlayEnv(t)
	s := e.start(e.admin, e.movie, map[string]any{"profile": external})
	e.expect(e.do("POST", "/playback/"+s.ID+"/progress", map[string]any{"positionMs": 60_000, "state": "playing"},
		bearer(e.admin)), http.StatusNoContent, "")
	e.expect(e.do("DELETE", "/playback/"+s.ID, map[string]any{"positionMs": 60_000}, bearer(e.admin)), http.StatusNoContent, "")

	type report struct {
		Totals struct {
			Titles     int   `json:"titles"`
			Bytes      int64 `json:"bytes"`
			Unwatched  int   `json:"unwatched"`
			Duplicates int   `json:"duplicates"`
		} `json:"totals"`
		Total int `json:"total"`
		Items []struct {
			Item struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
			} `json:"item"`
			Bytes int64 `json:"bytes"`
			Files int   `json:"files"`
			Plays int   `json:"plays"`
		} `json:"items"`
	}
	var all report
	r := e.do("GET", "/library-report", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&all)
	// Paddington, Dune and Dark; Paddington is the only one played.
	if all.Totals.Titles != 3 || all.Total != 3 || all.Totals.Unwatched != 2 || all.Totals.Duplicates != 0 {
		t.Fatalf("totals %+v, total %d", all.Totals, all.Total)
	}
	// Biggest first: Paddington's 64 KB file.
	if first := all.Items[0]; first.Item.ID != e.movie || first.Bytes != 64<<10 || first.Files != 1 || first.Plays != 1 {
		t.Errorf("first = %+v", first)
	}
	var show report
	for _, it := range all.Items {
		if it.Item.ID == e.show && (it.Files != 2 || it.Item.Kind != "show") {
			t.Errorf("Dark = %+v, want its 2 episode files", it)
		}
	}
	e.do("GET", "/library-report?show=unwatched&sort=title&limit=1", nil, bearer(e.admin)).json(&show)
	if show.Total != 2 || len(show.Items) != 1 || show.Items[0].Item.ID != e.show {
		t.Errorf("unwatched by title, one per page: %+v", show)
	}
	member := e.member(e.admin, "carol")
	e.expect(e.do("GET", "/library-report", nil, bearer(member.Token)), http.StatusForbidden, "forbidden")
}
