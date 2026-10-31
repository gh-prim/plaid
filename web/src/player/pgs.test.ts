import { describe, expect, it } from 'vitest'
import { decodeObject, parsePgs, setAt } from './pgs'

/** One PGS segment: "PG", PTS (90 kHz), DTS, type, size, body. */
function seg(ms: number, type: number, body: number[]): number[] {
  const pts = ms * 90
  const size = body.length
  return [
    0x50,
    0x47,
    (pts >>> 24) & 255,
    (pts >>> 16) & 255,
    (pts >>> 8) & 255,
    pts & 255,
    0,
    0,
    0,
    0,
    type,
    (size >> 8) & 255,
    size & 255,
    ...body,
  ]
}
const u16 = (n: number) => [(n >> 8) & 255, n & 255]

/** A 1920×1080 set at `ms` with one 4×2 object at (100, 900), and a clear at `endMs`. */
function sup(ms: number, endMs: number): ArrayBuffer {
  const pcs = [
    ...u16(1920),
    ...u16(1080),
    0x10,
    ...u16(0),
    0x80,
    0,
    0,
    1,
    ...u16(0),
    0,
    0,
    ...u16(100),
    ...u16(900),
  ]
  // Palette 0: entry 1 is opaque white (Y 235, Cr 128, Cb 128).
  const pds = [0, 0, 1, 235, 128, 128, 255]
  // 4×2 RLE: line 1 = 4 pixels of color 1, line 2 = 4 of color 0.
  const rle = [0x00, 0x84, 1, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00]
  const len = rle.length + 4
  const ods = [
    ...u16(0),
    0,
    0xc0,
    (len >> 16) & 255,
    (len >> 8) & 255,
    len & 255,
    ...u16(4),
    ...u16(2),
    ...rle,
  ]
  const clear = [...u16(1920), ...u16(1080), 0x10, ...u16(1), 0x00, 0, 0, 0]
  const bytes = [
    ...seg(ms, 0x16, pcs),
    ...seg(ms, 0x14, pds),
    ...seg(ms, 0x15, ods),
    ...seg(ms, 0x80, []),
    ...seg(endMs, 0x16, clear),
    ...seg(endMs, 0x80, []),
  ]
  return new Uint8Array(bytes).buffer
}

describe('PGS', () => {
  it('reads display sets, shows one between its start and the clear, and decodes it', () => {
    const sets = parsePgs(sup(1000, 3000))
    expect(sets.map((s) => [s.startMs, s.placements.length])).toEqual([
      [1000, 1],
      [3000, 0],
    ])
    expect(setAt(sets, 500)).toBeUndefined()
    const shown = setAt(sets, 2000)!
    expect(shown.placements[0]).toMatchObject({ x: 100, y: 900, width: 4, height: 2 })
    expect(setAt(sets, 3500)!.placements).toHaveLength(0)

    const img = decodeObject(shown.placements[0]!)
    expect([img.width, img.height]).toEqual([4, 2])
    // First line white and opaque, second line transparent.
    expect([...img.data.slice(0, 4)]).toEqual([255, 255, 255, 255])
    expect(img.data[4 * 4 + 3]).toBe(0)
  })
})
