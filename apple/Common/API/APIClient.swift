import Foundation

/// A failed API call: `code` is the server's stable error key, or `network`.
struct APIError: Error, Equatable {
    let status: Int
    let code: String

    static let network = APIError(status: 0, code: "network")
}

/// Talks to one PLAID server over /api/v1, with the TV's bearer token.
final class APIClient {
    let baseURL: URL
    var token: String?

    private let session: URLSession
    private let decoder: JSONDecoder = {
        let d = JSONDecoder()
        let withFraction = ISO8601DateFormatter()
        withFraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let plain = ISO8601DateFormatter()
        d.dateDecodingStrategy = .custom { decoder in
            let s = try decoder.singleValueContainer().decode(String.self)
            if let date = withFraction.date(from: s) ?? plain.date(from: s) { return date }
            throw DecodingError.dataCorrupted(.init(codingPath: decoder.codingPath, debugDescription: s))
        }
        return d
    }()

    /// Dates go out as RFC 3339, as the API reads them (a party's time).
    private let encoder: JSONEncoder = {
        let e = JSONEncoder()
        e.dateEncodingStrategy = .iso8601
        return e
    }()

    init(baseURL: URL, token: String? = nil) {
        self.baseURL = baseURL
        self.token = token
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 15
        config.urlCache = URLCache(memoryCapacity: 64 << 20, diskCapacity: 512 << 20)
        session = URLSession(configuration: config)
    }

    /// An absolute URL for a path the server returned (e.g. a stream url).
    func absolute(_ path: String) -> URL {
        URL(string: path, relativeTo: baseURL)?.absoluteURL ?? baseURL
    }

    func imageURL(_ art: Artwork, width: Int) -> URL {
        var c = URLComponents(url: baseURL.appending(path: "api/v1/images/\(art.imageId)"), resolvingAgainstBaseURL: false)!
        c.queryItems = [URLQueryItem(name: "w", value: String(width))]
        return c.url!
    }

    /// A profile picture (F009) at `size` pixels: `avatar.url` is a path
    /// under api/v1 that changes with the picture.
    func avatarURL(_ path: String, size: Int) -> URL {
        var c = URLComponents(string: baseURL.appending(path: "api/v1").absoluteString + path)!
        c.queryItems = (c.queryItems ?? []) + [URLQueryItem(name: "size", value: String(min(512, max(32, size))))]
        return c.url!
    }

    func authorized(_ url: URL) -> URLRequest {
        var r = URLRequest(url: url)
        if let token { r.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization") }
        return r
    }

    // MARK: Requests

    func get<T: Decodable>(_ path: String, query: [URLQueryItem] = [], headers: [String: String] = [:]) async throws -> T {
        let (data, _) = try await send("GET", path, query: query, body: Optional<Int>.none, headers: headers)
        return try decode(data)
    }

    func post<T: Decodable, B: Encodable>(_ path: String, _ body: B) async throws -> T {
        let (data, _) = try await send("POST", path, body: body)
        return try decode(data)
    }

    func postNoContent<B: Encodable>(_ path: String, _ body: B) async throws {
        _ = try await send("POST", path, body: body)
    }

    func put<T: Decodable, B: Encodable>(_ path: String, _ body: B) async throws -> T {
        let (data, _) = try await send("PUT", path, body: body)
        return try decode(data)
    }

    func patch<T: Decodable, B: Encodable>(_ path: String, _ body: B) async throws -> T {
        let (data, _) = try await send("PATCH", path, body: body)
        return try decode(data)
    }

    /// Sends raw bytes (a photo) and decodes the answer.
    func upload<T: Decodable>(_ method: String, _ path: String, data body: Data, contentType: String) async throws -> T {
        var r = authorized(baseURL.appending(path: "api/v1" + path))
        r.httpMethod = method
        r.setValue("application/json", forHTTPHeaderField: "Accept")
        r.setValue(contentType, forHTTPHeaderField: "Content-Type")
        r.httpBody = body
        let data: Data, response: URLResponse
        do {
            (data, response) = try await session.data(for: r)
        } catch {
            throw APIError.network
        }
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        if status >= 400 {
            let code = (try? decoder.decode(APIErrorBody.self, from: data))?.error.code ?? "generic"
            throw APIError(status: status, code: code)
        }
        return try decode(data)
    }

    /// The server's drawing of avatar options, for the builder (F009).
    func avatarRenderURL(_ options: AvatarOptions, size: Int, color: Int? = nil) -> URL {
        var c = URLComponents(url: baseURL.appending(path: "api/v1/avatar-builder/render"), resolvingAgainstBaseURL: false)!
        let json = (try? JSONEncoder().encode(options)).flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
        c.queryItems = [URLQueryItem(name: "options", value: json), URLQueryItem(name: "size", value: String(size))]
        if let color { c.queryItems?.append(URLQueryItem(name: "color", value: String(color))) }
        // "+" stays literal in a query: escape it so the colors' JSON survives.
        c.percentEncodedQuery = c.percentEncodedQuery?.replacingOccurrences(of: "+", with: "%2B")
        return c.url!
    }

    func delete<B: Encodable>(_ path: String, _ body: B) async throws {
        _ = try await send("DELETE", path, body: body)
    }

    /// Raw call: returns the body and the status, throwing APIError on >= 400.
    @discardableResult
    func send<B: Encodable>(
        _ method: String, _ path: String, query: [URLQueryItem] = [], body: B?, headers: [String: String] = [:]
    ) async throws -> (Data, Int) {
        var c = URLComponents(url: baseURL.appending(path: "api/v1" + path), resolvingAgainstBaseURL: false)!
        if !query.isEmpty { c.queryItems = query }
        var r = authorized(c.url!)
        r.httpMethod = method
        r.setValue("application/json", forHTTPHeaderField: "Accept")
        for (k, v) in headers { r.setValue(v, forHTTPHeaderField: k) }
        if let body {
            r.httpBody = try encoder.encode(body)
            r.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        let data: Data, response: URLResponse
        do {
            (data, response) = try await session.data(for: r)
        } catch {
            throw APIError.network
        }
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        if status >= 400 {
            let code = (try? decoder.decode(APIErrorBody.self, from: data))?.error.code ?? "generic"
            throw APIError(status: status, code: code)
        }
        return (data, status)
    }

    func decode<T: Decodable>(_ data: Data) throws -> T {
        try decoder.decode(T.self, from: data)
    }

    func data(for url: URL) async throws -> Data {
        let (data, response) = try await session.data(for: authorized(url))
        guard (response as? HTTPURLResponse)?.statusCode == 200 else { throw APIError.network }
        return data
    }
}

// MARK: Plaid Party (F010)

extension APIClient {
    /// Parties not over that I organize or am invited to, soonest first.
    func parties() async throws -> [Party] { try await get("/parties") }

    func party(_ id: String) async throws -> Party { try await get("/parties/\(id)") }

    func createParty(_ body: PartyCreate) async throws -> Party { try await post("/parties", body) }

    func updateParty(_ id: String, _ body: PartyUpdate) async throws -> Party { try await patch("/parties/\(id)", body) }

    /// Cancels a scheduled party, or ends a live one for everyone.
    func cancelParty(_ id: String) async throws {
        try await send("DELETE", "/parties/\(id)", body: Optional<Int>.none)
    }

    func invite(_ userIds: [String], to partyId: String) async throws {
        try await send("POST", "/parties/\(partyId)/invites", body: ["userIds": userIds])
    }

    /// yes, maybe or no.
    func answerParty(_ id: String, _ answer: String) async throws {
        try await send("PUT", "/parties/\(id)/answer", body: ["answer": answer])
    }

    func partyPeople() async throws -> [PartyPerson] { try await get("/parties/people") }

    /// A code as typed: the separator and case don't matter.
    func partyPreview(code: String) async throws -> PartyPreview {
        try await get("/party-codes/\(code.uppercased().filter { $0.isLetter || $0.isNumber })")
    }

    func partyVote(_ id: String) async throws -> PartyVote { try await get("/parties/\(id)/vote") }

    func propose(_ itemId: String, in partyId: String, _ on: Bool) async throws -> PartyVote {
        let (data, _) = try await send(on ? "PUT" : "DELETE", "/parties/\(partyId)/vote/proposals/\(itemId)", body: Optional<Int>.none)
        return try decode(data)
    }

    func castBallot(_ ballot: Ballot, in partyId: String) async throws -> PartyVote {
        try await put("/parties/\(partyId)/vote/ballot", ballot)
    }

    func voteSuggestions(_ partyId: String) async throws -> [PartyItem] { try await get("/parties/\(partyId)/vote/suggestions") }

    /// The Upcoming tab between two days (kind: all, episodes or parties).
    func upcoming(from: Date, to: Date, kind: String = "all") async throws -> Upcoming {
        let f = DateFormatter()
        f.calendar = Calendar(identifier: .gregorian)
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyy-MM-dd"
        return try await get("/upcoming", query: [
            .init(name: "from", value: f.string(from: from)), .init(name: "to", value: f.string(from: to)),
            .init(name: "kind", value: kind),
        ])
    }

    func showFollow(_ showId: String) async throws -> ShowFollow { try await get("/shows/\(showId)/follow") }

    func setShowFollow(_ showId: String, _ on: Bool) async throws -> ShowFollow {
        let (data, _) = try await send(on ? "PUT" : "DELETE", "/shows/\(showId)/follow", body: Optional<Int>.none)
        return try decode(data)
    }

    /// Notify me when this episode is on the server (R38).
    func setReleaseAlert(_ e: UpcomingEpisode, _ on: Bool) async throws {
        try await send(on ? "PUT" : "DELETE", "/shows/\(e.showId)/alerts/\(e.season)/\(e.episode)", body: Optional<Int>.none)
    }

    /// The realtime socket's address: /api/v1/ws on the same server.
    var socketURL: URL {
        var c = URLComponents(url: baseURL.appending(path: "api/v1/ws"), resolvingAgainstBaseURL: false)!
        c.scheme = c.scheme == "https" ? "wss" : "ws"
        return c.url!
    }
}
