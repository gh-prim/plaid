package keyframes

import (
	"io"
	"slices"
)

// Matroska element ids (with their length marker, as written).
const (
	idEBML           = 0x1A45DFA3
	idSegment        = 0x18538067
	idSeekHead       = 0x114D9B74
	idSeek           = 0x4DBB
	idSeekID         = 0x53AB
	idSeekPosition   = 0x53AC
	idInfo           = 0x1549A966
	idTimestampScale = 0x2AD7B1
	idTracks         = 0x1654AE6B
	idTrackEntry     = 0xAE
	idTrackNumber    = 0xD7
	idTrackType      = 0x83
	idCues           = 0x1C53BB6B
	idCuePoint       = 0xBB
	idCueTime        = 0xB3
	idCuePositions   = 0xB7
	idCueTrack       = 0xF7
	idCluster        = 0x1F43B675
)

// unknownSize is an element whose size field is all ones (live streams).
const unknownSize = -1

// ebmlHeader reads an element header at off: its id, body size and header
// length. Element ids are at most 4 bytes, sizes at most 8.
func ebmlHeader(r io.ReaderAt, size, off int64) (id uint32, body int64, n int64, err error) {
	b, err := readAt(r, size, off, min(12, size-off))
	if err != nil || len(b) < 2 {
		return 0, 0, 0, ErrNoIndex
	}
	idLen := vintLen(b[0])
	if idLen == 0 || idLen > 4 {
		return 0, 0, 0, ErrNoIndex
	}
	for _, c := range b[:idLen] {
		id = id<<8 | uint32(c)
	}
	if int(idLen) >= len(b) {
		return 0, 0, 0, ErrNoIndex
	}
	sizeLen := vintLen(b[idLen])
	if sizeLen == 0 || int(idLen+sizeLen) > len(b) {
		return 0, 0, 0, ErrNoIndex
	}
	v, all := vintValue(b[idLen : idLen+sizeLen])
	if all {
		return id, unknownSize, int64(idLen + sizeLen), nil
	}
	return id, int64(v), int64(idLen + sizeLen), nil //nolint:gosec // at most 56 bits
}

// vintLen is the length of a variable-size integer from its first byte.
func vintLen(first byte) uint8 {
	for i := uint8(0); i < 8; i++ {
		if first&(0x80>>i) != 0 {
			return i + 1
		}
	}
	return 0
}

// vintValue decodes a size without its marker bit; all reports the
// reserved all-ones value.
func vintValue(b []byte) (v uint64, all bool) {
	mask := byte(0xFF >> len(b))
	v = uint64(b[0] & mask)
	all = b[0]&mask == mask
	for _, c := range b[1:] {
		v = v<<8 | uint64(c)
		all = all && c == 0xFF
	}
	return v, all
}

// children walks the elements inside an element body already in memory.
func children(b []byte, fn func(id uint32, body []byte)) {
	for len(b) >= 2 {
		idLen := int(vintLen(b[0]))
		if idLen == 0 || idLen > 4 || idLen >= len(b) {
			return
		}
		var id uint32
		for _, c := range b[:idLen] {
			id = id<<8 | uint32(c)
		}
		sizeLen := int(vintLen(b[idLen]))
		if sizeLen == 0 || idLen+sizeLen > len(b) {
			return
		}
		v, all := vintValue(b[idLen : idLen+sizeLen])
		start := idLen + sizeLen
		if all || v > uint64(len(b)-start) { //nolint:gosec // len is positive
			return
		}
		fn(id, b[start:start+int(v)]) //nolint:gosec // bounded above
		b = b[start+int(v):]          //nolint:gosec // bounded above
	}
}

func uintOf(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

// readMatroska reads Cues through the SeekHead, looking only at the
// top-level elements before the first Cluster and at the ones the SeekHead
// points to.
func readMatroska(r io.ReaderAt, size int64) ([]int64, error) {
	id, body, n, err := ebmlHeader(r, size, 0)
	if err != nil || id != idEBML || body == unknownSize {
		return nil, ErrNoIndex
	}
	id, segSize, n2, err := ebmlHeader(r, size, n+body)
	if err != nil || id != idSegment {
		return nil, ErrNoIndex
	}
	segStart := n + body + n2
	segEnd := size
	if segSize != unknownSize && segStart+segSize < size {
		segEnd = segStart + segSize
	}

	var info, tracks, cues []byte
	positions := map[uint32]int64{}
	load := func(want uint32, off int64) ([]byte, error) {
		id, body, n, err := ebmlHeader(r, segEnd, off)
		if err != nil || id != want || body == unknownSize {
			return nil, ErrNoIndex
		}
		return readAt(r, segEnd, off+n, body)
	}
	readSeekHead := func(b []byte) {
		children(b, func(id uint32, seek []byte) {
			if id != idSeek {
				return
			}
			var target uint32
			pos := int64(-1)
			children(seek, func(id uint32, v []byte) {
				switch id {
				case idSeekID:
					target = uint32(uintOf(v)) //nolint:gosec // ids are 4 bytes
				case idSeekPosition:
					pos = int64(uintOf(v)) //nolint:gosec // offsets fit
				}
			})
			if _, seen := positions[target]; !seen && pos >= 0 {
				positions[target] = segStart + pos
			}
		})
	}

	// Top-level elements up to the first Cluster: a few small headers.
	off := segStart
	for i := 0; i < 32 && off < segEnd; i++ {
		id, body, n, err := ebmlHeader(r, segEnd, off)
		if err != nil || body == unknownSize || id == idCluster {
			break
		}
		switch id {
		case idSeekHead, idInfo, idTracks, idCues:
			b, err := readAt(r, segEnd, off+n, body)
			if err != nil {
				return nil, err
			}
			switch id {
			case idSeekHead:
				readSeekHead(b)
			case idInfo:
				info = b
			case idTracks:
				tracks = b
			case idCues:
				cues = b
			}
		}
		off += n + body
	}
	// A second SeekHead (some muxers put one at the end) can list Cues.
	if p, ok := positions[idSeekHead]; ok && p != segStart {
		if b, err := load(idSeekHead, p); err == nil {
			readSeekHead(b)
		}
	}
	for _, e := range []struct {
		id  uint32
		dst *[]byte
	}{{idInfo, &info}, {idTracks, &tracks}, {idCues, &cues}} {
		if *e.dst != nil {
			continue
		}
		p, ok := positions[e.id]
		if !ok {
			continue
		}
		b, err := load(e.id, p)
		if err != nil && e.id == idCues {
			return nil, ErrNoIndex
		}
		*e.dst = b
	}
	if cues == nil || tracks == nil {
		return nil, ErrNoIndex
	}

	scale := uint64(1_000_000) // ns per timestamp tick
	children(info, func(id uint32, v []byte) {
		if id == idTimestampScale && uintOf(v) > 0 {
			scale = uintOf(v)
		}
	})
	video := uint64(0)
	children(tracks, func(id uint32, entry []byte) {
		if id != idTrackEntry || video != 0 {
			return
		}
		var num, typ uint64
		children(entry, func(id uint32, v []byte) {
			switch id {
			case idTrackNumber:
				num = uintOf(v)
			case idTrackType:
				typ = uintOf(v)
			}
		})
		if typ == 1 {
			video = num
		}
	})
	if video == 0 {
		return nil, ErrNoIndex
	}

	var times []int64
	children(cues, func(id uint32, point []byte) {
		if id != idCuePoint {
			return
		}
		var (
			t     uint64
			match bool
		)
		children(point, func(id uint32, v []byte) {
			switch id {
			case idCueTime:
				t = uintOf(v)
			case idCuePositions:
				children(v, func(id uint32, v []byte) {
					if id == idCueTrack && uintOf(v) == video {
						match = true
					}
				})
			}
		})
		if match {
			times = append(times, int64(t*scale/1_000_000)) //nolint:gosec // film lengths
		}
	})
	if len(times) == 0 {
		return nil, ErrNoIndex
	}
	slices.Sort(times)
	return slices.Compact(times), nil
}
