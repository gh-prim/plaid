import SwiftUI

/// The V2 look on iPhone and iPad (canvas "PLAID Design", page V2, step 17):
/// near-black ground, cream text, Bricolage Grotesque titles, Liquid Glass.
enum Theme {
    static let bgBase = Color(hex: "#0b0b0c")
    static let surface = Color(hex: "#1c1c1e")
    static let surfaceHover = Color(hex: "#2a2a2d")
    static let textPrimary = Color(hex: "#f4f0e9")
    static let textSecondary = textPrimary.opacity(0.66)
    static let textTertiary = textPrimary.opacity(0.42)
    static let hairline = Color.white.opacity(0.07)
    /// The cream of primary buttons, with dark text on it.
    static let cream = textPrimary
    static let onCream = Color(hex: "#111111")
    static let error = Color(hex: "#ffc2b6")
    /// The graphite of the logo's tile.
    static let graphite = Color(hex: "#1c1c1e")

    /// `party-1` … `party-8`: oklch(0.8 0.06 h), as sRGB.
    static let party: [Color] = [
        "#e5b5ac", "#dcbe9a", "#c8c590", "#a9cba5", "#93cbc4", "#9ec3dc", "#bab8e2", "#d7b1d1",
    ].map { Color(hex: $0) }

    /// Page margin on iPhone; iPad pages use `wideGutter`.
    static let gutter: CGFloat = 20
    static let wideGutter: CGFloat = 32

    static func sans(_ size: CGFloat, _ weight: Font.Weight = .regular) -> Font {
        .custom("Instrument Sans", size: size).weight(weight)
    }

    static func display(_ size: CGFloat, _ weight: Font.Weight = .bold) -> Font {
        .custom("Bricolage Grotesque", size: size).weight(weight)
    }
}

extension View {
    /// A page title: Bricolage, tight tracking.
    func displayTitle(_ size: CGFloat) -> some View {
        font(Theme.display(size)).tracking(-size * 0.035)
    }
}

/// The cream capsule of a page's main action ("Lecture", "Se connecter").
struct CreamButtonStyle: ButtonStyle {
    var height: CGFloat = 50

    func makeBody(configuration: Configuration) -> some View {
        CreamButtonBody(configuration: configuration, height: height)
    }
}

private struct CreamButtonBody: View {
    let configuration: ButtonStyleConfiguration
    let height: CGFloat
    @Environment(\.isEnabled) private var enabled

    var body: some View {
        configuration.label
            .font(Theme.sans(16, .semibold))
            .frame(maxWidth: .infinity)
            .frame(height: height)
            .foregroundStyle(enabled ? Theme.onCream : Theme.textTertiary)
            .background(Capsule().fill(enabled ? Theme.cream : Color.white.opacity(0.1)))
            .shadow(color: enabled ? Theme.cream.opacity(0.25) : .clear, radius: 17, y: 10)
            .scaleEffect(configuration.isPressed ? 0.97 : 1)
            .animation(.smooth(duration: 0.2), value: configuration.isPressed)
    }
}

/// A round glass button with an icon (add, info, back, more).
struct GlassIconButton: View {
    let systemImage: String
    let label: LocalizedStringKey
    var size: CGFloat = 50
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: systemImage)
                .font(.system(size: size * 0.36, weight: .semibold))
                .frame(width: size, height: size)
        }
        .buttonStyle(.glass)
        .buttonBorderShape(.circle)
        .accessibilityLabel(label)
    }
}

/// A profile's round face: its picture (F009), else its initial on its party color.
struct ProfileFace: View {
    let profile: Profile
    var size: CGFloat = 36

    var body: some View {
        ProfilePicture(profile: profile, size: size) {
            Text(verbatim: String(profile.name.prefix(1)).uppercased())
                .font(Theme.display(size * 0.44))
                .foregroundStyle(Theme.onCream)
                .frame(width: size, height: size)
                .background(Circle().fill(Theme.party[profile.avatarColor % Theme.party.count]))
        }
    }
}

/// A person with no portrait: their initials on a grey disc.
struct InitialsDisc: View {
    let name: String
    var size: CGFloat = 64

    var body: some View {
        Text(verbatim: initials(name).uppercased())
            .font(Theme.display(size * 0.34))
            .foregroundStyle(Theme.textSecondary)
            .frame(width: size, height: size)
            .background(Circle().fill(LinearGradient(colors: [Color(hex: "#3a3a3e"), Color(hex: "#232325")],
                                                     startPoint: .topLeading, endPoint: .bottomTrailing)))
    }
}

/// A title's art, blurred very wide behind a page, as the boards' ambient glow.
struct AmbientGlow: View {
    let art: Artwork?
    var brightness: Double = -0.6

    var body: some View {
        RemoteImage(art: art, width: 300)
            .blur(radius: 90)
            .saturation(1.4)
            .brightness(brightness)
            .scaleEffect(1.4)
            .allowsHitTesting(false)
            .accessibilityHidden(true)
    }
}

/// The woven play button: the logo's graphite tile with the cream triangle
/// crossed by one warp and one weft thread.
struct LogoMark: View {
    var size: CGFloat = 30

    var body: some View {
        Canvas { ctx, s in
            let k = s.width / 100
            ctx.fill(Path(roundedRect: CGRect(origin: .zero, size: s), cornerRadius: 26 * k), with: .color(Theme.graphite))
            var tri = Path()
            tri.move(to: CGPoint(x: 33 * k, y: 22 * k))
            tri.addLine(to: CGPoint(x: 81 * k, y: 50 * k))
            tri.addLine(to: CGPoint(x: 33 * k, y: 78 * k))
            tri.closeSubpath()
            ctx.fill(tri, with: .color(Theme.textPrimary))
            ctx.stroke(tri, with: .color(Theme.textPrimary), style: StrokeStyle(lineWidth: 6 * k, lineJoin: .round))
            ctx.fill(Path(CGRect(x: 46 * k, y: 0, width: 5 * k, height: s.height)), with: .color(Theme.graphite))
            ctx.fill(Path(CGRect(x: 0, y: 47.5 * k, width: s.width, height: 5 * k)), with: .color(Theme.graphite))
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

/// The "plaid" lettering: circles and straight strokes of one weight, the i
/// dotted with a play button.
struct Wordmark: View {
    var height: CGFloat = 21

    var body: some View {
        Canvas { ctx, s in
            let k = s.height / 122
            func p(_ x: CGFloat, _ y: CGFloat) -> CGPoint { CGPoint(x: (x + 2) * k, y: (y - 14) * k) }
            let stroke = StrokeStyle(lineWidth: 12 * k)
            let ink = GraphicsContext.Shading.color(Theme.textPrimary)
            func ring(_ x: CGFloat) {
                ctx.stroke(Path(ellipseIn: CGRect(origin: p(x - 19, 56), size: CGSize(width: 38 * k, height: 38 * k))), with: ink, style: stroke)
            }
            func line(_ x: CGFloat, _ y0: CGFloat, _ y1: CGFloat) {
                var l = Path(); l.move(to: p(x, y0)); l.addLine(to: p(x, y1)); ctx.stroke(l, with: ink, style: stroke)
            }
            ring(31); line(6, 56, 130)
            var l = Path(); l.move(to: p(78, 20)); l.addLine(to: p(78, 84)); l.addQuadCurve(to: p(94, 100), control: p(78, 100))
            ctx.stroke(l, with: ink, style: stroke)
            ring(125); line(144, 56, 100)
            line(170, 56, 100)
            var dot = Path(); dot.move(to: p(164, 26)); dot.addLine(to: p(180, 35)); dot.addLine(to: p(164, 44)); dot.closeSubpath()
            ctx.fill(dot, with: ink)
            ring(214); line(233, 20, 100)
        }
        .frame(width: height * 245 / 122, height: height)
        .accessibilityLabel(Text(verbatim: "plaid"))
    }
}

struct LoadingView: View {
    var body: some View {
        ProgressView()
            .controlSize(.large)
            .tint(Theme.textPrimary)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// A page that couldn't load: what happened, and a way to try again.
struct ErrorState: View {
    let retry: () -> Void

    var body: some View {
        VStack(spacing: 16) {
            Text("error.generic").font(Theme.sans(16, .medium)).foregroundStyle(Theme.textSecondary)
            Button("common.retry", action: retry).buttonStyle(.glass)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// The 4pt progress track of a card.
struct ProgressThread: View {
    let value: Double

    var body: some View {
        GeometryReader { g in
            ZStack(alignment: .leading) {
                Capsule().fill(Color.white.opacity(0.28))
                Capsule().fill(Theme.textPrimary).frame(width: max(4, g.size.width * value))
            }
        }
        .frame(height: 4)
    }
}

extension EnvironmentValues {
    /// The status bar's height, for pages drawn under it.
    @Entry var topInset: CGFloat = 0

    /// True on iPad-sized layouts (regular width), where pages spread out.
    var isWide: Bool { horizontalSizeClass == .regular }
}
