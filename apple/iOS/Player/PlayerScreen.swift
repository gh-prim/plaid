import AVFoundation
import SwiftUI

/// Full-screen playback on iPhone (landscape) and iPad (IPh07–10, IPa07–10):
/// the picture, glass controls that hide while it plays, the subtitles and
/// audio panel, skip intro, the next episode over the credits, and a remote
/// view while the picture is on an AirPlay screen. The session, progress
/// reports and episode chaining are the TV's (PlayerModel).
struct PlayerScreen: View {
    let request: PlayRequest
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @Environment(\.isWide) private var wide
    @State private var model = PlayerModel()
    @State private var clock = PlaybackClock()
    @State private var pip = PictureInPicture()
    @State private var nowPlaying = NowPlaying()
    @State private var controls = true
    @State private var tracks = false
    @State private var fill = false
    @State private var rate: Float = 1
    @State private var hide: Task<Void, Never>?

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            if model.askStillWatching {
                stillWatching
            } else if model.endOfShow, let show = model.item?.show?.displayTitle {
                endOfShow(show)
            } else if let error = model.error {
                errorView(error)
            } else if let player = model.player {
                if clock.external {
                    AirPlayRemote(model: model, clock: clock, player: player, tracks: $tracks)
                } else {
                    playing(player)
                }
            } else {
                LoadingView()
            }
        }
        .statusBarHidden(!controls || !wide)
        .persistentSystemOverlays(controls ? .automatic : .hidden)
        .animation(.smooth(duration: 0.3), value: controls)
        .animation(.smooth(duration: 0.35), value: model.card?.next.id)
        .animation(.smooth(duration: 0.3), value: tracks)
        .overlay(alignment: .top) { toast }
        .task {
            OrientationLock.landscape(true)
            guard let client = app.client else { return }
            let profile = app.me?.activeProfile?.id ?? "default"
            await model.start(request.item, from: request.startMs, fileId: request.fileId, client: client, profileId: profile) { dismiss() }
        }
        .onChange(of: model.player) { _, p in
            guard let p else { return }
            clock.attach(p, fallbackDuration: Double(model.session?.durationMs ?? 0) / 1000)
            nowPlaying.start(player: p)
            scheduleHide()
        }
        .onChange(of: model.session?.id) { clock.setFallbackDuration(Double(model.session?.durationMs ?? 0) / 1000) }
        .onChange(of: Int(clock.seconds)) { updateNowPlaying() }
        .onChange(of: clock.playing) { _, on in if on { scheduleHide() } else { hide?.cancel(); controls = true } }
        .onDisappear {
            model.stop()
            clock.detach()
            nowPlaying.stop()
            OrientationLock.landscape(false)
        }
    }

    // MARK: Playing

    private func playing(_ player: AVPlayer) -> some View {
        ZStack {
            VideoSurface(player: player, pip: pip, fill: fill)
                .ignoresSafeArea()
                .allowsHitTesting(false)
            // Taps land on SwiftUI, not on the player's UIKit layer.
            Color.clear
                .contentShape(Rectangle())
                .ignoresSafeArea()
                .onTapGesture(count: 2) { fill.toggle() }
                .onTapGesture { toggleControls() }
            if let sub = model.drawnSubtitle, let path = sub.url, let client = app.client {
                PGSOverlay(url: client.absolute(path), player: player, fill: fill)
                    .ignoresSafeArea()
                    // Raised a little while the controls show, as on the web.
                    .offset(y: controls ? -40 : 0)
                    .animation(.smooth(duration: 0.3), value: controls)
            }
            if clock.buffering { ProgressView().controlSize(.large).tint(Theme.textPrimary) }
            if controls {
                controlsLayer(player).transition(.opacity)
            }
            if tracks {
                Color.black.opacity(0.001).ignoresSafeArea().onTapGesture { tracks = false }
                TracksPanel(model: model) { tracks = false }
                    .padding(.top, 60).padding(.trailing, wide ? 24 : 16)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: wide ? .bottomTrailing : .topTrailing)
                    .padding(.bottom, wide ? 100 : 0)
                    .transition(.scale(0.95, anchor: .topTrailing).combined(with: .opacity))
            }
            if let card = model.card {
                NextEpisodeCard(card: card, playNow: model.playNext, cancel: model.cancelNext)
                    .padding(.trailing, 20).padding(.bottom, controls ? 76 : 20)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomTrailing)
                    .transition(.move(edge: .trailing).combined(with: .opacity))
            } else if let skip = model.contextualActions.first {
                Button(skip.title) { skip.performWithSender(nil, target: nil) }
                    .font(Theme.sans(14, .semibold))
                    .buttonStyle(.glass)
                    .padding(.trailing, 20).padding(.bottom, controls ? 70 : 24)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomTrailing)
            }
        }
    }

    private func controlsLayer(_ player: AVPlayer) -> some View {
        ZStack {
            LinearGradient(stops: [.init(color: .black.opacity(0.55), location: 0), .init(color: .clear, location: 0.3),
                                   .init(color: .clear, location: 0.65), .init(color: .black.opacity(0.6), location: 1)],
                           startPoint: .top, endPoint: .bottom)
                .ignoresSafeArea()
                .allowsHitTesting(false)
            VStack(spacing: 0) {
                HStack(alignment: .top) {
                    GlassIconButton(systemImage: "xmark", label: "common.close", size: 40) { dismiss() }
                    Spacer()
                    if pip.supported {
                        GlassIconButton(systemImage: "pip.enter", label: "player.pip", size: 40) { pip.toggle() }
                            .disabled(!pip.possible)
                    }
                    AirPlayButton {
                        Image(systemName: "airplay.video").font(.system(size: 15, weight: .semibold)).frame(width: 40, height: 40)
                            .glassEffect(.regular, in: Circle())
                    }
                    .frame(width: 40, height: 40)
                    .accessibilityLabel(Text("player.airplay"))
                }
                .overlay(alignment: .top) { titleBlock }
                Spacer()
                HStack(spacing: 10) {
                    Spacer()
                    Menu {
                        Picker(selection: Binding(get: { rate }, set: { rate = $0; player.rate = $0; player.defaultRate = $0 })) {
                            ForEach([0.5, 0.75, 1, 1.25, 1.5, 2] as [Float], id: \.self) { r in
                                Text(verbatim: r.formatted(.number.precision(.fractionLength(0...2))) + "×").tag(r)
                            }
                        } label: { Text("player.speed") }
                        if let s = model.session, s.qualities.count > 1 {
                            Picker(selection: Binding(get: { s.quality }, set: { q in Task { await model.setQuality(q) } })) {
                                ForEach(s.qualities, id: \.self) { q in
                                    Text(verbatim: q == "original" ? tr("player.quality.original") : (q == "2160p" ? "4K" : q)).tag(q)
                                }
                            } label: { Text("player.quality") }
                        }
                    } label: {
                        Image(systemName: "gauge.with.needle").font(.system(size: 15, weight: .semibold)).frame(width: 40, height: 40)
                    }
                    .buttonStyle(.glass).buttonBorderShape(.circle)
                    .accessibilityLabel(Text("player.speed"))
                    GlassIconButton(systemImage: "captions.bubble", label: "player.subtitlesAndAudio", size: 40) {
                        tracks.toggle()
                        hide?.cancel()
                    }
                }
                Scrubber(clock: clock) { s in player.seek(toSeconds: s); scheduleHide() }
                    .padding(.top, 8)
            }
            .padding(.horizontal, wide ? 24 : 20)
            .padding(.vertical, wide ? 20 : 12)
            TransportButtons(player: player, clock: clock, big: wide ? 80 : 70)
        }
        .simultaneousGesture(TapGesture().onEnded { scheduleHide() })
    }

    private var titleBlock: some View {
        VStack(spacing: 2) {
            if let item = model.item {
                Text(verbatim: item.show?.displayTitle ?? item.name).font(Theme.sans(15, .bold)).lineLimit(1)
                Text(verbatim: item.kind == .episode ? cardSubtitle(item) : (item.year.map(String.init) ?? ""))
                    .font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary).lineLimit(1)
            }
        }
        .frame(maxWidth: 360)
        .padding(.top, 4)
        .allowsHitTesting(false)
    }

    // MARK: Ends and errors

    private var stillWatching: some View {
        VStack(spacing: 16) {
            Text("player.stillWatching").displayTitle(30)
            if let item = model.next ?? model.item {
                Text(verbatim: [item.show?.displayTitle, episodeCode(item), item.name].compactMap { $0 }.joined(separator: " · "))
                    .font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
            }
            HStack(spacing: 12) {
                Button { model.continueWatching() } label: { Label("player.stillWatching.continue", systemImage: "play.fill") }
                    .buttonStyle(CreamButtonStyle()).frame(width: 200)
                Button("player.stillWatching.stop") { dismiss() }.buttonStyle(.glass)
            }
            .padding(.top, 8)
        }
        .padding(24)
    }

    private func endOfShow(_ show: String) -> some View {
        VStack(spacing: 18) {
            Text("player.endOfShow.title \(show)").displayTitle(26).multilineTextAlignment(.center)
            Button { dismiss() } label: { Text("player.endOfShow.back \(show)") }
                .buttonStyle(CreamButtonStyle()).frame(maxWidth: 280)
        }
        .padding(24)
    }

    private func errorView(_ error: String) -> some View {
        VStack(spacing: 18) {
            Text(LocalizedStringKey(error)).font(Theme.sans(16, .medium)).foregroundStyle(Theme.textSecondary)
                .multilineTextAlignment(.center)
            HStack(spacing: 10) {
                Button { model.retry(lower: false) } label: { Label("common.retry", systemImage: "arrow.clockwise") }
                    .buttonStyle(.glass)
                Button { model.retry(lower: true) } label: { Label("player.error.lower", systemImage: "dial.low") }
                    .buttonStyle(.glass)
                Button("common.close") { dismiss() }.buttonStyle(.glass)
            }
        }
        .padding(24)
    }

    @ViewBuilder
    private var toast: some View {
        if let toast = model.toast {
            Text(LocalizedStringKey(toast))
                .font(Theme.sans(14, .semibold))
                .padding(.horizontal, 18).padding(.vertical, 12)
                .glassEffect(.regular, in: Capsule())
                .padding(.top, 16)
                .task { try? await Task.sleep(for: .seconds(4)); model.toast = nil }
        }
    }

    // MARK: Helpers

    private func toggleControls() {
        if tracks { tracks = false; return }
        controls.toggle()
        if controls { scheduleHide() }
    }

    /// The controls fade out after 4 s of playing untouched.
    private func scheduleHide() {
        hide?.cancel()
        hide = Task {
            try? await Task.sleep(for: .seconds(4))
            guard !Task.isCancelled, clock.playing, !tracks else { return }
            controls = false
        }
    }

    private func updateNowPlaying() {
        guard let item = model.item else { return }
        nowPlaying.update(title: item.show?.displayTitle ?? item.name,
                          subtitle: item.kind == .episode ? cardSubtitle(item) : nil,
                          seconds: clock.seconds, duration: clock.duration, rate: clock.playing ? rate : 0)
    }
}

/// While the picture is on an AirPlay screen (IPh10 / IPa10): the still with
/// where it plays, the title, the scrubber and transport, the subtitles,
/// and the AirPlay picker to bring it back.
private struct AirPlayRemote: View {
    let model: PlayerModel
    let clock: PlaybackClock
    let player: AVPlayer
    @Binding var tracks: Bool
    @Environment(\.isWide) private var wide
    @State private var subtitlesLabel = ""

    var body: some View {
        ZStack {
            AmbientGlow(art: model.item?.backdrop ?? model.item?.still, brightness: -0.35).ignoresSafeArea()
            let layout = wide ? AnyLayout(VStackLayout(spacing: 36)) : AnyLayout(HStackLayout(spacing: 36))
            layout {
                ZStack {
                    RemoteImage(art: model.item?.still ?? model.item?.backdrop, width: 900)
                    Color.black.opacity(0.25)
                    VStack(spacing: 8) {
                        Image(systemName: "airplay.video").font(.system(size: 28, weight: .medium))
                        Text("player.airplay.playingOn \(clock.routeName ?? tr("player.airplay.tv"))").font(Theme.sans(13, .semibold))
                    }
                }
                .frame(width: 300, height: 169)
                .clipShape(RoundedRectangle(cornerRadius: 16))
                .shadow(color: .black.opacity(0.5), radius: 20, y: 14)
                VStack(alignment: wide ? .center : .leading, spacing: 0) {
                    if let name = clock.routeName {
                        Text(verbatim: name).font(Theme.sans(12, .semibold)).foregroundStyle(Theme.textSecondary)
                    }
                    Text(verbatim: model.item?.show?.displayTitle ?? model.item?.name ?? "").displayTitle(28).lineLimit(1).padding(.top, 6)
                    Scrubber(clock: clock) { player.seek(toSeconds: $0) }.padding(.top, 14)
                    TransportButtons(player: player, clock: clock, big: 62).frame(maxWidth: .infinity).padding(.top, 14)
                    HStack(spacing: 10) {
                        Button { tracks.toggle() } label: { Label(subtitlesLabel, systemImage: "captions.bubble") }
                            .buttonStyle(.glass)
                        AirPlayButton {
                            Label("player.airplay.stop", systemImage: "airplay.video")
                                .font(Theme.sans(14, .semibold))
                                .padding(.horizontal, 14).frame(height: 36)
                                .glassEffect(.regular, in: Capsule())
                        }
                    }
                    .font(Theme.sans(14, .semibold))
                    .frame(maxWidth: .infinity)
                    .padding(.top, 18)
                }
                .frame(maxWidth: 360)
            }
            .padding(24)
            if tracks {
                Color.black.opacity(0.001).ignoresSafeArea().onTapGesture { tracks = false }
                TracksPanel(model: model) { tracks = false }.padding(24)
            }
        }
        .task(id: player.currentItem) { await loadSubtitlesLabel() }
        .onReceive(NotificationCenter.default.publisher(for: AVPlayerItem.mediaSelectionDidChangeNotification)) { _ in
            Task { await loadSubtitlesLabel() }
        }
    }

    /// The subtitles shown, read again whenever the selection changes.
    private func loadSubtitlesLabel() async {
        guard let item = player.currentItem,
              let group = try? await item.asset.loadMediaSelectionGroup(for: .legible),
              let o = item.currentMediaSelection.selectedMediaOption(in: group) else {
            subtitlesLabel = tr("player.subtitles.off")
            return
        }
        subtitlesLabel = languageName(o.extendedLanguageTag ?? "und")
    }
}
