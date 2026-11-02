import Foundation
import Observation

/// The app's end of the realtime socket (F010): open while the app is in
/// the foreground and signed in, it carries the party this device is in,
/// notifications (R41), a TV asked to play (`tv.cast`, R22) and a TV's offer
/// to this phone to be its companion (`party.companion`, R23).
@Observable
final class PartyCenter {
    let socket = PartySocket()
    /// The party this device is in, if any.
    private(set) var room: PartyRoom?
    /// Apple TV: open the player in party mode for this party.
    var cast: PartyCast?
    /// iPhone and iPad: a TV of this account opened a party.
    var companionOffer: PartyCast?
    /// An invitation that arrived while the app was open.
    var invitation: String?
    /// A vote just closed: the TV reveals the winner (VoteBig).
    var voteClosed: String?
    /// Bumped on every notification: lists of parties and Upcoming reload.
    private(set) var changes = 0

    private weak var client: APIClient?
    private var screensAsk: UUID?
    private var screensFound: [PartyScreenOption]?

    init() {
        socket.onMessage = { [weak self] type, raw in self?.handle(type, raw) }
        socket.onOpen = { [weak self] in
            guard let self, let room = self.room, !room.ended, !room.removed else { return }
            Task { await room.join() }
        }
    }

    func start(_ client: APIClient?) {
        guard let client, client.token != nil else { stop(); return }
        self.client = client
        socket.start(url: client.socketURL, token: client.token)
    }

    func stop() {
        socket.stop()
    }

    /// Joins a party by id (invitees) or code (others), as its screen or as
    /// a companion; `tvSession` asks one of the account's TVs to play.
    @discardableResult
    func enter(partyId: String?, code: String? = nil, role: PartyRoom.Role, tvSession: String? = nil, screenName: String? = nil) -> PartyRoom? {
        guard let client else { return nil }
        if let room, room.partyId != nil, room.partyId == partyId, room.role == role, !room.ended { return room }
        room?.leave()
        let r = PartyRoom(partyId: partyId, code: code, role: role, tvSession: tvSession, socket: socket, client: client)
        r.screenName = screenName
        room = r
        Task { await r.join() }
        return r
    }

    /// Leaves the room (the seat goes after 2 min) and forgets it.
    func leave() {
        room?.leave()
        room = nil
    }

    private func handle(_ type: String, _ raw: Data) {
        switch type {
        case "notify":
            guard let n = socketData(raw, as: PartyNotice.self) else { return }
            changes += 1
            switch n.kind {
            case "party.invite": invitation = n.partyId
            case "party.vote.closed": voteClosed = n.partyId
            default: break
            }
        case "party.screens":
            struct List: Decodable { let screens: [PartyScreenOption] }
            screensFound = socketData(raw, as: List.self)?.screens ?? []
        case "tv.cast":
            cast = socketData(raw, as: PartyCast.self)
        case "party.companion":
            guard let c = socketData(raw, as: PartyCast.self), room?.partyId != c.partyId else { return }
            companionOffer = c
        default:
            if type.hasPrefix("party.") { room?.handle(type, raw) }
        }
    }

    /// The account's Apple TVs with the app open (R22); empty when none
    /// answers within 3 s.
    func screens() async -> [PartyScreenOption] {
        guard socket.state == .open else { return [] }
        let asked = UUID()
        screensAsk = asked
        screensFound = nil
        try? await socket.send("party.screens")
        for _ in 0..<30 where screensFound == nil && screensAsk == asked {
            try? await Task.sleep(for: .milliseconds(100))
        }
        return screensFound ?? []
    }
}
