import AVFoundation
import SwiftUI
import UIKit

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

