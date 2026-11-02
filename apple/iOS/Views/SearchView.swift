import SwiftUI

/// Search (IPh03 / IPa03): the field sits in the tab bar; the best match
/// first, then people, then the other titles. On iPad, genres to browse on
/// the side; before typing, recent searches.
struct SearchView: View {
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @State private var query = ""
    @State private var results: SearchResults?
    @State private var searching = false
    @State private var recent: [String] = []
    @State private var genres: [(library: Library, genre: LibraryFacets.Genre)] = []
    @State private var scope: Scope = .all
    @FocusState private var typing: Bool

    /// What the results show: everything, or one kind.
    enum Scope: String, CaseIterable { case all, movies, shows, episodes, people }

    private let recentKey = "search.recent"

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                Text("search.title").displayTitle(36).padding(.top, 10)
                // The page's own field: the system's (.searchable) lands in the
                // tab bar, the navigation bar or nowhere depending on the iOS
                // release and the device, and went missing on some iPhones.
                GlassField(systemImage: "magnifyingglass") {
                    TextField(text: $query, prompt: Text("search.prompt").foregroundStyle(Theme.textTertiary)) { Text("nav.search") }
                        .focused($typing)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .submitLabel(.search)
                    if !query.isEmpty {
                        Button { query = "" } label: {
                            Image(systemName: "xmark.circle.fill").foregroundStyle(Theme.textTertiary)
                        }
                        .accessibilityLabel(Text("search.clear"))
                    }
                }
                .padding(.top, 16)
                if wide {
                    HStack(alignment: .top, spacing: 40) {
                        VStack(alignment: .leading, spacing: 0) { found }.frame(maxWidth: .infinity, alignment: .leading)
                        VStack(alignment: .leading, spacing: 26) {
                            if !people.isEmpty { peopleRow(people) }
                            browse
                        }
                        .frame(width: 260)
                    }
                } else {
                    found
                    if results == nil { browse.padding(.top, 26) }
                }
            }
            .padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
            .padding(.bottom, 120)
        }
        .background(alignment: .top) {
            AmbientGlow(art: best?.backdrop, brightness: -0.62).frame(height: 480).ignoresSafeArea()
        }
        .background(Theme.bgBase)
        .toolbarVisibility(.hidden, for: .navigationBar)
        .scrollDismissesKeyboard(.immediately)
        .onAppear { recent = UserDefaults.standard.stringArray(forKey: recentKey) ?? [] }
        .task { await loadGenres() }
        .onChange(of: query) { scope = .all }
        .task(id: query) {
            let q = query.trimmingCharacters(in: .whitespaces)
            guard !q.isEmpty, let client = app.client else { results = nil; return }
            try? await Task.sleep(for: .milliseconds(250))
            guard !Task.isCancelled else { return }
            searching = true
            let r: SearchResults? = try? await client.get("/search", query: [.init(name: "q", value: q), .init(name: "limit", value: "20")])
            guard !Task.isCancelled else { return }
            searching = false
            withAnimation(.smooth(duration: 0.25)) { results = r }
            // Kept once the viewer stops typing and something was found.
            try? await Task.sleep(for: .seconds(2))
            guard !Task.isCancelled, q.count >= 2, let r, !(r.movies.isEmpty && r.shows.isEmpty && r.people.isEmpty) else { return }
            recent = Array(([q] + recent.filter { $0.caseInsensitiveCompare(q) != .orderedSame }).prefix(8))
            UserDefaults.standard.set(recent, forKey: recentKey)
        }
    }

    /// Movies and shows first, as the server ranks them, then episodes;
    /// only the scope's kind when one is picked.
    private var titles: [ItemSummary] {
        guard let r = results else { return [] }
        switch scope {
        case .all: return r.movies + r.shows + r.episodes
        case .movies: return r.movies
        case .shows: return r.shows
        case .episodes: return r.episodes
        case .people: return []
        }
    }

    private var people: [PersonRef] {
        scope == .all || scope == .people ? results?.people ?? [] : []
    }

    /// The scopes that found something.
    private var scopes: [Scope] {
        guard let r = results else { return [] }
        return Scope.allCases.filter { s in
            switch s {
            case .all: true
            case .movies: !r.movies.isEmpty
            case .shows: !r.shows.isEmpty
            case .episodes: !r.episodes.isEmpty
            case .people: !r.people.isEmpty
            }
        }
    }

    private var best: ItemSummary? { titles.first }

    @ViewBuilder
    private var found: some View {
        if let r = results {
            if scopes.count > 2 {
                ScrollView(.horizontal) {
                    HStack(spacing: 8) {
                        ForEach(scopes, id: \.self) { s in
                            Button { withAnimation(.smooth(duration: 0.2)) { scope = s } } label: {
                                Text(LocalizedStringKey("search.scope." + s.rawValue)).font(Theme.sans(13, .semibold))
                                    .foregroundStyle(scope == s ? Theme.onCream : Theme.textPrimary)
                            }
                            .buttonStyle(.glass)
                            .tint(scope == s ? Theme.cream : nil)
                            .accessibilityAddTraits(scope == s ? .isSelected : [])
                        }
                    }
                }
                .scrollIndicators(.hidden)
                .padding(.top, 18)
            }
            if let best {
                NavigationLink(value: best) { BestResult(item: best, query: query) }
                    .buttonStyle(.plain)
                    .padding(.top, 22)
            }
            if !wide, !people.isEmpty { peopleRow(people).padding(.top, 26) }
            let rest = titles.dropFirst()
            if !rest.isEmpty {
                Text("search.alsoInLibrary").font(Theme.display(18)).tracking(-0.3).padding(.top, 22).padding(.bottom, 12)
                ForEach(Array(rest)) { item in
                    NavigationLink(value: item) { ResultRow(item: item) }.buttonStyle(.plain).cardMenu(item)
                }
            }
            if titles.isEmpty && r.people.isEmpty {
                VStack(alignment: .leading, spacing: 8) {
                    Text("search.empty").font(Theme.display(18))
                    Text("search.emptyHint \(app.serverName ?? "PLAID")").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                }
                .padding(.top, 30)
            }
        } else if searching {
            ProgressView().tint(Theme.textSecondary).frame(maxWidth: .infinity).padding(.top, 40)
        } else if !recent.isEmpty {
            HStack {
                Text("search.recent").font(Theme.display(18)).tracking(-0.3)
                Spacer()
                Button("search.recent.clear") { recent = []; UserDefaults.standard.removeObject(forKey: recentKey) }
                    .font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
            }
            .padding(.top, 22).padding(.bottom, 8)
            ForEach(recent, id: \.self) { q in
                Button { query = q } label: {
                    HStack(spacing: 12) {
                        Image(systemName: "clock.arrow.circlepath").foregroundStyle(Theme.textTertiary)
                        Text(verbatim: q).font(Theme.sans(15, .medium))
                        Spacer()
                    }
                    .padding(.vertical, 10)
                    .overlay(alignment: .top) { Theme.hairline.frame(height: 1) }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            }
        }
    }

    private func peopleRow(_ people: [PersonRef]) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("search.people").font(Theme.display(18)).tracking(-0.3)
            ScrollView(.horizontal) {
                HStack(alignment: .top, spacing: 12) {
                    ForEach(people) { p in
                        NavigationLink(value: PersonRoute(id: p.id, name: p.name)) {
                            VStack(spacing: 8) {
                                Portrait(art: p.portrait, name: p.name, size: 64)
                                Text(verbatim: p.name).font(Theme.sans(12, .medium)).multilineTextAlignment(.center).lineLimit(2)
                            }
                            .frame(width: 78)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
            .scrollIndicators(.hidden)
        }
    }

    /// Genres to browse (IPa03's "Browse"), from the libraries' facets.
    @ViewBuilder
    private var browse: some View {
        if !genres.isEmpty && (wide || results == nil) {
            VStack(alignment: .leading, spacing: 12) {
                Text("search.browse").font(Theme.display(18)).tracking(-0.3)
                LazyVGrid(columns: [GridItem(.flexible(), spacing: 10), GridItem(.flexible(), spacing: 10)], spacing: 10) {
                    ForEach(genres.prefix(8), id: \.genre.id) { g in
                        NavigationLink(value: GenreRoute(libraryId: g.library.id, genre: GenreRef(id: g.genre.id, name: g.genre.name))) {
                            Text(verbatim: g.genre.name)
                                .font(Theme.display(16)).tracking(-0.2)
                                .frame(maxWidth: .infinity, minHeight: 60, alignment: .bottomLeading)
                                .padding(10)
                                .background(RoundedRectangle(cornerRadius: 12).fill(genreColor(g.genre.id).gradient))
                                .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(Color.white.opacity(0.08)))
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
        }
    }

    private func genreColor(_ id: Int) -> Color {
        [Theme.party[0], Theme.party[5], Theme.party[3], Theme.party[6], Theme.party[1], Theme.party[4]][abs(id) % 6]
            .mix(with: .black, by: 0.55)
    }

    private func loadGenres() async {
        guard genres.isEmpty, let client = app.client, let libs: [Library] = try? await client.get("/libraries") else { return }
        var out: [(Library, LibraryFacets.Genre)] = []
        for lib in libs {
            guard let f: LibraryFacets = try? await client.get("/libraries/\(lib.id)/facets") else { continue }
            for g in f.genres.sorted(by: { $0.count > $1.count }) where !out.contains(where: { $0.1.name == g.name }) {
                out.append((lib, g))
            }
        }
        genres = out.map { (library: $0.0, genre: $0.1) }
    }
}

/// The top match: its still, "Best result", the title with the typed part
/// underlined, and its kind and year.
private struct BestResult: View {
    let item: ItemSummary
    let query: String

    var body: some View {
        HStack(spacing: 14) {
            RemoteImage(art: item.backdrop ?? item.still ?? item.poster, width: 450)
                .frame(width: 150, height: 84)
                .background(Theme.surface)
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .shadow(color: .black.opacity(0.6), radius: 17, y: 18)
            VStack(alignment: .leading, spacing: 0) {
                Text("search.best").font(Theme.sans(12, .semibold)).foregroundStyle(Theme.textTertiary)
                Text(highlighted).font(Theme.display(20)).lineLimit(2).padding(.top, 8)
                Text(verbatim: resultFacts(item)).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary).padding(.top, 6)
            }
            Spacer(minLength: 0)
        }
        .contentShape(Rectangle())
    }

    private var highlighted: AttributedString {
        var s = AttributedString(item.name)
        let q = query.trimmingCharacters(in: .whitespaces)
        if !q.isEmpty, let r = s.range(of: q, options: [.caseInsensitive, .diacriticInsensitive]) {
            s[r].underlineStyle = .single
        }
        return s
    }
}

/// A result in the list: a small poster, the title, its kind and year.
private struct ResultRow: View {
    let item: ItemSummary

    var body: some View {
        HStack(spacing: 12) {
            RemoteImage(art: item.kind == .episode ? (item.still ?? item.poster) : item.poster, width: 130)
                .frame(width: 42, height: item.kind == .episode ? 24 : 63)
                .background(Theme.surface)
                .clipShape(RoundedRectangle(cornerRadius: 6))
            VStack(alignment: .leading, spacing: 4) {
                Text(verbatim: item.kind == .episode ? (item.show?.displayTitle ?? item.name) : item.name)
                    .font(Theme.sans(15, .semibold)).lineLimit(1)
                Text(verbatim: resultFacts(item)).font(Theme.sans(12.5, .medium)).foregroundStyle(Theme.textTertiary).lineLimit(1)
            }
            Spacer()
            Image(systemName: "chevron.right").font(.system(size: 12, weight: .bold)).foregroundStyle(Theme.textTertiary)
        }
        .padding(.vertical, 10)
        .overlay(alignment: .top) { Theme.hairline.frame(height: 1) }
        .contentShape(Rectangle())
    }
}

/// "Film · 2012", "Série · 2013", "S1 É2 · Gran Dillama".
func resultFacts(_ item: ItemSummary) -> String {
    switch item.kind {
    case .episode: return cardSubtitle(item)
    case .show: return ([tr("kind.show")] + [item.year.map(String.init)].compactMap { $0 }).joined(separator: " · ")
    default: return ([tr("kind.movie")] + [item.year.map(String.init)].compactMap { $0 }).joined(separator: " · ")
    }
}

/// A person's portrait, or their initials when there is none.
struct Portrait: View {
    let art: Artwork?
    let name: String
    var size: CGFloat = 64

    var body: some View {
        if let art {
            RemoteImage(art: art, width: Int(size * 3))
                .frame(width: size, height: size)
                .clipShape(Circle())
                .background(Circle().fill(Theme.surface))
        } else {
            InitialsDisc(name: name, size: size)
        }
    }
}
