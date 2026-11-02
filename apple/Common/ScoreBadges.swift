import SwiftUI

/// The scores row of a movie or show, as on the web: TMDB's users, Rotten
/// Tomatoes' Tomatometer (critics) and Popcornmeter (audience), each with
/// its mark. A score nobody has is left out; with none, nothing is drawn.
struct ScoreBadges: View {
    let rating: Double?
    let metadata: ItemMetadata?
    /// The mark's side; the number's font follows it.
    var size: CGFloat = 22
    var font: Font = .system(size: 15, weight: .bold)
    var color: Color = .white

    /// TMDB ratings from fewer votes than this aren't shown (canvas rule).
    static let minVotes = 50
    /// Rotten Tomatoes calls 60 and up fresh (critics) or full (audience).
    static let fresh = 60

    private struct Score: Identifiable {
        let id: String
        let value: Int
        let label: String
        let mark: ScoreMark
    }

    private var scores: [Score] {
        var out: [Score] = []
        let pct = { (v: Int) in (Double(v) / 100).formatted(.percent.precision(.fractionLength(0)).locale(AppLanguage.locale)) }
        if let rating, rating > 0, (metadata?.ratingCount ?? 0) >= Self.minVotes {
            let v = Int((rating * 10).rounded())
            out.append(.init(id: "tmdb", value: v, label: tr("item.ratings.tmdb %@", pct(v)), mark: .tmdb))
        }
        if let v = metadata?.criticScore {
            out.append(.init(id: "critics", value: v, label: tr("item.ratings.critics %@", pct(v)),
                             mark: .tomato(rotten: v < Self.fresh)))
        }
        if let v = metadata?.audienceScore {
            out.append(.init(id: "audience", value: v, label: tr("item.ratings.audience %@", pct(v)),
                             mark: .popcorn(spilled: v < Self.fresh)))
        }
        return out
    }

    var body: some View {
        let list = scores
        if !list.isEmpty {
            HStack(spacing: size * 0.7) {
                ForEach(list) { s in
                    HStack(spacing: size * 0.3) {
                        ScoreMarkView(mark: s.mark).frame(width: size, height: size)
                        Text((Double(s.value) / 100).formatted(.percent.precision(.fractionLength(0)).locale(AppLanguage.locale)))
                            .font(font).monospacedDigit().foregroundStyle(color)
                    }
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(Text(verbatim: s.label))
                }
            }
        }
    }
}

enum ScoreMark: Hashable {
    case tmdb
    case tomato(rotten: Bool)
    case popcorn(spilled: Bool)
}

/// The marks, drawn on a 32-point grid like the web's SVGs.
struct ScoreMarkView: View {
    let mark: ScoreMark

    private static let red = Color(hex: "#fa320a")
    private static let green = Color(hex: "#00912d")
    private static let splat = Color(hex: "#0fc755")
    private static let corn = Color(hex: "#ffd34d")
    private static let cream = Color(hex: "#f4ecdc")

    var body: some View {
        Canvas { ctx, size in
            ctx.scaleBy(x: size.width / 32, y: size.height / 32)
            switch mark {
            case .tmdb: Self.tmdb(&ctx)
            case .tomato(let rotten): rotten ? Self.splat(&ctx) : Self.tomato(&ctx)
            case .popcorn(let spilled): Self.popcorn(&ctx, spilled: spilled)
            }
        }
        .accessibilityHidden(true)
    }

    private static func tmdb(_ ctx: inout GraphicsContext) {
        let pill = Path(roundedRect: CGRect(x: 1, y: 7, width: 30, height: 18), cornerRadius: 9)
        ctx.fill(pill, with: .linearGradient(Gradient(colors: [Color(hex: "#90cea1"), Color(hex: "#01b4e4")]),
                                             startPoint: CGPoint(x: 1, y: 16), endPoint: CGPoint(x: 31, y: 16)))
        ctx.draw(Text(verbatim: "TMDB").font(.system(size: 8.5, weight: .heavy)).foregroundStyle(Color(hex: "#0d253f")),
                 at: CGPoint(x: 16, y: 16))
    }

    private static func tomato(_ ctx: inout GraphicsContext) {
        ctx.fill(Path(ellipseIn: CGRect(x: 4, y: 6, width: 24, height: 24)), with: .color(red))
        var leaves = Path()
        for angle in [-50.0, 0, 50] {
            let t = CGAffineTransform(translationX: 16, y: 8).rotated(by: angle * .pi / 180)
            leaves.addPath(Path(ellipseIn: CGRect(x: -2.2, y: -6, width: 4.4, height: 8)), transform: t)
        }
        ctx.fill(leaves, with: .color(green))
    }

    private static func splat(_ ctx: inout GraphicsContext) {
        // A twelve-point star: the rotten splat.
        var p = Path()
        for i in 0..<24 {
            let r: CGFloat = i.isMultiple(of: 2) ? 14 : 8
            let a = CGFloat(i) * .pi / 12 - .pi / 2
            let pt = CGPoint(x: 16 + r * cos(a), y: 16 + r * sin(a))
            i == 0 ? p.move(to: pt) : p.addLine(to: pt)
        }
        p.closeSubpath()
        ctx.fill(p, with: .color(splat))
    }

    private static func bucket(_ ctx: inout GraphicsContext) {
        var b = Path()
        b.addLines([CGPoint(x: 8, y: 13), CGPoint(x: 24, y: 13), CGPoint(x: 22, y: 29), CGPoint(x: 10, y: 29)])
        b.closeSubpath()
        ctx.fill(b, with: .color(cream))
        var stripes = Path()
        stripes.addLines([CGPoint(x: 11.2, y: 13), CGPoint(x: 13.6, y: 13), CGPoint(x: 14.2, y: 29), CGPoint(x: 12, y: 29)])
        stripes.closeSubpath()
        stripes.addLines([CGPoint(x: 18.4, y: 13), CGPoint(x: 20.8, y: 13), CGPoint(x: 20, y: 29), CGPoint(x: 17.8, y: 29)])
        stripes.closeSubpath()
        ctx.fill(stripes, with: .color(red))
    }

    private static func kernels(_ ctx: inout GraphicsContext, _ dots: [(CGFloat, CGFloat, CGFloat)]) {
        var p = Path()
        for (x, y, r) in dots { p.addEllipse(in: CGRect(x: x - r, y: y - r, width: r * 2, height: r * 2)) }
        ctx.fill(p, with: .color(corn))
    }

    private static func popcorn(_ ctx: inout GraphicsContext, spilled: Bool) {
        if spilled {
            var tipped = ctx
            tipped.translateBy(x: 16, y: 21)
            tipped.rotate(by: .degrees(-100))
            tipped.translateBy(x: -16, y: -21)
            bucket(&tipped)
            kernels(&ctx, [(26, 27, 2.4), (29, 24.5, 2)])
        } else {
            kernels(&ctx, [(10, 11, 3), (14.5, 9, 3.4), (19, 9.5, 3.2), (22.5, 11.5, 2.6)])
            bucket(&ctx)
        }
    }
}
