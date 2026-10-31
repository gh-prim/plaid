package segments

import "testing"

// frames are black frames at 2 a second over [from, to].
func frames(from, to float64) []float64 {
	var out []float64
	for t := from; t <= to; t += frameStep {
		out = append(out, t)
	}
	return out
}

func join(parts ...[]float64) []float64 {
	var out []float64
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestCreditsStart(t *testing.T) {
	// For All Mankind S1 E5 (3575.6 s), as FFmpeg saw it: dark shots, then
	// credits from 3436 with 38 s over pictures, to the end.
	fam := join(frames(2931, 2941), frames(2970, 2975), frames(3015, 3024), frames(3134, 3144),
		frames(3210, 3218), frames(3247, 3249), frames(3411, 3411), frames(3436, 3465), frames(3503, 3570))
	for _, c := range []struct {
		name     string
		black    []float64
		duration float64
		want     float64
		ok       bool
	}{
		{"for all mankind", fam, 3575.6, 3436, true},
		{"one run to the end", frames(3400, 3595), 3600, 3400, true},
		{"dark scene far before the credits", join(frames(3000, 3030), frames(3500, 3598)), 3600, 3500, true},
		{"nothing black at the end", frames(3000, 3100), 3600, 0, false},
		{"too short to be credits", frames(3580, 3598), 3600, 0, false},
		{"no black frame", nil, 3600, 0, false},
	} {
		got, ok := creditsStart(c.black, c.duration)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%s: %v %v, want %v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
