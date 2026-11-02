import AVFoundation

/// Keeps a party screen's player on the room (F010-R8–R11, api/events.md):
/// every 250 ms it compares where the player is with where the room is at
/// the server's clock. Under 150 ms nothing; from 150 ms to 3 s it plays at
/// 1.05 (behind) or 0.95 (ahead) until within 50 ms; beyond 3 s it seeks.
/// When the player doesn't honour the rate, it seeks past 1 s instead. It
/// also reports what the screen shows: loading, ready, playing, paused,
/// buffering, ended, blocked and credits.
final class PartySync {
    private let room: PartyRoom
    private let model: PlayerModel
    private var timer: Timer?

    /// The rate set to catch up, nil at 1, and the drift it started from.
    private var correcting: Float?
    private var correctingFrom: Int64 = 0
    /// Where the player was when the correction began, to measure the rate
    /// it really plays at.
    private var correctionStart: (local: Int64, wall: Date)?
    /// The player ignored a rate change: seeks past 1 s instead.
    private var noRate = false
    private var seeking = false

    private weak var item: AVPlayerItem?
    private var reported = ""
    private var lastReport = Date.distantPast
    private var readySent = false
    private var creditsSent = false

    static let tolerance: Int64 = 150
    static let settled: Int64 = 50
    static let seekBeyond: Int64 = 3000
    static let seekBeyondWithoutRate: Int64 = 1000

    init(room: PartyRoom, model: PlayerModel) {
        self.room = room
        self.model = model
    }

    func start() {
        room.onItem = { [weak self] id in
            guard let self else { return }
            Task { await self.model.switchItem(to: id) }
        }
        room.onTranscode = { [weak self] apply in
            guard let self else { return }
            let mine = apply.waiting?.contains(self.room.me) ?? false
            Task {
                if apply.retry == true {
                    await self.model.partyRetry(quality: nil)
                } else if mine || self.model.session?.method == "transcode" {
                    await self.model.partyRetry(quality: apply.quality)
                }
            }
        }
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: 0.25, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated { self?.tick() }
        }
    }

    func stop() {
        timer?.invalidate()
        timer = nil
        room.onItem = { _ in }
        room.onTranscode = { _ in }
    }

    private func tick() {
        guard !room.ended, !room.removed else { return }
        if model.blocked { report("blocked"); return }
        guard let p = model.player, let current = p.currentItem else { report("loading"); return }
        if current !== item {
            item = current
            readySent = false
            creditsSent = false
            correcting = nil
            seeking = false
        }
        guard current.status == .readyToPlay else { report("loading"); return }
        if model.ended { report("ended"); return }

        let pb = room.playback
        let target = pb.position(atServerMs: room.serverNowMs)
        let local = model.currentMs

        if !creditsSent, let credits = model.creditsStart, Double(local) / 1000 >= credits, pb.state == "playing" {
            creditsSent = true
            report("credits")
        }

        if pb.state == "playing" {
            follow(p, local: local, target: target)
        } else {
            // Paused or waiting for someone: stay still on the room's position.
            correcting = nil
            if p.rate != 0 { p.pause() }
            if !seeking, abs(local - target) > Self.tolerance { seek(p, to: target) }
        }
        report(screenState(p, current, room: pb.state))
    }

    /// Playing: keep up with the room's moving position.
    private func follow(_ p: AVPlayer, local: Int64, target: Int64) {
        guard !seeking else { return }
        let drift = local - target
        let far = noRate ? Self.seekBeyondWithoutRate : Self.seekBeyond
        if abs(drift) > far {
            correcting = nil
            // Lands a little ahead: the stream needs a moment to start again.
            seek(p, to: target + 300, thenPlay: true)
            return
        }
        if p.timeControlStatus == .paused { p.rate = correcting ?? 1 }
        if let rate = correcting {
            let overshot = (rate > 1 && drift >= 0) || (rate < 1 && drift <= 0)
            if abs(drift) <= Self.settled || overshot {
                p.rate = 1
                correcting = nil
                if abs(correctingFrom) > 1000 {
                    let seconds = (Double(abs(correctingFrom)) / 1000).formatted(.number.precision(.fractionLength(1)).locale(AppLanguage.locale))
                    room.show(tr(correctingFrom < 0 ? "party.sync.caughtUp %@" : "party.sync.waitedUp %@", seconds))
                }
                return
            }
            // After 2 s at the new rate, check the picture really moved faster
            // or slower; HLS on some players ignores small rate changes.
            if let start = correctionStart, Date().timeIntervalSince(start.wall) >= 2, p.timeControlStatus == .playing {
                let wall = Date().timeIntervalSince(start.wall) * 1000
                let effective = Double(local - start.local) / wall
                if abs(effective - 1) < 0.015 {
                    noRate = true
                    p.rate = 1
                    correcting = nil
                }
                correctionStart = nil
            }
        } else if abs(drift) >= Self.tolerance {
            let rate: Float = drift < 0 ? 1.05 : 0.95
            correcting = rate
            correctingFrom = drift
            correctionStart = (local, Date())
            p.rate = rate
        }
    }

    private func seek(_ p: AVPlayer, to ms: Int64, thenPlay: Bool = false) {
        seeking = true
        p.seek(to: CMTime(value: max(0, ms), timescale: 1000), toleranceBefore: .zero, toleranceAfter: .zero) { [weak self] _ in
            Task { @MainActor in
                guard let self else { return }
                self.seeking = false
                if thenPlay, self.room.playback.state == "playing" { p.rate = 1 }
            }
        }
    }

    /// What the screen shows, as the room wants it (R11): a paused screen
    /// that has its picture is `ready` while the room waits or before it starts.
    private func screenState(_ p: AVPlayer, _ item: AVPlayerItem, room state: String) -> String {
        switch p.timeControlStatus {
        case .playing:
            readySent = true
            return "playing"
        case .waitingToPlayAtSpecifiedRate:
            return p.reasonForWaitingToPlay == .noItemToPlay ? "loading" : "buffering"
        case .paused:
            if seeking { return reported.isEmpty ? "loading" : reported }
            if state == "waiting" || !readySent || !room.started {
                guard item.isPlaybackLikelyToKeepUp || item.isPlaybackBufferFull else { return "buffering" }
                readySent = true
                return "ready"
            }
            return "paused"
        @unknown default:
            return "paused"
        }
    }

    /// On every change, and every 10 s while playing.
    private func report(_ state: String) {
        let now = Date()
        guard state != reported || (state == "playing" && now.timeIntervalSince(lastReport) >= 10) else { return }
        reported = state
        lastReport = now
        room.report(state, positionMs: model.currentMs, session: model.session?.id, quality: model.session?.quality)
    }
}
