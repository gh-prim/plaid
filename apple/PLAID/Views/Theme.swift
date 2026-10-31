import SwiftUI

/// Design tokens of web/src/design/tokens.css, dark theme.
enum Theme {
    static let bgBase = Color(hex: "#0e0c0a")
    static let bgRaised = Color(hex: "#171412")
    static let surface = Color(hex: "#201e1b")
    static let surfaceHover = Color(hex: "#292724")
    static let textPrimary = Color(hex: "#f4f0e9")
    static let textSecondary = Color(hex: "#d9d5cf")
    static let textTertiary = Color(hex: "#bfbcb6")
    // Apple TV look: focus is white with dark text, no accent colour.
    static let action = Color.white
    static let onAction = Color.black
    static let focus = Color.white
    /// Resting controls: grey translucent glass.
    static let control = Color.white.opacity(0.16)
    static let tint = Color(red: 14 / 255, green: 12 / 255, blue: 9 / 255)
    static let glow = Color(hex: "#a97c51")
    static let tan = Color(hex: "#d5b993")

    static let dyeNavy = Color(hex: "#1f2742")
    static let dyeGreen = Color(hex: "#233c30")
    static let dyeYellow = Color(hex: "#d9a545")

    /// `party-1` … `party-8`: oklch(0.8 0.06 h), as sRGB.
    static let party: [Color] = [
        "#e5b5ac", "#dcbe9a", "#c8c590", "#a9cba5", "#93cbc4", "#9ec3dc", "#bab8e2", "#d7b1d1",
    ].map { Color(hex: $0) }

    static let radius: CGFloat = 12
    /// Width kept for the side rail: content starts after it, so the focus
    /// engine sees the content to the right of the rail.
    static let rail: CGFloat = 104
    /// Left page margin, from the rail's edge.
    static let gutter: CGFloat = 32
    /// tvOS's horizontal safe margin on a 1920-point screen.
    static let systemMargin: CGFloat = 80
    /// How far a hero's backdrop fades out below the hero.
    static let heroFadeBelow: CGFloat = 140
    /// Right page margin.
    static let trailing: CGFloat = 80

    static func sans(_ size: CGFloat, _ weight: Font.Weight = .regular) -> Font {
        .custom("Instrument Sans", size: size).weight(weight)
    }

    static func serif(_ size: CGFloat) -> Font {
        .custom("Instrument Serif", size: size)
    }
}

extension Color {
    init(hex: String) {
        let s = hex.trimmingCharacters(in: CharacterSet(charactersIn: "#"))
        var v: UInt64 = 0
        Scanner(string: s).scanHexInt64(&v)
        self.init(
            red: Double((v >> 16) & 0xff) / 255,
            green: Double((v >> 8) & 0xff) / 255,
            blue: Double(v & 0xff) / 255
        )
    }
}

/// The amber tartan of the web progress thread, drawn instead of its PNG.
struct PlaidStripe: View {
    var body: some View {
        Canvas { ctx, size in
            ctx.fill(Path(CGRect(origin: .zero, size: size)), with: .color(Theme.action))
            var x: CGFloat = 0
            while x < size.width {
                ctx.fill(Path(CGRect(x: x, y: 0, width: 3, height: size.height)), with: .color(Theme.dyeYellow.opacity(0.9)))
                ctx.fill(Path(CGRect(x: x + 7, y: 0, width: 1.5, height: size.height)), with: .color(Theme.dyeNavy.opacity(0.5)))
                x += 12
            }
        }
    }
}

/// The 4pt progress track with its tartan fill.
struct ProgressThread: View {
    let value: Double

    var body: some View {
        GeometryReader { g in
            ZStack(alignment: .leading) {
                Capsule().fill(Color.white.opacity(0.28))
                Capsule().fill(Color.white)
                    .frame(width: max(6, g.size.width * value))
            }
        }
        .frame(height: 6)
    }
}

/// The lowercase "plaıd" wordmark with Foxy as the dot of the ı.
struct Wordmark: View {
    var size: CGFloat = 44

    var body: some View {
        HStack(spacing: 0) {
            Text(verbatim: "pla")
            Text(verbatim: "ı")
                .overlay(alignment: .top) {
                    Image("Foxy")
                        .resizable()
                        .scaledToFit()
                        .frame(width: size * 0.42)
                        .offset(y: size * 0.02)
                }
            Text(verbatim: "d")
        }
        .font(Theme.sans(size, .bold))
        .tracking(-size * 0.035)
        .foregroundStyle(Theme.textPrimary)
        .accessibilityLabel(Text(verbatim: "PLAID"))
    }
}

/// The amber primary button and the glass secondary one, as on the web hero.
struct PillButtonStyle: ButtonStyle {
    var primary = true

    func makeBody(configuration: Configuration) -> some View {
        PillButtonBody(configuration: configuration, primary: primary)
    }
}

private struct PillButtonBody: View {
    let configuration: ButtonStyleConfiguration
    let primary: Bool
    @Environment(\.isFocused) private var focused
    @Environment(\.onHeroFocus) private var onHeroFocus

    var body: some View {
        configuration.label
            .onChange(of: focused) { _, on in if on { onHeroFocus() } }
            .font(Theme.sans(28, .semibold))
            .padding(.horizontal, 40)
            .frame(height: 72)
            .foregroundStyle(focused ? Theme.onAction : Theme.textPrimary)
            .background {
                RoundedRectangle(cornerRadius: 12)
                    .fill(focused ? Theme.action : Theme.control)
            }
            .scaleEffect(focused ? 1.06 : 1)
            .shadow(color: focused ? .black.opacity(0.5) : .clear, radius: 24, y: 12)
            .animation(.smooth(duration: 0.25), value: focused)
            .opacity(configuration.isPressed ? 0.8 : 1)
    }
}

/// A wide list row (servers): cream when focused, dark text on it.
struct RowButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        RowButtonBody(configuration: configuration)
    }
}

private struct RowButtonBody: View {
    let configuration: ButtonStyleConfiguration
    @Environment(\.isFocused) private var focused

    var body: some View {
        configuration.label
            .frame(width: 640, alignment: .leading)
            .padding(.horizontal, 32)
            .padding(.vertical, 20)
            .foregroundStyle(focused ? Theme.bgBase : Theme.textPrimary)
            .background(RoundedRectangle(cornerRadius: Theme.radius).fill(focused ? Theme.textPrimary : Theme.surface))
            .scaleEffect(focused ? 1.04 : 1)
            .shadow(color: focused ? .black.opacity(0.5) : .clear, radius: 24, y: 12)
            .animation(.smooth(duration: 0.25), value: focused)
    }
}

/// Shimmer for anything still loading (never a frozen blank).
struct Shimmer: View {
    @State private var phase: CGFloat = -1

    var body: some View {
        LinearGradient(
            colors: [Theme.surface, Theme.surfaceHover, Theme.surface],
            startPoint: UnitPoint(x: phase, y: 0.5),
            endPoint: UnitPoint(x: phase + 1, y: 0.5)
        )
        .onAppear {
            withAnimation(.linear(duration: 1.4).repeatForever(autoreverses: false)) { phase = 1 }
        }
    }
}

struct LoadingView: View {
    var body: some View {
        ProgressView()
            .controlSize(.large)
            .tint(Theme.action)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// Called when a control of a page's hero takes focus: the page scrolls back
/// to its top, so the whole hero shows, not just the focused button.
private struct HeroFocusKey: EnvironmentKey {
    static let defaultValue: () -> Void = {}
}

extension EnvironmentValues {
    var onHeroFocus: () -> Void {
        get { self[HeroFocusKey.self] }
        set { self[HeroFocusKey.self] = newValue }
    }
}
