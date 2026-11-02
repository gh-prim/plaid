import Foundation

// Hand-written mirrors of the api/openapi.yaml schemas the Apple apps use.
// Only the fields the app reads are declared; unknown fields are ignored.

struct ServerInfo: Decodable {
    let name: String
    let version: String
    let setupComplete: Bool
    let attributions: [Attribution]?
}

/// A metadata provider whose notice the About page must show (F005 R37).
struct Attribution: Decodable, Hashable {
    let provider: String
    let name: String
    let url: String
    let notice: String
}

struct Device: Encodable {
    let name: String
    let type: String
    let clientVersion: String?
}

struct LoginRequest: Encodable {
    let username: String
    let password: String
    let device: Device
}

struct StartPairingRequest: Encodable {
    let device: Device
}

struct PairingStarted: Decodable {
    let id: String
    let code: String
    let pollSecret: String
    let expiresAt: Date
}

struct Profile: Decodable, Identifiable, Hashable {
    let id: String
    let name: String
    let avatarColor: Int
    /// What the profile shows (F009): its initial, a built avatar or a photo.
    let avatar: ProfileAvatar?
    let hasPin: Bool
    let locale: String?
    let isDefault: Bool
    /// The backdrop of what it watched last, to light the profile picker.
    let ambient: Artwork?
}

/// A profile's picture (F009). `url` is an API path, absent for the initial.
struct ProfileAvatar: Decodable, Hashable {
    let kind: String
    let url: String?
    let options: AvatarOptions?
}

/// The builder's choices (F009): a variant per part, nil for an optional
/// part left out.
struct AvatarOptions: Codable, Hashable {
    var head: String
    var hair: String
    var eyes: String
    var eyebrows: String
    var mouth: String
    var nose: String
    var glasses: String?
    var beard: String?
    var earrings: String?
    var freckles: String?
    var hairColor: String
    var skinColor: String

    subscript(part: String) -> String? {
        get {
            switch part {
            case "head": head
            case "hair": hair
            case "eyes": eyes
            case "eyebrows": eyebrows
            case "mouth": mouth
            case "nose": nose
            case "glasses": glasses
            case "beard": beard
            case "earrings": earrings
            case "freckles": freckles
            default: nil
            }
        }
        set {
            switch part {
            case "head": head = newValue ?? head
            case "hair": hair = newValue ?? hair
            case "eyes": eyes = newValue ?? eyes
            case "eyebrows": eyebrows = newValue ?? eyebrows
            case "mouth": mouth = newValue ?? mouth
            case "nose": nose = newValue ?? nose
            case "glasses": glasses = newValue
            case "beard": beard = newValue
            case "earrings": earrings = newValue
            case "freckles": freckles = newValue
            default: break
            }
        }
    }
}

/// The parts and colors of the avatar builder (GET /avatar-builder).
struct AvatarBuilderInfo: Decodable {
    struct Part: Decodable, Hashable {
        let name: String
        let optional: Bool
        let variants: [String]
    }
    let parts: [Part]
    let hairColors: [String]
    let skinColors: [String]
    let defaults: AvatarOptions
}

struct AvatarUpdate: Encodable {
    let kind: String
    let options: AvatarOptions?
}

struct ProfileColorUpdate: Encodable {
    let avatarColor: Int
}

/// What an invitation is for (POST /invite/check).
struct InvitationPreview: Decodable {
    let serverName: String
    let name: String
    let invitedBy: String
    let expiresAt: Date
}

struct InvitationToken: Encodable {
    let token: String
}

struct AcceptInvitationRequest: Encodable {
    let token: String
    let displayName: String
    let username: String
    let password: String
    let pin: String
    let locale: String?
    let device: Device
}

struct User: Decodable {
    let id: String
    let username: String?
    let displayName: String?
    let locale: String?
}

struct AuthResult: Decodable {
    let token: String?
    let expiresAt: Date
    let user: User
    let profiles: [Profile]
    let activeProfile: Profile?
}

struct Me: Decodable {
    let user: User
    let profiles: [Profile]
    let activeProfile: Profile?
}

struct SelectProfileRequest: Encodable {
    let profileId: String
    let pin: String?
}

struct AmbientColors: Decodable, Hashable {
    let dominant: String
    let vibrant: String
    let muted: String
    let dark: String
}

struct Artwork: Decodable, Hashable {
    let imageId: String
    let width: Int?
    let height: Int?
    let colors: AmbientColors?
}

struct UserState: Decodable, Hashable {
    let positionMs: Int64
    let durationMs: Int64?
    let watched: Bool
    let watchedEpisodes: Int?
    let totalEpisodes: Int?
}

struct ShowRef: Decodable, Hashable {
    let id: String
    let displayTitle: String
    let logo: Artwork?
}

enum ItemKind: String, Decodable {
    case movie, show, season, episode
}

struct ItemSummary: Decodable, Identifiable, Hashable {
    let id: String
    let kind: ItemKind
    let libraryId: String?
    let parentId: String?
    let displayTitle: String
    let title: String?
    let releaseDate: String?
    let rating: Double?
    let certification: String?
    let genres: [String]
    let poster: Artwork?
    let backdrop: Artwork?
    let still: Artwork?
    let year: Int?
    let seasonNumber: Int?
    let episodeNumber: Int?
    let childCount: Int
    let userState: UserState?
    let runtimeMinutes: Int?
    let overview: String?
    let show: ShowRef?

    /// What to show as the title, falling back to the parsed one.
    var name: String { displayTitle.isEmpty ? (title ?? "") : displayTitle }

    /// 0…1 when there is a resume point.
    var progress: Double? {
        guard let s = userState, s.positionMs > 0, let d = s.durationMs, d > 0 else { return nil }
        return min(1, Double(s.positionMs) / Double(d))
    }
}

struct GenreRef: Decodable, Hashable {
    let id: Int
    let name: String
}

struct Credit: Decodable, Hashable {
    let personId: String
    let name: String
    let role: String
    let character: String?
    let portrait: Artwork?
}

struct ItemMetadata: Decodable, Hashable {
    let overview: String?
    let tagline: String?
    let runtimeMinutes: Int?
    let credits: [Credit]
    let logo: Artwork?
    let backdrops: [Artwork]
    let trailers: [Trailer]
    let genres: [GenreRef]
    let studios: [String]
    let originalTitle: String?
    let endDate: String?
    let showStatus: String?
    let collection: CollectionRef?
    let ratingCount: Int?
    /// Rotten Tomatoes' Tomatometer and Popcornmeter, out of 100.
    let criticScore: Int?
    let audienceScore: Int?
}

struct CollectionRef: Decodable, Hashable {
    let id: String
    let name: String
}

struct CollectionSummary: Decodable, Identifiable, Hashable {
    let id: String
    let name: String
    let overview: String?
    let poster: Artwork?
    let backdrop: Artwork?
    let movieCount: Int
}

struct CollectionDetail: Decodable {
    let summary: CollectionSummary
    let items: [ItemSummary]

    init(from decoder: Decoder) throws {
        summary = try CollectionSummary(from: decoder)
        items = try decoder.container(keyedBy: Keys.self).decode([ItemSummary].self, forKey: .items)
    }

    private enum Keys: String, CodingKey { case items }
}

struct CollectionList: Decodable {
    let collections: [CollectionSummary]
}

struct Person: Decodable {
    let id: String
    let name: String
    let portrait: Artwork?
    let credits: [PersonCredit]
}

struct PersonCredit: Decodable {
    let item: ItemSummary
    let role: String
    let character: String?
}

struct MediaStream: Decodable, Hashable {
    let index: Int
    let type: String
    let codec: String?
    let language: String
    let title: String?
    let channels: Int?
    let forced: Bool
    let sdh: Bool
}

struct MediaFile: Decodable, Identifiable, Hashable {
    let id: String
    let size: Int64
    let edition: String?
    let container: String?
    let durationMs: Int64?
    let width: Int?
    let height: Int?
    let videoCodec: String?
    let hdrFormat: String?
    let missingSince: String?
    let streams: [MediaStream]
}

struct Trailer: Decodable, Hashable {
    let site: String
    let key: String
    let name: String?
    let language: String
    let type: String
    let official: Bool
}

struct ItemDetail: Decodable {
    let summary: ItemSummary
    let metadata: ItemMetadata?
    let files: [MediaFile]

    init(from decoder: Decoder) throws {
        summary = try ItemSummary(from: decoder)
        let c = try decoder.container(keyedBy: Keys.self)
        metadata = try c.decodeIfPresent(ItemMetadata.self, forKey: .metadata)
        files = (try? c.decodeIfPresent([MediaFile].self, forKey: .files)) ?? []
    }

    private enum Keys: String, CodingKey { case metadata, files }
}

enum HomeRowType: String, Decodable {
    case continueWatching = "continue_watching"
    case recentlyAdded = "recently_added"
    case genre
    case unknown

    init(from decoder: Decoder) throws {
        self = HomeRowType(rawValue: try decoder.singleValueContainer().decode(String.self)) ?? .unknown
    }
}

struct HomeRow: Decodable, Identifiable {
    let id: String
    let type: HomeRowType
    let libraryId: String?
    let libraryName: String?
    let genre: GenreRef?
    let items: [ItemSummary]
    let nextCursor: String?
    let seeAll: SeeAll?
}

/// The library grid query a home row's "See all" opens.
struct SeeAll: Decodable, Hashable {
    let libraryId: String
    let sort: String
    let order: String
    let genre: Int?
}

struct Home: Decodable {
    let ambient: ItemSummary?
    let rows: [HomeRow]
}

struct ItemPage: Decodable {
    let items: [ItemSummary]
    let nextCursor: String?
    let total: Int?
}

/// What a library's grid can be filtered by, with counts (F008 R27).
struct LibraryFacets: Decodable {
    struct Genre: Decodable, Hashable { let id: Int; let name: String; let count: Int }
    struct Decade: Decodable, Hashable { let from: Int; let count: Int }
    struct Resolution: Decodable, Hashable { let value: String; let count: Int }
    struct Letter: Decodable, Hashable { let letter: String; let count: Int }

    let total: Int
    let genres: [Genre]
    let decades: [Decade]
    let resolutions: [Resolution]
    let hdr: Int
    let letters: [Letter]
}

struct Library: Decodable, Identifiable, Hashable {
    let id: String
    let name: String
    let type: String
    /// Absent on the routes built in the app (a row's "See all").
    var scan: ScanProgress? = nil
}

/// A library's scan: idle, queued, scanning or analyzing, with its counters.
struct ScanProgress: Decodable, Hashable {
    struct Counters: Decodable, Hashable {
        let found: Int
        let analyzed: Int
    }

    let state: String
    let currentFolder: String?
    let counters: Counters
}

// MARK: Account

struct ClaimPairingRequest: Encodable {
    let code: String
    let profileId: String
    let pin: String?
}

struct SessionInfo: Decodable {
    let id: String
    let deviceName: String
}

struct ChangePasswordRequest: Encodable {
    let currentPassword: String
    let newPassword: String
}

struct SearchResults: Decodable {
    let movies: [ItemSummary]
    let shows: [ItemSummary]
    let episodes: [ItemSummary]
    let people: [PersonRef]
}

struct PersonRef: Decodable, Identifiable, Hashable {
    let id: String
    let name: String
    let portrait: Artwork?
}

struct ItemSegments: Decodable {
    let segments: [Segment]
}

/// A part of a title: intro, recap, credits or preview (TheIntroDB).
struct Segment: Decodable {
    let kind: String
    let startMs: Int64?
    let endMs: Int64?

    func contains(_ ms: Int64) -> Bool {
        ms >= (startMs ?? 0) && ms < (endMs ?? .max)
    }
}

struct UpNext: Decodable {
    let item: ItemSummary
}

// MARK: Playback

struct VideoCapability: Encodable {
    let codec: String
    var profiles: [String]?
    var maxLevel: Int?
    var maxWidth: Int?
    var maxHeight: Int?
    var maxBitDepth: Int?
    var hdr: [String]?
}

struct AudioCapability: Encodable {
    let codec: String
    let maxChannels: Int
}

struct SubtitleCapability: Encodable {
    let text: [String]
    /// Image formats the app draws itself from their own stream (.sup):
    /// never burned in, so the picture stays the original.
    var separate: [String]?
}

struct CapabilityProfile: Encodable {
    let protocols: [String]
    let containers: [String]
    let video: [VideoCapability]
    let audio: [AudioCapability]
    let subtitles: SubtitleCapability
    let canSwitchAudioTrack: Bool
}

struct PlaybackRequest: Encodable {
    let profile: CapabilityProfile
    let startPositionMs: Int64
    var fileId: String?
    var quality: String?
}

struct PlaybackSession: Decodable {
    let id: String
    let itemId: String
    let method: String
    let url: String
    let durationMs: Int64
    let startPositionMs: Int64
    let audioTracks: [PlaybackAudioTrack]
    let quality: String
    let qualities: [String]
    let subtitles: [PlaybackSubtitle]
}

struct PlaybackSubtitle: Decodable, Hashable {
    let index: Int
    let codec: String
    let language: String
    let title: String?
    let forced: Bool
    let selected: Bool
    let delivery: String
    /// Where an external track is fetched: a .sup for PGS drawn by the app.
    let url: String?

    /// A PGS track the app draws over the picture.
    var drawnByApp: Bool { delivery == "external" && url?.hasSuffix(".sup") == true }
}

struct PlaybackAudioTrack: Decodable, Hashable {
    let index: Int
    let codec: String
    let channels: Int
    let language: String
    let title: String?
    let selected: Bool
}

/// PATCH /playback/{id}: only what is set is sent; `subtitlesOff` sends
/// subtitleStreamIndex: null.
struct PlaybackUpdate: Encodable {
    let positionMs: Int64
    var audioStreamIndex: Int?
    var quality: String?
    var subtitleStreamIndex: Int?
    var subtitlesOff = false
    var burnSubtitles: Bool?

    private enum Keys: String, CodingKey { case positionMs, audioStreamIndex, quality, subtitleStreamIndex, burnSubtitles }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: Keys.self)
        try c.encode(positionMs, forKey: .positionMs)
        try c.encodeIfPresent(audioStreamIndex, forKey: .audioStreamIndex)
        try c.encodeIfPresent(quality, forKey: .quality)
        try c.encodeIfPresent(burnSubtitles, forKey: .burnSubtitles)
        if subtitlesOff { try c.encodeNil(forKey: .subtitleStreamIndex) } else { try c.encodeIfPresent(subtitleStreamIndex, forKey: .subtitleStreamIndex) }
    }
}

struct PlaybackProgress: Encodable {
    let positionMs: Int64
    let state: String
}

struct PlaybackStop: Encodable {
    let positionMs: Int64
}

struct APIErrorBody: Decodable {
    struct Inner: Decodable { let code: String; let message: String? }
    let error: Inner
}
