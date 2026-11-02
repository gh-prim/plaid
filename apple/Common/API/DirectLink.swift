import Foundation

/// getplaid.direct (F012): the six-character code of a server's short link
/// (k3x7qa in getplaid.direct/k3x7qa) gives the addresses where it answers,
/// so a TV away from home can be pointed at it without typing a long URL.
enum DirectLink {
    static let service = URL(string: "https://getplaid.direct")!
    private static let zone = ".getplaid.direct"

    enum Failure: Error { case unknownCode }

    private struct Connections: Decodable {
        struct Connection: Decodable {
            let url: URL
            let local: Bool
        }
        let connections: [Connection]
    }

    /// The server code in what was typed: the code alone, a short link
    /// (getplaid.direct/k3x7qa, or with its label, …/living-room-k3x7qa) or a
    /// getplaid.direct hostname. Nil when it can't be one.
    static func code(in input: String) -> String? {
        var s = input.trimmingCharacters(in: .whitespaces).lowercased()
        if let c = URLComponents(string: s.contains("://") ? s : "https://" + s), let host = c.host {
            if host == "getplaid.direct" {
                s = c.path.split(separator: "/").first.map(String.init) ?? ""
            } else if host.hasSuffix(zone) {
                // 192-168-1-20.living-room-k3x7qa.getplaid.direct: the label is last.
                s = host.dropLast(zone.count).split(separator: ".").last.map(String.init) ?? ""
            }
        }
        // A label ends with the code: living-room-k3x7qa.
        if let dash = s.lastIndex(of: "-") { s = String(s[s.index(after: dash)...]) }
        return s.count == 6 && s.allSatisfy({ "abcdefghijklmnopqrstuvwxyz234567".contains($0) }) ? s : nil
    }

    /// Where the server with this code can be reached, its local address first.
    static func addresses(for code: String) async throws -> [URL] {
        let (data, response) = try await URLSession.shared.data(from: service.appending(path: "api/v1/servers/\(code)/connections"))
        switch (response as? HTTPURLResponse)?.statusCode {
        case 200: break
        case 404: throw Failure.unknownCode
        default: throw APIError.network
        }
        let found = try JSONDecoder().decode(Connections.self, from: data).connections
        return found.filter(\.local).map(\.url) + found.filter { !$0.local }.map(\.url)
    }
}
