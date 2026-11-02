import SwiftUI

/// The app once signed in: Home, Movies and Shows in the Liquid Glass tab
/// bar with Search apart (IPh01), which floats at the top on iPad (IPa01).
struct MainTabs: View {
    enum Section: Hashable { case home, movies, shows, search }

    @Environment(AppState.self) private var app
    @State private var section: Section = .home
    @State private var libraries: [Library] = []
    @State private var playing: PlayRequest?
    @State private var homePath = NavigationPath()
    /// A pairing code from a plaid://pair link.
    @State private var pairCode: PairCode?

    struct PairCode: Identifiable { let id: String }

    var body: some View {
        TabView(selection: $section) {
            Tab("nav.home", systemImage: "house", value: Section.home) {
                NavigationStack(path: $homePath) { HomeView().withItemDestinations() }
            }
            Tab("nav.movies", systemImage: "film", value: Section.movies) {
                NavigationStack { LibraryTab(libraries: libraries.filter { $0.type == "movies" }, kind: .movies).withItemDestinations() }
            }
            Tab("nav.shows", systemImage: "tv", value: Section.shows) {
                NavigationStack { LibraryTab(libraries: libraries.filter { $0.type == "shows" }, kind: .shows).withItemDestinations() }
            }
            Tab("search.title", systemImage: "magnifyingglass", value: Section.search) {
                NavigationStack { SearchView().withItemDestinations() }
            }
        }
        .tabViewStyle(.tabBarOnly)
        .tabBarMinimizeBehavior(.onScrollDown)
        .environment(\.play, PlayAction { playing = $0 })
        .environment(\.previewsActive, playing == nil)
        .fullScreenCover(item: $playing) { PlayerScreen(request: $0) }
        .sheet(item: $pairCode) { c in NavigationStack { PairTVView(code: c.id) } }
        .task {
            if let client = app.client, let libs: [Library] = try? await client.get("/libraries") { libraries = libs }
        }
        // plaid://item/<id> opens a title's page, plaid://play/<id> plays it,
        // plaid://pair?code=… pairs a TV.
        .onOpenURL { url in
            // plaid://pair?code=K7P4QX opens Pair a TV with the code typed in.
            if url.scheme == "plaid", url.host() == "pair",
               let code = pairingCode(in: url.absoluteString.replacingOccurrences(of: "plaid://pair", with: "/pair")) {
                pairCode = PairCode(id: code)
                return
            }
            guard url.scheme == "plaid", let id = url.pathComponents.dropFirst().first else { return }
            switch url.host() {
            case "item":
                section = .home
                homePath = NavigationPath()
                homePath.append(ItemRoute(id: id))
            case "play":
                Task {
                    guard let client = app.client, let d: ItemDetail = try? await client.get("/items/\(id)") else { return }
                    playing = PlayRequest(item: d.summary, startMs: d.summary.userState?.positionMs ?? 0)
                }
            default:
                break
            }
        }
    }
}

extension View {
    func withItemDestinations() -> some View {
        navigationDestination(for: ItemSummary.self) { ItemView(summary: $0) }
            .navigationDestination(for: ItemRoute.self) { ItemByIdView(route: $0) }
            .navigationDestination(for: PersonRoute.self) { PersonView(route: $0) }
            .navigationDestination(for: CollectionRoute.self) { CollectionView(route: $0) }
            .navigationDestination(for: CollectionsRoute.self) { CollectionsView(library: $0.library) }
            .navigationDestination(for: SeeAllRoute.self) { r in
                var q = GridQuery()
                q.sort = GridQuery.Sort(rawValue: r.seeAll.sort) ?? .added
                q.ascending = r.seeAll.order == "asc"
                q.genres = r.seeAll.genre.map { [$0] } ?? []
                return LibraryGrid(library: Library(id: r.seeAll.libraryId, name: r.title, type: ""),
                                   title: Text(verbatim: r.title), preset: q)
            }
            .navigationDestination(for: GenreRoute.self) { r in
                var q = GridQuery()
                q.genres = [r.genre.id]
                return LibraryGrid(library: Library(id: r.libraryId, name: r.genre.name, type: ""),
                                   title: Text(verbatim: r.genre.name), preset: q)
            }
    }
}

/// The current profile's face, top right of each page: opens Settings.
struct ProfileMenu: View {
    @Environment(AppState.self) private var app
    @State private var settings = false

    var body: some View {
        Button { settings = true } label: {
            if let p = app.me?.activeProfile {
                ProfileFace(profile: p, size: 36)
                    .overlay(Circle().strokeBorder(Color.white.opacity(0.2), lineWidth: 2))
            } else {
                Image(systemName: "person.crop.circle").font(.system(size: 30))
            }
        }
        .buttonStyle(.plain)
        .accessibilityLabel(Text("nav.settings"))
        .sheet(isPresented: $settings) { SettingsView() }
    }
}
