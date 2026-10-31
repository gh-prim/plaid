import TVServices

/// PLAID's Top Shelf: "Continue watching", as the app last saw it.
/// Selecting a title opens its page; Play starts it.
final class ContentProvider: TVTopShelfContentProvider {
    override func loadTopShelfContent() async -> (any TVTopShelfContent)? {
        let entries = ShelfStore.load()
        guard !entries.isEmpty else { return nil }
        let items = entries.map { e -> TVTopShelfSectionedItem in
            let item = TVTopShelfSectionedItem(identifier: e.id)
            item.title = e.subtitle.map { "\(e.title) · \($0)" } ?? e.title
            item.imageShape = .hdtv
            if let name = e.image, let url = ShelfStore.imageURL(name) {
                item.setImageURL(url, for: [.screenScale1x, .screenScale2x])
            }
            if let p = e.progress { item.playbackProgress = p }
            if let open = URL(string: "plaid://item/\(e.id)") { item.displayAction = TVTopShelfAction(url: open) }
            if let play = URL(string: "plaid://play/\(e.id)") { item.playAction = TVTopShelfAction(url: play) }
            return item
        }
        let section = TVTopShelfItemCollection(items: items)
        section.title = NSLocalizedString("shelf.continueWatching", comment: "")
        return TVTopShelfSectionedContent(sections: [section])
    }
}
