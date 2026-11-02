import SwiftUI

/// The app once signed in: Home, Movies, Shows and Upcoming (F010) in the
/// Liquid Glass tab bar with Search apart (IPh01), which floats at the top
/// on iPad (IPa01). The Plaid Party sheets and screen open from here.
struct MainTabs: View {
    enum Section: Hashable { case home, movies, shows, upcoming, search }

    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var party
    @Environment(PartyNav.self) private var partyNav
    @Environment(\.scenePhase) private var scenePhase
    /// Invitations to answer: the Upcoming tab's badge (R39).
    @State private var invitations = 0
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
            Tab("upcoming.title", systemImage: "calendar", value: Section.upcoming) {
                NavigationStack { UpcomingView().withItemDestinations() }
            }
            .badge(invitations)
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
        .modifier(PartyPresenter(nav: partyNav))
        .task {
            if let client = app.client, let libs: [Library] = try? await client.get("/libraries") { libraries = libs }
        }
        // The socket stays open while the app is in the foreground, and while
        // this device plays a party.
        .task { party.start(app.client) }
        .onChange(of: scenePhase) { _, phase in
            switch phase {
            case .active: party.start(app.client)
            case .background: if party.room?.role != .screen { party.stop() }
            default: break
            }
        }
        .task(id: party.changes) {
            guard let client = app.client, let all = try? await client.parties() else { return }
            let me = app.me?.user.id
            invitations = all.filter { $0.myAnswer == "pending" && $0.organizer.userId != me && !$0.isOpen }.count
        }
        .onChange(of: party.invitation) { _, id in
            guard let id else { return }
            party.invitation = nil
            partyNav.detail = .init(id: id)
        }
        .alert(Text("party.companion.title"), isPresented: Binding(get: { party.companionOffer != nil }, set: { if !$0 { party.companionOffer = nil } })) {
            Button("party.companion.join") {
                if let offer = party.companionOffer {
                    partyNav.live = PartyEntry(partyId: offer.partyId, role: .companion, screenName: offer.screen)
                }
                party.companionOffer = nil
            }
            Button("common.notNow", role: .cancel) { party.companionOffer = nil }
        } message: {
            Text("party.companion.offer \(party.companionOffer?.screen ?? "Apple TV")")
        }
        .onReceive(NotificationCenter.default.publisher(for: .openParty)) { n in
            if let id = n.object as? String { partyNav.detail = .init(id: id) }
        }
        // plaid://item/<id> opens a title's page, plaid://play/<id> plays it,
        // plaid://pair?code=… pairs a TV.
        .onOpenURL { url in
            // A party link (https://server/p/K7P4QX, plaid://party?code=…)
            // opens Join a party with its code (F010-R2).
            if let code = partyCode(in: url) {
                partyNav.joinCode = .init(code: code)
                return
            }
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

/// The Plaid Party sheets and the party screen, opened from anywhere
/// through PartyNav (F010).
private struct PartyPresenter: ViewModifier {
    @Bindable var nav: PartyNav

    func body(content: Content) -> some View {
        content
            .sheet(item: $nav.launch) { PartyLaunchSheet(target: $0) }
            .sheet(item: $nav.joinCode) { JoinCodeSheet(request: $0) }
            .sheet(item: $nav.detail) { PartyDetailSheet(ref: $0) }
            .sheet(item: $nav.vote) { VoteView(ref: $0) }
            .fullScreenCover(item: $nav.live) { PartyScreen(entry: $0) }
    }
}

extension Notification.Name {
    /// A party reminder was tapped: its id is the object.
    static let openParty = Notification.Name("plaid.openParty")
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
