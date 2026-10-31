package slug

import (
	"strings"
	"testing"
)

func TestMake(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"Living room", "living-room"},
		{"  Living   Room!! ", "living-room"},
		{"PLAID", "plaid"},
		{"Salón de Gauthier", "sal-n-de-gauthier"},
		{"---", "plaid"},
		{"", "plaid"},
		{"日本語", "plaid"},
		{"a_b.c", "a-b-c"},
		{"Server 2", "server-2"},
		{"abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrst"},
		{"living room upstairs again", "living-room-upstairs"},
		{"living room upstair again", "living-room-upstair"}, // the cut lands on a dash
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Make(tt.name)
			if got != tt.want {
				t.Fatalf("Make(%q) = %q, want %q", tt.name, got, tt.want)
			}
			if !validSlug(got) {
				t.Fatalf("Make(%q) = %q is not a valid slug", tt.name, got)
			}
		})
	}
}

func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id := NewID()
		if !ValidID(id) {
			t.Fatalf("NewID() = %q, not a valid id", id)
		}
		seen[id] = true
	}
	if len(seen) < 990 {
		t.Fatalf("only %d distinct ids out of 1000", len(seen))
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		label, slug, id string
		ok              bool
	}{
		{"living-room-k3x7qa", "living-room", "k3x7qa", true},
		{"plaid-abcdef", "plaid", "abcdef", true},
		{"a-234567", "a", "234567", true},
		{"k3x7qa", "", "", false},
		{"-k3x7qa", "", "", false},
		{"living-room-k3x7q", "", "", false},
		{"living-room-k3x7q1", "", "", false}, // 1 is not base32
		{"living-room-K3X7QA", "", "", false},
		{"living--room-k3x7qa", "", "", false},
		{"living-room--k3x7qa", "", "", false},
		{strings.Repeat("a", 21) + "-k3x7qa", "", "", false},
		{"liv_ing-k3x7qa", "", "", false},
		{"", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			s, id, ok := Parse(tt.label)
			if s != tt.slug || id != tt.id || ok != tt.ok {
				t.Fatalf("Parse(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.label, s, id, ok, tt.slug, tt.id, tt.ok)
			}
		})
	}
}
