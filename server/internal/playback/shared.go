package playback

import (
	"fmt"
	"time"
)

// Shared encodes (F010-R25): the screens of one Plaid Party that need the
// same transcode of the same file (same audio track, burned-in subtitle and
// quality) read one FFmpeg run and its segments. It counts once toward
// maxTranscodes (R26). Each session keeps its own token; only the stream
// behind it is shared. Party screens play together, so they ask for the
// same segments; one that drifts away from the party leaves the shared run
// for its own (detachGap).

// detachGap is how far from the party's position a reader may ask before it
// gets its own run, so its seek doesn't move the run under the others.
const detachGap = 30 * time.Second

// shareKey is the identity of a shareable stream, "" when the session
// can't share (not in a party, or not a transcode).
func shareKey(partyID string, d Decision) string {
	if partyID == "" || d.Method != Transcode {
		return ""
	}
	audio, burned := -1, -1
	if d.Audio != nil {
		audio = d.Audio.Index
	}
	if d.Subtitle != nil && d.SubtitleDelivery == DeliveryBurnIn {
		burned = d.Subtitle.Index
	}
	return fmt.Sprintf("%s|%s|a%d|s%d|%s|%dx%d|%d|%s|%t|%s|%d|%s", partyID, d.Media.FileID, audio, burned,
		d.Quality, d.Width, d.Height, d.VideoBitrate, d.VideoCodec, d.Tonemap, d.AudioCodec, d.AudioChannels, d.HWAccel)
}

// canShare reports whether a party session with decision d would join a
// running shared stream, and so needs no transcode slot.
func (s *Service) canShare(partyID string, d Decision) bool {
	key := shareKey(partyID, d)
	if key == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shared[key] != nil
}

// attachShared registers sess on the shared stream of its key, if one
// runs. The reader is added before the stream is listed under the session,
// so the other readers leaving in between never close it.
func (s *Service) attachShared(sess Session) bool {
	key := shareKey(sess.PartyID, sess.Decision)
	if key == "" {
		return false
	}
	s.mu.Lock()
	st := s.shared[key]
	s.mu.Unlock()
	if st == nil {
		return false
	}
	st.mu.Lock()
	if st.closed {
		st.mu.Unlock()
		return false
	}
	st.readers[sess.ID] = st.layout.SegmentAt(sess.StartPositionMs)
	st.mu.Unlock()
	s.mu.Lock()
	s.streams[sess.ID] = st
	s.mu.Unlock()
	s.o.Log.Info("playback joins a shared encode", "playback_id", sess.ID, "party_id", sess.PartyID,
		"quality", sess.Decision.Quality, "with", st.sessionID)
	return true
}

// SharedReaders is how many sessions read a session's stream (1 when it
// isn't shared), for the admin view and tests.
func (s *Service) SharedReaders(sessionID string) int {
	st := s.stream(sessionID)
	if st == nil {
		return 0
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.share == "" {
		return 1
	}
	return len(st.readers)
}

// ownsSlot reports whether closing sessionID's stream frees a slot it can
// hand to its next stream: a shared stream's slot stays with its other
// readers. Callers hold st.mu.
func (st *hlsStream) ownsSlot(sessionID string) bool {
	if !st.slot {
		return false
	}
	if st.share == "" {
		return true
	}
	_, mine := st.readers[sessionID]
	return mine && len(st.readers) == 1
}

// readFrom records a reader's request; lastSeg is the furthest one (for
// throttling), behind() the slowest (for deleting old segments). Callers
// hold st.mu.
func (st *hlsStream) readFrom(sessionID string, n int) {
	if st.share == "" {
		st.lastSeg = n
		return
	}
	st.readers[sessionID] = n
	st.lastSeg = n
	for _, r := range st.readers {
		st.lastSeg = max(st.lastSeg, r)
	}
}

// behind is the slowest reader's segment. Callers hold st.mu.
func (st *hlsStream) behind() int {
	b := st.lastSeg
	for _, r := range st.readers {
		b = min(b, r)
	}
	return b
}

// drifted reports whether a party reader asking for a segment starting at
// startMs is too far from its party to keep sharing.
func (s *Service) drifted(st *hlsStream, sess Session, startMs int64) bool {
	if st.share == "" || s.o.Party == nil {
		return false
	}
	st.mu.Lock()
	others := len(st.readers) > 1
	st.mu.Unlock()
	if !others {
		return false
	}
	pos, ok := s.o.Party.Position(sess.PartyID)
	if !ok {
		return false
	}
	gap := startMs - pos
	if gap < 0 {
		gap = -gap
	}
	return gap > detachGap.Milliseconds()
}

// detach gives sess a run of its own (it needs a free transcode slot).
func (s *Service) detach(sess Session) error {
	if !s.acquireSlot() {
		return ErrTranscodeLimit
	}
	old := s.stream(sess.ID)
	gen := 0
	if old != nil {
		gen = genOf(old.dir) + 1
	}
	s.closeStream(sess.ID, false)
	sess.PartyID = "" // a solo stream
	s.newStream(sess, gen, true)
	s.o.Log.Info("playback leaves a shared encode", "playback_id", sess.ID)
	return nil
}
