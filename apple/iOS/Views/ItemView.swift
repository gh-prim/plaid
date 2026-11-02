import SwiftUI

/// A movie or a show (IPh04, IPh05 / IPa04, IPa05): the backdrop full bleed,
/// its preview looping over it, the title, Play or Resume, the trailer and
/// the version; then the synopsis, the seasons and episodes of a show, cast
/// and crew, the collection, About, and titles in the same vein.
struct ItemView: View {
    let summary: ItemSummary
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @Environment(\.play) private var play
    @State private var detail: ItemDetail?
    @State private var seasons: [ItemSummary] = []
    @State private var season: ItemSummary?
    @State private var episodes: [ItemSummary] = []
    @State private var upNext: ItemSummary?
    @State private var similar: [ItemSummary] = []
    @State private var watched: Bool?
    @State private var collection: CollectionDetail?
    /// The version picked; nil lets the server choose.
    @State private var fileId: String?
    /// The server no longer has it, or this profile can't see it.
    @State private var unavailable = false
    @Environment(\.previewsActive) private var previewsActive

    private var item: ItemSummary { detail?.summary ?? summary }
    private var gutter: CGFloat { wide ? Theme.wideGutter : Theme.gutter }

    var body: some View {
        if unavailable {
            Unavailable()
        } else {
            page
        }
    }

    private var page: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                hero
                VStack(alignment: .leading, spacing: 0) {
                    if !wide, let o = overview {
                        Text(verbatim: o).font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary)
                            .lineSpacing(4).padding(.top, 22)
                    }
                    if item.kind == .show || item.kind == .season { episodesSection.padding(.top, 26) }
                    cast.padding(.top, 28)
                }
                .padding(.horizontal, gutter)
                if let collection, collection.items.count > 1 { collectionRow(collection).padding(.top, 28) }
                if let detail {
                    AboutSection(detail: detail, seasonCount: seasons.filter { ($0.seasonNumber ?? 0) > 0 }.count)
                        .padding(.horizontal, gutter)
                        .padding(.top, 30)
                }
                if !similar.isEmpty { similarRow.padding(.top, 28) }
            }
            .padding(.bottom, 120)
        }
        .background(alignment: .top) {
            AmbientGlow(art: item.backdrop ?? item.poster, brightness: -0.55).frame(height: 1100).ignoresSafeArea()
        }
        .background(Theme.bgBase)
        .ignoresSafeArea(edges: .top)
        .toolbar { toolbar }
        .toolbarBackgroundVisibility(.hidden, for: .navigationBar)
        .navigationBarTitleDisplayMode(.inline)
        .task(id: summary.id) { await load() }
        .onReceive(NotificationCenter.default.publisher(for: .watchChanged)) { _ in Task { await load() } }
    }

    private var overview: String? { detail?.metadata?.overview ?? item.overview }

    // MARK: Hero

    private var hero: some View {
        let height: CGFloat = wide ? 560 : 500
        return ZStack(alignment: .bottomLeading) {
            RemoteImage(art: item.backdrop ?? item.poster, width: wide ? 2400 : 1200)
                .frame(height: height)
                .mask(LinearGradient(stops: [.init(color: .black, location: 0.5), .init(color: .clear, location: 1)],
                                     startPoint: .top, endPoint: .bottom))
                .overlay {
                    LinearGradient(stops: [.init(color: Theme.bgBase.opacity(0.4), location: 0), .init(color: .clear, location: 0.25),
                                           .init(color: .clear, location: 0.55), .init(color: Theme.bgBase.opacity(0.5), location: 1)],
                                   startPoint: .top, endPoint: .bottom)
                }
                .overlay {
                    // The scenes loop over the backdrop, under its shades.
                    if (item.kind == .movie || item.kind == .show), app.previewsOn {
                        HeroPreview(itemId: item.id, active: previewsActive)
                            .mask(LinearGradient(stops: [.init(color: .black, location: 0.5), .init(color: .clear, location: 1)],
                                                 startPoint: .top, endPoint: .bottom))
                    }
                }
                .overlay { if wide { LinearGradient(colors: [Theme.bgBase.opacity(0.55), .clear], startPoint: .leading, endPoint: .center) } }
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 0) {
                Text(verbatim: item.name).displayTitle(wide ? 52 : 44).lineLimit(3).minimumScaleFactor(0.6)
                Text(verbatim: factsLine).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary).padding(.top, 10)
                if !badges.isEmpty {
                    HStack(spacing: 6) {
                        ForEach(badges, id: \.self) { b in
                            Text(verbatim: b).font(Theme.sans(11, .bold))
                                .padding(.horizontal, 7).frame(height: 22)
                                .background(RoundedRectangle(cornerRadius: 6).fill(Color.white.opacity(0.12)))
                                .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(Color.white.opacity(0.18)))
                        }
                    }
                    .padding(.top, 12)
                }
                if wide, let o = overview {
                    Text(verbatim: o).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                        .lineLimit(3).frame(maxWidth: 480, alignment: .leading).padding(.top, 14)
                }
                actions.padding(.top, 18)
                extras.padding(.top, 10)
            }
            .padding(.horizontal, gutter)
            .padding(.bottom, 4)
        }
        .frame(height: height)
    }

    /// Play, Resume, or the show's next episode; and watched on or off.
    private var actions: some View {
        HStack(spacing: 10) {
            if let target = playTarget {
                Button { play(target, from: target.userState?.positionMs ?? 0, fileId: target.id == item.id ? fileId : nil) } label: {
                    Label { Text(verbatim: playTitle(target)) } icon: { Image(systemName: "play.fill") }
                }
                .buttonStyle(CreamButtonStyle())
                .frame(maxWidth: wide ? 240 : .infinity)
            } else if detail == nil {
                Shimmer().frame(height: 50).clipShape(Capsule()).frame(maxWidth: wide ? 240 : .infinity)
            }
            let isWatched = watched ?? item.userState?.watched ?? false
            GlassIconButton(systemImage: isWatched ? "checkmark" : "plus",
                            label: isWatched ? "item.markUnwatched" : "item.markWatched") {
                Task { await toggleWatched(isWatched) }
            }
        }
    }

    /// The trailer, the version when there are several, and the preview's sound.
    @ViewBuilder
    private var extras: some View {
        let showPreviewSound = (item.kind == .movie || item.kind == .show) && app.previewsOn
        if bestTrailer != nil || versions.count > 1 || showPreviewSound {
            HStack(spacing: 8) {
                if let trailer = bestTrailer { TrailerButton(trailer: trailer) }
                if versions.count > 1 {
                    Menu {
                        Picker(selection: $fileId) {
                            Text("item.versions.auto").tag(String?.none)
                            ForEach(versions) { f in Text(verbatim: versionLabel(f)).tag(Optional(f.id)) }
                        } label: { Text("item.version") }
                    } label: {
                        Label { Text(verbatim: versions.first { $0.id == fileId }.map(versionLabel) ?? tr("item.version")) }
                            icon: { Image(systemName: "film.stack") }
                            .font(Theme.sans(13, .semibold))
                    }
                    .buttonStyle(.glass)
                }
                Spacer(minLength: 0)
                if showPreviewSound { PreviewSoundButton() }
            }
        }
    }

    /// A YouTube trailer, in the app's language first, official first.
    private var bestTrailer: Trailer? {
        let all = (detail?.metadata?.trailers ?? []).filter { $0.site == "YouTube" }
        let lang = (app.language ?? AppLanguage.locale.language.languageCode?.identifier ?? "en").prefix(2)
        func rank(_ t: Trailer) -> Int {
            (t.language.hasPrefix(lang) ? 0 : 4) + (t.type == "Trailer" ? 0 : 2) + (t.official ? 0 : 1)
        }
        return all.min { rank($0) < rank($1) }
    }

    /// The files a movie can play.
    private var versions: [MediaFile] {
        guard item.kind == .movie else { return [] }
        return (detail?.files ?? []).filter { $0.missingSince == nil }
    }

    private func versionLabel(_ f: MediaFile) -> String {
        [f.edition, resolutionLabel(f.height), f.hdrFormat.map(hdrLabel), f.videoCodec?.uppercased()]
            .compactMap { $0 }.joined(separator: " · ")
    }

    /// What Play starts: the movie, or the show's next episode.
    private var playTarget: ItemSummary? {
        switch item.kind {
        case .movie, .episode: detail == nil ? nil : item
        case .show, .season: upNext ?? episodes.first
        }
    }

    private func playTitle(_ target: ItemSummary) -> String {
        let started = (target.userState?.positionMs ?? 0) > 0
        if target.kind == .episode, item.kind != .episode, let code = episodeCode(target) {
            return tr(started ? "item.resumeEpisode %@" : "item.playEpisode %@", code)
        }
        return started ? tr("item.resume") : tr("item.play")
    }

    /// "2021 · Animation · 10 min", or "Série · 2013 à 2016 · Animation · 1 saison".
    private var factsLine: String {
        var parts: [String] = []
        if item.kind == .show {
            parts.append(tr("kind.show"))
            if let first = item.year {
                let last = detail?.metadata?.endDate.flatMap { Int($0.prefix(4)) }
                parts.append(last.map { $0 == first ? "\(first)" : tr("item.years %lld %lld", first, $0) } ?? "\(first)")
            }
        } else if let y = item.year {
            parts.append(String(y))
        }
        parts += item.genres.prefix(1)
        if item.kind == .show {
            let n = seasons.filter { ($0.seasonNumber ?? 0) > 0 }.count
            if n > 0 { parts.append(tr("item.seasonCount %lld", n)) }
        } else if let m = detail?.metadata?.runtimeMinutes ?? item.runtimeMinutes {
            parts.append(duration(minutes: m))
        }
        return parts.joined(separator: " · ")
    }

    /// 4K, HDR, 5.1, "ST FR": what the file offers.
    private var badges: [String] {
        guard let f = detail?.files.first(where: { $0.missingSince == nil }) else { return [] }
        var out: [String] = []
        if let r = resolutionLabel(f.height), r == "4K" || r == "1080p" { out.append(r == "4K" ? "4K" : "HD") }
        if f.hdrFormat != nil { out.append("HDR") }
        if let ch = f.streams.filter({ $0.type == "audio" }).compactMap(\.channels).max(), ch > 2 { out.append("\(ch - 1).1") }
        let subs = unique(f.streams.filter { $0.type == "subtitle" && !$0.forced }.map { String($0.language.prefix(2)).uppercased() })
            .filter { $0 != "UN" }
        if !subs.isEmpty { out.append(tr("item.badge.subtitles %@", subs.prefix(2).joined(separator: " "))) }
        return out
    }

    // MARK: Episodes

    @ViewBuilder
    private var episodesSection: some View {
        HStack {
            ChipMenu(label: Text(verbatim: seasonName(season)), active: false) {
                if seasons.count > 1 {
                    Picker(selection: Binding(get: { season?.id }, set: { id in
                        guard let s = seasons.first(where: { $0.id == id }) else { return }
                        Task { season = s; await loadEpisodes(s) }
                    })) {
                        ForEach(seasons) { s in Text(verbatim: seasonName(s)).tag(Optional(s.id)) }
                    } label: { Text("item.seasons") }
                }
                if let season, item.kind == .show {
                    let seen = season.userState?.watched ?? false
                    Button { Task { await setWatched(season, !seen) } } label: {
                        Label(seen ? "item.markSeasonUnwatched" : "item.markSeasonWatched",
                              systemImage: seen ? "checkmark.circle.fill" : "checkmark.circle")
                    }
                }
            }
            Spacer()
            if !episodes.isEmpty {
                Text("item.episodeCount \(episodes.count)").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textTertiary)
            }
        }
        if wide {
            LazyVGrid(columns: [GridItem(.adaptive(minimum: 250), spacing: 18, alignment: .top)], alignment: .leading, spacing: 22) {
                ForEach(episodes) { e in EpisodeTile(episode: e).cardMenu(e) }
            }
            .padding(.top, 16)
        } else {
            VStack(spacing: 0) {
                ForEach(episodes) { e in EpisodeRow(episode: e).cardMenu(e) }
            }
            .padding(.top, 8)
        }
        if episodes.isEmpty && season != nil {
            ProgressView().tint(Theme.textSecondary).frame(maxWidth: .infinity).padding(.top, 20)
        }
    }

    private func seasonName(_ s: ItemSummary?) -> String {
        guard let s else { return "" }
        if s.seasonNumber == 0 { return tr("item.specials") }
        return s.seasonNumber.map { tr("item.season %lld", $0) } ?? s.name
    }

    // MARK: Cast, facts, similar

    @ViewBuilder
    private var cast: some View {
        // One card per person, with their first role.
        var seen = Set<String>()
        let credits = Array((detail?.metadata?.credits ?? []).filter { seen.insert($0.personId).inserted }.prefix(20))
        if !credits.isEmpty {
            VStack(alignment: .leading, spacing: 14) {
                RowHeader(title: tr("item.castAndCrew"))
                ScrollView(.horizontal) {
                    HStack(alignment: .top, spacing: 12) {
                        ForEach(credits, id: \.self) { c in
                            NavigationLink(value: PersonRoute(id: c.personId, name: c.name)) {
                                VStack(spacing: 6) {
                                    Portrait(art: c.portrait, name: c.name, size: 60)
                                    Text(verbatim: c.name).font(Theme.sans(12, .semibold)).multilineTextAlignment(.center).lineLimit(2)
                                    Text(verbatim: c.character ?? localized("person.role." + c.role))
                                        .font(Theme.sans(11, .medium)).foregroundStyle(Theme.textTertiary).lineLimit(1)
                                }
                                .frame(width: 76)
                            }
                            .buttonStyle(.plain)
                        }
                    }
                }
                .scrollIndicators(.hidden)
            }
        }
    }

    private func collectionRow(_ c: CollectionDetail) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            NavigationLink(value: CollectionRoute(id: c.summary.id, name: c.summary.name)) {
                RowHeader(title: tr("item.inCollection %@", c.summary.name), more: true)
            }
            .buttonStyle(.plain)
            .padding(.horizontal, gutter)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 10) {
                    ForEach(c.items) { m in
                        NavigationLink(value: m) {
                            PosterCard(item: m, width: wide ? 140 : 112, showTitle: true,
                                       subtitle: m.id == item.id ? tr("item.collection.thisMovie") : nil)
                                .opacity(m.id == item.id ? 0.55 : 1)
                        }
                        .buttonStyle(.plain)
                        .disabled(m.id == item.id)
                        .cardMenu(m)
                    }
                }
            }
            .scrollIndicators(.hidden)
            .contentMargins(.horizontal, gutter, for: .scrollContent)
        }
    }

    private var similarRow: some View {
        VStack(alignment: .leading, spacing: 12) {
            RowHeader(title: tr("item.similar")).padding(.horizontal, gutter)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 10) {
                    ForEach(similar) { s in
                        NavigationLink(value: s) { PosterCard(item: s, width: wide ? 140 : 112, showTitle: wide) }
                            .buttonStyle(.plain)
                            .cardMenu(s)
                    }
                }
            }
            .scrollIndicators(.hidden)
            .contentMargins(.horizontal, gutter, for: .scrollContent)
        }
    }

    // MARK: Toolbar

    @ToolbarContentBuilder
    private var toolbar: some ToolbarContent {
        ToolbarItemGroup(placement: .topBarTrailing) {
            if let base = app.client?.baseURL {
                ShareLink(item: base.appending(path: "item/\(item.id)")) { Image(systemName: "square.and.arrow.up") }
            }
            Menu {
                let isWatched = watched ?? item.userState?.watched ?? false
                Button { Task { await toggleWatched(isWatched) } } label: {
                    Label(isWatched ? "item.markUnwatched" : "item.markWatched",
                          systemImage: isWatched ? "checkmark.circle.fill" : "checkmark.circle")
                }
                if let show = item.show, item.kind == .episode {
                    NavigationLink(value: ItemRoute(id: show.id)) { Label("card.goToShow", systemImage: "tv") }
                }
                if let target = playTarget, (target.userState?.positionMs ?? 0) > 0 {
                    Button { play(target, from: 0) } label: { Label("playback.startOver", systemImage: "arrow.counterclockwise") }
                }
            } label: { Image(systemName: "ellipsis") }
            .accessibilityLabel(Text("item.more"))
        }
    }

    // MARK: Loading

    private func load() async {
        guard let client = app.client else { return }
        do {
            detail = try await client.get("/items/\(summary.id)")
        } catch let e as APIError where e.status == 404 || e.status == 403 {
            unavailable = true
            return
        } catch {}
        watched = nil
        switch summary.kind {
        case .show:
            async let next = fetchUpNext(client)
            if let lib = item.libraryId {
                let page: ItemPage? = try? await client.get("/libraries/\(lib)/items", query: [
                    .init(name: "parentId", value: summary.id), .init(name: "limit", value: "200"),
                ])
                // Specials last, as on the web.
                seasons = (page?.items ?? []).sorted { a, b in
                    let x = a.seasonNumber ?? 0, y = b.seasonNumber ?? 0
                    return (x == 0 ? Int.max : x) < (y == 0 ? Int.max : y)
                }
            }
            upNext = await next
            let start = seasons.first { $0.id == season?.id } ?? seasons.first { $0.seasonNumber == upNext?.seasonNumber }
                ?? seasons.first { ($0.seasonNumber ?? 0) > 0 } ?? seasons.first
            if let start { season = start; await loadEpisodes(start) }
        case .season:
            upNext = await fetchUpNext(client)
            season = summary
            await loadEpisodes(summary)
        default:
            break
        }
        if let ref = detail?.metadata?.collection {
            collection = try? await client.get("/collections/\(ref.id)")
        }
        await loadSimilar(client)
    }

    private func setWatched(_ s: ItemSummary, _ on: Bool) async {
        guard let client = app.client else { return }
        _ = try? await client.send(on ? "PUT" : "DELETE", "/items/\(s.id)/watched", body: Optional<Int>.none)
        // Every page reloads, this one with it.
        NotificationCenter.default.post(name: .watchChanged, object: nil)
    }

    private func fetchUpNext(_ client: APIClient) async -> ItemSummary? {
        guard let (data, status) = try? await client.send("GET", "/items/\(summary.id)/up-next", body: Optional<Int>.none),
              status == 200 else { return nil }
        return (try? client.decode(data) as UpNext)?.item
    }

    private func loadEpisodes(_ s: ItemSummary) async {
        guard let client = app.client, let lib = s.libraryId ?? item.libraryId else { return }
        let page: ItemPage? = try? await client.get("/libraries/\(lib)/items", query: [
            .init(name: "parentId", value: s.id), .init(name: "limit", value: "200"),
        ])
        if season?.id == s.id { episodes = page?.items ?? [] }
    }

    /// Titles of the same library sharing its first genre, best rated first.
    private func loadSimilar(_ client: APIClient) async {
        guard item.kind == .movie || item.kind == .show, let lib = item.libraryId,
              let genre = detail?.metadata?.genres.first else { similar = []; return }
        let page: ItemPage? = try? await client.get("/libraries/\(lib)/items", query: [
            .init(name: "genre", value: String(genre.id)), .init(name: "sort", value: "rating"),
            .init(name: "order", value: "desc"), .init(name: "limit", value: "16"),
        ])
        similar = (page?.items ?? []).filter { $0.id != item.id }
    }

    private func toggleWatched(_ isWatched: Bool) async {
        guard let client = app.client else { return }
        do {
            try await client.send(isWatched ? "DELETE" : "PUT", "/items/\(item.id)/watched", body: Optional<Int>.none)
            watched = !isWatched
            if let s = season { await loadEpisodes(s) }
        } catch {}
    }
}

/// An episode in the phone's list: its still with progress, then the
/// number and length, the title and the synopsis.
private struct EpisodeRow: View {
    let episode: ItemSummary
    @Environment(\.play) private var play

    var body: some View {
        Button { play(episode, from: episode.userState?.watched == true ? 0 : (episode.userState?.positionMs ?? 0)) } label: {
            HStack(alignment: .top, spacing: 14) {
                EpisodeStill(episode: episode).frame(width: 150, height: 84)
                VStack(alignment: .leading, spacing: 0) {
                    Text(verbatim: episodeFacts(episode)).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                    Text(verbatim: episode.name).font(Theme.sans(15, .semibold)).lineLimit(2).padding(.top, 4)
                    if let o = episode.overview {
                        Text(verbatim: o).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary).lineLimit(2).padding(.top, 6)
                    }
                }
                Spacer(minLength: 0)
            }
            .padding(.vertical, 12)
            .overlay(alignment: .top) { Theme.hairline.frame(height: 1) }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

/// An episode in the iPad's grid: the still above its facts and title.
private struct EpisodeTile: View {
    let episode: ItemSummary
    @Environment(\.play) private var play

    var body: some View {
        Button { play(episode, from: episode.userState?.watched == true ? 0 : (episode.userState?.positionMs ?? 0)) } label: {
            VStack(alignment: .leading, spacing: 0) {
                EpisodeStill(episode: episode).aspectRatio(16 / 9, contentMode: .fit)
                Text(verbatim: episodeFacts(episode)).font(Theme.sans(11, .medium)).foregroundStyle(Theme.textTertiary).padding(.top, 10)
                Text(verbatim: episode.name).font(Theme.sans(15, .semibold)).lineLimit(1).padding(.top, 3)
                if let o = episode.overview {
                    Text(verbatim: o).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary).lineLimit(2).padding(.top, 4)
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

private struct EpisodeStill: View {
    let episode: ItemSummary

    var body: some View {
        ZStack(alignment: .bottom) {
            Theme.surface
            RemoteImage(art: episode.still ?? episode.backdrop, width: 600)
            if let p = episode.progress, episode.userState?.watched != true { ProgressThread(value: p).padding(8) }
        }
        .clipShape(RoundedRectangle(cornerRadius: 12))
        .overlay(alignment: .topTrailing) { WatchBadge(item: episode).padding(6) }
        .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(Color.white.opacity(0.07)))
    }
}

/// "Épisode 2 · 3 min".
private func episodeFacts(_ e: ItemSummary) -> String {
    [e.episodeNumber.map { tr("card.episode %lld", $0) }, e.runtimeMinutes.map { duration(minutes: $0) }]
        .compactMap { $0 }.joined(separator: " · ")
}

/// A person (IPh06 / IPa06): portrait or initials, their main job and how
/// many titles of theirs are here, then those titles.
struct PersonView: View {
    let route: PersonRoute
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @State private var person: Person?
    @State private var failed = false

    var body: some View {
        Group {
            if let person {
                if wide { wideLayout(person) } else { phoneLayout(person) }
            } else if failed {
                ErrorState { failed = false; Task { await load() } }
            } else {
                LoadingView()
            }
        }
        .background(Theme.bgBase)
        .toolbarBackgroundVisibility(.hidden, for: .navigationBar)
        .navigationBarTitleDisplayMode(.inline)
        .task { await load() }
    }

    private func phoneLayout(_ p: Person) -> some View {
        ScrollView {
            VStack(spacing: 0) {
                header(p, size: 116).frame(maxWidth: .infinity)
                LazyVGrid(columns: [GridItem(.flexible(), spacing: 18), GridItem(.flexible(), spacing: 18)], alignment: .leading, spacing: 20) {
                    ForEach(credits(p), id: \.item.id) { c in credit(c) }
                }
                .padding(.top, 24)
            }
            .padding(.horizontal, Theme.gutter)
            .padding(.bottom, 120)
        }
    }

    private func wideLayout(_ p: Person) -> some View {
        HStack(alignment: .center, spacing: 40) {
            header(p, size: 84, leading: true).frame(width: 260, alignment: .leading)
            VStack(alignment: .leading, spacing: 12) {
                RowHeader(title: tr("person.inLibrary"))
                ScrollView(.horizontal) {
                    LazyHStack(alignment: .top, spacing: 14) {
                        ForEach(credits(p), id: \.item.id) { c in credit(c).frame(width: 130) }
                    }
                }
                .scrollIndicators(.hidden)
            }
        }
        .padding(.horizontal, Theme.wideGutter)
        .frame(maxHeight: .infinity)
    }

    private func header(_ p: Person, size: CGFloat, leading: Bool = false) -> some View {
        VStack(alignment: leading ? .leading : .center, spacing: 0) {
            Portrait(art: p.portrait, name: p.name, size: size)
            Text(verbatim: p.name).displayTitle(34).multilineTextAlignment(leading ? .leading : .center).padding(.top, 18)
            Text(verbatim: summaryLine(p)).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary).padding(.top, 8)
        }
    }

    private func credit(_ c: PersonCredit) -> some View {
        NavigationLink(value: c.item) {
            GeometryReader { g in
                PosterCard(item: c.item, width: g.size.width, showTitle: true,
                           subtitle: [c.item.year.map(String.init), c.character ?? localized("person.role." + c.role)]
                               .compactMap { $0 }.joined(separator: " · "))
            }
            .aspectRatio(2 / 3.5, contentMode: .fit)
        }
        .buttonStyle(.plain)
    }

    /// One credit per title, newest first.
    private func credits(_ p: Person) -> [PersonCredit] {
        var seen = Set<String>()
        return p.credits.filter { seen.insert($0.item.id).inserted }.sorted { ($0.item.year ?? 0) < ($1.item.year ?? 0) }
    }

    /// "Production · 4 films chez toi".
    private func summaryLine(_ p: Person) -> String {
        let roles = Dictionary(grouping: p.credits, by: \.role).max { $0.value.count < $1.value.count }?.key
        let count = Set(p.credits.map(\.item.id)).count
        return ([roles.map { localized("person.role." + $0) }].compactMap { $0 } + [tr("person.titleCount %lld", count)])
            .joined(separator: " · ")
    }

    private func load() async {
        guard let client = app.client else { return }
        do { person = try await client.get("/people/\(route.id)") } catch { failed = true }
    }
}
