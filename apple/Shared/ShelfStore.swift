import Foundation

/// What the app shares with its Top Shelf extension: the "Continue
/// watching" titles, their images saved as files (the server's need the
/// session token, which the extension doesn't have).
nonisolated enum ShelfStore {
    static let group = "group.app.plaid.tv"

    struct Entry: Codable, Hashable {
        let id: String
        let title: String
        let subtitle: String?
        /// 0…1.
        let progress: Double?
        /// File name of the image in the group's shelf folder.
        let image: String?
    }

    static var folder: URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: group)?
            .appending(path: "shelf", directoryHint: .isDirectory)
    }

    static func save(_ entries: [Entry]) {
        guard let folder else { return }
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        if let data = try? JSONEncoder().encode(entries) {
            try? data.write(to: folder.appending(path: "entries.json"), options: .atomic)
        }
        // Images no entry uses any more go.
        let keep = Set(entries.compactMap(\.image) + ["entries.json"])
        for name in (try? FileManager.default.contentsOfDirectory(atPath: folder.path())) ?? [] where !keep.contains(name) {
            try? FileManager.default.removeItem(at: folder.appending(path: name))
        }
    }

    static func load() -> [Entry] {
        guard let folder, let data = try? Data(contentsOf: folder.appending(path: "entries.json")) else { return [] }
        return (try? JSONDecoder().decode([Entry].self, from: data)) ?? []
    }

    static func imageURL(_ name: String) -> URL? { folder?.appending(path: name) }
}
