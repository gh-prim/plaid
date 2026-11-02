import AVFoundation
import CoreGraphics
import SwiftUI

// PGS (Blu-ray image subtitles, a .sup stream), as web/src/player/pgs.ts:
// the server sends the track as is and the app draws it over the picture,
// instead of burning it into a transcode, so the picture stays the original.
//
// A .sup is a list of segments ("PG", PTS, DTS, type, size, data) grouped
// into display sets ending with an END segment. Each set says, at its PTS,
// which objects (RLE images) show where, with which palette; a set with no
// object clears the screen. Images are decoded only when shown.

nonisolated enum PGS {
    struct Object {
        let width: Int
        let height: Int
        /// The RLE bytes, joined across ODS segments.
        let rle: [UInt8]
    }

    struct Placement {
        let x: Int
        let y: Int
        let object: Object
        /// RGBA per palette entry.
        let palette: [UInt32]
    }

    /// What shows from `startMs` until the next set (no placement clears).
    struct DisplaySet {
        let startMs: Int64
        /// The composition's size, which the placements are in.
        let width: Int
        let height: Int
        let placements: [Placement]
    }

    private static let pcs: UInt8 = 0x16, pds: UInt8 = 0x14, ods: UInt8 = 0x15, end: UInt8 = 0x80

    /// Reads a .sup into its display sets, in time order.
    static func parse(_ data: [UInt8]) -> [DisplaySet] {
        func u16(_ i: Int) -> Int { Int(data[i]) << 8 | Int(data[i + 1]) }
        func u32(_ i: Int) -> Int64 { Int64(u16(i)) << 16 | Int64(u16(i + 2)) }

        var sets: [DisplaySet] = []
        var palettes: [Int: [UInt32]] = [:]
        var objects: [Int: Object] = [:]
        // Objects arriving in pieces, until their last ODS.
        var partial: [Int: (width: Int, height: Int, rle: [UInt8])] = [:]
        var composition: (startMs: Int64, width: Int, height: Int, paletteId: Int, refs: [(id: Int, x: Int, y: Int)])?

        var at = 0
        while at + 13 <= data.count {
            guard data[at] == 0x50, data[at + 1] == 0x47 else { at += 1; continue } // resync on "PG"
            let pts = u32(at + 2)
            let type = data[at + 10]
            let size = u16(at + 11)
            let body = at + 13
            at = body + size
            guard at <= data.count else { break }

            switch type {
            case pcs:
                // An epoch start forgets every object and palette before it.
                if data[body + 7] & 0x80 != 0 {
                    objects.removeAll()
                    palettes.removeAll()
                }
                let count = Int(data[body + 10])
                var refs: [(id: Int, x: Int, y: Int)] = []
                var p = body + 11
                for _ in 0..<count where p + 8 <= body + size {
                    let cropped = data[p + 3] & 0x40 != 0
                    refs.append((u16(p), u16(p + 4), u16(p + 6)))
                    p += cropped ? 16 : 8
                }
                composition = (pts / 90, u16(body), u16(body + 2), Int(data[body + 9]), refs)
            case pds:
                let id = Int(data[body])
                var palette = palettes[id] ?? Array(repeating: 0, count: 256)
                var p = body + 2
                while p + 5 <= body + size {
                    palette[Int(data[p])] = rgba(y: data[p + 1], cb: data[p + 3], cr: data[p + 2], a: data[p + 4])
                    p += 5
                }
                palettes[id] = palette
            case ods:
                let id = u16(body)
                let seq = data[body + 3]
                var start = body + 4
                if seq & 0x80 != 0 {
                    // First piece: a 3-byte length, then the size.
                    partial[id] = (u16(body + 7), u16(body + 9), [])
                    start = body + 11
                }
                guard start <= body + size, partial[id] != nil else { break }
                partial[id]!.rle.append(contentsOf: data[start..<(body + size)])
                if seq & 0x40 != 0, let piece = partial.removeValue(forKey: id) {
                    objects[id] = Object(width: piece.width, height: piece.height, rle: piece.rle)
                }
            case end:
                guard let c = composition else { break }
                let palette = palettes[c.paletteId] ?? Array(repeating: 0, count: 256)
                let placements = c.refs.compactMap { r in objects[r.id].map { Placement(x: r.x, y: r.y, object: $0, palette: palette) } }
                sets.append(DisplaySet(startMs: c.startMs, width: c.width, height: c.height, placements: placements))
                composition = nil
            default:
                break
            }
        }
        return sets.sorted { $0.startMs < $1.startMs }
    }

    /// The set showing at `ms`: the last one started, if any.
    static func set(at ms: Int64, in sets: [DisplaySet]) -> DisplaySet? {
        var lo = 0, hi = sets.count - 1
        var found: DisplaySet?
        while lo <= hi {
            let mid = (lo + hi) / 2
            if sets[mid].startMs <= ms { found = sets[mid]; lo = mid + 1 } else { hi = mid - 1 }
        }
        return found
    }

    /// Decodes a placement's RLE into an image with its palette.
    static func image(_ p: Placement) -> CGImage? {
        let w = p.object.width, h = p.object.height
        guard w > 0, h > 0 else { return nil }
        var out = [UInt32](repeating: 0, count: w * h)
        let rle = p.object.rle
        var i = 0, x = 0, y = 0
        func next() -> UInt8 { defer { i += 1 }; return i < rle.count ? rle[i] : 0 }
        while i < rle.count && y < h {
            let b = next()
            var run = 1
            var color = Int(b)
            if b == 0 {
                let f = next()
                if f == 0 { x = 0; y += 1; continue } // end of line
                run = f & 0x40 != 0 ? Int(f & 0x3f) << 8 | Int(next()) : Int(f & 0x3f)
                color = f & 0x80 != 0 ? Int(next()) : 0
            }
            let px = p.palette[color]
            var k = 0
            while k < run && x < w { out[y * w + x] = px; x += 1; k += 1 }
        }
        let bytes = out.withUnsafeBytes { Data($0) }
        guard let provider = CGDataProvider(data: bytes as CFData) else { return nil }
        // Bytes in memory: R, G, B, A, not premultiplied.
        return CGImage(width: w, height: h, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: w * 4,
                       space: CGColorSpace(name: CGColorSpace.sRGB)!,
                       bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.last.rawValue),
                       provider: provider, decode: nil, shouldInterpolate: true, intent: .defaultIntent)
    }

    /// BT.709 (HD Blu-ray) to R, G, B, A bytes in memory order.
    private static func rgba(y: UInt8, cb: UInt8, cr: UInt8, a: UInt8) -> UInt32 {
        func c(_ v: Double) -> UInt32 { UInt32(max(0, min(255, v.rounded()))) }
        let yy = 1.164 * (Double(y) - 16)
        let r = c(yy + 1.793 * (Double(cr) - 128))
        let g = c(yy - 0.213 * (Double(cb) - 128) - 0.533 * (Double(cr) - 128))
        let b = c(yy + 2.112 * (Double(cb) - 128))
        return (UInt32(a) << 24 | b << 16 | g << 8 | r).littleEndian
    }
}

/// A PGS track drawn over the picture: fetched from its own stream (the
/// part around the position first, then the whole track), and the set
/// showing at the player's time drawn where the picture is.
struct PGSOverlay: View {
    let url: URL
    let player: AVPlayer
    /// The picture fills the screen (cropped) rather than fitting in it.
    let fill: Bool
    @State private var sets: [PGS.DisplaySet] = []
    @State private var shown: (startMs: Int64, images: [(PGS.Placement, CGImage)])?

    /// How often a partial track is asked again, until the whole one is there.
    private static let retry: Duration = .seconds(15)

    var body: some View {
        TimelineView(.periodic(from: .now, by: 0.1)) { context in
            Canvas { ctx, size in
                guard let set = current(at: context.date), !set.placements.isEmpty else { return }
                let video = pictureRect(in: size)
                let sx = video.width / CGFloat(max(set.width, 1))
                let sy = video.height / CGFloat(max(set.height, 1))
                for (p, image) in images(for: set) {
                    let rect = CGRect(x: video.minX + CGFloat(p.x) * sx, y: video.minY + CGFloat(p.y) * sy,
                                      width: CGFloat(p.object.width) * sx, height: CGFloat(p.object.height) * sy)
                    ctx.draw(Image(decorative: image, scale: 1), in: rect)
                }
            }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
        .task(id: url) { await load() }
    }

    private func current(at _: Date) -> PGS.DisplaySet? {
        let t = player.currentTime().seconds
        guard t.isFinite else { return nil }
        return PGS.set(at: Int64(t * 1000), in: sets)
    }

    /// Decoded once per set, while it shows.
    private func images(for set: PGS.DisplaySet) -> [(PGS.Placement, CGImage)] {
        if let shown, shown.startMs == set.startMs { return shown.images }
        let decoded = set.placements.compactMap { p in PGS.image(p).map { (p, $0) } }
        Task { @MainActor in shown = (set.startMs, decoded) }
        return decoded
    }

    /// Where the picture is in the view: fitted, or filling it.
    private func pictureRect(in size: CGSize) -> CGRect {
        let natural = player.currentItem?.presentationSize ?? .zero
        guard natural.width > 0, natural.height > 0 else { return CGRect(origin: .zero, size: size) }
        let scale = fill ? max(size.width / natural.width, size.height / natural.height)
            : min(size.width / natural.width, size.height / natural.height)
        let w = natural.width * scale, h = natural.height * scale
        return CGRect(x: (size.width - w) / 2, y: (size.height - h) / 2, width: w, height: h)
    }

    private func load() async {
        sets = []
        while !Task.isCancelled {
            let t = player.currentTime().seconds
            guard var c = URLComponents(url: url, resolvingAgainstBaseURL: false) else { return }
            let from = URLQueryItem(name: "from", value: String(Int64(max(0, t.isFinite ? t : 0) * 1000)))
            c.queryItems = (c.queryItems ?? []) + [from]
            guard let ask = c.url else { return }
            var partial = true
            if let (data, response) = try? await URLSession.shared.data(from: ask),
               let http = response as? HTTPURLResponse, http.statusCode == 200 {
                let parsed = await Task.detached { PGS.parse([UInt8](data)) }.value
                guard !Task.isCancelled else { return }
                sets = parsed
                partial = http.value(forHTTPHeaderField: "X-Subtitle-Until") != nil
            }
            if !partial { return }
            try? await Task.sleep(for: Self.retry)
        }
    }
}
