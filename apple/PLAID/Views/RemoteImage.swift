import SwiftUI
import UIKit

/// Images need the bearer token, so AsyncImage won't do: this loads them
/// through the API client and keeps them in memory (ids are immutable).
final class ImageCache {
    static let shared = ImageCache()
    private let cache = NSCache<NSURL, UIImage>()

    init() { cache.totalCostLimit = 200 << 20 }

    func image(for url: URL) -> UIImage? { cache.object(forKey: url as NSURL) }

    func store(_ image: UIImage, for url: URL) {
        cache.setObject(image, forKey: url as NSURL, cost: Int(image.size.width * image.size.height * 4))
    }
}

struct RemoteImage: View {
    let art: Artwork?
    let width: Int
    var contentMode: ContentMode = .fill

    @Environment(AppState.self) private var app
    @State private var image: UIImage?
    @State private var failed = false

    var body: some View {
        Color.clear.overlay {
            content
        }
        .clipped()
        .task(id: art?.imageId) { await load() }
    }

    @ViewBuilder
    private var content: some View {
        ZStack {
            if let image {
                Image(uiImage: image)
                    .resizable()
                    .aspectRatio(contentMode: contentMode)
                    .transition(.opacity)
            } else if art != nil && !failed {
                Shimmer()
            }
        }
    }

    private func load() async {
        guard let art, let client = app.client else { return }
        let url = client.imageURL(art, width: width)
        if let hit = ImageCache.shared.image(for: url) { image = hit; return }
        image = nil
        do {
            let data = try await client.data(for: url)
            guard let img = UIImage(data: data) else { failed = true; return }
            ImageCache.shared.store(img, for: url)
            withAnimation(.easeOut(duration: 0.2)) { image = img }
        } catch {
            failed = true
        }
    }
}
