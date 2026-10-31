import AVFoundation
import SwiftUI

/// Whether hero previews play with their sound; the hero's button flips it.
let previewSoundKey = "preview.sound"

/// The scenes loop of a title (GET /items/{id}/preview), downloaded once
/// with the bearer token and kept on disk, as the web hero plays it.
enum PreviewStore {
    private static let maxTries = 40

    static func file(for itemId: String, client: APIClient) async -> URL? {
        let dir = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0]
            .appending(path: "previews", directoryHint: .isDirectory)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let local = dir.appending(path: "\(itemId).mp4")
        if FileManager.default.fileExists(atPath: local.path()) { return local }

        let url = client.baseURL.appending(path: "api/v1/items/\(itemId)/preview")
        for _ in 0..<maxTries {
            guard !Task.isCancelled,
                  let (data, response) = try? await URLSession.shared.data(for: client.authorized(url)),
                  let http = response as? HTTPURLResponse else { return nil }
            switch http.statusCode {
            case 200:
                try? data.write(to: local, options: .atomic)
                return local
            case 202:
                // Being cut: ask again after Retry-After.
                let wait = Double(http.value(forHTTPHeaderField: "Retry-After") ?? "") ?? 5
                try? await Task.sleep(for: .seconds(max(1, wait)))
            default:
                return nil
            }
        }
        return nil
    }
}

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

/// An AVQueuePlayer looping one file, telling once it really plays.
@Observable
final class PreviewLooper {
    let player = AVQueuePlayer()
    private var looper: AVPlayerLooper?
    private var status: NSKeyValueObservation?

    init(url: URL, onPlaying: @escaping @MainActor () -> Void) {
        looper = AVPlayerLooper(player: player, templateItem: AVPlayerItem(url: url))
        status = player.observe(\.timeControlStatus) { p, _ in
            if p.timeControlStatus == .playing { Task { @MainActor in onPlaying() } }
        }
    }
}

struct PlayerLayer: UIViewRepresentable {
    let player: AVPlayer

    func makeUIView(context: Context) -> PlayerUIView {
        let v = PlayerUIView()
        v.playerLayer.player = player
        v.playerLayer.videoGravity = .resizeAspectFill
        return v
    }

    func updateUIView(_ v: PlayerUIView, context: Context) {
        if v.playerLayer.player !== player { v.playerLayer.player = player }
    }

    final class PlayerUIView: UIView {
        override class var layerClass: AnyClass { AVPlayerLayer.self }
        var playerLayer: AVPlayerLayer { layer as! AVPlayerLayer }
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
    func makeBody(configuration: Configuration) -> some View {
        Round(configuration: configuration)
    }

    private struct Round: View {
        let configuration: ButtonStyleConfiguration
        @Environment(\.isFocused) private var focused
        @Environment(\.onHeroFocus) private var onHeroFocus

        var body: some View {
            configuration.label
                .onChange(of: focused) { _, on in if on { onHeroFocus() } }
                .font(.system(size: 26, weight: .semibold))
                .frame(width: 72, height: 72)
                .foregroundStyle(focused ? Theme.onAction : Theme.textPrimary)
                .background(Circle().fill(focused ? Theme.action : Theme.control))
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
