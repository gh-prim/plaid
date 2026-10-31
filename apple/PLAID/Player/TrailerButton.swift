import SwiftUI

/// Opens a title's trailer in the YouTube app. TMDB's trailers are YouTube
/// videos, which YouTube only lets its own player play.
struct TrailerButton: View {
    let trailer: Trailer
    @Environment(\.openURL) private var openURL
    @State private var noYouTube = false

    var body: some View {
        Button {
            guard let url = URL(string: "youtube://www.youtube.com/watch?v=\(trailer.key)") else { return }
            openURL(url) { opened in if !opened { noYouTube = true } }
        } label: {
            Label("item.trailer", systemImage: "play.rectangle")
        }
        .buttonStyle(PillButtonStyle(primary: false))
        .alert(Text("item.trailer.noYouTube"), isPresented: $noYouTube) {
            Button("common.ok") {}
        } message: {
            Text("item.trailer.noYouTubeHint")
        }
    }
}
