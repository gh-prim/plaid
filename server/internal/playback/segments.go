package playback

import (
	"fmt"
	"math"
	"strings"
)

// Segment lengths (R12, R13), in milliseconds.
const (
	segmentMs    = 6000
	minSegmentMs = 2000
	maxSegmentMs = 10_000
)

// Segment is one HLS media segment. It's made of the FFmpeg output files
// ("pieces") First to Last-1: a transcode writes one piece per segment; a
// Direct Stream run writes one piece per source keyframe, and PLAID joins
// a segment's pieces when serving it, because FFmpeg's own cut points
// depend on where a run started.
type Segment struct {
	StartMs, EndMs int64
	First, Last    int
}

// Layout is a session's whole segment list, fixed when the session starts.
type Layout struct {
	Segments []Segment
	// PieceStartMs is where each piece starts: the -ss of a run that begins
	// with it.
	PieceStartMs []int64
}

// TranscodeLayout cuts a duration into 6 s segments: segment N starts at
// N × 6 s (R12).
func TranscodeLayout(durationMs int64) Layout {
	n := max(1, int((durationMs+segmentMs-1)/segmentMs))
	l := Layout{Segments: make([]Segment, n), PieceStartMs: make([]int64, n)}
	for i := range n {
		start := int64(i) * segmentMs
		l.Segments[i] = Segment{StartMs: start, EndMs: min(start+segmentMs, max(durationMs, start+1)), First: i, Last: i + 1}
		l.PieceStartMs[i] = start
	}
	return l
}

// StreamLayout groups source keyframes into segments of about 6 s, never
// shorter than 2 s nor longer than 10 s unless a single keyframe interval
// is (R13). keyframes must be ascending and non-empty.
func StreamLayout(keyframes []int64, durationMs int64) Layout {
	l := Layout{PieceStartMs: append([]int64(nil), keyframes...)}
	l.PieceStartMs[0] = 0 // the first piece starts at the beginning
	start := 0
	closeAt := func(i int) {
		l.Segments = append(l.Segments, Segment{StartMs: l.PieceStartMs[start], EndMs: keyframes[i], First: start, Last: i})
		start = i
	}
	for i := 1; i < len(keyframes); i++ {
		length := keyframes[i] - l.PieceStartMs[start]
		switch {
		case length >= segmentMs:
			closeAt(i)
		case length >= minSegmentMs && i+1 < len(keyframes) && keyframes[i+1]-l.PieceStartMs[start] > maxSegmentMs:
			closeAt(i) // the next keyframe would make it too long
		}
	}
	end := max(durationMs, keyframes[len(keyframes)-1]+1)
	last := Segment{StartMs: l.PieceStartMs[start], EndMs: end, First: start, Last: len(keyframes)}
	if n := len(l.Segments); n > 0 && last.EndMs-last.StartMs < minSegmentMs {
		l.Segments[n-1].EndMs, l.Segments[n-1].Last = last.EndMs, last.Last // too short: merged
	} else {
		l.Segments = append(l.Segments, last)
	}
	return l
}

// Pieces is the number of pieces in the layout.
func (l Layout) Pieces() int { return len(l.PieceStartMs) }

// SegmentOf returns the segment a piece belongs to.
func (l Layout) SegmentOf(piece int) int {
	lo, hi := 0, len(l.Segments)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if l.Segments[mid].First <= piece {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// SegmentAt returns the segment playing at a position.
func (l Layout) SegmentAt(ms int64) int {
	for i, s := range l.Segments {
		if ms < s.EndMs {
			return i
		}
	}
	return len(l.Segments) - 1
}

// MediaPlaylist is the VOD playlist of every segment (R11).
func (l Layout) MediaPlaylist() string {
	var b strings.Builder
	target := 1.0
	for _, s := range l.Segments {
		target = math.Max(target, math.Ceil(float64(s.EndMs-s.StartMs)/1000))
	}
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n", int(target))
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	for i, s := range l.Segments {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n%d.m4s\n", float64(s.EndMs-s.StartMs)/1000, i)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// SubtitlePlaylist lists one WebVTT file covering the whole duration (R16).
func SubtitlePlaylist(durationMs int64, index int) string {
	secs := float64(max(durationMs, 1)) / 1000
	return fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n"+
		"#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:%.3f,\n%d.vtt\n#EXT-X-ENDLIST\n", int(math.Ceil(secs)), secs, index)
}
