import SwiftUI

/// What the grid shows: its sort and filters (F008 R12, R13).
struct GridQuery: Equatable {
    enum Sort: String, CaseIterable { case title, added, released, rating, played, random }
    enum Watch: String { case unwatched, inProgress = "in_progress" }

    var sort: Sort = .added
    var ascending = false
    var seed = Int64.random(in: 1...Int64(Int32.max))
    var watch: Watch?
    /// An item matches any of them.
    var genres: Set<Int> = []
    var decade: Int?
    var resolution: String?
    var hdr = false

    var filtered: Bool { watch != nil || !genres.isEmpty || decade != nil || resolution != nil || hdr }

    var items: [URLQueryItem] {
        var q: [URLQueryItem] = [.init(name: "sort", value: sort.rawValue)]
        if sort == .random {
            q.append(.init(name: "seed", value: String(seed)))
        } else {
            q.append(.init(name: "order", value: ascending ? "asc" : "desc"))
        }
        if let watch { q.append(.init(name: "watch", value: watch.rawValue)) }
        q += genres.sorted().map { .init(name: "genre", value: String($0)) }
        if let decade {
            q.append(.init(name: "yearFrom", value: String(decade)))
            q.append(.init(name: "yearTo", value: String(decade + 9)))
        }
        if let resolution { q.append(.init(name: "resolution", value: resolution)) }
        if hdr { q.append(.init(name: "hdr", value: "true")) }
        return q
    }

    /// Title ascending; the others newest or best first.
    mutating func set(_ s: Sort) {
        sort = s
        ascending = s == .title
        if s == .random { seed = Int64.random(in: 1...Int64(Int32.max)) }
    }

    mutating func clearFilters() {
        watch = nil
        genres = []
        decade = nil
        resolution = nil
        hdr = false
    }
}

/// The Movies or Shows tab (IPh02 / IPa02): the libraries of that kind, one
/// at a time, in a filtered poster grid.
struct LibraryTab: View {
    enum Kind { case movies, shows }

    let libraries: [Library]
    let kind: Kind
    @State private var picked: Library?

    var body: some View {
        Group {
            if let library = picked ?? libraries.first {
                LibraryGrid(library: library, title: Text(kind == .movies ? "nav.movies" : "nav.shows"), shows: kind == .shows,
                            libraries: libraries.count > 1 ? libraries : [], picked: $picked)
                    .id(library.id)
            } else {
                Text("library.none").font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .toolbarVisibility(.hidden, for: .navigationBar)
            }
        }
    }
}

/// A library's titles in a poster grid, loaded page by page, with chips for
/// the library, the genres, the decade, what's watched and the picture, a
/// sort menu, the collections, and the letters down the side when sorted by
/// title.
struct LibraryGrid: View {
    let library: Library
    let title: Text
    var shows = false
    var libraries: [Library] = []
    var picked: Binding<Library?>?
    var preset: GridQuery?

    init(library: Library, title: Text, shows: Bool = false, libraries: [Library] = [], picked: Binding<Library?>? = nil,
         preset: GridQuery? = nil) {
        self.library = library
        self.title = title
        self.shows = shows
        self.libraries = libraries
        self.picked = picked
        self.preset = preset
        _query = State(initialValue: preset ?? GridQuery())
    }

    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @State private var query: GridQuery
    @State private var facets: LibraryFacets?
    @State private var hasCollections = false
    @State private var items: [ItemSummary] = []
    @State private var total: Int?
    @State private var done = false
    @State private var loading = false
    @State private var failed = false
    @State private var generation = 0
    @State private var position = ScrollPosition(idType: String.self)

    /// The letter index shows on a title sort of the whole library.
    private var showsIndex: Bool {
        query.sort == .title && query.ascending && !query.filtered && !(facets?.letters.isEmpty ?? true)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                header
                chips.padding(.top, 16)
                Text(verbatim: countLine)
                    .font(Theme.sans(13, .medium)).foregroundStyle(Theme.textTertiary)
                    .padding(.horizontal, gutter).padding(.top, 16)
                grid.padding(.top, 12)
            }
            .scrollTargetLayout()
            .padding(.bottom, 120)
        }
        .scrollPosition($position, anchor: .top)
        .overlay(alignment: .trailing) {
            if showsIndex, let letters = facets?.letters {
                LetterIndex(letters: letters) { l in Task { await jump(to: l, letters: letters) } }
                    .padding(.trailing, 2)
            }
        }
        .background(alignment: .top) {
            AmbientGlow(art: items.first?.poster, brightness: -0.62).frame(height: 520).ignoresSafeArea()
        }
        .background(Theme.bgBase)
        // A tab's root has no bar; a grid opened from a row or a genre keeps
        // its back button.
        .toolbarVisibility(preset == nil ? .hidden : .automatic, for: .navigationBar)
        .refreshable { await reload() }
        .task(id: query) { await reload() }
        .task {
            guard let client = app.client else { return }
            facets = try? await client.get("/libraries/\(library.id)/facets")
            if !shows {
                let list: CollectionList? = try? await client.get("/collections", query: [.init(name: "libraryId", value: library.id)])
                hasCollections = !(list?.collections.isEmpty ?? true)
            }
        }
        .onReceive(NotificationCenter.default.publisher(for: .watchChanged)) { _ in Task { await reload() } }
    }

    private var gutter: CGFloat { wide ? Theme.wideGutter : Theme.gutter }

    private var header: some View {
        HStack(spacing: 10) {
            title.displayTitle(36).lineLimit(1).minimumScaleFactor(0.6)
            Spacer()
            sortMenu
            ProfileMenu()
        }
        .padding(.horizontal, gutter)
        .padding(.trailing, showsIndex ? 14 : 0)
        .padding(.top, 10)
    }

    private var sortMenu: some View {
        Menu {
            Picker(selection: Binding(get: { query.sort }, set: { query.set($0) })) {
                ForEach(GridQuery.Sort.allCases, id: \.self) { s in Text(LocalizedStringKey("library.sort." + s.rawValue)).tag(s) }
            } label: { Text("library.sort") }
            if query.sort != .random {
                Picker(selection: $query.ascending) {
                    Text("library.order.asc").tag(true)
                    Text("library.order.desc").tag(false)
                } label: { Text("library.order") }
            } else {
                Button { query.set(.random) } label: { Label("library.shuffle", systemImage: "shuffle") }
            }
        } label: {
            Image(systemName: "arrow.up.arrow.down").font(.system(size: 15, weight: .semibold)).frame(width: 38, height: 38)
        }
        .buttonStyle(.glass).buttonBorderShape(.circle)
        .accessibilityLabel(Text("library.sort"))
    }

    private var chips: some View {
        ScrollView(.horizontal) {
            HStack(spacing: 8) {
                if !libraries.isEmpty, let picked {
                    ChipMenu(label: Text(verbatim: library.name), active: false) {
                        Picker(selection: Binding(get: { library.id }, set: { id in picked.wrappedValue = libraries.first { $0.id == id } })) {
                            ForEach(libraries) { lib in Text(verbatim: lib.name).tag(lib.id) }
                        } label: { EmptyView() }
                    }
                }
                ChipMenu(label: Text(verbatim: genreLabel), active: !query.genres.isEmpty) {
                    Button { query.genres = [] } label: { check(Text("library.genre.all"), on: query.genres.isEmpty) }
                    // Several at once: a title matches any of them.
                    ForEach(facets?.genres ?? [], id: \.id) { g in
                        Toggle(isOn: Binding(get: { query.genres.contains(g.id) }, set: { on in
                            if on { query.genres.insert(g.id) } else { query.genres.remove(g.id) }
                        })) { Text(verbatim: "\(g.name) (\(g.count))") }
                    }
                }
                .menuActionDismissBehavior(.disabled)
                ChipMenu(label: Text(verbatim: query.decade.map { tr("library.decade %lld", $0) } ?? tr("library.filter.decade")),
                         active: query.decade != nil) {
                    Picker(selection: $query.decade) {
                        Text("library.decade.all").tag(Int?.none)
                        ForEach(facets?.decades ?? [], id: \.from) { d in
                            Text(verbatim: tr("library.decade %lld", d.from)).tag(Optional(d.from))
                        }
                    } label: { EmptyView() }
                }
                ChipMenu(label: Text(watchLabel), active: query.watch != nil) {
                    Picker(selection: $query.watch) {
                        Text("library.watch.all").tag(GridQuery.Watch?.none)
                        Text(shows ? "library.watch.newEpisodes" : "library.watch.unwatched").tag(Optional(GridQuery.Watch.unwatched))
                        Text("library.watch.inProgress").tag(Optional(GridQuery.Watch.inProgress))
                    } label: { EmptyView() }
                }
                if let f = facets, f.resolutions.count > 1 || f.hdr > 0 {
                    ChipMenu(label: Text(verbatim: pictureLabel), active: query.resolution != nil || query.hdr) {
                        Picker(selection: $query.resolution) {
                            Text("library.filter.anyResolution").tag(String?.none)
                            ForEach(f.resolutions, id: \.value) { r in
                                Text(verbatim: "\(resolutionName(r.value)) (\(r.count))").tag(Optional(r.value))
                            }
                        } label: { Text("library.filter.resolution") }
                        if f.hdr > 0 {
                            Toggle(isOn: $query.hdr) { Text(verbatim: "HDR (\(f.hdr))") }
                        }
                    }
                }
                if hasCollections {
                    NavigationLink(value: CollectionsRoute(library: library)) {
                        Label("library.collections", systemImage: "square.stack").font(Theme.sans(14, .semibold))
                            .frame(height: 26)
                    }
                    .buttonStyle(.glass)
                }
            }
        }
        .scrollIndicators(.hidden)
        .contentMargins(.horizontal, gutter, for: .scrollContent)
    }

    private var genreLabel: String {
        let names = (facets?.genres ?? []).filter { query.genres.contains($0.id) }.map(\.name)
        switch names.count {
        case 0: return tr("library.filter.genre")
        case 1: return names[0]
        default: return tr("library.filter.genres %lld", names.count)
        }
    }

    private var pictureLabel: String {
        let parts = [query.resolution.map(resolutionName), query.hdr ? "HDR" : nil].compactMap { $0 }
        return parts.isEmpty ? tr("library.filter.picture") : parts.joined(separator: " · ")
    }

    private func resolutionName(_ v: String) -> String {
        ["2160p": "4K", "4k": "4K", "1080p": "1080p", "720p": "720p", "sd": "SD"][v.lowercased()] ?? v
    }

    private var watchLabel: LocalizedStringKey {
        switch query.watch {
        case .none: "library.watch.all"
        case .unwatched: shows ? "library.watch.newEpisodes" : "library.watch.unwatched"
        case .inProgress: "library.watch.inProgress"
        }
    }

    @ViewBuilder
    private func check(_ text: Text, on: Bool) -> some View {
        if on { Label { text } icon: { Image(systemName: "checkmark") } } else { text }
    }

    /// "12 films · recently added".
    private var countLine: String {
        let sort = localized("library.sort." + query.sort.rawValue).lowercased(with: AppLanguage.locale)
        guard let total = total ?? (query.filtered ? nil : facets?.total) else { return sort }
        return tr("library.count %lld", total) + " · " + sort
    }

    private var grid: some View {
        let width: CGFloat = wide ? 132 : 108
        let columns = [GridItem(.adaptive(minimum: width, maximum: wide ? 160 : 124), spacing: wide ? 18 : 12, alignment: .top)]
        return LazyVGrid(columns: columns, alignment: .leading, spacing: 20) {
            ForEach(items) { item in
                NavigationLink(value: item) {
                    PosterCard(item: item, width: width, showTitle: true)
                }
                .buttonStyle(.plain)
                .cardMenu(item)
                .id(item.id)
                .onAppear { if item.id == items.last?.id { Task { await loadMore() } } }
            }
            if loading && items.isEmpty {
                ForEach(0..<9, id: \.self) { _ in
                    Shimmer().frame(width: width, height: width * 1.5).clipShape(RoundedRectangle(cornerRadius: 9))
                }
            }
        }
        .padding(.horizontal, gutter)
        .padding(.trailing, showsIndex ? 14 : 0)
        .overlay(alignment: .top) {
            if !loading && items.isEmpty {
                VStack(spacing: 12) {
                    if failed {
                        ErrorState { Task { await reload() } }
                    } else if query.filtered {
                        Text("library.empty.filtered").font(Theme.display(18))
                        Button("library.filters.clear") { query.clearFilters() }.buttonStyle(.glass)
                    } else {
                        Text("library.empty").font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary)
                    }
                }
                .multilineTextAlignment(.center)
                .padding(.top, 60)
            }
        }
    }

    private func reload() async {
        generation += 1
        items = []
        total = nil
        done = false
        while loading { try? await Task.sleep(for: .milliseconds(50)) }
        await loadMore()
    }

    /// The next page, by offset: the letter index jumps far ahead with it.
    private func loadMore(limit: Int = 60) async {
        guard !loading, !done, let client = app.client else { return }
        loading = true
        defer { loading = false }
        let gen = generation
        let q = query.items + [.init(name: "limit", value: String(limit)), .init(name: "offset", value: String(items.count)),
                               .init(name: "total", value: "true")]
        do {
            let page: ItemPage = try await client.get("/libraries/\(library.id)/items", query: q)
            guard gen == generation else { return }
            items += page.items
            if let t = page.total { total = t }
            done = page.items.count < limit || items.count >= (total ?? .max)
            failed = false
        } catch {
            if gen == generation { failed = true }
        }
    }

    /// Loads up to the letter's first title, then scrolls to it.
    private func jump(to letter: String, letters: [LibraryFacets.Letter]) async {
        let offset = letters.prefix { $0.letter != letter }.reduce(0) { $0 + $1.count }
        while items.count <= offset && !done {
            while loading { try? await Task.sleep(for: .milliseconds(50)) }
            let before = items.count
            await loadMore(limit: 200)
            if items.count == before { break }
        }
        guard offset < items.count else { return }
        withAnimation(.smooth(duration: 0.35)) { position.scrollTo(id: items[offset].id, anchor: .top) }
    }
}

/// The letters of a title-sorted library down the right edge (F008 R15):
/// a tap or a slide along them jumps there.
private struct LetterIndex: View {
    let letters: [LibraryFacets.Letter]
    let jump: (String) -> Void
    @State private var last: String?

    var body: some View {
        VStack(spacing: 0) {
            ForEach(letters, id: \.letter) { l in
                Text(verbatim: l.letter)
                    .font(Theme.sans(11, .bold))
                    .foregroundStyle(last == l.letter ? Theme.textPrimary : Theme.textSecondary)
                    .frame(width: 18, height: 15)
            }
        }
        .padding(.vertical, 6)
        .background(Capsule().fill(Color.black.opacity(0.25)))
        .contentShape(Rectangle())
        .gesture(DragGesture(minimumDistance: 0).onChanged { v in
            let i = Int(v.location.y / 15)
            guard letters.indices.contains(i), letters[i].letter != last else { return }
            last = letters[i].letter
            UISelectionFeedbackGenerator().selectionChanged()
            jump(letters[i].letter)
        }.onEnded { _ in last = nil })
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Text("library.letters"))
        .accessibilityAdjustableAction { dir in
            let i = letters.firstIndex { $0.letter == last } ?? -1
            let n = min(max(0, i + (dir == .increment ? 1 : -1)), letters.count - 1)
            last = letters[n].letter
            jump(letters[n].letter)
        }
    }
}

/// A glass capsule that opens a menu: a filter of the grid.
struct ChipMenu<Content: View>: View {
    let label: Text
    let active: Bool
    @ViewBuilder let content: Content

    var body: some View {
        Menu { content } label: {
            HStack(spacing: 6) {
                label.font(Theme.sans(14, .semibold)).lineLimit(1)
                Image(systemName: "chevron.down").font(.system(size: 10, weight: .heavy)).foregroundStyle(Theme.textSecondary)
            }
            .padding(.leading, 4).padding(.trailing, 0)
            .frame(height: 26)
        }
        .buttonStyle(.glass)
        .tint(active ? Theme.textPrimary.opacity(0.25) : nil)
    }
}
