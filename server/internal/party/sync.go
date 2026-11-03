package party

import (
	"math"
	"slices"
)

// Drift correction thresholds (F010-R10). Clients (web, Apple) implement the
// same rules; api/events.md states them.
const (
	DriftIgnoreMs  = 150  // below: nothing
	DriftSettledMs = 50   // a rate correction stops once within this
	DriftSeekMs    = 3000 // beyond: seek
	RateFast       = 1.05
	RateSlow       = 0.95
)

// Correction is what a client does about its drift.
type Correction struct {
	Rate float64
	// Seek, when set, jumps to that position.
	Seek *int64
}

// Correct decides from the local and target positions and the rate the
// client is playing at. Positive drift means the client is behind.
func Correct(localMs, targetMs int64, rate float64) Correction {
	drift := targetMs - localMs
	abs := drift
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs > DriftSeekMs:
		t := targetMs
		return Correction{Rate: 1, Seek: &t}
	case rate != 1 && abs <= DriftSettledMs:
		return Correction{Rate: 1}
	case rate == 1 && abs < DriftIgnoreMs:
		return Correction{Rate: 1}
	case drift > 0:
		return Correction{Rate: RateFast}
	default:
		return Correction{Rate: RateSlow}
	}
}

// ClockSample is one sync.ping round trip: local send and receive times and
// the server's time in the pong, all in milliseconds.
type ClockSample struct {
	SentMs, ServerMs, ReceivedMs int64
}

// RTT is the sample's round trip.
func (s ClockSample) RTT() int64 { return s.ReceivedMs - s.SentMs }

// Offset is server time minus local time, assuming a symmetric path.
func (s ClockSample) Offset() int64 { return s.ServerMs - (s.SentMs+s.ReceivedMs)/2 }

// ClockEstimate keeps the offset of the sample with the lowest round trip
// among the last few (R9): the shortest trip has the least asymmetry.
type ClockEstimate struct {
	samples []ClockSample
}

// clockWindow is how many samples are kept.
const clockWindow = 8

// Add records a sample.
func (c *ClockEstimate) Add(s ClockSample) {
	c.samples = append(c.samples, s)
	if len(c.samples) > clockWindow {
		c.samples = c.samples[len(c.samples)-clockWindow:]
	}
}

// Offset returns the best offset estimate, 0 without samples.
func (c *ClockEstimate) Offset() int64 {
	if len(c.samples) == 0 {
		return 0
	}
	best := slices.MinFunc(c.samples, func(a, b ClockSample) int { return int(a.RTT() - b.RTT()) })
	return best.Offset()
}

// Percentile returns the p-th percentile (0–100) of values.
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := slices.Clone(values)
	slices.Sort(v)
	i := int(math.Ceil(p/100*float64(len(v)))) - 1
	return v[max(0, min(i, len(v)-1))]
}
