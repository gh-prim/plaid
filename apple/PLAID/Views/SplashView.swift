import SwiftUI

/// The launch intro from the V2 board "Logo · Woven play, splash": cream on
/// graphite, the four pieces of the woven play button fly in from the four
/// edges and lock together with a small hit each, the mark slides left, the
/// name rises letter by letter and the i gets its play button, then the whole
/// splash fades into the app. One 6 s timeline, played once; with Reduce
/// Motion it is the still lockup, then the fade. Select or Play/Pause skips
/// to the fade.
struct SplashView: View {
    /// Called once the splash has faded out.
    let onFinish: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var start = Date.now
    @State private var skipped = false

    /// The design's loop length; every keyframe below is a fraction of it.
    static let duration: TimeInterval = 6
    /// Where the fade starts and ends.
    static let fadeFrom = 0.88, fadeTo = 0.96
    /// Reduce Motion: how long the still lockup holds before the fade.
    static let stillHold: TimeInterval = 1.2

    var body: some View {
        TimelineView(.animation) { timeline in
            let p = progress(at: timeline.date)
            Canvas { ctx, size in
                SplashFrame(p: reduceMotion ? min(p, Self.fadeFrom) : p).draw(in: &ctx, size: size)
            }
            .background(Color("Splash"))
            .opacity(1 - Self.ramp(p, Self.fadeFrom, Self.fadeTo))
        }
        .ignoresSafeArea()
        .focusable()
        .focusEffectDisabled()
        .onTapGesture(perform: skip)
        .onPlayPauseCommand(perform: skip)
        .accessibilityElement()
        .accessibilityLabel(Text("splash.label"))
        .task(id: skipped) {
            let end = endTime.timeIntervalSince(.now)
            if end > 0 { try? await Task.sleep(for: .seconds(end)) }
            guard !Task.isCancelled else { return }
            onFinish()
        }
    }

    /// The timeline's fraction at `date`. With Reduce Motion the still lockup
    /// stands in for everything before the fade.
    private func progress(at date: Date) -> Double {
        let t = date.timeIntervalSince(start)
        guard reduceMotion, !skipped else { return t / Self.duration }
        return t < Self.stillHold ? Self.fadeFrom : Self.fadeFrom + (t - Self.stillHold) / Self.duration
    }

    private var endTime: Date {
        let fade = (Self.fadeTo - Self.fadeFrom) * Self.duration
        if reduceMotion && !skipped { return start + Self.stillHold + fade }
        return start + Self.fadeTo * Self.duration
    }

    private func skip() {
        guard !skipped else { return }
        let now = Date.now
        // Jump to the fade, unless it has already begun.
        if progress(at: now) < Self.fadeFrom {
            start = now - Self.fadeFrom * Self.duration
        } else if reduceMotion {
            start = now - progress(at: now) * Self.duration
        }
        skipped = true
    }

    /// 0 before `a`, 1 after `b`, linear in between.
    static func ramp(_ p: Double, _ a: Double, _ b: Double) -> Double {
        min(1, max(0, (p - a) / (b - a)))
    }
}

/// One frame of the splash at fraction `p` of its 6 s timeline, drawn in the
/// board's 1440 × 900 space scaled to fit the screen.
struct SplashFrame {
    let p: Double

    private static let cream = Color(hex: "#f4f0e9")
    /// cubic-bezier(0.32, 0.72, 0, 1): the design's glide.
    private static let glide = Bezier(0.32, 0.72, 0, 1)
    /// cubic-bezier(.6, 0, .9, .5): a piece speeding toward its place.
    private static let fall = Bezier(0.6, 0, 0.9, 0.5)
    private static let easeOut = Bezier(0, 0, 0.58, 1)
    private static let ease = Bezier(0.25, 0.1, 0.25, 1)
    private static let easeInOut = Bezier(0.42, 0, 0.58, 1)
    /// cubic-bezier(.34, 1.56, .64, 1): the i's play button pops past its size.
    private static let pop = Bezier(0.34, 1.56, 0.64, 1)

    /// The four pieces of the play button: their clip in the mark's 100-unit
    /// box, the side they come from, how far, and the moment (in %) they land.
    private static let pieces: [(clip: CGRect, from: CGVector, distance: Double, hit: Double)] = [
        (CGRect(x: 0, y: 0, width: 46, height: 47.5), CGVector(dx: 0, dy: -1), 230, 13),
        (CGRect(x: 51, y: 0, width: 49, height: 47.5), CGVector(dx: 1, dy: 0), 300, 20),
        (CGRect(x: 51, y: 52.5, width: 49, height: 47.5), CGVector(dx: 0, dy: 1), 230, 27),
        (CGRect(x: 0, y: 52.5, width: 46, height: 47.5), CGVector(dx: -1, dy: 0), 300, 34),
    ]

    private static let triangle = Path { p in
        p.move(to: CGPoint(x: 33, y: 22))
        p.addLine(to: CGPoint(x: 81, y: 50))
        p.addLine(to: CGPoint(x: 33, y: 78))
        p.closeSubpath()
    }

    private static let iDot = Path { p in
        p.move(to: CGPoint(x: 164, y: 26))
        p.addLine(to: CGPoint(x: 180, y: 35))
        p.addLine(to: CGPoint(x: 164, y: 44))
        p.closeSubpath()
    }

    /// The five letters of "plaid", in the lettering's own units.
    private static let letters: [Path] = [
        Path { p in
            p.addEllipse(in: CGRect(x: 12, y: 56, width: 38, height: 38))
            p.move(to: CGPoint(x: 6, y: 56)); p.addLine(to: CGPoint(x: 6, y: 130))
        },
        Path { p in
            p.move(to: CGPoint(x: 78, y: 20)); p.addLine(to: CGPoint(x: 78, y: 84))
            p.addQuadCurve(to: CGPoint(x: 94, y: 100), control: CGPoint(x: 78, y: 100))
        },
        Path { p in
            p.addEllipse(in: CGRect(x: 106, y: 56, width: 38, height: 38))
            p.move(to: CGPoint(x: 144, y: 56)); p.addLine(to: CGPoint(x: 144, y: 100))
        },
        Path { p in
            p.move(to: CGPoint(x: 170, y: 56)); p.addLine(to: CGPoint(x: 170, y: 100))
        },
        Path { p in
            p.addEllipse(in: CGRect(x: 195, y: 56, width: 38, height: 38))
            p.move(to: CGPoint(x: 233, y: 20)); p.addLine(to: CGPoint(x: 233, y: 100))
        },
    ]

    /// Eased progress of the segment from `a` % to `b` %.
    private func seg(_ a: Double, _ b: Double, _ curve: Bezier) -> Double {
        curve(SplashView.ramp(p * 100, a, b))
    }

    private func lerp(_ a: Double, _ b: Double, _ t: Double) -> Double { a + (b - a) * t }

    /// How far a piece still is from its place, along the side it comes from:
    /// it falls in, overshoots by 3, bounces back by 1 and settles.
    private func pieceDistance(_ distance: Double, hit h: Double) -> Double {
        let pct = p * 100
        if pct < h - 8 { return distance }
        if pct < h { return lerp(distance, -3, seg(h - 8, h, Self.fall)) }
        if pct < h + 1.2 { return lerp(-3, 1, seg(h, h + 1.2, Self.easeOut)) }
        return lerp(1, 0, seg(h + 1.2, h + 2.5, Self.ease))
    }

    func draw(in ctx: inout GraphicsContext, size: CGSize) {
        // Fit the board's 1440 × 900 frame, centered.
        let k = min(size.width / 1440, size.height / 900)
        ctx.translateBy(x: (size.width - 1440 * k) / 2, y: (size.height - 900 * k) / 2)
        ctx.scaleBy(x: k, y: k)

        drawMark(in: ctx)
        drawLettering(in: ctx)
    }

    private func drawMark(in parent: GraphicsContext) {
        var ctx = parent
        // The mark glides from the center, big, to the left of the name.
        let move = seg(44, 56, Self.glide)
        ctx.translateBy(x: lerp(600, 500, move), y: lerp(330, 370, move))
        let s = lerp(2.4, 1.6, move)
        ctx.scaleBy(x: s, y: s)
        // It breathes once when the name is in.
        let breath = 1 + 0.025 * (seg(66, 77, Self.easeInOut) - seg(77, 88, Self.easeInOut))
        ctx.translateBy(x: 57, y: 50)
        ctx.scaleBy(x: breath, y: breath)
        ctx.translateBy(x: -57, y: -50)
        // Each hit shakes the whole mark a little, away from the incoming piece.
        for piece in Self.pieces {
            let jolt = seg(piece.hit, piece.hit + 0.6, Self.ease) - seg(piece.hit + 0.6, piece.hit + 2.2, Self.ease)
            ctx.translateBy(x: -piece.from.dx * 2.7 * jolt, y: -piece.from.dy * 2.7 * jolt)
        }

        for piece in Self.pieces {
            var pc = ctx
            let d = pieceDistance(piece.distance, hit: piece.hit)
            pc.translateBy(x: piece.from.dx * d, y: piece.from.dy * d)
            pc.clip(to: Path(piece.clip))
            pc.fill(Self.triangle, with: .color(Self.cream))
            pc.stroke(Self.triangle, with: .color(Self.cream), style: StrokeStyle(lineWidth: 6, lineJoin: .round))

            // A thin ring spreads from the center as the piece lands.
            guard p * 100 >= piece.hit else { continue }
            let ring = seg(piece.hit, piece.hit + 8, Self.glide)
            var rc = ctx
            rc.opacity = 0.7 * (1 - ring)
            let r = 30 * lerp(0.45, 1.9, ring)
            rc.stroke(Path(ellipseIn: CGRect(x: 52 - r, y: 50 - r, width: 2 * r, height: 2 * r)),
                      with: .color(Self.cream), lineWidth: 1.2 * lerp(0.45, 1.9, ring))
        }
    }

    private func drawLettering(in parent: GraphicsContext) {
        var ctx = parent
        ctx.translateBy(x: 700, y: 395)
        for (i, letter) in Self.letters.enumerated() {
            let a = 52 + 1.6 * Double(i)
            let rise = seg(a, a + 9, Self.glide)
            guard rise > 0 else { continue }
            var lc = ctx
            lc.opacity = rise
            lc.translateBy(x: 0, y: 34 * (1 - rise))
            lc.stroke(letter, with: .color(Self.cream), lineWidth: 12)
        }
        // The i gets its play button.
        let pop = seg(63, 71, Self.pop)
        guard pop > 0 else { return }
        var dc = ctx
        dc.translateBy(x: 172, y: 35)
        dc.rotate(by: .degrees(-40 * (1 - pop)))
        dc.scaleBy(x: pop, y: pop)
        dc.translateBy(x: -172, y: -35)
        dc.fill(Self.iDot, with: .color(Self.cream))
        dc.stroke(Self.iDot, with: .color(Self.cream), style: StrokeStyle(lineWidth: 3, lineJoin: .round))
    }
}

/// A CSS cubic-bezier timing function: maps time 0…1 to progress.
struct Bezier {
    let x1, y1, x2, y2: Double

    init(_ x1: Double, _ y1: Double, _ x2: Double, _ y2: Double) {
        (self.x1, self.y1, self.x2, self.y2) = (x1, y1, x2, y2)
    }

    private func coord(_ t: Double, _ a: Double, _ b: Double) -> Double {
        3 * a * t * (1 - t) * (1 - t) + 3 * b * t * t * (1 - t) + t * t * t
    }

    func callAsFunction(_ x: Double) -> Double {
        if x <= 0 { return 0 }
        if x >= 1 { return 1 }
        // Bisection on the curve's x, which is monotonic for x1, x2 in 0…1.
        var lo = 0.0, hi = 1.0, t = x
        for _ in 0..<24 {
            if coord(t, x1, x2) < x { lo = t } else { hi = t }
            t = (lo + hi) / 2
        }
        return coord(t, y1, y2)
    }
}
