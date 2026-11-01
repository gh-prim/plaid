package playback

import (
	"slices"
	"strings"
	"testing"
)

func TestTranscodeLayout(t *testing.T) {
	l := TranscodeLayout(61_500)
	if len(l.Segments) != 11 {
		t.Fatalf("%d segments, want 11", len(l.Segments))
	}
	for i, s := range l.Segments[:10] {
		if s.StartMs != int64(i)*6000 || s.EndMs-s.StartMs != 6000 || s.First != i || s.Last != i+1 {
			t.Fatalf("segment %d: %+v", i, s)
		}
	}
	if last := l.Segments[10]; last.StartMs != 60_000 || last.EndMs != 61_500 {
		t.Fatalf("last: %+v", last)
	}
	if l.SegmentAt(0) != 0 || l.SegmentAt(5999) != 0 || l.SegmentAt(6000) != 1 || l.SegmentAt(99_999) != 10 {
		t.Fatal("SegmentAt")
	}
}

func TestStreamLayout(t *testing.T) {
	type seg struct {
		start, end  int64
		first, last int
	}
	for _, tc := range []struct {
		name      string
		keyframes []int64
		duration  int64
		want      []seg
	}{
		{
			name:      "regular 2 s GOPs make 6 s segments",
			keyframes: []int64{0, 2000, 4000, 6000, 8000, 10_000, 12_000, 14_000},
			duration:  15_000,
			want:      []seg{{0, 6000, 0, 3}, {6000, 12_000, 3, 6}, {12_000, 15_000, 6, 8}},
		},
		{
			name:      "short tail merged into the previous segment",
			keyframes: []int64{0, 2000, 4000, 6000, 8000, 10_000, 12_000},
			duration:  13_000,
			want:      []seg{{0, 6000, 0, 3}, {6000, 13_000, 3, 7}},
		},
		{
			name:      "first keyframe after 0 still starts at 0",
			keyframes: []int64{83, 2083, 4083, 6083},
			duration:  9000,
			want:      []seg{{0, 6083, 0, 3}, {6083, 9000, 3, 4}},
		},
		{
			name:      "closes early rather than going over 10 s",
			keyframes: []int64{0, 3000, 11_000, 13_000, 19_000},
			duration:  25_000,
			want:      []seg{{0, 3000, 0, 1}, {3000, 11_000, 1, 2}, {11_000, 19_000, 2, 4}, {19_000, 25_000, 4, 5}},
		},
		{
			name:      "a 15 s GOP stays one segment",
			keyframes: []int64{0, 15_000, 17_000, 19_000, 21_000},
			duration:  24_000,
			want:      []seg{{0, 15_000, 0, 1}, {15_000, 21_000, 1, 4}, {21_000, 24_000, 4, 5}},
		},
		{
			name:      "dense keyframes",
			keyframes: []int64{0, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000, 5500, 6000, 6500},
			duration:  7000,
			want:      []seg{{0, 7000, 0, 14}},
		},
		{
			name:      "one keyframe",
			keyframes: []int64{0},
			duration:  4000,
			want:      []seg{{0, 4000, 0, 1}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := StreamLayout(tc.keyframes, tc.duration)
			var got []seg
			for _, s := range l.Segments {
				got = append(got, seg{s.StartMs, s.EndMs, s.First, s.Last})
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got  %v\nwant %v", got, tc.want)
			}
			for i, s := range l.Segments {
				for p := s.First; p < s.Last; p++ {
					if l.SegmentOf(p) != i {
						t.Fatalf("SegmentOf(%d) = %d, want %d", p, l.SegmentOf(p), i)
					}
				}
			}
		})
	}
}

func TestPlaylists(t *testing.T) {
	got := StreamLayout([]int64{0, 2000, 4000, 6000, 8000, 10_000, 12_000, 14_000}, 15_000).MediaPlaylist()
	want := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.000,
0.m4s
#EXTINF:6.000,
1.m4s
#EXTINF:3.000,
2.m4s
#EXT-X-ENDLIST
`
	if got != want {
		t.Fatalf("media playlist:\n%s\nwant:\n%s", got, want)
	}
	// A segment of 6.5 s rounds the target duration up.
	if got := StreamLayout([]int64{0, 6500}, 9000).MediaPlaylist(); !strings.Contains(got, "#EXT-X-TARGETDURATION:7\n") {
		t.Fatalf("target duration:\n%s", got)
	}

	got = SubtitlePlaylist(61_500, 3)
	want = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:62
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:61.500,
3.vtt?seg=0
#EXT-X-ENDLIST
`
	if got != want {
		t.Fatalf("subtitle playlist:\n%s\nwant:\n%s", got, want)
	}
	// A film is cut in 5-minute segments, the last one shorter.
	got = SubtitlePlaylist(700_000, 2)
	want = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:300
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:300.000,
2.vtt?seg=0
#EXTINF:300.000,
2.vtt?seg=1
#EXTINF:100.000,
2.vtt?seg=2
#EXT-X-ENDLIST
`
	if got != want {
		t.Fatalf("long subtitle playlist:\n%s\nwant:\n%s", got, want)
	}
}

func TestCutWebVTT(t *testing.T) {
	vtt := "WEBVTT\r\n\r\nNOTE made by FFmpeg\r\n\r\n" +
		"00:04:50.000 --> 00:05:10.000\r\nAcross the boundary\r\n\r\n" +
		"1\n00:05:20.500 --> 00:05:22.000 align:start\nInside\n\n" +
		"01:00:00.000 --> 01:00:02.000\nLater\n\n" +
		"04:00.000 --> 04:10.000\nBefore\n"
	got := string(CutWebVTT([]byte(vtt), 300_000, 600_000))
	want := "WEBVTT\n\nNOTE made by FFmpeg\n\n00:04:50.000 --> 00:05:10.000\nAcross the boundary\n\n" +
		"1\n00:05:20.500 --> 00:05:22.000 align:start\nInside\n\n"
	if got != want {
		t.Fatalf("cut:\n%q\nwant:\n%q", got, want)
	}
	// The first segment keeps the short form and the boundary cue too.
	if got := string(CutWebVTT([]byte(vtt), 0, 300_000)); !strings.Contains(got, "Before") || !strings.Contains(got, "Across") || strings.Contains(got, "Inside") {
		t.Fatalf("first segment:\n%s", got)
	}
}
