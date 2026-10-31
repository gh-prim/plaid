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

@Observable
final class PlayerModel {
    private(set) var player: AVPlayer?
    private(set) var error: String?
    /// The title playing, and the episode after it, if any.
    private(set) var item: ItemSummary?
    private(set) var next: ItemSummary?
    private(set) var askStillWatching = false
    /// The next episode's card, and the last episode's end.
    private(set) var card: NextCard?
    private(set) var endOfShow = false
    /// The "Next episode" buttons shown in the last 30 s.
    private(set) var contextualActions: [UIAction] = []
    /// The transport bar's Audio menu.
    private(set) var menus: [UIMenuElement] = []
    /// A short message over the picture (a change the server refused).
    var toast: String?
    /// Where playback was when it failed, to try again from there.
    private var lastPositionMs: Int64 = 0

    private var client: APIClient?
    private var profileId = "default"
    private var onEnd: @MainActor () -> Void = {}
    private var session: PlaybackSession?
    private var timer: Timer?
    private var timeObserver: Any?
    private var observers: [NSObjectProtocol] = []
    private var statusObservation: NSKeyValueObservation?
    private var stopped = false
    /// Episodes started on their own since the remote was last used.
    private var autoplayed = 0
    private var nextCancelled = false
    /// Intro, recap and credits of the title (TheIntroDB, via the server).
    private var segments: [Segment] = []
    /// When the credits countdown began, in the title's seconds.
    private var countdown: Task<Void, Never>?
    /// Seconds before the end when segments are asked again: the server
    /// looks for the credits in the file once the episode plays.
    private var refetchAt: [Double] = []
    private static let countdownSeconds = 10

    /// Seconds before the end when the next episode is offered.
    private static let offerNextAt = 30.0

    func start(_ item: ItemSummary, from startMs: Int64, fileId: String?, client: APIClient, profileId: String,
               onEnd: @escaping @MainActor () -> Void) async {
        self.client = client
        self.profileId = profileId
        self.onEnd = onEnd
        await load(item, from: startMs, fileId: fileId)
    }

    // MARK: Loading a title

    private func load(_ item: ItemSummary, from startMs: Int64, fileId: String? = nil, quality: String? = nil) async {
        guard let client else { return }
        self.item = item
        next = nil
        nextCancelled = false
        contextualActions = []
        segments = []
        card = nil
        countdown?.cancel()
        refetchAt = item.kind == .episode ? [600, 180] : []
        do {
            var s: PlaybackSession = try await client.post(
                "/items/\(item.id)/playback",
                PlaybackRequest(profile: Capabilities.profile, startPositionMs: startMs, fileId: fileId, quality: quality)
            )
            // The profile's audio language, when the file has it and the
            // server picked another track.
            if let lang = memory.audio,
               let track = s.audioTracks.first(where: { sameLanguage($0.language, lang) }), !track.selected {
                if let updated = try? await patch(s, audio: track.index, at: startMs) { s = updated }
            }
            session = s
            let playerItem = AVPlayerItem(asset: AVURLAsset(url: client.absolute(s.url)))
            playerItem.externalMetadata = metadata(item)
            let p = player ?? AVPlayer()
            p.replaceCurrentItem(with: playerItem)
            await applySubtitleMemory(to: playerItem)
            // Every stream covers the whole file: the player seeks to the start point.
            if s.startPositionMs > 0 {
                await p.seek(to: CMTime(value: s.startPositionMs, timescale: 1000), toleranceBefore: .zero, toleranceAfter: .zero)
            }
            if player == nil {
                player = p
                observePlayer(p)
            }
            observeItem(playerItem)
            buildAudioMenu()
            p.play()
            async let segs = fetchSegments(item)
            if item.kind == .episode { next = await upNext(after: item) }
            segments = await segs
        } catch let e as APIError {
            error = "playback.error.\(e.code)"
            if Bundle.main.localizedString(forKey: error!, value: "?", table: nil) == "?" { error = "error.playback" }
            player = nil
        } catch {
            self.error = "error.playback"
            player = nil
        }
    }

    private func upNext(after item: ItemSummary) async -> ItemSummary? {
        guard let client,
              let (data, status) = try? await client.send("GET", "/items/\(item.id)/up-next", body: Optional<Int>.none),
              status == 200 else { return nil }
        let next = (try? client.decode(data) as UpNext)?.item
        return next?.id == item.id ? nil : next
    }

    private func fetchSegments(_ item: ItemSummary) async -> [Segment] {
        guard let client, item.kind == .movie || item.kind == .episode,
              let found: ItemSegments = try? await client.get("/items/\(item.id)/segments") else { return [] }
        return found.segments
    }

    private func patch(_ s: PlaybackSession, audio: Int, at positionMs: Int64) async throws -> PlaybackSession {
        try await patch(s, PlaybackUpdate(positionMs: positionMs, audioStreamIndex: audio))
    }

    private func patch(_ s: PlaybackSession, _ update: PlaybackUpdate) async throws -> PlaybackSession {
        guard let client else { throw APIError.network }
        let (data, _) = try await client.send("PATCH", "/playback/\(s.id)", body: update)
        return try client.decode(data)
    }

    // MARK: Next episode

    private var autoplayOn: Bool {
        UserDefaults.standard.object(forKey: autoplayKey(profileId)) as? Bool ?? true
    }

    /// Over the picture, as tvOS's own buttons: "Skip intro" or "Skip
    /// recap" during them; "Next episode (n)" and "Cancel" from the credits
    /// (TheIntroDB), or in the last 30 s when they're not known. From the
    /// credits, the next episode starts after a 10 s countdown.
    private func tick() {
        guard let p = player, let current = p.currentItem else { return }
        let now = p.currentTime().seconds
        // The player may not know an HLS stream's length: the session does.
        let length = current.duration.seconds.isFinite ? current.duration.seconds
            : Double(session?.durationMs ?? 0) / 1000
        guard now.isFinite, length > 0 else { return }
        let ms = Int64(now * 1000)
        if let first = refetchAt.first, length - now <= first, let item {
            refetchAt.removeAll { length - now <= $0 }
            Task { [weak self] in
                guard let self else { return }
                let found = await self.fetchSegments(item)
                if !found.isEmpty, self.item?.id == item.id { self.segments = found }
            }
        }

        if let skip = segments.first(where: { ($0.kind == "intro" || $0.kind == "recap") && $0.contains(ms) }) {
            let title = tr(skip.kind == "intro" ? "player.skipIntro" : "player.skipRecap")
            if contextualActions.first?.title != title {
                let to = skip.endMs.map { Double($0) / 1000 } ?? length
                contextualActions = [UIAction(title: title, image: UIImage(systemName: "forward.fill")) { [weak self] _ in
                    Task { @MainActor in
                        await self?.player?.seek(to: CMTime(seconds: to, preferredTimescale: 1000), toleranceBefore: .zero, toleranceAfter: .zero)
                        self?.contextualActions = []
                    }
                }]
            }
            return
        }

        if !contextualActions.isEmpty { contextualActions = [] }

        // The next episode's card: from the credits (the last ones in the
        // second half), or in the last 30 s when they're not known.
        guard let next, autoplayOn, !nextCancelled else { return }
        let credits = segments.last { $0.kind == "credits" && Double($0.startMs ?? 0) / 1000 > length / 2 }
        let offerAt = credits.flatMap { $0.startMs }.map { Double($0) / 1000 } ?? length - Self.offerNextAt
        if now >= offerAt, card == nil {
            card = NextCard(next: next, left: nil)
            // Known credits: no need to sit through them, count down now.
            if credits != nil { startCountdown() }
        }
    }

    /// Ten seconds on the card's ring, then the next episode.
    private func startCountdown() {
        guard countdown == nil || countdown?.isCancelled == true, let next else { return }
        countdown = Task { [weak self] in
            for left in stride(from: Self.countdownSeconds, through: 1, by: -1) {
                self?.card?.left = left
                try? await Task.sleep(for: .seconds(1))
                if Task.isCancelled { return }
            }
            guard let self, !self.nextCancelled else { return }
            self.autoplayed += 1
            await self.advance(to: next)
        }
    }

    private func reachedEnd() {
        guard let next, autoplayOn, !nextCancelled else {
            if item?.kind == .episode, self.next == nil, item?.show != nil { endOfShow = true } else { onEnd() }
            return
        }
        if autoplayed >= 3 {
            countdown?.cancel()
            card = nil
            askStillWatching = true
            return
        }
        if card == nil { card = NextCard(next: next, left: nil) }
        startCountdown()
    }

    /// "Play now" on the card.
    func playNext() {
        guard let next else { return }
        countdown?.cancel()
        autoplayed = 0
        Task { await advance(to: next) }
    }

    /// "Cancel" on the card: this episode plays on to its end, then closes.
    func cancelNext() {
        countdown?.cancel()
        nextCancelled = true
        card = nil
    }

    func continueWatching() {
        askStillWatching = false
        autoplayed = 0
        if let next { Task { await advance(to: next) } }
    }

    /// Ends this session at its last position and starts the next title.
    private func advance(to next: ItemSummary) async {
        countdown?.cancel()
        countdown = nil
        card = nil
        endSession()
        contextualActions = []
        await load(next, from: next.userState?.positionMs ?? 0)
    }

    // MARK: Audio and subtitle memory

    private var memory: TrackMemory { TrackMemory.load(profileId) }

    private func applySubtitleMemory(to playerItem: AVPlayerItem) async {
        guard let choice = memory.subtitles,
              let group = try? await playerItem.asset.loadMediaSelectionGroup(for: .legible) else { return }
        switch choice {
        case .off:
            playerItem.select(nil, in: group)
        case let .language(lang, forced):
            let match = group.options.first {
                ($0.extendedLanguageTag.map { sameLanguage($0, lang) } ?? false)
                    && $0.hasMediaCharacteristic(.containsOnlyForcedSubtitles) == forced
            }
            if let match { playerItem.select(match, in: group) }
        }
    }

    /// Remembers what the viewer picks in the system's subtitles menu.
    private func rememberSubtitles(of playerItem: AVPlayerItem) {
        Task {
            guard let group = try? await playerItem.asset.loadMediaSelectionGroup(for: .legible) else { return }
            var m = memory
            if let option = playerItem.currentMediaSelection.selectedMediaOption(in: group) {
                guard let lang = option.extendedLanguageTag else { return }
                m.subtitles = .language(lang, forced: option.hasMediaCharacteristic(.containsOnlyForcedSubtitles))
            } else {
                m.subtitles = .off
            }
            m.save(profileId)
        }
    }

    /// The server sends one audio track: picking another asks it again and
    /// carries on from the same point.
    private func buildAudioMenu() {
        guard let s = session else { menus = []; return }
        var out: [UIMenuElement] = []
        if s.audioTracks.count > 1 {
            let actions = s.audioTracks.map { track in
                UIAction(title: audioLabel(track), state: track.selected ? .on : .off) { [weak self] _ in
                    Task { @MainActor in await self?.switchAudio(to: track) }
                }
            }
            out.append(UIMenu(title: tr("player.audio"), image: UIImage(systemName: "speaker.wave.2"), children: actions))
        }
        // Image subtitles (Blu-ray PGS) aren't in the HLS stream: the server
        // draws them into the picture.
        let images = s.subtitles.filter { ["hdmv_pgs_subtitle", "pgs", "dvd_subtitle", "vobsub"].contains($0.codec.lowercased()) }
        if !images.isEmpty {
            let burned = images.first { $0.selected && $0.delivery == "burn_in" }
            var actions = [UIAction(title: tr("player.subtitles.off"), state: burned == nil ? .on : .off) { [weak self] _ in
                Task { @MainActor in
                    guard let self else { return }
                    await self.apply(PlaybackUpdate(positionMs: 0, subtitlesOff: true))
                }
            }]
            actions += images.map { sub in
                UIAction(title: subtitleLabel(sub), state: sub.index == burned?.index ? .on : .off) { [weak self] _ in
                    Task { @MainActor in
                        await self?.apply(PlaybackUpdate(positionMs: 0, subtitleStreamIndex: sub.index, burnSubtitles: true))
                    }
                }
            }
            out.append(UIMenu(title: tr("player.subtitles.image"), image: UIImage(systemName: "captions.bubble"), children: actions))
        }
        if s.qualities.count > 1 {
            let actions = s.qualities.map { q in
                UIAction(title: qualityLabel(q), state: q == s.quality ? .on : .off) { [weak self] _ in
                    Task { @MainActor in await self?.apply(PlaybackUpdate(positionMs: 0, quality: q)) }
                }
            }
            out.append(UIMenu(title: tr("player.quality"), image: UIImage(systemName: "dial.medium"), children: actions))
        }
        menus = out
    }

    private func subtitleLabel(_ s: PlaybackSubtitle) -> String {
        let name = s.language.isEmpty || s.language == "und" ? tr("item.about.unknownLanguage") : languageName(s.language)
        return [name, s.forced ? tr("player.subtitles.forced") : nil, s.title].compactMap { $0 }.joined(separator: " · ")
    }

    private func qualityLabel(_ q: String) -> String {
        q == "original" ? tr("player.quality.original") : (q == "2160p" ? "4K" : q)
    }

    private func switchAudio(to track: PlaybackAudioTrack) async {
        guard !track.selected else { return }
        var m = memory
        m.audio = track.language
        m.save(profileId)
        await apply(PlaybackUpdate(positionMs: positionMs, audioStreamIndex: track.index))
    }

    /// Asks the server again with a change (audio, quality, subtitles) and,
    /// when the stream changed, carries on from the same point in the new one.
    private func apply(_ update: PlaybackUpdate) async {
        guard let client, let s = session, let p = player else { return }
        let at = positionMs
        var change = update
        change = PlaybackUpdate(positionMs: at, audioStreamIndex: update.audioStreamIndex, quality: update.quality,
                                subtitleStreamIndex: update.subtitleStreamIndex, subtitlesOff: update.subtitlesOff,
                                burnSubtitles: update.burnSubtitles)
        let updated: PlaybackSession
        do {
            updated = try await patch(s, change)
        } catch let e as APIError where e.code == "transcode_limit_reached" {
            toast = "playback.error.transcode_limit_reached"
            return
        } catch {
            return
        }
        session = updated
        buildAudioMenu()
        guard updated.url != s.url else { return }
        let playerItem = AVPlayerItem(asset: AVURLAsset(url: client.absolute(updated.url)))
        if let item { playerItem.externalMetadata = metadata(item) }
        p.replaceCurrentItem(with: playerItem)
        await applySubtitleMemory(to: playerItem)
        await p.seek(to: CMTime(value: at, timescale: 1000), toleranceBefore: .zero, toleranceAfter: .zero)
        observeItem(playerItem)
        p.play()
    }

    /// Plays again after an error, from where it stopped; `lower` asks for
    /// 720p, which the server converts to.
    func retry(lower: Bool) {
        guard let item else { return }
        let at = lastPositionMs
        error = nil
        endSession()
        Task { await load(item, from: at, quality: lower ? "720p" : nil) }
    }

    private func audioLabel(_ t: PlaybackAudioTrack) -> String {
        let name = languageName(t.language)
        let channels = t.channels > 2 ? " \(t.channels - 1).1" : ""
        return [name + channels, t.title].compactMap { $0 }.joined(separator: " · ")
    }

    // MARK: Observing

    /// The position in the file: every stream covers the whole of it.
    private var positionMs: Int64 {
        guard let player, let s = session else { return 0 }
        let t = player.currentTime().seconds
        guard t.isFinite else { return s.startPositionMs }
        return Int64(t * 1000)
    }

    private func observePlayer(_ p: AVPlayer) {
        timer = Timer.scheduledTimer(withTimeInterval: 10, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.report(p.rate > 0 ? "playing" : "paused") }
        }
        timeObserver = p.addPeriodicTimeObserver(forInterval: CMTime(value: 1, timescale: 2), queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.tick() }
        }
        statusObservation = p.observe(\.timeControlStatus, options: [.new]) { [weak self] p, _ in
            Task { @MainActor in
                switch p.timeControlStatus {
                case .paused: self?.report("paused")
                case .playing: self?.report("playing")
                case .waitingToPlayAtSpecifiedRate: self?.report("buffering")
                @unknown default: break
                }
            }
        }
    }

    /// End, failure and subtitle choice of one title.
    private func observeItem(_ playerItem: AVPlayerItem) {
        observers.forEach(NotificationCenter.default.removeObserver)
        observers = [
            NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: playerItem, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.reachedEnd() }
            },
            NotificationCenter.default.addObserver(forName: .AVPlayerItemFailedToPlayToEndTime, object: playerItem, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated {
                    guard let self else { return }
                    self.lastPositionMs = self.positionMs
                    self.player?.pause()
                    self.error = "error.playback"
                }
            },
            NotificationCenter.default.addObserver(forName: AVPlayerItem.mediaSelectionDidChangeNotification, object: playerItem, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.rememberSubtitles(of: playerItem) }
            },
        ]
    }

    private func report(_ state: String) {
        guard let client, let s = session, !stopped else { return }
        let body = PlaybackProgress(positionMs: positionMs, state: state)
        Task { try? await client.postNoContent("/playback/\(s.id)/progress", body) }
    }

    /// Stops the current session on the server, at the current position.
    private func endSession() {
        guard let client, let s = session else { return }
        session = nil
        let pos = positionMs
        Task { try? await client.delete("/playback/\(s.id)", PlaybackStop(positionMs: pos)) }
    }

    func stop() {
        guard !stopped else { return }
        countdown?.cancel()
        timer?.invalidate()
        if let timeObserver { player?.removeTimeObserver(timeObserver) }
        observers.forEach(NotificationCenter.default.removeObserver)
        statusObservation = nil
        player?.pause()
        endSession()
        stopped = true
    }

    private func metadata(_ item: ItemSummary) -> [AVMetadataItem] {
        func meta(_ id: AVMetadataIdentifier, _ value: String) -> AVMetadataItem {
            let m = AVMutableMetadataItem()
            m.identifier = id
            m.value = value as NSString
            m.extendedLanguageTag = "und"
            return m
        }
        var out = [meta(.commonIdentifierTitle, item.show?.displayTitle ?? item.name)]
        if item.kind == .episode, let code = episodeCode(item) {
            out.append(meta(.iTunesMetadataTrackSubTitle, "\(code) · \(item.name)"))
        }
        if let o = item.overview { out.append(meta(.commonIdentifierDescription, o)) }
        return out
    }
}

func autoplayKey(_ profileId: String) -> String { "autoplayNext.\(profileId)" }

/// A profile's last audio language and subtitle choice (F008 R38).
struct TrackMemory: Codable {
    enum Subtitles: Codable, Equatable {
        case off
        case language(String, forced: Bool)
    }

    var audio: String?
    var subtitles: Subtitles?

    static func load(_ profileId: String) -> TrackMemory {
        guard let data = UserDefaults.standard.data(forKey: "tracks.\(profileId)"),
              let m = try? JSONDecoder().decode(TrackMemory.self, from: data) else { return TrackMemory() }
        return m
    }

    func save(_ profileId: String) {
        if let data = try? JSONEncoder().encode(self) { UserDefaults.standard.set(data, forKey: "tracks.\(profileId)") }
    }
}

// ISO 639-2 has two codes for some languages (fre/fra), and HLS may carry
// the two-letter one: compare them as one language.
private let bibliographic = [
    "sqi": "alb", "hye": "arm", "eus": "baq", "mya": "bur", "zho": "chi", "ces": "cze", "nld": "dut",
    "fra": "fre", "kat": "geo", "deu": "ger", "ell": "gre", "isl": "ice", "mkd": "mac", "mri": "mao",
    "msa": "may", "fas": "per", "ron": "rum", "slk": "slo", "bod": "tib", "cym": "wel",
]

func sameLanguage(_ a: String, _ b: String) -> Bool {
    func norm(_ s: String) -> String {
        let l = s.lowercased().split(separator: "-").first.map(String.init) ?? s.lowercased()
        if l.count == 2, let three = Locale.LanguageCode(l).identifier(.alpha3) { return bibliographic[three] ?? three }
        return bibliographic[l] ?? l
    }
    return norm(a) == norm(b)
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

/// What Apple TV's AVPlayer plays. HLS only, so the server remuxes MKV and
/// sends subtitles as HLS renditions the system menu shows.
enum Capabilities {
    static var profile: CapabilityProfile {
        let hdr = AVPlayer.eligibleForHDRPlayback
        return CapabilityProfile(
            protocols: ["hls"],
            containers: ["mp4", "mov"],
            video: [
                VideoCapability(codec: "h264", profiles: ["baseline", "constrained baseline", "main", "high"],
                                maxLevel: 52, maxWidth: 3840, maxHeight: 2160, maxBitDepth: 8),
                VideoCapability(codec: "hevc", profiles: ["main", "main 10"], maxLevel: 153,
                                maxWidth: 3840, maxHeight: 2160, maxBitDepth: 10,
                                hdr: hdr ? ["hdr10", "hlg", "dolby_vision"] : nil),
            ],
            audio: [
                AudioCapability(codec: "aac", maxChannels: 8),
                AudioCapability(codec: "ac3", maxChannels: 6),
                AudioCapability(codec: "eac3", maxChannels: 8),
                AudioCapability(codec: "mp3", maxChannels: 2),
                AudioCapability(codec: "flac", maxChannels: 8),
                AudioCapability(codec: "alac", maxChannels: 8),
            ],
            subtitles: SubtitleCapability(text: ["vtt"]),
            canSwitchAudioTrack: false
        )
    }
}
