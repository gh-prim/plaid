import AVFoundation
import AVKit
import MediaPlayer
import SwiftUI

/// Where the player is, twice a second: the time, the length, whether it
/// plays, and whether the picture went to an AirPlay screen.
@Observable
final class PlaybackClock {
    private(set) var seconds: Double = 0
    private(set) var duration: Double = 0
    private(set) var playing = false
    private(set) var buffering = false
    /// The picture plays on an AirPlay screen (IPh10), with its name.
    private(set) var external = false
    private(set) var routeName: String?

    private weak var player: AVPlayer?
    private var observer: Any?
    private var kvo: [NSKeyValueObservation] = []
    private var fallbackDuration: Double = 0

    func attach(_ p: AVPlayer, fallbackDuration: Double) {
        self.fallbackDuration = fallbackDuration
        guard player !== p else { return }
        detach()
        player = p
        observer = p.addPeriodicTimeObserver(forInterval: CMTime(value: 1, timescale: 4), queue: .main) { [weak self] t in
            MainActor.assumeIsolated { self?.tick(t) }
        }
        kvo = [
            p.observe(\.timeControlStatus, options: [.initial, .new]) { [weak self] p, _ in
                let status = p.timeControlStatus
                Task { @MainActor in
                    self?.playing = status != .paused
                    self?.buffering = status == .waitingToPlayAtSpecifiedRate
                }
            },
            p.observe(\.isExternalPlaybackActive, options: [.initial, .new]) { [weak self] p, _ in
                Task { @MainActor in self?.updateRoute(p) }
            },
        ]
    }

    func setFallbackDuration(_ d: Double) { fallbackDuration = d }

    func detach() {
        if let observer { player?.removeTimeObserver(observer) }
        observer = nil
        kvo = []
    }

    private func tick(_ t: CMTime) {
        seconds = t.seconds.isFinite ? t.seconds : 0
        // The player may not know an HLS stream's length: the session does.
        let d = player?.currentItem?.duration.seconds ?? .nan
        duration = d.isFinite && d > 0 ? d : fallbackDuration
    }

    private func updateRoute(_ p: AVPlayer) {
        external = p.isExternalPlaybackActive
        routeName = AVAudioSession.sharedInstance().currentRoute.outputs
            .first { $0.portType == .airPlay || $0.portType == .HDMI }?.portName
    }
}

/// The picture: an AVPlayerLayer, which Picture in Picture takes over.
struct VideoSurface: UIViewRepresentable {
    let player: AVPlayer
    let pip: PictureInPicture
    var fill = false

    final class LayerView: UIView {
        override class var layerClass: AnyClass { AVPlayerLayer.self }
        var playerLayer: AVPlayerLayer { layer as! AVPlayerLayer }
    }

    func makeUIView(context: Context) -> LayerView {
        let v = LayerView()
        v.backgroundColor = .black
        v.playerLayer.player = player
        pip.attach(v.playerLayer)
        return v
    }

    func updateUIView(_ v: LayerView, context: Context) {
        if v.playerLayer.player !== player { v.playerLayer.player = player }
        v.playerLayer.videoGravity = fill ? .resizeAspectFill : .resizeAspect
    }
}

/// Picture in Picture for the player layer: from the button, and on its own
/// when the app goes to the background while playing.
@Observable
final class PictureInPicture: NSObject, AVPictureInPictureControllerDelegate {
    private var controller: AVPictureInPictureController?
    private(set) var possible = false
    private(set) var active = false
    private var kvo: NSKeyValueObservation?

    var supported: Bool { AVPictureInPictureController.isPictureInPictureSupported() }

    func attach(_ layer: AVPlayerLayer) {
        guard supported, controller == nil, let c = AVPictureInPictureController(playerLayer: layer) else { return }
        c.canStartPictureInPictureAutomaticallyFromInline = true
        c.delegate = self
        kvo = c.observe(\.isPictureInPicturePossible, options: [.initial, .new]) { [weak self] c, _ in
            let possible = c.isPictureInPicturePossible
            Task { @MainActor in self?.possible = possible }
        }
        controller = c
    }

    func toggle() {
        guard let controller else { return }
        if controller.isPictureInPictureActive { controller.stopPictureInPicture() } else { controller.startPictureInPicture() }
    }

    nonisolated func pictureInPictureControllerDidStartPictureInPicture(_ c: AVPictureInPictureController) {
        Task { @MainActor in self.active = true }
    }

    nonisolated func pictureInPictureControllerDidStopPictureInPicture(_ c: AVPictureInPictureController) {
        Task { @MainActor in self.active = false }
    }
}

/// The system's AirPlay picker, drawn by us: its own button is made
/// invisible and laid over `label`, so a tap opens the route list.
struct AirPlayButton<Label: View>: View {
    @ViewBuilder let label: Label

    var body: some View {
        label
            .allowsHitTesting(false)
            .overlay { RoutePicker() }
    }

    private struct RoutePicker: UIViewRepresentable {
        func makeUIView(context: Context) -> AVRoutePickerView {
            let v = AVRoutePickerView()
            v.tintColor = .clear
            v.activeTintColor = .clear
            v.prioritizesVideoDevices = true
            return v
        }

        func updateUIView(_ v: AVRoutePickerView, context: Context) {}
    }
}

/// The phone turns to landscape for the player and back to portrait after;
/// the iPad keeps whatever way it's held.
enum OrientationLock {
    nonisolated(unsafe) static var mask: UIInterfaceOrientationMask = .allButUpsideDown

    static func landscape(_ on: Bool) {
        guard UIDevice.current.userInterfaceIdiom == .phone else { return }
        mask = on ? .landscape : .allButUpsideDown
        let scene = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first
        scene?.requestGeometryUpdate(.iOS(interfaceOrientations: on ? .landscapeRight : .portrait))
        scene?.keyWindow?.rootViewController?.setNeedsUpdateOfSupportedInterfaceOrientations()
    }
}

final class AppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication, supportedInterfaceOrientationsFor window: UIWindow?) -> UIInterfaceOrientationMask {
        OrientationLock.mask
    }
}

/// The lock screen and Control Center: what plays, with play, pause and
/// the 10-second skips.
@MainActor
final class NowPlaying {
    private var targets: [(MPRemoteCommand, Any)] = []

    func start(player: AVPlayer) {
        try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .moviePlayback)
        try? AVAudioSession.sharedInstance().setActive(true)
        let c = MPRemoteCommandCenter.shared()
        func on(_ cmd: MPRemoteCommand, _ run: @escaping (MPRemoteCommandEvent) -> Void) {
            targets.append((cmd, cmd.addTarget { e in MainActor.assumeIsolated { run(e) }; return .success }))
        }
        on(c.playCommand) { _ in player.play() }
        on(c.pauseCommand) { _ in player.pause() }
        on(c.togglePlayPauseCommand) { _ in player.rate > 0 ? player.pause() : player.play() }
        c.skipForwardCommand.preferredIntervals = [10]
        c.skipBackwardCommand.preferredIntervals = [10]
        on(c.skipForwardCommand) { _ in player.seek(by: 10) }
        on(c.skipBackwardCommand) { _ in player.seek(by: -10) }
        on(c.changePlaybackPositionCommand) { e in
            if let e = e as? MPChangePlaybackPositionCommandEvent { player.seek(toSeconds: e.positionTime) }
        }
    }

    func update(title: String, subtitle: String?, seconds: Double, duration: Double, rate: Float) {
        var info: [String: Any] = [
            MPMediaItemPropertyTitle: title,
            MPMediaItemPropertyPlaybackDuration: duration,
            MPNowPlayingInfoPropertyElapsedPlaybackTime: seconds,
            MPNowPlayingInfoPropertyPlaybackRate: rate,
            MPNowPlayingInfoPropertyMediaType: MPNowPlayingInfoMediaType.video.rawValue,
        ]
        if let subtitle { info[MPMediaItemPropertyArtist] = subtitle }
        MPNowPlayingInfoCenter.default().nowPlayingInfo = info
    }

    func stop() {
        for (cmd, target) in targets { cmd.removeTarget(target) }
        targets = []
        MPNowPlayingInfoCenter.default().nowPlayingInfo = nil
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }
}

extension AVPlayer {
    func seek(by delta: Double) {
        seek(toSeconds: max(0, currentTime().seconds + delta))
    }

    func seek(toSeconds s: Double) {
        seek(to: CMTime(seconds: s, preferredTimescale: 600), toleranceBefore: .zero, toleranceAfter: .zero)
    }
}

/// "4:52", "1:02:03"; negative for what's left: "-7:22".
func clockTime(_ seconds: Double) -> String {
    let s = Int(abs(seconds).rounded(.down))
    let text = s >= 3600 ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    return seconds < 0 ? "-" + text : text
}
