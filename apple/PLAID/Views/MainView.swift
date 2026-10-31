import SwiftUI

/// What the player is asked to play, and from where.
struct PlayRequest: Identifiable, Hashable {
    let id = UUID()
    let item: ItemSummary
    let startMs: Int64
    /// The version picked on the item page; nil lets the server choose.
    var fileId: String?
}

/// Pages reached from an item: a person, a collection, a library by genre
/// or a library's collections.
struct PersonRoute: Hashable { let id: String; let name: String }
struct CollectionRoute: Hashable { let id: String; let name: String }
struct GenreRoute: Hashable { let libraryId: String; let genre: GenreRef }
struct CollectionsRoute: Hashable { let library: Library }

/// The sections of the side rail.
enum Section: Hashable {
    case home, library(Library), search, settings
}

/// The web's capsule nav, turned vertical on the left: icon chips for Home,
/// each library, Search and Settings, then the profile avatar.
struct MainView: View {
    @Environment(AppState.self) private var app
    @State private var libraries: [Library] = []
    @State private var section: Section = .home
    @State private var playing: PlayRequest?
    @FocusState private var railFocus: RailItem?
    /// The menu takes no focus until the first page has focused its main
    /// control, so the app never opens on the menu.
    @State private var railEnabled = false
    /// A page opened from a link is still loading: the pages under it
    /// don't give the menu focus, only that page does (linkReady).
    @State private var holdRail = false
    /// Bumped when the current section is picked again: the page goes back
    /// to its root and takes focus, which closes the menu.
    @State private var visit = 0
    /// Bumped to send focus from the menu to the page's main control.
    @State private var pageFocusTick = 0
    /// The pages pushed over the section's root (item pages).
    @State private var path = NavigationPath()

    var body: some View {
        ZStack(alignment: .leading) {
            content
                .id(PageID(section: section, visit: visit))
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .transition(.asymmetric(
                    insertion: .opacity.combined(with: .offset(x: 40)),
                    removal: .opacity))
                // Back on the remote at a page's root goes to the rail, as in
                // Apple's apps; pushed pages handle it themselves first.
                // Back closes a pushed page first; at a section's root it
                // goes to the menu, as in Apple's apps.
                .onExitCommand {
                    if path.isEmpty { railFocus = .section(section) } else { path.removeLast() }
                }
                .onChange(of: section) { path = NavigationPath() }
            // The page dims while the menu is open.
            Color.black.opacity(railFocus == nil ? 0 : 0.5)
                .ignoresSafeArea()
                .allowsHitTesting(false)
                .animation(.smooth(duration: 0.3), value: railFocus == nil)
            // The rail's focus area is the whole left column, so pressing
            // left from anywhere (the search keyboard at the top too) lands
            // on it.
            SideRail(libraries: libraries, section: $section, focus: $railFocus,
                     reselect: { path = NavigationPath(); visit += 1 },
                     focusPage: {
                         // The open panel covers the page's edge, so tvOS
                         // can't move right on its own: the page takes focus.
                         Task { try? await Task.sleep(for: .milliseconds(150)); pageFocusTick += 1 }
                     })
                .disabled(!railEnabled)
                .padding(.leading, 24)
                .padding(.vertical, 24)
                .frame(maxHeight: .infinity)
                .contentShape(Rectangle())
                .focusSection()
                .ignoresSafeArea()
        }
        .environment(\.pageReady, { if !holdRail { railEnabled = true } })
        .environment(\.linkReady, { holdRail = false; railEnabled = true; pageFocusTick += 1 })
        .environment(\.pageFocusTick, pageFocusTick)
        // From the Top Shelf: plaid://item/<id> opens a title, plaid://play/<id> plays it.
        .onOpenURL { url in
            guard url.scheme == "plaid", let id = url.pathComponents.dropFirst().first else { return }
            switch url.host() {
            case "item":
                if section != .home { section = .home }
                // The page has nothing to focus while it loads: keep focus off
                // the menu until it is ready, so the app doesn't open on it.
                railEnabled = false
                holdRail = true
                path = NavigationPath()
                path.append(ItemRoute(id: id))
            case "play":
                Task {
                    guard let client = app.client, let d: ItemDetail = try? await client.get("/items/\(id)") else { return }
                    playing = PlayRequest(item: d.summary, startMs: d.summary.userState?.positionMs ?? 0)
                }
            default:
                break
            }
        }
        .environment(\.play, PlayAction { playing = $0 })
        .environment(\.previewsActive, playing == nil)
        .fullScreenCover(item: $playing) { req in
            PlayerView(request: req)
        }
        .task {
            if let client = app.client, let libs: [Library] = try? await client.get("/libraries") {
                libraries = libs
            }
        }
        .task {
            // A page that never reports (offline, empty): the menu still opens.
            try? await Task.sleep(for: .seconds(6))
            railEnabled = true
        }
    }

    @ViewBuilder
    private var content: some View {
        switch section {
        // Home and libraries run edge to edge, their backdrops under the rail.
        case .home:
            NavigationStack(path: $path) { HomeView().withItemDestinations() }
                .ignoresSafeArea(edges: .horizontal)
        case let .library(lib):
            NavigationStack(path: $path) { LibraryView(library: lib).withItemDestinations() }
                .ignoresSafeArea(edges: .horizontal)
        // No backdrop runs under the rail here: the whole page starts after
        // it, so focus moves right from the rail into it.
        case .search:
            // Keeps tvOS's safe margins: the system keyboard and its
            // dictation hint lay out inside them.
            NavigationStack(path: $path) { SearchView().withItemDestinations() }
                // A safe margin, not a padding: the system search bar lays
                // its dictation hint out against the full width.
                .safeAreaPadding(.leading, Theme.rail - Theme.systemMargin)
                .focusSection()
                // The system keyboard holds focus at its left edge: left there
                // goes to the menu.
                .onMoveCommand { if $0 == .left { railFocus = .section(.search) } }
        case .settings:
            SettingsView()
                .focusSection()
                .padding(.leading, Theme.rail - Theme.systemMargin)
        }
    }
}

/// What the rail can focus: a section or the profile avatar.
enum RailItem: Hashable {
    case section(Section)
    case profile
}

/// Snappy spring with a small bounce, for the rail's moving highlight.
private let railSpring = Animation.spring(response: 0.38, dampingFraction: 0.68)

/// The side menu, as on Plex: folded, four icons (profile, search, home,
/// ☰); as soon as it has focus it opens into a glass panel with labels:
/// the profile, Search, Home, the libraries and Settings.
struct SideRail: View {
    let libraries: [Library]
    @Binding var section: Section
    var focus: FocusState<RailItem?>.Binding
    /// The section shown was picked again.
    var reselect: () -> Void = {}
    /// Moves focus to the page shown.
    var focusPage: () -> Void = {}
    @Environment(AppState.self) private var app
    /// Distance from the panel's top to the first entry.
    private static let top: CGFloat = 40
    /// The open panel: its width, how far it starts left of the rows, and
    /// the width of a row's highlight.
    private static let panelWidth: CGFloat = 420
    private static let panelInset: CGFloat = 20
    private static let rowWidth: CGFloat = 380

    private var open: Bool { focus.wrappedValue != nil }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            if let p = app.me?.activeProfile {
                row(.profile, action: { app.switchProfile() }) { on in
                    Avatar(profile: p, size: 44)
                } label: {
                    HStack {
                        Text(verbatim: p.name)
                        Spacer()
                        Image(systemName: "chevron.right").font(.system(size: 20, weight: .semibold))
                    }
                }
                .padding(.bottom, 8)
            }
            sectionRow(.search, icon: "magnifyingglass", Text("nav.search"))
            sectionRow(.home, icon: "house", Text("nav.home"))
            // Folded, the first of the rest shows as ☰ and the others are not
            // there. Nothing that can hold focus ever goes away while focused:
            // opening only adds rows.
            if open, !libraries.isEmpty { header(Text("nav.libraries")) }
            ForEach(Array(rest.enumerated()), id: \.element.0) { i, entry in
                let (item, icon, label) = entry
                if open || i == 0 {
                    if item == .settings, open { Spacer(minLength: 24) }
                    sectionRow(item, icon: open ? icon : "line.3.horizontal", label)
                        .padding(.bottom, item == .settings && open ? 12 : 0)
                }
            }
        }
        // Folded, the icons sit in the middle of the screen's height; open,
        // the panel runs the full height, profile on top, Settings at the bottom.
        // The rows keep a 64-point focus frame, open or folded: labels and
        // the panel are drawn over the page, so tvOS still sees the page to
        // the right of the menu and can move into it.
        .padding(.top, open ? Self.top : 0)
        .frame(width: 64, alignment: .leading)
        .frame(maxHeight: open ? .infinity : nil, alignment: .top)
        .frame(maxHeight: .infinity, alignment: open ? .top : .center)
        .background(alignment: .leading) {
            if open {
                RoundedRectangle(cornerRadius: 32).fill(Theme.bgBase.opacity(0.55)).railGlass(cornerRadius: 32)
                    .frame(width: Self.panelWidth)
                    .padding(.leading, -Self.panelInset)
                    .transition(.opacity)
            }
        }
        .animation(.spring(response: 0.42, dampingFraction: 0.82), value: open)
        .onMoveCommand { if $0 == .right { focusPage() } }
    }

    /// The libraries, then Settings.
    private var rest: [(Section, String, Text)] {
        libraries.map { (Section.library($0), $0.type == "shows" ? "tv" : "film", Text(verbatim: $0.name)) }
            + [(.settings, "gearshape", Text("nav.settings"))]
    }

    private func sectionRow(_ item: Section, icon: String, _ label: Text) -> some View {
        row(.section(item), current: section == item, action: {
            if section == item { reselect() } else { withAnimation(.smooth(duration: 0.4)) { section = item } }
            focusPage()
        }) { _ in
            Image(systemName: icon).font(.system(size: 26, weight: .semibold))
        } label: {
            label
        }
    }

    private func header(_ title: Text) -> some View {
        Color.clear
            .frame(width: 64, height: 44)
            .overlay(alignment: .leading) {
                HStack(spacing: 12) {
                    title.font(Theme.sans(20, .semibold)).foregroundStyle(Theme.textTertiary)
                    Rectangle().fill(Color.white.opacity(0.2)).frame(height: 1)
                }
                .padding(.leading, 4)
                .frame(width: Self.rowWidth, alignment: .leading)
                .fixedSize()
            }
            .padding(.top, 10)
            .transition(.opacity)
    }

    /// One entry: its icon always, its label when the panel is open. White
    /// when focused, outlined when it's the page shown.
    private func row<Icon: View, Label: View>(
        _ item: RailItem, current: Bool = false, action: @escaping () -> Void,
        @ViewBuilder icon: (Bool) -> Icon, @ViewBuilder label: () -> Label
    ) -> some View {
        let focused = focus.wrappedValue == item
        return Button(action: action) {
            Color.clear
                .frame(width: 64, height: 64)
                .overlay(alignment: .leading) {
                    HStack(spacing: 18) {
                        icon(focused).frame(width: 44, height: 44)
                        if open {
                            label()
                                .font(Theme.sans(28, .semibold))
                                .lineLimit(1)
                                .transition(.opacity.combined(with: .offset(x: -12)))
                        }
                    }
                    .foregroundStyle(focused ? Theme.onAction : Theme.textPrimary)
                    .padding(.horizontal, 10)
                    .frame(width: open ? Self.rowWidth : 64, height: 64, alignment: .leading)
                    .background {
                        Capsule().fill(focused ? Color.white : .clear)
                        Capsule().strokeBorder(Color.white.opacity(current && !focused && open ? 0.85 : 0), lineWidth: 3)
                    }
                    .shadow(color: .black.opacity(open ? 0 : 0.6), radius: 8)
                    .scaleEffect(focused ? 1.03 : 1, anchor: .leading)
                    .animation(railSpring, value: focused)
                    .fixedSize()
                }
        }
        .buttonStyle(RailButtonStyle())
        .focused(focus, equals: item)
    }
}

/// A profile's initial on its party colour.
struct Avatar: View {
    let profile: Profile
    var size: CGFloat = 46

    var body: some View {
        Text(verbatim: String(profile.name.prefix(1)).uppercased())
            .font(Theme.sans(size * 0.48, .semibold))
            .foregroundStyle(Theme.bgBase)
            .frame(width: size, height: size)
            .background(Circle().fill(Theme.party[profile.avatarColor % Theme.party.count]))
    }
}

/// No system focus effect: the rail draws its own.
private struct RailButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.92 : 1)
            .animation(railSpring, value: configuration.isPressed)
    }
}

struct PlayAction {
    let run: (PlayRequest) -> Void
    func callAsFunction(_ item: ItemSummary, from startMs: Int64 = 0, fileId: String? = nil) {
        run(PlayRequest(item: item, startMs: startMs, fileId: fileId))
    }
}

private struct PlayKey: EnvironmentKey {
    static let defaultValue = PlayAction { _ in }
}

extension EnvironmentValues {
    var play: PlayAction {
        get { self[PlayKey.self] }
        set { self[PlayKey.self] = newValue }
    }
}

extension View {
    func withItemDestinations() -> some View {
        navigationDestination(for: ItemSummary.self) { ItemView(summary: $0) }
            .navigationDestination(for: PersonRoute.self) { PersonView(route: $0) }
            .navigationDestination(for: CollectionRoute.self) { CollectionView(route: $0) }
            .navigationDestination(for: CollectionsRoute.self) { CollectionsView(library: $0.library) }
            .navigationDestination(for: ItemRoute.self) { ItemByIdView(route: $0) }
            .navigationDestination(for: SeeAllRoute.self) { r in
                LibraryView(library: Library(id: r.seeAll.libraryId, name: r.title, type: ""), preset: r.seeAll)
            }
            .navigationDestination(for: GenreRoute.self) {
                LibraryView(library: Library(id: $0.libraryId, name: $0.genre.name, type: ""), genre: $0.genre)
            }
    }
}

struct SettingsView: View {
    @Environment(AppState.self) private var app
    @Environment(\.pageFocusTick) private var pageFocusTick
    @FocusState private var firstFocused: Bool

    var body: some View {
        @Bindable var app = app
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 40) {
                Text("nav.settings").font(Theme.sans(56, .bold))
                VStack(alignment: .leading, spacing: 8) {
                    if let p = app.me?.activeProfile {
                        Text("settings.profile \(p.name)").font(Theme.sans(30))
                    }
                    if let name = app.serverName {
                        Text("settings.server \(name)").font(Theme.sans(26)).foregroundStyle(Theme.textSecondary)
                    }
                }
                HStack(spacing: 24) {
                    Button("settings.switchProfile") { app.switchProfile() }
                        .buttonStyle(PillButtonStyle(primary: false))
                        .focused($firstFocused)
                    Button("settings.signOut") { Task { await app.signOut() } }
                        .buttonStyle(PillButtonStyle(primary: false))
                    Button("pair.changeServer") { app.forgetServer() }
                        .buttonStyle(PillButtonStyle(primary: false))
                }
                .focusSection()

                Text("settings.playback").font(Theme.sans(34, .semibold)).padding(.top, 12)
                VStack(alignment: .leading, spacing: 20) {
                    if let p = app.me?.activeProfile {
                        SettingSwitch(title: "settings.autoplayNext", key: autoplayKey(p.id))
                    }
                    Toggle(isOn: $app.previewsOn) { Text("settings.previews") }
                        .toggleStyle(SwitchPill())
                }
                .focusSection()

                Text("settings.about").font(Theme.sans(34, .semibold)).padding(.top, 12)
                VStack(alignment: .leading, spacing: 16) {
                    let appVersion = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "?"
                    Text("settings.versions \(appVersion) \(app.server?.version ?? "?")")
                        .font(Theme.sans(26)).foregroundStyle(Theme.textSecondary)
                    ForEach(app.server?.attributions ?? [], id: \.provider) { a in
                        VStack(alignment: .leading, spacing: 4) {
                            Text(verbatim: a.name).font(Theme.sans(26, .semibold))
                            Text(verbatim: attributionText(a)).font(Theme.sans(24)).foregroundStyle(Theme.textSecondary)
                                .frame(maxWidth: 1300, alignment: .leading)
                            Text(verbatim: a.url).font(Theme.sans(22)).foregroundStyle(Theme.textTertiary)
                        }
                    }
                }
                // Text only: something focusable below lets the page scroll down to it.
                Button("settings.backToTop") {}
                    .buttonStyle(PillButtonStyle(primary: false))
                    .opacity(0.001)
            }
            .padding(Theme.gutter)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .onChange(of: pageFocusTick) { firstFocused = true }
    }

    /// The provider's notice in the app's language, else as the server sent it.
    private func attributionText(_ a: Attribution) -> String {
        let key = "about.attribution." + a.provider
        let text = localized(key)
        return text == key ? a.notice : text
    }
}

/// A switch drawn as a pill with its state, like the other settings.
private struct SwitchPill: ToggleStyle {
    func makeBody(configuration: Configuration) -> some View {
        Button { configuration.isOn.toggle() } label: {
            HStack(spacing: 24) {
                configuration.label
                Image(systemName: configuration.isOn ? "checkmark.circle.fill" : "circle")
            }
        }
        .buttonStyle(PillButtonStyle(primary: false))
    }
}

/// An on/off setting kept per profile on this TV, on by default.
private struct SettingSwitch: View {
    let title: LocalizedStringKey
    @AppStorage private var on: Bool

    init(title: LocalizedStringKey, key: String) {
        self.title = title
        _on = AppStorage(wrappedValue: true, key)
    }

    var body: some View {
        Button { on.toggle() } label: {
            HStack(spacing: 24) {
                Text(title)
                Image(systemName: on ? "checkmark.circle.fill" : "circle")
            }
        }
        .buttonStyle(PillButtonStyle(primary: false))
    }
}

extension View {
    /// Liquid Glass on tvOS 26 and later; frosted material before.
    @ViewBuilder
    func railGlass(cornerRadius: CGFloat) -> some View {
        if #available(tvOS 26, *) {
            glassEffect(.regular, in: RoundedRectangle(cornerRadius: cornerRadius))
        } else {
            background(RoundedRectangle(cornerRadius: cornerRadius).fill(.ultraThinMaterial))
        }
    }
}

/// Called by a page once it has put focus on its main control.
private struct PageReadyKey: EnvironmentKey {
    static let defaultValue: () -> Void = {}
}

/// Called by a page opened from a link once it is loaded.
private struct LinkReadyKey: EnvironmentKey {
    static let defaultValue: () -> Void = {}
}

extension EnvironmentValues {
    var pageReady: () -> Void {
        get { self[PageReadyKey.self] }
        set { self[PageReadyKey.self] = newValue }
    }
    var linkReady: () -> Void {
        get { self[LinkReadyKey.self] }
        set { self[LinkReadyKey.self] = newValue }
    }
}

/// Identity of the page shown: a new visit to the same section starts it over.
private struct PageID: Hashable {
    let section: Section
    let visit: Int
}

private struct PageFocusTickKey: EnvironmentKey {
    static let defaultValue = 0
}

extension EnvironmentValues {
    /// Changes when the page shown should put focus on its main control.
    var pageFocusTick: Int {
        get { self[PageFocusTickKey.self] }
        set { self[PageFocusTickKey.self] = newValue }
    }
}
