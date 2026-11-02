import Foundation

// Plaid Party (F010): hand-written mirrors of the party schemas of
// api/openapi.yaml, then the WebSocket payloads of api/events.md. Only the
// fields the apps read are declared; unknown fields are ignored.

// MARK: REST

/// A party's title. Episodes carry their show and numbers.
struct PartyItem: Decodable, Hashable {
    let id: String
    let kind: String
    let title: String
    let year: Int?
    let showId: String?
    let showTitle: String?
    let season: Int?
    let episode: Int?
    let runtimeMs: Int64?

    /// "Nosferatu", or "Caminandes · S1 E3 · Llamigos".
    var label: String {
        guard kind == "episode", let s = season, let e = episode else { return title }
        return [showTitle, tr("card.episodeShort %lld %lld", s, e), title].compactMap { $0 }.joined(separator: " · ")
    }
}

/// Someone who can be invited: display name and picture only.
struct PartyPerson: Decodable, Hashable, Identifiable {
    let userId: String
    let name: String
    let avatarColor: Int
    let avatar: ProfileAvatar?

    var id: String { userId }
}

struct PartyInvite: Decodable, Hashable {
    let person: PartyPerson
    /// pending, yes, maybe or no.
    let answer: String
}

/// The end screen (R21).
struct PartySummary: Decodable, Hashable {
    let people: [String]
    let togetherMs: Int64
    let reactions: Int
    let pauseRequests: Int
    let peakPositionMs: Int64?
    let peakEmoji: String?
}

struct PartyLive: Decodable, Hashable {
    let count: Int
    let max: Int
    let state: String
    let positionMs: Int64
}

struct PartyVoteState: Decodable, Hashable {
    let proposalsUntil: Date
    let rankingUntil: Date
    let closed: Bool
}

struct FutureEpisode: Decodable, Hashable {
    let showId: String
    let season: Int
    let episode: Int
}

struct Party: Decodable, Identifiable, Hashable {
    enum Status: String, Decodable {
        case scheduled, voting, lobby, live, ended, cancelled, unknown

        init(from decoder: Decoder) throws {
            self = Status(rawValue: try decoder.singleValueContainer().decode(String.self)) ?? .unknown
        }
    }

    let id: String
    let code: String
    let link: String
    let status: Status
    let item: PartyItem?
    let futureEpisode: FutureEpisode?
    let organizer: PartyPerson
    let controlMode: String
    let scheduledAt: Date?
    let note: String?
    let createdAt: Date
    let startedAt: Date?
    let invites: [PartyInvite]
    let myAnswer: String?
    let live: PartyLive?
    let vote: PartyVoteState?
    let summary: PartySummary?

    /// Open now: people can join it.
    var isOpen: Bool { status == .live || status == .lobby }
}

struct PartyCreate: Encodable {
    struct Vote: Encodable {
        let proposalsUntil: Date
        let rankingUntil: Date
    }

    var itemId: String?
    var controlMode = "host"
    var scheduledAt: Date?
    var note: String?
    var invite: [String] = []
    var vote: Vote?
    var showId: String?
    var season: Int?
    var episode: Int?
}

struct PartyUpdate: Encodable {
    var scheduledAt: Date?
    var note: String?
    var itemId: String?
    var controlMode: String?
}

/// What a code leads to (GET /party-codes/{code}).
struct PartyPreview: Decodable {
    let code: String
    let status: String
    let hostName: String
    let item: PartyItem?
    let startedAt: Date?
    let scheduledAt: Date?
    let watching: Int
    let full: Bool
    let guests: Bool
}

struct PartyVote: Decodable {
    struct Proposal: Decodable, Hashable {
        let item: PartyItem
        let proposedBy: String
        let mine: Bool
    }

    struct Voter: Decodable, Hashable {
        let name: String
        let voted: Bool
    }

    struct Score: Decodable, Hashable {
        let itemId: String
        let points: Int
        let vetoed: Bool
    }

    struct Result: Decodable {
        let scores: [Score]
        let tied: [String]?
        let winner: String?
        let seed: String?
    }

    let proposalsUntil: Date
    let rankingUntil: Date
    /// proposals, ranking or closed.
    let phase: String
    let proposals: [Proposal]
    let voters: [Voter]
    let myRanking: [String]
    let myVeto: String?
    let result: Result?

    func item(_ id: String) -> PartyItem? { proposals.first { $0.item.id == id }?.item }
}

struct Ballot: Encodable {
    let ranking: [String]
    let veto: String?

    private enum Keys: String, CodingKey { case ranking, veto }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: Keys.self)
        try c.encode(ranking, forKey: .ranking)
        // null clears a veto given before.
        if let veto { try c.encode(veto, forKey: .veto) } else { try c.encodeNil(forKey: .veto) }
    }
}

struct ShowFollow: Decodable {
    let following: Bool
    let explicit: Bool
}

struct UpcomingEpisode: Decodable, Hashable, Identifiable {
    let showId: String
    let showTitle: String
    let season: Int
    let episode: Int
    let title: String?
    /// The date only: TMDB gives no time.
    let airDate: String
    /// released_on_server, released_pending_file or upcoming.
    let status: String
    let itemId: String?
    let alert: Bool

    var id: String { "\(showId).\(season).\(episode)" }
    var day: Date? { parseDay(airDate) }
}

struct Upcoming: Decodable {
    let invitations: [Party]
    let episodes: [UpcomingEpisode]
    let parties: [Party]
}

/// One of the account's Apple TVs with the app open (party.screens).
struct PartyScreenOption: Decodable, Hashable, Identifiable {
    let sessionId: String
    let name: String
    let type: String

    var id: String { sessionId }
}

// MARK: WebSocket payloads (api/events.md)

/// The room's reference state (R8): while playing, the position at server
/// time t is positionMs + (t − atServerMs) × rate.
struct RoomPlayback: Decodable, Equatable {
    let state: String
    let positionMs: Int64
    let atServerMs: Int64
    let rate: Double
    /// Who did what, on party.playback: play, pause, seek, wait, resume, item.
    var action: String?
    var by: String?
    var byName: String?

    func position(atServerMs now: Int64) -> Int64 {
        guard state == "playing" else { return positionMs }
        return positionMs + Int64(Double(now - atServerMs) * rate)
    }

    static let idle = RoomPlayback(state: "paused", positionMs: 0, atServerMs: 0, rate: 1)
}

/// One person in party.presence (R20).
struct RoomPerson: Decodable, Identifiable, Hashable {
    let id: String
    let name: String
    let avatarColor: Int
    let avatarUrl: String?
    let host: Bool
    let guest: Bool
    let canControl: Bool
    /// here, loading, waiting or away.
    let state: String
    /// The device type of the person's screen: tvos, ios, ipados, browser.
    let screen: String?
    let quality: String?
    let companion: Bool
}

struct RoomPresence: Decodable, Equatable {
    let people: [RoomPerson]
    let count: Int
    let max: Int
    let queue: Int

    static let empty = RoomPresence(people: [], count: 0, max: 0, queue: 0)
}

/// A line of the chat (R18): someone's text, or a system line (joined,
/// left, removed, host) the app phrases itself from its values.
struct ChatLine: Decodable, Identifiable, Hashable {
    let id: Int64
    let from: String?
    let name: String?
    let text: String?
    let positionMs: Int64
    let system: String?
    let values: [String: String]?
}

struct PauseAsk: Decodable, Identifiable, Hashable {
    let requestId: Int64
    let by: String
    let name: String

    var id: Int64 { requestId }
}

struct RoomSnapshot: Decodable {
    let partyId: String
    let itemId: String?
    let mode: String
    let host: String
    let me: String
    let playback: RoomPlayback
    let presence: RoomPresence
    let chat: [ChatLine]
    let pauseRequests: [PauseAsk]
    let reactions: [String]
    let pauseAtServerMs: Int64?
    let waitingFor: [String]?
}

struct RoomWaiting: Decodable, Equatable {
    let `for`: [String]
    let names: [String]
    let positionMs: Int64
    let resumeWithoutAtMs: Int64?
}

struct RoomReaction: Decodable {
    let from: String
    let name: String
    let emoji: String
}

/// The next episode together (R30), or `stopped` when the host said no.
struct RoomNext: Decodable, Equatable {
    let itemId: String?
    let title: String?
    let season: Int?
    let episode: Int?
    let runtimeMs: Int64?
    let startsAtServerMs: Int64?
    let ready: Int?
    let of: Int?
    let stopped: Bool?
}

/// The server can't convert for everyone (R26): who waits and the choices.
struct TranscodeLimit: Decodable, Equatable {
    struct Waiter: Decodable, Equatable, Identifiable {
        let id: String
        let name: String
        let quality: String?
    }

    let waiting: [Waiter]
    let sharedQuality: String?
    let options: [String]
}

struct TranscodeApply: Decodable {
    let quality: String?
    let waiting: [String]?
    let retry: Bool?
}

struct PartyNotice: Decodable {
    let kind: String
    let partyId: String?
    let status: String?
}

struct PartyCast: Decodable, Equatable {
    let partyId: String
    let screen: String?
}
