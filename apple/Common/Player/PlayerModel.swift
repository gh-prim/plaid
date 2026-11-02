import AVFoundation
import SwiftUI
import UIKit

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
    /// Plaid Party (F010): the party this screen plays for. The room drives
    /// play, pause, seeks and the next episode (PartySync), not this model.
    var partyId: String?
    /// A party screen the server couldn't convert for (503): the host
    /// decides (R26) and the room tells it to try again.
    private(set) var blocked = false
    /// A party screen reached the end of its title.
    private(set) var ended = false

    private var client: APIClient?
    private var profileId = "default"
    private var onEnd: @MainActor () -> Void = {}
    /// What the server is sending: its audio tracks, subtitles and qualities.
    private(set) var session: PlaybackSession?
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

    /// A party screen (F010): the title is known by its id only.
    func start(itemId: String, partyId: String, client: APIClient, profileId: String,
               onEnd: @escaping @MainActor () -> Void) async {
        self.partyId = partyId
        self.client = client
        self.profileId = profileId
        self.onEnd = onEnd
        await switchItem(to: itemId)
    }

    /// The party moved to another title (next episode, vote winner): a new
    /// session on it, started where the room is.
    func switchItem(to itemId: String) async {
        guard let client else { return }
        guard let d: ItemDetail = try? await client.get("/items/\(itemId)") else {
            error = "error.playback"
            return
        }
        if session != nil { endSession() }
        await load(d.summary, from: 0)
    }

    /// The host chose (R26): try again, at the shared quality when given;
    /// a screen already converting moves to it too.
    func partyRetry(quality: String?) async {
        guard let item else { return }
        if blocked {
            blocked = false
            await load(item, from: lastPositionMs, quality: quality)
        } else if let quality, session?.method == "transcode", session?.quality != quality {
            await setQuality(quality)
        }
    }

    /// Where the player is in the title, in ms.
    var currentMs: Int64 { positionMs }

    /// Where the end credits start, in seconds: the known credits in the
    /// second half, else the last 30 s (R30).
    var creditsStart: Double? {
        let length = player?.currentItem.map(\.duration.seconds).flatMap { $0.isFinite && $0 > 0 ? $0 : nil }
            ?? Double(session?.durationMs ?? 0) / 1000
        guard length > 0 else { return nil }
        let credits = segments.last { $0.kind == "credits" && Double($0.startMs ?? 0) / 1000 > length / 2 }
        return credits.flatMap(\.startMs).map { Double($0) / 1000 } ?? length - Self.offerNextAt
    }

    // MARK: Loading a title

    private func load(_ item: ItemSummary, from startMs: Int64, fileId: String? = nil, quality: String? = nil) async {
        guard let client else { return }
        self.item = item
        next = nil
        ended = false
        blocked = false
        nextCancelled = false
        contextualActions = []
        segments = []
        card = nil
        countdown?.cancel()
        refetchAt = item.kind == .episode ? [600, 180] : []
        do {
            var s: PlaybackSession = try await client.post(
                "/items/\(item.id)/playback",
                PlaybackRequest(profile: Capabilities.profile, startPositionMs: startMs, fileId: fileId, quality: quality,
                                partyId: partyId)
            )
            // The profile's audio language, when the file has it and the
            // server picked another track.
            if let lang = memory.audio,
               let track = s.audioTracks.first(where: { sameLanguage($0.language, lang) }), !track.selected {
                if let updated = try? await patch(s, audio: track.index, at: startMs) { s = updated }
            }
            session = s
            let playerItem = AVPlayerItem(asset: AVURLAsset(url: client.absolute(s.url)))
            #if os(tvOS)
            playerItem.externalMetadata = metadata(item)
            #endif
            let p = player ?? AVPlayer()
            p.replaceCurrentItem(with: playerItem)
            await applySubtitleMemory(to: playerItem)
            await pickDrawn(s, item: playerItem)
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
            // A party screen waits for the room to say play (PartySync).
            if partyId == nil { p.play() }
            async let segs = fetchSegments(item)
            if item.kind == .episode, partyId == nil { next = await upNext(after: item) }
            segments = await segs
        } catch let e as APIError where partyId != nil && e.code == "transcode_limit_reached" {
            lastPositionMs = startMs
            blocked = true
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
        // second half), or in the last 30 s when they're not known. A party
        // moves on together instead (R30).
        guard partyId == nil, let next, autoplayOn, !nextCancelled else { return }
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
        if partyId != nil {
            ended = true
            return
        }
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
                // No text track because a drawn one shows: that's its choice.
                if self.drawnIndex != nil { return }
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
        let images = imageSubtitles
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

    func subtitleLabel(_ s: PlaybackSubtitle) -> String {
        let name = s.language.isEmpty || s.language == "und" ? tr("item.about.unknownLanguage") : languageName(s.language)
        return [name, s.forced ? tr("player.subtitles.forced") : nil, s.title].compactMap { $0 }.joined(separator: " · ")
    }

    private func qualityLabel(_ q: String) -> String {
        q == "original" ? tr("player.quality.original") : (q == "2160p" ? "4K" : q)
    }

    func switchAudio(to track: PlaybackAudioTrack) async {
        guard !track.selected else { return }
        var m = memory
        m.audio = track.language
        m.save(profileId)
        await apply(PlaybackUpdate(positionMs: positionMs, audioStreamIndex: track.index))
    }

    /// The image subtitles of the file: not in the HLS stream, the server
    /// draws the chosen one into the picture.
    var imageSubtitles: [PlaybackSubtitle] {
        (session?.subtitles ?? []).filter {
            ["hdmv_pgs_subtitle", "pgs", "dvd_subtitle", "vobsub"].contains($0.codec.lowercased()) && !$0.drawnByApp
        }
    }

    /// The PGS tracks the app draws itself (iPhone and iPad).
    var drawnSubtitles: [PlaybackSubtitle] { (session?.subtitles ?? []).filter(\.drawnByApp) }

    /// The drawn track showing, if any: picked by the viewer, by their
    /// memory, or forced by the server for the audio's language.
    var drawnSubtitle: PlaybackSubtitle? { drawnSubtitles.first { $0.index == drawnIndex } }
    var drawnIndex: Int?

    /// Shows a drawn track, or none; remembered for the profile as any choice.
    func showDrawn(_ sub: PlaybackSubtitle?) {
        drawnIndex = sub?.index
        var m = memory
        m.subtitles = sub.map { .language($0.language, forced: $0.forced) } ?? .off
        m.save(profileId)
    }

    /// The drawn track a new session starts on: the one the server selected,
    /// else the profile's language when the stream has no text track for it.
    private func pickDrawn(_ s: PlaybackSession, item playerItem: AVPlayerItem) async {
        let drawn = s.subtitles.filter(\.drawnByApp)
        guard !drawn.isEmpty else { drawnIndex = nil; return }
        if let selected = drawn.first(where: \.selected) { drawnIndex = selected.index; return }
        guard case let .language(lang, forced) = memory.subtitles else { drawnIndex = nil; return }
        if let group = try? await playerItem.asset.loadMediaSelectionGroup(for: .legible),
           playerItem.currentMediaSelection.selectedMediaOption(in: group) != nil {
            drawnIndex = nil
            return
        }
        drawnIndex = drawn.first { sameLanguage($0.language, lang) && $0.forced == forced }?.index
    }

    /// Image subtitles (Blu-ray PGS), drawn into the picture by the server;
    /// nil turns them off.
    func burnSubtitles(_ sub: PlaybackSubtitle?) async {
        if let sub {
            await apply(PlaybackUpdate(positionMs: 0, subtitleStreamIndex: sub.index, burnSubtitles: true))
        } else {
            await apply(PlaybackUpdate(positionMs: 0, subtitlesOff: true))
        }
    }

    func setQuality(_ q: String) async {
        await apply(PlaybackUpdate(positionMs: 0, quality: q))
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
        #if os(tvOS)
        if let item { playerItem.externalMetadata = metadata(item) }
        #endif
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

    func audioLabel(_ t: PlaybackAudioTrack) -> String {
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

    #if os(tvOS)
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
    #endif
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

#if os(tvOS)
private let separateSubtitles: [String]? = nil
#else
private let separateSubtitles: [String]? = ["pgs"]
#endif

/// What AVPlayer plays, on Apple TV, iPhone and iPad. HLS only, so the server remuxes MKV and
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
            // iPhone and iPad draw Blu-ray subtitles over the picture
            // themselves; Apple TV has them burned in.
            subtitles: SubtitleCapability(text: ["vtt"], separate: separateSubtitles),
            canSwitchAudioTrack: false
        )
    }
}

/// The next episode on the card, and the seconds left when counting down.
struct NextCard: Equatable {
    let next: ItemSummary
    var left: Int?
}
