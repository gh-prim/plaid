package party

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/realtime"
)

func TestCorrect(t *testing.T) {
	for _, tc := range []struct {
		local, target int64
		rate          float64
		want          float64
		seek          bool
	}{
		{10_000, 10_100, 1, 1, false},        // 100 ms: ignored
		{10_000, 11_200, 1, RateFast, false}, // 1.2 s behind: catch up gently
		{11_200, 10_000, 1, RateSlow, false}, // ahead: slow down
		{10_000, 10_100, RateFast, RateFast, false},
		{10_000, 10_040, RateFast, 1, false}, // settled
		{10_000, 14_000, 1, 1, true},         // 4 s: seek
		{14_000, 10_000, RateSlow, 1, true},
	} {
		c := Correct(tc.local, tc.target, tc.rate)
		if c.Rate != tc.want || (c.Seek != nil) != tc.seek {
			t.Errorf("Correct(%d, %d, %v) = %+v", tc.local, tc.target, tc.rate, c)
		}
		if tc.seek && *c.Seek != tc.target {
			t.Errorf("seek to %d", *c.Seek)
		}
	}
}

func TestClockEstimateKeepsShortestTrip(t *testing.T) {
	var c ClockEstimate
	c.Add(ClockSample{SentMs: 0, ServerMs: 1_000_300, ReceivedMs: 400}) // slow, asymmetric
	c.Add(ClockSample{SentMs: 1000, ServerMs: 1_001_040, ReceivedMs: 1080})
	if got := c.Offset(); got != 1_000_000 {
		t.Errorf("offset = %d", got)
	}
}

// simClient is a simulated player: its own clock skew, network latency
// both ways, and the R10 rules applied every 250 ms.
type simClient struct {
	name     string
	skewMs   float64
	oneWayMs float64
	jitterMs float64
	clock    ClockEstimate

	pos       float64
	rate      float64
	playing   bool
	stallTill float64
	room      Playback // latest reference state received
	have      bool
}

func (c *simClient) local(t float64) int64 { return int64(t + c.skewMs) }

func (c *simClient) delay(rng *rand.Rand) float64 {
	return c.oneWayMs + (rng.Float64()*2-1)*c.jitterMs
}

// seekCost is how long a player is frozen by a seek.
const seekCost = 250.0

func (c *simClient) apply(t float64) {
	if !c.have {
		return
	}
	switch c.room.State {
	case Paused, Waiting:
		c.playing = false
		if math.Abs(c.pos-float64(c.room.PositionMs)) > DriftIgnoreMs {
			c.pos, c.stallTill = float64(c.room.PositionMs), t+seekCost
		}
		c.rate = 1
	case Playing:
		target := c.room.At(c.local(t) + c.clock.Offset())
		corr := Correct(int64(c.pos), target, c.rate)
		if corr.Seek != nil {
			// A seek lands where the room will be once the player is back.
			c.pos, c.stallTill = float64(*corr.Seek)+seekCost, t+seekCost
		}
		c.rate = corr.Rate
		c.playing = true
	}
}

func (c *simClient) step(t, dt float64) {
	if c.playing && t >= c.stallTill {
		c.pos += dt * c.rate
	}
}

type delivery struct {
	at     float64
	client *simClient
	pb     Playback
}

// TestSyncSimulation is the spec's acceptance test: two clients with 80 ms
// round trips and a 40 ms clock skew stay within 500 ms (p95) through 20
// play, pause and seek actions.
func TestSyncSimulation(t *testing.T) {
	for _, tc := range []struct {
		name             string
		rtt, jitter      float64
		skewA, skewB     float64
		wantP95, wantP50 float64
	}{
		{"lan", 4, 1, 0, 15, 500, 60},
		{"4g", 80, 15, 40, -40, 500, 120},
		{"asymmetric", 150, 40, 300, -200, 500, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p95, p50 := simulate(t, tc.rtt, tc.jitter, tc.skewA, tc.skewB)
			t.Logf("%s: gap p50 %.0f ms, p95 %.0f ms", tc.name, p50, p95)
			if p95 >= tc.wantP95 || p50 >= tc.wantP50 {
				t.Errorf("gap p50 %.0f ms, p95 %.0f ms; want under %.0f / %.0f", p50, p95, tc.wantP50, tc.wantP95)
			}
		})
	}
}

func simulate(t *testing.T, rtt, jitter, skewA, skewB float64) (p95, p50 float64) {
	t.Helper()
	rng := rand.New(rand.NewPCG(1, 2))
	start := time.UnixMilli(1_800_000_000_000)
	var queue []delivery
	clients := []*simClient{
		{name: "a", skewMs: skewA, oneWayMs: rtt / 2, jitterMs: jitter, rate: 1},
		{name: "b", skewMs: skewB, oneWayMs: rtt / 2, jitterMs: jitter, rate: 1},
	}
	trueMs := 0.0
	r := NewRoom("p", "item", ControlHost, 8, start, func(to Target, typ string, data any) {
		if typ != "party.playback" {
			return
		}
		m := data.(map[string]any)
		pb := Playback{State: m["state"].(string), PositionMs: m["positionMs"].(int64), AtMs: m["atServerMs"].(int64), Rate: 1}
		for _, c := range clients {
			queue = append(queue, delivery{at: trueMs + c.delay(rng), client: c, pb: pb})
		}
	})
	r.Join(start, Person{ID: "u:host", UserID: "host", Name: "Host"}, "c", KindScreen, realtime.Device{})

	// Five clock pings each (R9). Server time is the simulation's true time.
	for _, c := range clients {
		for i := 0; i < 5; i++ {
			sent := trueMs + float64(i)*50
			up, down := c.delay(rng), c.delay(rng)
			c.clock.Add(ClockSample{
				SentMs: c.local(sent), ServerMs: start.UnixMilli() + int64(sent+up), ReceivedMs: c.local(sent + up + down),
			})
		}
	}

	const dt = 10.0
	var gaps []float64
	nextAction, actions := 2000.0, 0
	nextCheck := 0.0
	for trueMs = 0; trueMs < 600_000; trueMs += dt {
		now := start.Add(time.Duration(trueMs) * time.Millisecond)
		if trueMs >= nextAction && actions < 20 {
			pos := r.Playback().At(now.UnixMilli())
			switch {
			case actions == 0 || r.Playback().State != Playing:
				r.Control(now, "u:host", "play", nil)
			case actions%3 == 0:
				p := pos + int64(rng.IntN(600_000)) - 300_000
				r.Control(now, "u:host", "seek", ptr(max(p, 0)))
			default:
				r.Control(now, "u:host", "pause", nil)
			}
			actions++
			nextAction = trueMs + 5000 + rng.Float64()*20000
		}
		// Deliver what arrived.
		kept := queue[:0]
		for _, d := range queue {
			if d.at <= trueMs {
				d.client.room, d.client.have = d.pb, true
				d.client.apply(trueMs)
			} else {
				kept = append(kept, d)
			}
		}
		queue = kept
		if trueMs >= nextCheck {
			for _, c := range clients {
				c.apply(trueMs)
			}
			nextCheck = trueMs + 250
		}
		for _, c := range clients {
			c.step(trueMs, dt)
		}
		if r.Playback().State == Playing && int(trueMs)%100 == 0 {
			gaps = append(gaps, math.Abs(clients[0].pos-clients[1].pos))
		}
	}
	if actions < 20 || len(gaps) < 1000 {
		t.Fatalf("only %d actions, %d samples", actions, len(gaps))
	}
	return Percentile(gaps, 95), Percentile(gaps, 50)
}
