import AVFoundation
import AVKit
import SwiftUI

/// Full-screen playback with the system player: starts a session, reports
/// progress every 10 s and on pause, stops the session on leave. Episodes
/// run on into the next one (F008 R43, R44), and each profile's audio and
/// subtitle choice carries over (R38).
struct PlayerView: View {
    let request: PlayRequest
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @State private var model = PlayerModel()

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            if model.askStillWatching {
                StillWatching(item: model.next ?? model.item) {
                    model.continueWatching()
                } stop: {
                    dismiss()
                }
            } else if model.endOfShow, let show = model.item?.show?.displayTitle {
                EndOfShow(show: show) { dismiss() }
            } else if model.error == nil, let player = model.player {
                SystemPlayer(player: player, actions: model.contextualActions, menus: model.menus)
                    .ignoresSafeArea()
                if let card = model.card {
                    NextEpisodeCard(card: card, playNow: model.playNext, cancel: model.cancelNext)
                        .padding(.trailing, 64)
                        .padding(.bottom, 120)
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomTrailing)
                        .transition(.move(edge: .trailing).combined(with: .opacity))
                }
            } else if let error = model.error {
                VStack(spacing: 32) {
                    Text(LocalizedStringKey(error)).font(Theme.sans(32)).foregroundStyle(Theme.textSecondary)
                    HStack(spacing: 24) {
                        Button { model.retry(lower: false) } label: { Label("common.retry", systemImage: "arrow.clockwise") }
                            .buttonStyle(PillButtonStyle())
                        Button { model.retry(lower: true) } label: { Label("player.error.lower", systemImage: "dial.low") }
                            .buttonStyle(PillButtonStyle(primary: false))
                        Button("common.close") { dismiss() }.buttonStyle(PillButtonStyle(primary: false))
                    }
                }
            } else {
                LoadingView()
            }
        }
        .animation(.smooth(duration: 0.35), value: model.card?.next.id)
        .overlay(alignment: .top) {
            if let toast = model.toast {
                Text(LocalizedStringKey(toast))
                    .font(Theme.sans(26, .semibold))
                    .padding(.horizontal, 32).padding(.vertical, 18)
                    .background(Capsule().fill(Theme.bgRaised.opacity(0.92)))
                    .padding(.top, 60)
                    .task { try? await Task.sleep(for: .seconds(4)); model.toast = nil }
            }
        }
        .task {
            guard let client = app.client else { return }
            let profile = app.me?.activeProfile?.id ?? "default"
            await model.start(request.item, from: request.startMs, fileId: request.fileId, client: client, profileId: profile) { dismiss() }
        }
        .onDisappear { model.stop() }
    }
}

/// After three episodes in a row with no one touching the remote.
private struct StillWatching: View {
    let item: ItemSummary?
    let keepOn: () -> Void
    let stop: () -> Void

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            HeroBackdrop(art: item?.still ?? item?.backdrop).ignoresSafeArea()
            VStack(alignment: .leading, spacing: 24) {
                Text("player.stillWatching").font(Theme.sans(64, .bold))
                if let item {
                    Text(verbatim: [item.show?.displayTitle, episodeCode(item), item.name].compactMap { $0 }.joined(separator: " · "))
                        .font(Theme.sans(30))
                        .foregroundStyle(Theme.textSecondary)
                }
                HStack(spacing: 24) {
                    Button(action: keepOn) { Label("player.stillWatching.continue", systemImage: "play.fill") }
                        .buttonStyle(PillButtonStyle())
                    Button("player.stillWatching.stop", action: stop)
                        .buttonStyle(PillButtonStyle(primary: false))
                }
                .padding(.top, 12)
            }
            .padding(Theme.gutter + Theme.rail)
        }
    }
}

/// AVPlayerViewController: the tvOS transport bar and subtitle menu, plus
/// our "Next episode" buttons and Audio menu.
struct SystemPlayer: UIViewControllerRepresentable {
    let player: AVPlayer
    var actions: [UIAction] = []
    var menus: [UIMenuElement] = []

    func makeUIViewController(context: Context) -> AVPlayerViewController {
        let vc = AVPlayerViewController()
        vc.player = player
        vc.appliesPreferredDisplayCriteriaAutomatically = true
        return vc
    }

    func updateUIViewController(_ vc: AVPlayerViewController, context: Context) {
        if vc.player !== player { vc.player = player }
        vc.contextualActions = actions
        vc.transportBarCustomMenuItems = menus
    }
}
