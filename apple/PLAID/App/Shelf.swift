import Foundation
import TVServices

/// Hands "Continue watching" to the Top Shelf extension: titles and their
/// stills, saved in the app group, then tells tvOS the shelf changed.
enum Shelf {
    static func update(_ items: [ItemSummary], client: APIClient) async {
        var entries: [ShelfStore.Entry] = []
        let folder = ShelfStore.folder
        for item in items.prefix(10) {
            var image: String?
            if let art = item.still ?? item.backdrop, let folder {
                let name = "\(art.imageId).jpg"
                let file = folder.appending(path: name)
                if FileManager.default.fileExists(atPath: file.path()) {
                    image = name
                } else if let data = try? await client.data(for: client.imageURL(art, width: 780)) {
                    try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
                    if (try? data.write(to: file, options: .atomic)) != nil { image = name }
                }
            }
            let subtitle = item.kind == .episode ? [episodeCode(item), item.name.isEmpty ? nil : item.name].compactMap { $0 }.joined(separator: " · ") : nil
            entries.append(.init(id: item.id, title: item.show?.displayTitle ?? item.name, subtitle: subtitle,
                                 progress: item.progress, image: image))
        }
        ShelfStore.save(entries)
        TVTopShelfContentProvider.topShelfContentDidChange()
    }
}
