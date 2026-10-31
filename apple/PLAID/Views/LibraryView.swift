import SwiftUI

/// What the grid shows: its sort and filters (F008 R12, R13).
struct GridQuery: Equatable {
    enum Sort: String, CaseIterable { case title, added, released, rating, played, random }
    enum Watch: String { case unwatched, inProgress = "in_progress" }

    var sort: Sort = .title
    var ascending = true
    var seed = Int64.random(in: 1...Int64(Int32.max))
    var watch: Watch?
    var genres: Set<Int> = []
    var decade: Int?
    var resolution: String?
    var hdr = false

    var filtered: Bool { watch != nil || !genres.isEmpty || decade != nil || resolution != nil || hdr }
    var filterCount: Int { genres.count + (decade == nil ? 0 : 1) + (resolution == nil ? 0 : 1) + (hdr ? 1 : 0) }

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

    /// Title ascending by default; the others newest or best first.
    mutating func set(_ s: Sort) {
        sort = s
        ascending = s == .title
        if s == .random { seed = Int64.random(in: 1...Int64(Int32.max)) }
    }
}

/// A library's titles in a poster grid, loaded page by page, with its
/// sort, filters and letter index.
struct LibraryView: View {
    let library: Library
    /// Shown by genre, from an item's genre link.
    var genre: GenreRef?
    @Environment(AppState.self) private var app
    @Environment(\.pageReady) private var pageReady
    @State private var query = GridQuery()
    @State private var facets: LibraryFacets?
    @State private var items: [ItemSummary] = []
    @State private var total: Int?
    @State private var loading = false
    @State private var done = false
    @State private var generation = 0
    @State private var announced = false
    @FocusState private var focusedId: String?
    @State private var position = ScrollPosition(idType: String.self)
    @Environment(\.pageFocusTick) private var pageFocusTick

    private let columns = [GridItem(.adaptive(minimum: 240, maximum: 300), spacing: 40)]

    init(library: Library, genre: GenreRef? = nil, preset: SeeAll? = nil) {
        self.library = library
        self.genre = genre
        var q = GridQuery()
        if let genre { q.genres = [genre.id] }
        if let preset {
            if let s = GridQuery.Sort(rawValue: preset.sort) { q.set(s) }
            q.ascending = preset.order == "asc"
            if let g = preset.genre { q.genres = [g] }
        }
        _query = State(initialValue: q)
    }

    /// The letter index shows on a title sort of the whole library.
    private var showsIndex: Bool { query.sort == .title && query.ascending && !query.filtered && genre == nil && !(facets?.letters.isEmpty ?? true) }

    var body: some View {
        ScrollView(.vertical) {
                VStack(alignment: .leading, spacing: 24) {
                    header
                    controls
                }
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                .padding(.top, 60)
                .padding(.leading, Theme.rail)

                LazyVGrid(columns: columns, spacing: 64) {
                    ForEach(items) { item in
                        PosterCard(item: item)
                            .focused($focusedId, equals: item.id)
                            .id(item.id)
                            .onAppear { if item.id == items.last?.id { Task { await loadMore() } } }
                    }
                    if loading && items.isEmpty {
                        ForEach(0..<12, id: \.self) { _ in
                            Shimmer().frame(width: 260, height: 390).clipShape(RoundedRectangle(cornerRadius: Theme.radius))
                        }
                    }
                }
                .scrollTargetLayout()
                .padding(.leading, Theme.gutter).padding(.trailing, showsIndex ? 150 : Theme.trailing)
                .padding(.vertical, 40)
                .padding(.leading, Theme.rail)
                .focusSection()

                if done && items.isEmpty && !loading {
                    Text(query.filtered ? "library.empty.filtered" : "library.empty")
                        .font(Theme.sans(30)).foregroundStyle(Theme.textTertiary)
                        .padding(.leading, Theme.rail + Theme.gutter)
                }
            }
            .scrollClipDisabled()
            .scrollPosition($position)
            .ignoresSafeArea(edges: .horizontal)
            .overlay(alignment: .trailing) {
                if showsIndex, let letters = facets?.letters {
                    LetterIndex(letters: letters) { letter in Task { await jump(to: letter, letters: letters) } }
                        .padding(.trailing, 24)
                }
            }
        .task {
            async let f: LibraryFacets? = try? await app.client?.get("/libraries/\(library.id)/facets")
            await reload()
            facets = await f
        }
        .onChange(of: query) { Task { await reload() } }
        .onChange(of: pageFocusTick) { focusedId = items.first?.id }
        .onReceive(NotificationCenter.default.publisher(for: .watchChanged)) { _ in Task { await reload() } }
    }

    private var header: some View {
        HStack(alignment: .firstTextBaseline, spacing: 24) {
            Text(verbatim: genre?.name ?? library.name).font(Theme.sans(56, .bold))
            if let total {
                Text("library.count \(total)").font(Theme.sans(28)).foregroundStyle(Theme.textTertiary)
            }
        }
    }

    private var controls: some View {
        ScrollView(.horizontal) {
            HStack(spacing: 20) {
                Menu {
                    ForEach(GridQuery.Sort.allCases, id: \.self) { s in
                        Button { query.set(s) } label: {
                            if query.sort == s { Label(sortName(s), systemImage: "checkmark") } else { Text(sortName(s)) }
                        }
                    }
                } label: {
                    Label(sortName(query.sort), systemImage: "arrow.up.arrow.down")
                }
                .buttonStyle(SeasonPillStyle(selected: false))

                if query.sort == .random {
                    Button { query.seed = Int64.random(in: 1...Int64(Int32.max)) } label: {
                        Label("library.shuffle", systemImage: "shuffle")
                    }
                    .buttonStyle(SeasonPillStyle(selected: false))
                } else {
                    Button { query.ascending.toggle() } label: {
                        Image(systemName: query.ascending ? "arrow.up" : "arrow.down")
                            .accessibilityLabel(Text(query.ascending ? "library.order.asc" : "library.order.desc"))
                    }
                    .buttonStyle(SeasonPillStyle(selected: false))
                }

                watchChip(nil, "library.watch.all")
                watchChip(.unwatched, library.type == "shows" ? "library.watch.newEpisodes" : "library.watch.unwatched")
                watchChip(.inProgress, "library.watch.inProgress")

                if let facets, genre == nil {
                    Menu {
                        if !facets.genres.isEmpty {
                            SwiftUI.Section("library.filter.genres") {
                                ForEach(facets.genres, id: \.id) { g in
                                    Button { toggle(genre: g.id) } label: {
                                        check("\(g.name) (\(g.count))", query.genres.contains(g.id))
                                    }
                                }
                            }
                        }
                        if !facets.decades.isEmpty {
                            SwiftUI.Section("library.filter.decades") {
                                ForEach(facets.decades, id: \.from) { d in
                                    Button { query.decade = query.decade == d.from ? nil : d.from } label: {
                                        check(tr("library.decade %lld", d.from) + " (\(d.count))", query.decade == d.from)
                                    }
                                }
                            }
                        }
                        if !facets.resolutions.isEmpty {
                            SwiftUI.Section("library.filter.resolution") {
                                ForEach(facets.resolutions, id: \.value) { r in
                                    Button { query.resolution = query.resolution == r.value ? nil : r.value } label: {
                                        check(resolutionName(r.value) + " (\(r.count))", query.resolution == r.value)
                                    }
                                }
                            }
                        }
                        if facets.hdr > 0 {
                            Button { query.hdr.toggle() } label: { check("HDR (\(facets.hdr))", query.hdr) }
                        }
                    } label: {
                        Label(query.filterCount > 0 ? "library.filters.count \(query.filterCount)" : "library.filters",
                              systemImage: "line.3.horizontal.decrease")
                    }
                    .buttonStyle(SeasonPillStyle(selected: query.filterCount > 0))
                }

                if query.filtered && genre == nil {
                    Button("library.filters.clear") {
                        query.watch = nil; query.genres = []; query.decade = nil; query.resolution = nil; query.hdr = false
                    }
                    .buttonStyle(SeasonPillStyle(selected: false))
                }

                if genre == nil && library.type == "movies" {
                    NavigationLink(value: CollectionsRoute(library: library)) {
                        Label("library.collections", systemImage: "square.stack")
                    }
                    .buttonStyle(SeasonPillStyle(selected: false))
                }
            }
            .padding(.vertical, 16)
        }
        .scrollClipDisabled()
        .focusSection()
    }

    private func watchChip(_ w: GridQuery.Watch?, _ title: LocalizedStringKey) -> some View {
        Button(title) { query.watch = w }
            .buttonStyle(SeasonPillStyle(selected: query.watch == w))
    }

    @ViewBuilder
    private func check(_ title: String, _ on: Bool) -> some View {
        if on { Label(title, systemImage: "checkmark") } else { Text(verbatim: title) }
    }

    private func toggle(genre id: Int) {
        if query.genres.contains(id) { query.genres.remove(id) } else { query.genres.insert(id) }
    }

    private func sortName(_ s: GridQuery.Sort) -> String {
        localized("library.sort." + s.rawValue)
    }

    private func resolutionName(_ r: String) -> String {
        ["2160p": "4K", "1080p": "1080p", "720p": "720p", "sd": "SD"][r] ?? r
    }

    // MARK: Loading

    private func reload() async {
        generation += 1
        items = []
        done = false
        total = nil
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
        guard let page: ItemPage = try? await client.get("/libraries/\(library.id)/items", query: q), gen == generation else { return }
        let first = items.isEmpty
        items += page.items
        if let t = page.total { total = t }
        done = page.items.count < limit || items.count >= (total ?? .max)
        if first && !announced {
            announced = true
            try? await Task.sleep(for: .milliseconds(100))
            focusedId = items.first?.id
            pageReady()
        }
    }

    /// Loads up to the letter's first title, then scrolls and focuses it.
    private func jump(to letter: String, letters: [LibraryFacets.Letter]) async {
        let offset = letters.prefix { $0.letter != letter }.reduce(0) { $0 + $1.count }
        while items.count <= offset && !done {
            while loading { try? await Task.sleep(for: .milliseconds(50)) }
            let before = items.count
            await loadMore(limit: 200)
            if items.count == before { break }
        }
        guard offset < items.count else { return }
        let id = items[offset].id
        withAnimation(.smooth(duration: 0.4)) { position.scrollTo(id: id, anchor: .center) }
        try? await Task.sleep(for: .milliseconds(300))
        focusedId = id
    }
}

/// The letters of a title-sorted library down the right edge (F008 R15).
private struct LetterIndex: View {
    let letters: [LibraryFacets.Letter]
    let jump: (String) -> Void

    var body: some View {
        VStack(spacing: 2) {
            ForEach(letters, id: \.letter) { l in
                Button { jump(l.letter) } label: {
                    Text(verbatim: l.letter).font(Theme.sans(20, .semibold)).frame(width: 44, height: 30)
                }
                .buttonStyle(LetterStyle())
            }
        }
        .padding(6)
        .railGlass(cornerRadius: 22)
        .focusSection()
    }

    private struct LetterStyle: ButtonStyle {
        func makeBody(configuration: Configuration) -> some View { Key(configuration: configuration) }

        private struct Key: View {
            let configuration: ButtonStyleConfiguration
            @Environment(\.isFocused) private var focused
            var body: some View {
                configuration.label
                    .foregroundStyle(focused ? Theme.onAction : Theme.textSecondary)
                    .background(Capsule().fill(focused ? Theme.action : .clear))
                    .scaleEffect(focused ? 1.25 : 1)
                    .animation(.spring(response: 0.3, dampingFraction: 0.7), value: focused)
            }
        }
    }
}

/// Search (F008 R28): movies, shows, episodes and people as you type;
/// with an empty field, the profile's recent searches.
struct SearchView: View {
    @Environment(AppState.self) private var app
    @State private var query = ""
    @State private var results: SearchResults?
    @State private var recent: [String] = []

    private var recentKey: String { "search.recent.\(app.me?.activeProfile?.id ?? "default")" }

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 40) {
                if let r = results, !query.isEmpty {
                    if !r.movies.isEmpty { ItemRow(title: "search.movies", items: r.movies) }
                    if !r.shows.isEmpty { ItemRow(title: "search.shows", items: r.shows) }
                    if !r.episodes.isEmpty { ItemRow(title: "search.episodes", items: r.episodes, landscape: true) }
                    if !r.people.isEmpty { PeopleRow(people: r.people) }
                    if r.movies.isEmpty && r.shows.isEmpty && r.episodes.isEmpty && r.people.isEmpty {
                        Text("search.empty").font(Theme.sans(30)).foregroundStyle(Theme.textTertiary)
                            .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                    }
                } else if query.isEmpty, !recent.isEmpty {
                    VStack(alignment: .leading, spacing: 16) {
                        Text("search.recent").font(Theme.sans(30, .semibold))
                        ScrollView(.horizontal) {
                            HStack(spacing: 20) {
                                ForEach(recent, id: \.self) { q in
                                    Button { query = q } label: { Label(q, systemImage: "clock.arrow.circlepath") }
                                        .buttonStyle(SeasonPillStyle(selected: false))
                                }
                                Button("search.recent.clear") { recent = []; save() }
                                    .buttonStyle(SeasonPillStyle(selected: false))
                            }
                            .padding(.vertical, 12)
                        }
                        .scrollClipDisabled()
                    }
                    .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                    .focusSection()
                }
            }
        }
        .searchable(text: $query, prompt: Text("search.prompt"))
        .onAppear { recent = UserDefaults.standard.stringArray(forKey: recentKey) ?? [] }
        .task(id: query) {
            let q = query.trimmingCharacters(in: .whitespaces)
            guard !q.isEmpty, let client = app.client else { results = nil; return }
            try? await Task.sleep(for: .milliseconds(300))
            guard !Task.isCancelled else { return }
            results = try? await client.get("/search", query: [.init(name: "q", value: q), .init(name: "limit", value: "20")])
            // Kept once the viewer stops typing and something was found.
            try? await Task.sleep(for: .seconds(2))
            guard !Task.isCancelled, q.count >= 2, let r = results,
                  !(r.movies.isEmpty && r.shows.isEmpty && r.episodes.isEmpty && r.people.isEmpty) else { return }
            recent = Array(([q] + recent.filter { $0.caseInsensitiveCompare(q) != .orderedSame }).prefix(8))
            save()
        }
    }

    private func save() { UserDefaults.standard.set(recent, forKey: recentKey) }
}

/// People found by a search, as round portraits that open their page.
private struct PeopleRow: View {
    let people: [PersonRef]

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("search.people").font(Theme.sans(34, .semibold))
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 40) {
                    ForEach(people) { p in
                        NavigationLink(value: PersonRoute(id: p.id, name: p.name)) {
                            VStack(spacing: 12) {
                                ZStack {
                                    Circle().fill(Theme.surface)
                                    Text(verbatim: initials(p.name)).font(Theme.sans(44, .semibold)).foregroundStyle(Theme.textTertiary)
                                    RemoteImage(art: p.portrait, width: 342)
                                }
                                .frame(width: 180, height: 180)
                                .clipShape(Circle())
                                Text(verbatim: p.name).font(Theme.sans(22, .semibold)).lineLimit(1)
                            }
                            .frame(width: 200)
                        }
                        .buttonStyle(CardButtonStyle())
                    }
                }
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                .padding(.vertical, 30)
            }
            .scrollClipDisabled()
        }
        .focusSection()
    }
}
