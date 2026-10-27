package keyframes

import (
	"encoding/binary"
	"io"
	"slices"
)

// boxHeader reads an MP4 box header at off: its type, total size and
// header length.
func boxHeader(r io.ReaderAt, size, off int64) (typ string, total, n int64, err error) {
	b, err := readAt(r, size, off, min(16, size-off))
	if err != nil || len(b) < 8 {
		return "", 0, 0, ErrNoIndex
	}
	total, n, typ = int64(binary.BigEndian.Uint32(b)), 8, string(b[4:8])
	switch total {
	case 0:
		total = size - off
	case 1:
		if len(b) < 16 {
			return "", 0, 0, ErrNoIndex
		}
		total, n = int64(binary.BigEndian.Uint64(b[8:])), 16 //nolint:gosec // file sizes
	}
	if total < n || off+total > size {
		return "", 0, 0, ErrNoIndex
	}
	return typ, total, n, nil
}

// boxes walks the boxes of a body already in memory.
func boxes(b []byte, fn func(typ string, body []byte)) {
	for len(b) >= 8 {
		total, n := uint64(binary.BigEndian.Uint32(b)), uint64(8)
		if total == 1 && len(b) >= 16 {
			total, n = binary.BigEndian.Uint64(b[8:]), 16
		} else if total == 0 {
			total = uint64(len(b))
		}
		if total < n || total > uint64(len(b)) {
			return
		}
		fn(string(b[4:8]), b[n:total])
		b = b[total:]
	}
}

// readMP4 reads the video track's sync samples (stss) and turns them into
// presentation times with stts, ctts and the edit list. Fragmented files
// (mvex, moof) have no such index.
func readMP4(r io.ReaderAt, size int64) ([]int64, error) {
	var moov []byte
	for off := int64(0); off < size && moov == nil; {
		typ, total, n, err := boxHeader(r, size, off)
		if err != nil {
			return nil, err
		}
		switch typ {
		case "moov":
			if moov, err = readAt(r, size, off+n, total-n); err != nil {
				return nil, err
			}
		case "moof":
			return nil, ErrNoIndex
		}
		off += total
	}
	if moov == nil {
		return nil, ErrNoIndex
	}
	var (
		times []int64
		found bool
	)
	boxes(moov, func(typ string, body []byte) {
		switch typ {
		case "mvex":
			found, times = true, nil // fragmented: no usable index
		case "trak":
			if found {
				return
			}
			if t, ok := videoTrack(body); ok {
				found, times = true, t
			}
		}
	})
	if len(times) == 0 {
		return nil, ErrNoIndex
	}
	slices.Sort(times)
	return slices.Compact(times), nil
}

// videoTrack returns the keyframe times of a trak box when it's a video
// track; ok is false for other tracks.
func videoTrack(trak []byte) (times []int64, ok bool) {
	var (
		elst                   []byte
		hdlr, mdhd             []byte
		stss, stts, ctts       []byte
		mdia, minf, stbl, edts []byte
	)
	boxes(trak, func(typ string, b []byte) {
		switch typ {
		case "mdia":
			mdia = b
		case "edts":
			edts = b
		}
	})
	boxes(edts, func(typ string, b []byte) {
		if typ == "elst" {
			elst = b
		}
	})
	boxes(mdia, func(typ string, b []byte) {
		switch typ {
		case "hdlr":
			hdlr = b
		case "mdhd":
			mdhd = b
		case "minf":
			minf = b
		}
	})
	if len(hdlr) < 12 || string(hdlr[8:12]) != "vide" {
		return nil, false
	}
	boxes(minf, func(typ string, b []byte) {
		if typ == "stbl" {
			stbl = b
		}
	})
	boxes(stbl, func(typ string, b []byte) {
		switch typ {
		case "stss":
			stss = b
		case "stts":
			stts = b
		case "ctts":
			ctts = b
		}
	})
	timescale := mediaTimescale(mdhd)
	if timescale == 0 || stss == nil || stts == nil {
		return nil, true // a video track without sync samples listed: no index
	}
	sync := table(stss, 4)
	if len(sync) == 0 {
		return nil, true
	}
	deltas := table(stts, 8)
	offsets := table(ctts, 8)
	shift := editShift(elst)

	// Walk samples in order, computing decode and composition times; only
	// the sync samples are kept.
	var (
		dts         int64
		sample      uint32 = 1
		di, dn      int
		ci, cn      int
		next        int
		ctsVersion1 = len(ctts) > 0 && ctts[0] == 1
	)
	for next < len(sync) && di < len(deltas) {
		count, delta := binary.BigEndian.Uint32(deltas[di]), binary.BigEndian.Uint32(deltas[di][4:])
		for dn < int(count) && next < len(sync) {
			if sample == binary.BigEndian.Uint32(sync[next]) {
				pts := dts + compOffset(offsets, ci, ctsVersion1) - shift
				times = append(times, max(0, pts)*1000/int64(timescale))
				next++
			}
			dts += int64(delta)
			sample++
			dn++
			if ci < len(offsets) {
				if cn++; cn >= int(binary.BigEndian.Uint32(offsets[ci])) {
					ci, cn = ci+1, 0
				}
			}
		}
		di, dn = di+1, 0
	}
	return times, true
}

func mediaTimescale(mdhd []byte) uint32 {
	switch {
	case len(mdhd) >= 16 && mdhd[0] == 0:
		return binary.BigEndian.Uint32(mdhd[12:])
	case len(mdhd) >= 24 && mdhd[0] == 1:
		return binary.BigEndian.Uint32(mdhd[20:])
	}
	return 0
}

// table returns the entries of a full box holding a counted table of
// fixed-size entries (stss, stts, ctts).
func table(box []byte, entry int) [][]byte {
	if len(box) < 8 {
		return nil
	}
	n := int(binary.BigEndian.Uint32(box[4:]))
	body := box[8:]
	if n < 0 || n > len(body)/entry {
		return nil
	}
	out := make([][]byte, n)
	for i := range out {
		out[i] = body[i*entry : (i+1)*entry]
	}
	return out
}

func compOffset(offsets [][]byte, i int, signed bool) int64 {
	if i >= len(offsets) {
		return 0
	}
	v := binary.BigEndian.Uint32(offsets[i][4:])
	if signed {
		return int64(int32(v)) //nolint:gosec // ctts version 1 is signed
	}
	return int64(v)
}

// editShift is the media time the first edit starts at (B-frame delay),
// skipping an initial empty edit. Edits that start later aren't supported:
// FFmpeg handles them, but they're rare enough to ignore here.
func editShift(elst []byte) int64 {
	if len(elst) < 8 {
		return 0
	}
	n := int(binary.BigEndian.Uint32(elst[4:]))
	size, body := 12, elst[8:]
	if elst[0] == 1 {
		size = 20
	}
	for i := 0; i < n && (i+1)*size <= len(body); i++ {
		e := body[i*size:]
		var mediaTime int64
		if size == 12 {
			mediaTime = int64(int32(binary.BigEndian.Uint32(e[4:]))) //nolint:gosec // signed field
		} else {
			mediaTime = int64(binary.BigEndian.Uint64(e[8:])) //nolint:gosec // signed field
		}
		if mediaTime >= 0 {
			return mediaTime
		}
	}
	return 0
}
