import AVFoundation
import SwiftUI

/// The preview over the hero's backdrop: the backdrop shows first, then the
/// scenes fade in once they play, looping. Paused while `active` is false.
struct HeroPreview: View {
    let itemId: String
    let active: Bool

    @Environment(AppState.self) private var app
    @AppStorage(previewSoundKey) private var soundOn = true
    @State private var looper: PreviewLooper?
    @State private var playing = false

    /// The backdrop shows alone this long before the scenes fade in.
    private static let backdropFirst: Duration = .seconds(2)

    var body: some View {
        ZStack {
            if let looper {
                PlayerLayer(player: looper.player)
                    .opacity(playing ? 1 : 0)
                    .animation(.easeInOut(duration: 1.2), value: playing)
            }
        }
        .task(id: itemId) {
            looper = nil
            playing = false
            guard let client = app.client else { return }
            async let wait: Void = (try? await Task.sleep(for: Self.backdropFirst)) ?? ()
            let file = await PreviewStore.file(for: itemId, client: client)
            await wait
            guard let file, !Task.isCancelled else { return }
            let l = PreviewLooper(url: file) { playing = true }
            l.player.isMuted = !soundOn
            looper = l
            if active { l.player.play() }
        }
        .onChange(of: active) { _, on in
            if on { looper?.player.play() } else { looper?.player.pause() }
        }
        .onChange(of: soundOn) { _, on in looper?.player.isMuted = !on }
        .onDisappear { looper?.player.pause() }
        .allowsHitTesting(false)
    }
}

/// The hero's sound button: a glass circle, white when focused.
struct PreviewSoundButton: View {
    @AppStorage(previewSoundKey) private var soundOn = true

    var body: some View {
        Button { soundOn.toggle() } label: {
            Image(systemName: soundOn ? "speaker.wave.2.fill" : "speaker.slash.fill")
                .accessibilityLabel(Text(soundOn ? "item.preview.mute" : "item.preview.unmute"))
        }
        .buttonStyle(RoundGlassButtonStyle())
    }
}

struct RoundGlassButtonStyle: ButtonStyle {
    var size: CGFloat = 72

    func makeBody(configuration: Configuration) -> some View {
        Round(configuration: configuration, size: size)
    }

    private struct Round: View {
        let configuration: ButtonStyleConfiguration
        let size: CGFloat
        @Environment(\.isFocused) private var focused
        @Environment(\.onHeroFocus) private var onHeroFocus

        var body: some View {
            configuration.label
                .onChange(of: focused) { _, on in if on { onHeroFocus() } }
                .font(.system(size: size * 0.36, weight: .semibold))
                .frame(width: size, height: size)
                .foregroundStyle(focused ? Theme.onAction : Theme.textPrimary)
                .background(Circle().fill(focused ? Theme.action : Theme.control))
                .focusHalo(Circle())
                .scaleEffect(focused ? 1.08 : 1)
                .shadow(color: focused ? .black.opacity(0.5) : .clear, radius: 24, y: 12)
                .animation(.smooth(duration: 0.25), value: focused)
        }
    }
}

/// Whether hero previews may play: false while the main player is open.
private struct PreviewsActiveKey: EnvironmentKey {
    static let defaultValue = true
}

extension EnvironmentValues {
    var previewsActive: Bool {
        get { self[PreviewsActiveKey.self] }
        set { self[PreviewsActiveKey.self] = newValue }
    }
}
