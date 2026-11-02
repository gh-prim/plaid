package playback

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/gh-prim/plaid/server/internal/auth"
)

// fakeParty is a PartyAccess for one party at a settable position.
type fakeParty struct{ pos atomic.Int64 }

func (*fakeParty) InParty(context.Context, auth.Principal, string, string) (string, bool, error) {
	return "", true, nil
}
func (*fakeParty) GuestActive(context.Context, string) (bool, error) { return true, nil }
func (f *fakeParty) Position(string) (int64, bool)                   { return f.pos.Load(), true }

// addPartySession is addSession for a party screen: it joins a shared
// encode instead of taking a slot when it can (F010-R25).
func addPartySession(t *testing.T, s *Service, d Decision, user string) (Session, error) {
	t.Helper()
	slot := false
	if !s.canShare("party1", d) {
		var err error
		if slot, err = s.admit(d); err != nil {
			return Session{}, err
		}
	}
	sess := s.manager.Add(Session{UserID: user, ProfileID: "p-" + user, ItemID: "i", Decision: d, PartyID: "party1"})
	s.newStream(sess, 0, slot)
	return sess, nil
}

// TestSharedEncode covers F010-R25: two party screens needing the same
// transcode read one FFmpeg run, counted once toward maxTranscodes; it
// lives until its last reader leaves; a reader far from the party leaves
// it for a run of its own.
func TestSharedEncode(t *testing.T) {
	s, _ := hlsService(t, fastTimings)
	party := &fakeParty{}
	s.o.Party = party
	s.setConfig(func(c *Config) { c.MaxTranscodes = 1 })
	m := longMedia(t)
	d := transcodeOf(m)

	sam, err := addPartySession(t, s, d, "sam")
	if err != nil {
		t.Fatal(err)
	}
	leo, err := addPartySession(t, s, d, "leo")
	if err != nil {
		t.Fatalf("second party screen of the same encode: %v", err)
	}
	if n := s.SharedReaders(sam.ID); n != 2 {
		t.Fatalf("readers = %d", n)
	}
	// A solo transcode is still refused: the shared one holds the only slot.
	if _, err := s.admit(d); !errors.Is(err, ErrTranscodeLimit) {
		t.Fatalf("solo transcode: %v", err)
	}
	// Both read the same segments from the one run.
	ctx := context.Background()
	a, err := s.Segment(ctx, sam, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Segment(ctx, leo, 0)
	if err != nil || len(a) == 0 || a[0] != b[0] {
		t.Fatalf("segments %v / %v (%v)", a, b, err)
	}
	if s.stream(sam.ID) != s.stream(leo.ID) {
		t.Fatal("two streams")
	}

	// Sam leaves: Léo keeps the run and its slot.
	if err := s.manager.End(principalOf(sam), sam.ID, nil); err != nil {
		t.Fatal(err)
	}
	if n := s.SharedReaders(leo.ID); n != 1 {
		t.Fatalf("readers after sam left = %d", n)
	}
	if _, err := s.Segment(ctx, leo, 1); err != nil {
		t.Fatalf("léo after sam left: %v", err)
	}
	if _, err := s.admit(d); !errors.Is(err, ErrTranscodeLimit) {
		t.Fatal("slot freed while léo still reads")
	}
	// Léo leaves: the slot is free.
	if err := s.manager.End(principalOf(leo), leo.ID, nil); err != nil {
		t.Fatal(err)
	}
	if slot, err := s.admit(d); err != nil || !slot {
		t.Fatalf("slot after the last reader: %v", err)
	}
	s.releaseSlot()
}

func TestSharedEncodeDetach(t *testing.T) {
	s, _ := hlsService(t, fastTimings)
	party := &fakeParty{}
	s.o.Party = party
	s.setConfig(func(c *Config) { c.MaxTranscodes = 2 })
	m := longMedia(t)
	d := transcodeOf(m)
	sam, _ := addPartySession(t, s, d, "sam")
	leo, _ := addPartySession(t, s, d, "leo")
	ctx := context.Background()
	if _, err := s.Segment(ctx, sam, 0); err != nil {
		t.Fatal(err)
	}
	// The party is at the start; Léo asks for a segment minutes away: he
	// gets his own run, Sam's isn't moved.
	far := s.stream(leo.ID).layout.SegmentAt(m.DurationMs / 2)
	if _, err := s.Segment(ctx, leo, far); err != nil {
		t.Fatal(err)
	}
	if s.stream(sam.ID) == s.stream(leo.ID) {
		t.Fatal("léo still shares after drifting")
	}
	if s.SharedReaders(sam.ID) != 1 || s.SharedReaders(leo.ID) != 1 {
		t.Errorf("readers sam %d, léo %d", s.SharedReaders(sam.ID), s.SharedReaders(leo.ID))
	}
}

func TestShareKey(t *testing.T) {
	m := Media{FileID: "f"}
	d := Decision{Method: Transcode, Media: m, Quality: "720p", Width: 1280, Height: 720}
	if shareKey("", d) != "" {
		t.Error("shared outside a party")
	}
	if shareKey("p", Decision{Method: DirectStream, Media: m}) != "" {
		t.Error("direct stream shared")
	}
	other := d
	other.Quality, other.Height = "1080p", 1080
	same := d
	if shareKey("p", d) == shareKey("p", other) || shareKey("p", d) != shareKey("p", same) || shareKey("p", d) == shareKey("q", d) {
		t.Error("keys don't tell streams apart")
	}
}
