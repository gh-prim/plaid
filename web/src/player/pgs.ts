/**
 * PGS (Blu-ray image subtitles, a .sup stream) for the web player: the
 * server sends the track as is and the player draws it over the video
 * (V2, like Plex), instead of burning it into a transcode.
 *
 * A .sup is a list of segments ("PG", PTS, DTS, type, size, data) grouped
 * into display sets that end with an END segment. Each set says, at its
 * PTS, which objects (RLE images) show where, with which palette; a set
 * with no object clears the screen. Images are decoded only when shown.
 */

const PCS = 0x16
const WDS = 0x17
const PDS = 0x14
const ODS = 0x15
const END = 0x80

/** One RGBA color per palette entry, as 0xAABBGGRR for an ImageData view. */
type Palette = Uint32Array

interface PgsObject {
  width: number
  height: number
  /** The RLE bytes, joined across ODS segments. */
  rle: Uint8Array
}

export interface PgsPlacement {
  x: number
  y: number
  width: number
  height: number
  object: PgsObject
  palette: Palette
}

/** What shows from `startMs` until the next set (an empty list clears). */
export interface PgsDisplaySet {
  startMs: number
  /** The composition's size, which the placements are in. */
  width: number
  height: number
  placements: PgsPlacement[]
}

/** Reads a .sup into its display sets, in time order. */
export function parsePgs(buf: ArrayBuffer): PgsDisplaySet[] {
  const data = new Uint8Array(buf)
  const view = new DataView(buf)
  const sets: PgsDisplaySet[] = []
  const palettes = new Map<number, Palette>()
  const objects = new Map<number, PgsObject>()
  // Objects arriving in pieces, until their last ODS.
  const partial = new Map<number, { width: number; height: number; parts: Uint8Array[] }>()
  let pcs:
    | {
        startMs: number
        width: number
        height: number
        paletteId: number
        refs: { objectId: number; x: number; y: number }[]
      }
    | undefined

  let at = 0
  while (at + 13 <= data.length) {
    if (data[at] !== 0x50 || data[at + 1] !== 0x47) {
      at++ // not a segment start: resync on the next "PG"
      continue
    }
    const pts = view.getUint32(at + 2)
    const type = data[at + 10]!
    const size = view.getUint16(at + 11)
    const body = at + 13
    at = body + size
    if (at > data.length) break

    switch (type) {
      case PCS: {
        const state = data[body + 7]!
        // An epoch start forgets every object and palette before it.
        if (state & 0x80) {
          objects.clear()
          palettes.clear()
        }
        const count = data[body + 10]!
        const refs: { objectId: number; x: number; y: number }[] = []
        let p = body + 11
        for (let i = 0; i < count && p + 8 <= body + size; i++) {
          const cropped = data[p + 3]! & 0x40
          refs.push({
            objectId: view.getUint16(p),
            x: view.getUint16(p + 4),
            y: view.getUint16(p + 6),
          })
          p += cropped ? 16 : 8
        }
        pcs = {
          startMs: Math.round(pts / 90),
          width: view.getUint16(body),
          height: view.getUint16(body + 2),
          paletteId: data[body + 9]!,
          refs,
        }
        break
      }
      case PDS: {
        const id = data[body]!
        const palette = palettes.get(id) ?? new Uint32Array(256)
        for (let p = body + 2; p + 5 <= body + size; p += 5) {
          palette[data[p]!] = ycbcrToRgba(data[p + 1]!, data[p + 3]!, data[p + 2]!, data[p + 4]!)
        }
        palettes.set(id, palette)
        break
      }
      case ODS: {
        const id = view.getUint16(body)
        const seq = data[body + 3]!
        let start = body + 4
        if (seq & 0x80) {
          // First piece: a 3-byte length, then the size.
          partial.set(id, {
            width: view.getUint16(body + 7),
            height: view.getUint16(body + 9),
            parts: [],
          })
          start = body + 11
        }
        const piece = partial.get(id)
        if (!piece) break
        piece.parts.push(data.subarray(start, body + size))
        if (seq & 0x40) {
          const len = piece.parts.reduce((n, x) => n + x.length, 0)
          const rle = new Uint8Array(len)
          let o = 0
          for (const x of piece.parts) {
            rle.set(x, o)
            o += x.length
          }
          objects.set(id, { width: piece.width, height: piece.height, rle })
          partial.delete(id)
        }
        break
      }
      case END: {
        if (!pcs) break
        const palette = palettes.get(pcs.paletteId) ?? new Uint32Array(256)
        const placements: PgsPlacement[] = []
        for (const r of pcs.refs) {
          const object = objects.get(r.objectId)
          if (object)
            placements.push({
              x: r.x,
              y: r.y,
              width: object.width,
              height: object.height,
              object,
              palette,
            })
        }
        sets.push({ startMs: pcs.startMs, width: pcs.width, height: pcs.height, placements })
        pcs = undefined
        break
      }
      case WDS:
      default:
        break
    }
  }
  return sets.sort((a, b) => a.startMs - b.startMs)
}

/** The set showing at `ms`: the last one started, if any. */
export function setAt(sets: PgsDisplaySet[], ms: number): PgsDisplaySet | undefined {
  let lo = 0
  let hi = sets.length - 1
  let found: PgsDisplaySet | undefined
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (sets[mid]!.startMs <= ms) {
      found = sets[mid]
      lo = mid + 1
    } else hi = mid - 1
  }
  return found
}

/** Decodes an object's RLE into RGBA pixels with a palette. */
export function decodeObject(p: PgsPlacement): {
  width: number
  height: number
  data: Uint8ClampedArray<ArrayBuffer>
} {
  const { width, height, rle } = p.object
  const out = new Uint32Array(width * height)
  let i = 0
  let x = 0
  let y = 0
  while (i < rle.length && y < height) {
    const b = rle[i++]!
    let run = 1
    let color = b
    if (b === 0) {
      const f = rle[i++] ?? 0
      if (f === 0) {
        // End of line.
        x = 0
        y++
        continue
      }
      const long = f & 0x40
      run = long ? ((f & 0x3f) << 8) | (rle[i++] ?? 0) : f & 0x3f
      color = f & 0x80 ? (rle[i++] ?? 0) : 0
    }
    const rgba = p.palette[color] ?? 0
    const row = y * width
    for (let k = 0; k < run && x < width; k++) out[row + x++] = rgba
  }
  return { width, height, data: new Uint8ClampedArray(out.buffer) }
}

/** BT.709 (HD Blu-ray) to RGBA, little-endian for an ImageData view. */
function ycbcrToRgba(y: number, cb: number, cr: number, a: number): number {
  const c = (v: number) => Math.max(0, Math.min(255, Math.round(v)))
  const yy = 1.164 * (y - 16)
  const r = c(yy + 1.793 * (cr - 128))
  const g = c(yy - 0.213 * (cb - 128) - 0.533 * (cr - 128))
  const bl = c(yy + 2.112 * (cb - 128))
  return ((a << 24) | (bl << 16) | (g << 8) | r) >>> 0
}
