package handlers_test

import (
	"math"
	"net/http"
	"testing"
	"time"
)

// Step 21: the admin dashboard's activity and playback history.
func TestActivity(t *testing.T) {
	e := newPlayEnv(t)
	s := e.start(e.admin, e.movie, map[string]any{"profile": external})
	e.clock.advance(20 * time.Minute)
	e.expect(e.do("POST", "/playback/"+s.ID+"/progress", map[string]any{"positionMs": 1_200_000, "state": "playing"},
		bearer(e.admin)), http.StatusNoContent, "")
	// A client reporting more than the time that passed counts as the time.
	e.expect(e.do("DELETE", "/playback/"+s.ID, map[string]any{"positionMs": 9_000_000}, bearer(e.admin)), http.StatusNoContent, "")

	var a struct {
		Days []struct {
			Date  string  `json:"date"`
			Hours float64 `json:"hours"`
			Plays int     `json:"plays"`
		} `json:"days"`
		Recent []struct {
			ItemID    string `json:"itemId"`
			Title     string `json:"title"`
			Method    string `json:"method"`
			WatchedMs int64  `json:"watchedMs"`
		} `json:"recent"`
		RecentlyAdded []struct {
			ID string `json:"id"`
		} `json:"recentlyAdded"`
		ActivePeople int `json:"activePeople"`
	}
	r := e.do("GET", "/activity?days=7", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&a)
	if len(a.Days) != 7 {
		t.Fatalf("%d days, want 7", len(a.Days))
	}
	today := a.Days[6]
	if today.Plays != 1 || math.Abs(today.Hours-1.0/3) > 0.01 {
		t.Errorf("today = %+v, want 1 play and 20 minutes", today)
	}
	if len(a.Recent) != 1 || a.Recent[0].ItemID != e.movie || a.Recent[0].Title != "Paddington" ||
		a.Recent[0].WatchedMs != 1_200_000 || a.Recent[0].Method == "" {
		t.Errorf("recent = %+v", a.Recent)
	}
	if len(a.RecentlyAdded) < 2 || a.ActivePeople != 1 {
		t.Errorf("recently added %d, active people %d", len(a.RecentlyAdded), a.ActivePeople)
	}

	var h struct {
		Samples []struct {
			Streams int `json:"streams"`
		} `json:"samples"`
	}
	e.do("GET", "/server/stats/history", nil, bearer(e.admin)).json(&h)
	if h.Samples == nil {
		t.Error("no samples list")
	}
	member := e.member(e.admin, "bob")
	e.expect(e.do("GET", "/activity", nil, bearer(member.Token)), http.StatusForbidden, "forbidden")
	e.expect(e.do("GET", "/server/stats/history", nil, bearer(member.Token)), http.StatusForbidden, "forbidden")
}
