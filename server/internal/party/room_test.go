package party

import (
	"errors"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/realtime"
)

type sent struct {
	to   Target
	typ  string
	data any
}

// roomTest wraps a room with a fake clock and records what it emits.
type roomTest struct {
	t    *testing.T
	now  time.Time
	r    *Room
	sent []sent
}

func newRoomTest(t *testing.T, maxPeople int) *roomTest {
	rt := &roomTest{t: t, now: t0}
	rt.r = NewRoom("party1", "nosferatu", ControlHost, maxPeople, rt.now, func(to Target, typ string, data any) {
		rt.sent = append(rt.sent, sent{to, typ, data})
	})
	return rt
}

func (rt *roomTest) advance(d time.Duration) {
	rt.now = rt.now.Add(d)
	rt.r.Tick(rt.now)
}

var (
	alex    = Person{ID: "u:alex", UserID: "alex", Name: "Alex"}
	mia     = Person{ID: "u:mia", UserID: "mia", Name: "Mia"}
	sam     = Person{ID: "u:sam", UserID: "sam", Name: "Sam"}
	leo     = Person{ID: "u:leo", UserID: "leo", Name: "Léo"}
	camille = Person{ID: "g:cam", GuestID: "cam", Name: "Camille"}
)

func (rt *roomTest) join(p Person, conn string) JoinResult {
	rt.t.Helper()
	res, err := rt.r.Join(rt.now, p, conn, KindScreen, realtime.Device{Type: "web"})
	if err != nil {
		rt.t.Fatalf("join %s: %v", p.Name, err)
	}
	return res
}

func (rt *roomTest) report(conn, state string) {
	rt.t.Helper()
	if err := rt.r.Report(rt.now, conn, Report{State: state, Quality: "1080p"}); err != nil {
		rt.t.Fatalf("report %s %s: %v", conn, state, err)
	}
}

// last returns the last event of type typ, or fails.
func (rt *roomTest) last(typ string) sent {
	rt.t.Helper()
	for i := len(rt.sent) - 1; i >= 0; i-- {
		if rt.sent[i].typ == typ {
			return rt.sent[i]
		}
	}
	rt.t.Fatalf("no %s sent", typ)
	return sent{}
}

func (rt *roomTest) count(typ string) int {
	n := 0
	for _, s := range rt.sent {
		if s.typ == typ {
			n++
		}
	}
	return n
}

func ptr(v int64) *int64 { return &v }

func TestRoomJoinHostAndSnapshot(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(camille, "c-cam") // a guest is never host
	if rt.r.Host() != "" {
		t.Fatalf("guest became host")
	}
	rt.join(alex, "c-alex")
	if rt.r.Host() != alex.ID {
		t.Fatalf("host = %q", rt.r.Host())
	}
	snap := rt.last("party.snapshot")
	if snap.to.Conn != "c-alex" {
		t.Errorf("snapshot to %+v", snap.to)
	}
	m := snap.data.(map[string]any)
	if m["me"] != alex.ID || m["itemId"] != "nosferatu" {
		t.Errorf("snapshot = %+v", m)
	}
	pres := rt.r.presence()["people"].([]PresenceEntry)
	if len(pres) != 2 || pres[0].State != "loading" || !pres[0].Guest || !pres[1].Host {
		t.Errorf("presence = %+v", pres)
	}
}

func TestRoomQueueWhenFull(t *testing.T) {
	rt := newRoomTest(t, 2)
	rt.join(alex, "c-alex")
	rt.join(mia, "c-mia")
	if res := rt.join(sam, "c-sam"); !res.Queued || res.Position != 1 {
		t.Fatalf("sam = %+v", res)
	}
	if res := rt.join(leo, "c-leo"); res.Position != 2 {
		t.Fatalf("leo = %+v", res)
	}
	if q := rt.last("party.queue"); !q.to.Host {
		t.Errorf("host not told: %+v", q)
	}
	// Mia leaves: Sam gets her seat, Léo moves up.
	rt.r.Leave(rt.now, "c-mia", true)
	if !rt.r.Has(sam.ID) || rt.r.Has(leo.ID) {
		t.Fatal("queue didn't advance")
	}
	if q := rt.last("party.queued"); q.to.Conn != "c-leo" || q.data.(map[string]int)["position"] != 1 {
		t.Errorf("leo's position = %+v", q)
	}
	// An admin host raising the limit lets Léo in.
	rt.r.SetMaxPeople(rt.now, 3)
	if !rt.r.Has(leo.ID) {
		t.Error("raising the limit didn't admit léo")
	}
}

func TestRoomControlModes(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-alex")
	rt.join(mia, "c-mia")
	if err := rt.r.Control(rt.now, mia.ID, "pause", nil); !errors.Is(err, ErrNoControl) {
		t.Fatalf("mia paused in host mode: %v", err)
	}
	if err := rt.r.Control(rt.now, alex.ID, "play", ptr(1000)); err != nil {
		t.Fatal(err)
	}
	rt.now = rt.now.Add(10 * time.Second)
	if got := rt.r.Playback().At(rt.now.UnixMilli()); got != 11000 {
		t.Errorf("position after 10 s = %d", got)
	}
	// The host gives Mia control.
	if err := rt.r.Grant(rt.now, mia.ID, mia.ID, true); !errors.Is(err, ErrNotHost) {
		t.Errorf("mia granted herself: %v", err)
	}
	rt.r.Grant(rt.now, alex.ID, mia.ID, true)
	if err := rt.r.Control(rt.now, mia.ID, "seek", ptr(37400)); err != nil {
		t.Fatal(err)
	}
	pb := rt.last("party.playback").data.(map[string]any)
	if pb["action"] != "seek" || pb["by"] != mia.ID || pb["positionMs"] != int64(37400) || pb["state"] != Playing {
		t.Errorf("seek = %+v", pb)
	}
	// Everyone mode.
	rt.join(sam, "c-sam")
	rt.r.SetMode(rt.now, alex.ID, ControlEveryone)
	if err := rt.r.Control(rt.now, sam.ID, "pause", nil); err != nil {
		t.Errorf("sam can't pause in everyone mode: %v", err)
	}
}

func TestRoomWaitsForBuffering(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-alex")
	rt.join(sam, "c-sam")
	rt.report("c-alex", ScreenPlaying)
	rt.report("c-sam", ScreenPlaying)
	rt.r.Control(rt.now, alex.ID, "play", ptr(38000))

	rt.advance(time.Second)
	rt.report("c-sam", ScreenBuffering)
	rt.advance(time.Second) // 1 s of buffering: not yet
	if rt.r.Playback().State != Playing {
		t.Fatal("waiting before the grace")
	}
	rt.advance(600 * time.Millisecond)
	pb := rt.r.Playback()
	if pb.State != Waiting || pb.PositionMs != 40600 {
		t.Fatalf("after 1.6 s of buffering: %+v", pb)
	}
	w := rt.last("party.waiting").data.(map[string]any)
	if ids := w["for"].([]string); len(ids) != 1 || ids[0] != sam.ID {
		t.Errorf("waiting for %v", ids)
	}
	// Resuming without Sam needs 20 s of waiting.
	if err := rt.r.ResumeWithout(rt.now, alex.ID); !errors.Is(err, ErrTooSoon) {
		t.Errorf("resume without too soon: %v", err)
	}
	// Sam is ready again: everyone resumes together from the same point.
	rt.advance(3 * time.Second)
	rt.report("c-sam", ScreenReady)
	pb = rt.r.Playback()
	if pb.State != Playing || pb.PositionMs != 40600 || pb.AtMs != rt.now.UnixMilli() {
		t.Fatalf("after sam's ready: %+v", pb)
	}

	// Another stall, then the host goes on without him.
	rt.report("c-sam", ScreenBuffering)
	rt.advance(2 * time.Second)
	rt.advance(20 * time.Second)
	if rt.r.Playback().State != Waiting {
		t.Fatal("not waiting")
	}
	if err := rt.r.ResumeWithout(rt.now, alex.ID); err != nil {
		t.Fatal(err)
	}
	if rt.r.Playback().State != Playing {
		t.Error("resume without didn't play")
	}
}

func TestRoomPauseRequest(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-alex")
	rt.join(mia, "c-mia")
	rt.r.Control(rt.now, alex.ID, "play", ptr(0))
	if err := rt.r.PauseRequest(rt.now, mia.ID); err != nil {
		t.Fatal(err)
	}
	req := rt.last("party.pause.requested").data.(map[string]any)
	if req["name"] != "Mia" {
		t.Errorf("request = %+v", req)
	}
	if err := rt.r.PauseRequest(rt.now.Add(30*time.Second), mia.ID); !errors.Is(err, ErrTooSoon) {
		t.Errorf("second request within a minute: %v", err)
	}
	if err := rt.r.PauseAnswer(rt.now, mia.ID, req["requestId"].(int64), "now"); !errors.Is(err, ErrNotHost) {
		t.Errorf("mia answered: %v", err)
	}
	// "In 5 min": a countdown, then the pause.
	rt.r.PauseAnswer(rt.now, alex.ID, req["requestId"].(int64), "in5")
	at := rt.last("party.pause.scheduled").data.(map[string]int64)["atServerMs"]
	if at != rt.now.Add(5*time.Minute).UnixMilli() {
		t.Errorf("pause at %d", at)
	}
	rt.advance(4 * time.Minute)
	if rt.r.Playback().State != Playing {
		t.Fatal("paused early")
	}
	rt.advance(time.Minute)
	if pb := rt.r.Playback(); pb.State != Paused || pb.PositionMs != 300000 {
		t.Fatalf("scheduled pause = %+v", pb)
	}

	// With the host gone (still within their grace), a request pauses at once.
	rt.r.Control(rt.now, alex.ID, "play", nil)
	rt.r.Leave(rt.now, "c-alex", false)
	rt.advance(30 * time.Second)
	if err := rt.r.PauseRequest(rt.now, mia.ID); err != nil {
		t.Fatal(err)
	}
	if rt.r.Playback().State != Paused {
		t.Error("no host, no pause")
	}
}

func TestRoomHostRules(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-alex")
	rt.join(camille, "c-cam")
	rt.join(mia, "c-mia")
	rt.join(leo, "c-leo")
	if err := rt.r.HandHost(rt.now, alex.ID, camille.ID); !errors.Is(err, ErrNoSuchPerson) {
		t.Errorf("host handed to a guest: %v", err)
	}
	if err := rt.r.HandHost(rt.now, alex.ID, leo.ID); err != nil || rt.r.Host() != leo.ID {
		t.Fatalf("hand to léo: %v, host %s", err, rt.r.Host())
	}
	// Léo vanishes: after 2 minutes the role goes to the first account
	// person still connected (Alex), skipping the guest.
	rt.r.Leave(rt.now, "c-leo", false)
	rt.advance(time.Minute)
	if rt.r.Host() != leo.ID {
		t.Fatal("host passed before the grace")
	}
	rt.advance(time.Minute)
	if rt.r.Host() != alex.ID {
		t.Fatalf("host = %s, want alex", rt.r.Host())
	}
	if rt.r.Has(leo.ID) {
		t.Error("léo kept his seat past the grace")
	}
	// Removal: out for good.
	conns, err := rt.r.Remove(rt.now, alex.ID, mia.ID)
	if err != nil || len(conns) != 1 || conns[0] != "c-mia" {
		t.Fatalf("remove = %v, %v", conns, err)
	}
	if _, err := rt.r.Join(rt.now, mia, "c-mia2", KindScreen, realtime.Device{}); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("removed person rejoined: %v", err)
	}
	// An explicit leave by the host hands over at once.
	rt.join(sam, "c-sam")
	rt.r.Leave(rt.now, "c-alex", true)
	if rt.r.Host() != sam.ID {
		t.Errorf("host after alex left = %s", rt.r.Host())
	}
}

func TestRoomEndsWhenEmpty(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-alex")
	rt.r.Leave(rt.now, "c-alex", false)
	rt.advance(9 * time.Minute)
	if rt.r.Ended() {
		t.Fatal("ended early")
	}
	rt.advance(time.Minute)
	if !rt.r.Ended() || rt.count("party.ended") != 1 {
		t.Fatal("not ended after 10 min empty")
	}
	if _, err := rt.r.Join(rt.now, alex, "c", KindScreen, realtime.Device{}); !errors.Is(err, ErrPartyEnded) {
		t.Errorf("joined an ended party: %v", err)
	}
}

func TestRoomChatReactionsSummary(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-alex")
	rt.join(mia, "c-mia")
	rt.report("c-mia", ScreenPlaying)
	joined := rt.last("party.chat").data.(ChatMessage)
	if joined.System != "joined" || joined.Values["name"] != "Mia" || joined.Values["quality"] != "1080p" {
		t.Errorf("joined line = %+v", joined)
	}
	rt.r.Control(rt.now, alex.ID, "play", ptr(38*60000+2000))
	if err := rt.r.Chat(rt.now, mia.ID, "  Il arrive par la porte ou par le mur cette fois ?  "); err != nil {
		t.Fatal(err)
	}
	m := rt.last("party.chat").data.(ChatMessage)
	if m.Text != "Il arrive par la porte ou par le mur cette fois ?" || m.PositionMs != 38*60000+2000 || m.Name != "Mia" {
		t.Errorf("message = %+v", m)
	}
	for _, bad := range []string{"", "   ", string(make([]rune, 501))} {
		if err := rt.r.Chat(rt.now, mia.ID, bad); !errors.Is(err, ErrBadText) {
			t.Errorf("chat %q: %v", bad, err)
		}
	}
	// 1 a second, burst 5.
	var limited bool
	for i := 0; i < 6; i++ {
		if errors.Is(rt.r.Chat(rt.now, alex.ID, "hop"), ErrRateLimited) {
			limited = true
		}
	}
	if !limited {
		t.Error("chat not rate limited")
	}
	if err := rt.r.React(rt.now, mia.ID, "🤖"); !errors.Is(err, ErrBadReaction) {
		t.Errorf("unknown emoji: %v", err)
	}
	rt.now = rt.now.Add(34*time.Minute + 38*time.Second) // 1:12:40
	for _, e := range []string{"😱", "😱", "😂"} {
		if err := rt.r.React(rt.now, mia.ID, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := rt.r.React(rt.now, mia.ID, "😱"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("4th reaction in a second: %v", err)
	}
	rt.r.React(rt.now.Add(time.Second), alex.ID, "🍿")
	rt.r.PauseRequest(rt.now, mia.ID)
	s := rt.r.Summarize(rt.now, nil)
	if s.Reactions != 4 || s.PauseRequests != 1 || len(s.People) != 2 || s.PeakEmoji != "😱" || s.PeakMinuteMs != 72*60000 {
		t.Errorf("summary = %+v", s)
	}
}

func TestRoomChatHistoryCap(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-alex")
	for i := 0; i < 250; i++ {
		rt.now = rt.now.Add(time.Second)
		rt.r.Chat(rt.now, alex.ID, "x")
	}
	if n := len(rt.r.Snapshot(rt.now, alex.ID)["chat"].([]ChatMessage)); n != chatHistory {
		t.Errorf("history = %d", n)
	}
}

func TestRoomOneScreenPerPerson(t *testing.T) {
	rt := newRoomTest(t, 8)
	rt.join(alex, "c-phone")
	// The Apple TV joins as Alex's screen; the phone becomes the companion.
	rt.r.Join(rt.now, alex, "c-tv", KindScreen, realtime.Device{Type: "tvos"})
	pres := rt.r.presence()["people"].([]PresenceEntry)
	if len(pres) != 1 || pres[0].Screen != "tvos" || !pres[0].Companion {
		t.Fatalf("presence = %+v", pres)
	}
	if rt.r.Count() != 1 {
		t.Error("two devices took two seats")
	}
}
