import Foundation
import Observation

/// One Plaid Party as this device sees it (F010): the server's room state
/// from party.snapshot and the messages after it, and the actions sent back.
/// A screen plays the title (PartySync keeps it in step); a companion shows
/// the chat and controls while a TV of the same account plays (R22).
@Observable
final class PartyRoom {
    enum Role: Equatable { case screen, companion }

    /// A reaction floating over the picture for a few seconds.
    struct Floating: Identifiable, Equatable {
        let id = UUID()
        let emoji: String
        let name: String
        let mine: Bool
    }

    /// A short line about what someone did (R13), or the gentle catch-up (R10).
    struct Event: Identifiable, Equatable {
        let id = UUID()
        let text: String
    }

    private(set) var partyId: String?
    /// The code typed by someone outside the invitations (R3).
    private let code: String?
    let role: Role
    /// The F003 session of the TV that plays, when this device chose one.
    private let tvSession: String?
    /// The TV this companion follows ("On Apple TV · Salon").
    var screenName: String?
    /// The party over REST: its code, link, title, status and time.
    private(set) var party: Party?

    private(set) var joined = false
    /// Why joining failed: a socket error code (party_not_found, not_allowed…).
    private(set) var failure: String?
    private(set) var me = ""
    private(set) var host = ""
    private(set) var mode = "host"
    private(set) var itemId: String?
    private(set) var playback = RoomPlayback.idle
    private(set) var presence = RoomPresence.empty
    private(set) var chat: [ChatLine] = []
    private(set) var floating: [Floating] = []
    private(set) var pauseRequests: [PauseAsk] = []
    private(set) var pauseAtServerMs: Int64?
    private(set) var waiting: RoomWaiting?
    private(set) var next: RoomNext?
    private(set) var transcodeLimit: TranscodeLimit?
    /// In the queue of a full party (R4): our place in it.
    private(set) var queuedAt: Int?
    /// Host: who waits for a seat.
    private(set) var queue: [String] = []
    private(set) var ended = false
    private(set) var removed = false
    private(set) var event: Event?
    /// The room played at least once: the open-party panel and the lobby go away.
    private(set) var started = false
    /// Reactions available (the six of R19).
    private(set) var emojis = ["😂", "😱", "❤️", "👏", "🦇", "🍿"]

    /// The screen's player follows these: a new title (next episode, vote
    /// winner) and the host's transcode choice (R26).
    var onItem: (String) -> Void = { _ in }
    var onTranscode: (TranscodeApply) -> Void = { _ in }

    private weak var socket: PartySocket?
    private weak var client: APIClient?

    var isHost: Bool { !me.isEmpty && me == host }
    var mePerson: RoomPerson? { presence.people.first { $0.id == me } }
    var canControl: Bool { mePerson?.canControl ?? (isHost || mode == "everyone") }
    var hostPerson: RoomPerson? { presence.people.first { $0.id == host } }
    var others: [RoomPerson] { presence.people.filter { $0.id != me } }
    var loadingPeople: [RoomPerson] { presence.people.filter { $0.state == "loading" } }
    var lastMessage: ChatLine? { chat.last { $0.system == nil } }
    var serverNowMs: Int64 { socket?.serverNowMs ?? PartySocket.localMs }
    var reconnecting: Bool { socket?.state == .reconnecting }

    init(partyId: String?, code: String? = nil, role: Role, tvSession: String? = nil, socket: PartySocket, client: APIClient) {
        self.partyId = partyId
        self.code = code
        self.role = role
        self.tvSession = tvSession
        self.socket = socket
        self.client = client
    }

    // MARK: Joining

    /// Joins (or joins again after a reconnect: the seat is kept 2 min).
    /// Invitees join by id, others by code (R3).
    func join() async {
        // Not connected yet: the socket joins as soon as it opens (PartyCenter).
        guard let socket, socket.state == .open else { return }
        struct Join: Encodable { let partyId: String?; let code: String?; let screen: String }
        // A TV is asked to play only once: after that this device is its companion.
        let screen = role == .screen ? "this" : (joined ? "companion" : tvSession ?? "companion")
        do {
            try await socket.send("party.join", Join(partyId: partyId, code: partyId == nil ? code : nil, screen: screen))
            failure = nil
        } catch let e as SocketError where e == .offline || e == .timeout {
            // The socket dropped meanwhile: it joins again when it reopens.
        } catch let e as SocketError {
            if !joined { failure = e.code }
        } catch {}
        await refresh()
    }

    /// The party over REST, for its code, link, title and time.
    func refresh() async {
        guard let partyId, let client, let p = try? await client.party(partyId) else { return }
        party = p
        if p.status == .ended || p.status == .cancelled { ended = true }
    }

    func leave() {
        guard joined, let socket else { return }
        Task { try? await socket.send("party.leave") }
        joined = false
    }

    // MARK: Server messages

    func handle(_ type: String, _ raw: Data) {
        switch type {
        case "party.snapshot":
            guard let s = socketData(raw, as: RoomSnapshot.self) else { return }
            let first = partyId == nil
            partyId = s.partyId
            me = s.me
            host = s.host
            mode = s.mode
            if let id = s.itemId, !id.isEmpty, id != itemId {
                let changed = itemId != nil
                itemId = id
                if changed { onItem(id) }
            }
            playback = s.playback
            presence = s.presence
            chat = s.chat
            pauseRequests = s.pauseRequests
            pauseAtServerMs = s.pauseAtServerMs
            emojis = s.reactions.isEmpty ? emojis : s.reactions
            queuedAt = nil
            joined = true
            if s.playback.state == "playing" || s.playback.positionMs > 0 { started = true }
            if first { Task { await refresh() } }
        case "party.presence":
            if let p = socketData(raw, as: RoomPresence.self) { presence = p }
        case "party.playback":
            guard let p = socketData(raw, as: RoomPlayback.self) else { return }
            let before = playback
            playback = p
            if p.state == "playing" { started = true }
            if p.state != "waiting" { waiting = nil }
            if p.state == "paused" { pauseAtServerMs = nil }
            announce(p, after: before)
        case "party.waiting":
            waiting = socketData(raw, as: RoomWaiting.self)
        case "party.chat":
            guard let line = socketData(raw, as: ChatLine.self) else { return }
            chat.append(line)
            if chat.count > 200 { chat.removeFirst(chat.count - 200) }
        case "party.reaction":
            guard let r = socketData(raw, as: RoomReaction.self) else { return }
            float(r.emoji, name: r.name, mine: r.from == me)
        case "party.pause.requested":
            if let a = socketData(raw, as: PauseAsk.self) { pauseRequests.append(a) }
        case "party.pause.answered":
            struct Answered: Decodable { let requestId: Int64 }
            if let a = socketData(raw, as: Answered.self) { pauseRequests.removeAll { $0.requestId == a.requestId } }
        case "party.pause.scheduled":
            struct At: Decodable { let atServerMs: Int64 }
            pauseAtServerMs = socketData(raw, as: At.self)?.atServerMs
        case "party.mode":
            struct Mode: Decodable { let mode: String }
            if let m = socketData(raw, as: Mode.self) { mode = m.mode }
        case "party.host.changed":
            struct Host: Decodable { let host: String }
            if let h = socketData(raw, as: Host.self) { host = h.host }
        case "party.next":
            let n = socketData(raw, as: RoomNext.self)
            next = n?.stopped == true ? nil : n
        case "party.item":
            struct Item: Decodable { let itemId: String }
            guard let i = socketData(raw, as: Item.self) else { return }
            next = nil
            itemId = i.itemId
            onItem(i.itemId)
            Task { await refresh() }
        case "party.transcode.limit":
            transcodeLimit = socketData(raw, as: TranscodeLimit.self)
        case "party.transcode.apply":
            if let a = socketData(raw, as: TranscodeApply.self) { onTranscode(a) }
        case "party.transcode.wait", "party.transcode.cancelled":
            break
        case "party.queued":
            struct Pos: Decodable { let position: Int }
            queuedAt = socketData(raw, as: Pos.self)?.position
        case "party.queue":
            struct Waiting: Decodable { let waiting: [String] }
            queue = socketData(raw, as: Waiting.self)?.waiting ?? []
        case "party.removed":
            removed = true
            joined = false
        case "party.ended":
            ended = true
            joined = false
            Task { await refresh() }
        default:
            break
        }
    }

    /// "Mia paused", "Léo went back to 37:40 · Everyone followed" (R13).
    private func announce(_ p: RoomPlayback, after before: RoomPlayback) {
        guard let by = p.by, by != me, let name = p.byName else { return }
        switch p.action {
        case "pause":
            show(tr("party.sync.paused %@", name))
        case "seek":
            let from = before.position(atServerMs: p.atServerMs)
            let key = p.positionMs < from ? "party.sync.seeked %@ %@" : "party.sync.skipped %@ %@"
            show(tr(key, name, partyClock(p.positionMs)))
        default:
            break
        }
    }

    func show(_ text: String) {
        let e = Event(text: text)
        event = e
        Task {
            try? await Task.sleep(for: .seconds(4))
            if event?.id == e.id { event = nil }
        }
    }

    private func float(_ emoji: String, name: String, mine: Bool) {
        let f = Floating(emoji: emoji, name: name, mine: mine)
        floating.append(f)
        if floating.count > 24 { floating.removeFirst(floating.count - 24) }
        Task {
            try? await Task.sleep(for: .seconds(4))
            floating.removeAll { $0.id == f.id }
        }
    }

    // MARK: Actions

    /// Play, pause or seek for everyone (R13); without control, a pause
    /// becomes a pause request (R15).
    func control(_ action: String, at positionMs: Int64? = nil) async {
        guard canControl else {
            if action == "pause" { await askPause() }
            return
        }
        struct Control: Encodable { let action: String; let positionMs: Int64? }
        await run("party.control", Control(action: action, positionMs: positionMs))
    }

    /// Seeks by a number of seconds from where the room is.
    func skip(_ seconds: Int64) async {
        let at = max(0, playback.position(atServerMs: serverNowMs) + seconds * 1000)
        await control("seek", at: at)
    }

    func askPause() async {
        do {
            try await socket?.send("party.pause.request")
            show(tr("party.pause.asked"))
        } catch let e as SocketError where e.code == "too_soon" {
            show(tr("party.error.too_soon"))
        } catch {}
    }

    /// The host's answer: now, in5 or no.
    func answerPause(_ ask: PauseAsk, _ answer: String) async {
        struct Answer: Encodable { let requestId: Int64; let answer: String }
        pauseRequests.removeAll { $0.requestId == ask.requestId }
        await run("party.pause.answer", Answer(requestId: ask.requestId, answer: answer))
    }

    func react(_ emoji: String) {
        struct React: Encodable { let emoji: String }
        socket?.post("party.react", React(emoji: emoji))
    }

    func send(chat text: String) async {
        let t = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !t.isEmpty else { return }
        struct Chat: Encodable { let text: String }
        await run("party.chat", Chat(text: String(t.prefix(500))))
    }

    func setMode(_ m: String) async {
        struct Mode: Encodable { let mode: String }
        await run("party.mode.set", Mode(mode: m))
    }

    func grant(_ person: RoomPerson, _ on: Bool) async {
        struct Grant: Encodable { let to: String; let on: Bool }
        await run("party.control.grant", Grant(to: person.id, on: on))
    }

    func makeHost(_ person: RoomPerson) async {
        struct To: Encodable { let to: String }
        await run("party.host.set", To(to: person.id))
    }

    func remove(_ person: RoomPerson) async {
        struct Who: Encodable { let who: String }
        await run("party.remove", Who(who: person.id))
    }

    func end() async { await run("party.end", PartySocket.Empty()) }

    /// The host starts a lobby or a fresh party now.
    func start() async {
        await run("party.start", PartySocket.Empty())
        if !started { await control("play", at: playback.positionMs) }
    }

    func resumeWithout() async { await run("party.resume.without", PartySocket.Empty()) }

    func stopNext() async {
        next = nil
        await run("party.next.stop", PartySocket.Empty())
    }

    /// shared (with the common quality), order (with who goes first) or cancel.
    func chooseTranscode(_ choice: String, quality: String? = nil, first: String? = nil) async {
        struct Choose: Encodable { let choice: String; let quality: String?; let first: String? }
        transcodeLimit = nil
        await run("party.transcode.choose", Choose(choice: choice, quality: quality, first: first))
    }

    func leaveQueue() {
        socket?.post("party.queue.leave", PartySocket.Empty())
        queuedAt = nil
        removed = true
    }

    /// What the screen shows (R11, R26, R30): loading, ready, playing,
    /// paused, buffering, ended, blocked or credits.
    func report(_ state: String, positionMs: Int64, session: String?, quality: String?) {
        struct Report: Encodable { let state: String; let positionMs: Int64; let playbackSessionId: String?; let quality: String? }
        socket?.post("party.report", Report(state: state, positionMs: positionMs, playbackSessionId: session, quality: quality))
    }

    /// What the screen did to stay in sync (seek, rate, settled, noRate),
    /// for the server's logs (api/events.md).
    func syncLog(_ event: String, driftMs: Int64, rate: Float, positionMs: Int64) {
        struct SyncLog: Encodable { let event: String; let driftMs: Int64; let rate: Float; let positionMs: Int64 }
        socket?.post("party.sync.log", SyncLog(event: event, driftMs: driftMs, rate: rate, positionMs: positionMs))
    }

    private func run(_ type: String, _ data: some Encodable) async {
        do {
            try await socket?.send(type, data)
        } catch let e as SocketError {
            show(partyError(e.code))
        } catch {}
    }
}

/// A position in a title: "38:12", "1:12:40".
func partyClock(_ ms: Int64) -> String {
    let s = Int(max(0, ms) / 1000)
    return s >= 3600 ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
}

/// The words for a socket or REST error code, with a generic fallback.
func partyError(_ code: String) -> String {
    let key = "party.error.\(code)"
    let text = localized(key)
    return text == key ? tr("party.error.generic") : text
}

/// "Apple TV", "iPhone", "iPad", "browser": where someone watches.
func partyDevice(_ type: String?) -> String {
    switch type {
    case "tvos": "Apple TV"
    case "ios": "iPhone"
    case "ipados": "iPad"
    case "android", "androidtv": "Android"
    default: tr("party.device.browser")
    }
}

/// A chat system line in the app's language (joined, left, removed, host).
func partySystemLine(_ line: ChatLine) -> String {
    let v = line.values ?? [:]
    let name = v["name"] ?? ""
    switch line.system {
    case "joined":
        if let q = v["quality"], !q.isEmpty {
            return tr("party.chat.joined %@ %@ %@", name, partyDevice(v["device"]), partyQuality(q))
        }
        return tr("party.chat.joinedPlain %@", name)
    case "left": return tr("party.chat.left %@", name)
    case "removed": return tr("party.chat.removed %@", name)
    case "host": return tr("party.chat.host %@", name)
    default: return name
    }
}

/// "4K", "1080p", "Original".
func partyQuality(_ q: String) -> String {
    q == "original" ? tr("player.quality.original") : (q == "2160p" ? "4K" : q)
}
