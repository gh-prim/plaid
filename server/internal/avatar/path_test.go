package avatar

import (
	"strings"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"M10 20l5 5h5v-5z", "M10.000 20.000L15.000 25.000L20.000 25.000L20.000 20.000Z"},
		// Pairs after a moveto are linetos; relative ones from the last point.
		{"m1 1 2 2 3 3", "M1.000 1.000L3.000 3.000L6.000 6.000"},
		// Numbers run together: "1.5.5" is 1.5 then .5, "-1-2" two numbers.
		{"M1.5.5L-1-2", "M1.500 0.500L-1.000 -2.000"},
		{"M0 0q10 0 10 10", "M0.000 0.000C6.667 0.000 10.000 3.333 10.000 10.000"},
	} {
		got, err := normalizePath(c.in)
		if err != nil || got != c.want {
			t.Errorf("%q: got %q (%v), want %q", c.in, got, err, c.want)
		}
	}
	// Arc flags with no separator, as minifiers write them: a quarter
	// circle from (0,10) to (10,0) is one cubic ending there.
	got, err := normalizePath("M0 10a10 10 0 01 10-10")
	if err != nil || strings.Count(got, "C") != 1 || !strings.HasSuffix(got, "10.000 0.000") {
		t.Errorf("arc: %q (%v)", got, err)
	}
	if _, err := normalizePath("M0 0a10 10 0 2 1 5 5"); err == nil {
		t.Error("a flag of 2 was accepted")
	}
	// Every path of the style reads.
	for _, c := range def.Components {
		for _, v := range c.Variants {
			var walk func([]element)
			walk = func(els []element) {
				for _, e := range els {
					if d, ok := e.Attributes["d"].(string); ok {
						if _, err := normalizePath(d); err != nil {
							t.Fatalf("%v: %.60s", err, d)
						}
					}
					walk(e.Children)
				}
			}
			walk(v.Elements)
		}
	}
}
