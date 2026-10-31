import SwiftUI

/// A movie, show, season or episode: hero, actions, episodes and cast.
struct ItemView: View {
    let summary: ItemSummary
    @Environment(AppState.self) private var app
    @Environment(\.play) private var play
    @State private var detail: ItemDetail?
    @State private var upNext: ItemSummary?
    @State private var seasons: [ItemSummary] = []
    @State private var season: ItemSummary?
    @State private var episodes: [ItemSummary] = []
    @State private var watched: Bool?
    @State private var visible = true
    @State private var scroll = ScrollPosition(edge: .top)
    @Environment(\.pageFocusTick) private var pageFocusTick
    @FocusState private var mainFocused: Bool
    @State private var collection: CollectionDetail?
    /// The version picked when a movie has several files.
    @State private var fileId: String?
    @Environment(\.previewsActive) private var previewsActive

    private var item: ItemSummary { detail?.summary ?? summary }
    private var hasEpisodes: Bool { item.kind == .show || item.kind == .season }

    /// An official trailer in the profile's language first, then any
    /// trailer, then a teaser (TMDB lists them by language).
    private var bestTrailer: Trailer? {
        let all = (detail?.metadata?.trailers ?? []).filter { $0.site == "YouTube" }
        let lang = (app.language ?? AppLanguage.locale.language.languageCode?.identifier ?? "en").prefix(2)
        func rank(_ t: Trailer) -> Int {
            (t.language.hasPrefix(lang) ? 0 : 4) + (t.type == "Trailer" ? 0 : 2) + (t.official ? 0 : 1)
        }
        return all.min { rank($0) < rank($1) }
    }


    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: hasEpisodes ? 20 : 56) {
                hero
                    .environment(\.onHeroFocus, { withAnimation(.smooth(duration: 0.35)) { scroll.scrollTo(edge: .top) } })
                if item.kind == .show || item.kind == .season {
                    seasonsSection
                }
                if let collection, collection.items.count > 1 {
                    ItemRow(title: "item.inCollection \(collection.summary.name)", items: collection.items)
                }
                if let detail {
                    AboutSection(detail: detail, seasonCount: seasons.filter { ($0.seasonNumber ?? 0) > 0 }.count)
                }
                if let credits = detail?.metadata?.credits, !credits.isEmpty {
                    CastRow(credits: credits)
                }
            }
            .padding(.bottom, 80)
            .padding(.leading, Theme.rail)
        }
        .scrollClipDisabled()
        .scrollPosition($scroll)
        .onChange(of: pageFocusTick) { mainFocused = true }
        .ignoresSafeArea(edges: [.horizontal, .top])
        .task { await load() }
    }

    private var hero: some View {
        ZStack(alignment: .bottomLeading) {
            Color.clear
            VStack(alignment: .leading, spacing: 16) {
                if let show = item.show {
                    Text(verbatim: [show.displayTitle, item.seasonNumber.map { tr("item.season %lld", $0) }]
                        .compactMap { $0 }.joined(separator: " · "))
                        .font(Theme.sans(26, .semibold))
                        .foregroundStyle(Theme.textSecondary)
                }
                Text(verbatim: item.name)
                    .font(Theme.sans(item.kind == .movie ? 88 : 72, .bold))
                    .tracking(-2)
                    .lineLimit(2)
                    .minimumScaleFactor(0.6)
                MetaLine(item: item, runtime: detail?.metadata?.runtimeMinutes, extra: showFacts)
                if let rating = item.rating, rating > 0 {
                    Text(verbatim: "★ " + rating.formatted(.number.precision(.fractionLength(1)).locale(AppLanguage.locale)))
                        .font(Theme.sans(24, .semibold))
                        .foregroundStyle(Theme.textSecondary)
                }
                if let overview = detail?.metadata?.overview ?? item.overview {
                    Text(verbatim: overview)
                        .font(Theme.sans(28))
                        .lineSpacing(6)
                        .lineLimit(hasEpisodes ? 2 : 4)
                        .foregroundStyle(Theme.textSecondary)
                }
                actions.fixedSize().padding(.top, 8)
            }
            .frame(maxWidth: 1300, alignment: .leading)
            .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
            .padding(.bottom, 8)
        }
        // A show keeps its seasons and first episodes on screen.
        .frame(height: hasEpisodes ? 540 : 760)
        .background {
            HeroBackdrop(art: detail?.metadata?.backdrops.first ?? item.backdrop ?? item.still,
                         previewId: item.kind == .movie || item.kind == .show ? item.id : nil,
                         active: visible && previewsActive)
                .padding(.leading, -Theme.rail)
                // A show's backdrop runs to about two thirds of the screen,
                // under its seasons and episodes, though the hero is shorter.
                .padding(.bottom, -Theme.heroFadeBelow - (hasEpisodes ? 200 : 0))
        }
        .onScrollVisibilityChange(threshold: 0.4) { visible = $0 }
        .focusSection()
    }

    @ViewBuilder
    private var actions: some View {
        HStack(spacing: 24) {
            if item.kind == .movie || item.kind == .episode {
                let pos = item.userState?.positionMs ?? 0
                Button { play(item, from: pos, fileId: fileId) } label: {
                    Label(pos > 0 ? "item.resume" : "item.play", systemImage: "play.fill")
                }
                .buttonStyle(PillButtonStyle())
                .focused($mainFocused)
                if pos > 0 {
                    Button { play(item, from: 0, fileId: fileId) } label: {
                        Label("playback.startOver", systemImage: "arrow.counterclockwise")
                    }
                    .buttonStyle(PillButtonStyle(primary: false))
                }
            } else if let next = upNext {
                let pos = next.userState?.positionMs ?? 0
                Button { play(next, from: pos) } label: {
                    Label(pos > 0 ? "item.resumeEpisode \(episodeCode(next) ?? "")" : "item.playEpisode \(episodeCode(next) ?? "")",
                          systemImage: "play.fill")
                }
                .buttonStyle(PillButtonStyle())
                .focused($mainFocused)
            }
            if versions.count > 1 {
                Menu {
                    ForEach(versions) { f in
                        Button { fileId = f.id } label: {
                            if f.id == (fileId ?? versions.first?.id) { Label(versionLabel(f), systemImage: "checkmark") } else { Text(verbatim: versionLabel(f)) }
                        }
                    }
                } label: {
                    Label(versionLabel(versions.first { $0.id == fileId } ?? versions[0]), systemImage: "film.stack")
                }
                .buttonStyle(PillButtonStyle(primary: false))
            }
            if let trailer = bestTrailer { TrailerButton(trailer: trailer) }
            if item.kind == .movie || item.kind == .show, app.previewsOn { PreviewSoundButton() }
            let isWatched = watched ?? item.userState?.watched ?? false
            Button { Task { await toggleWatched(isWatched) } } label: {
                Label(isWatched ? "item.markUnwatched" : "item.markWatched",
                      systemImage: isWatched ? "checkmark.circle.fill" : "checkmark.circle")
            }
            .buttonStyle(PillButtonStyle(primary: false))
            if let p = item.progress { TimeLeft(item: item, progress: p) }
        }
    }

    @ViewBuilder
    private var seasonsSection: some View {
        VStack(alignment: .leading, spacing: 24) {
            if seasons.count > 1 {
                ScrollView(.horizontal) {
                    HStack(spacing: 20) {
                        ForEach(seasons) { s in
                            Button {
                                season = s
                                Task { await loadEpisodes(s) }
                            } label: {
                                Text(verbatim: s.seasonNumber == 0 ? tr("item.specials")
                                    : s.name.isEmpty ? tr("item.season %lld", s.seasonNumber ?? 0) : s.name)
                            }
                            .buttonStyle(SeasonPillStyle(selected: season?.id == s.id))
                        }
                        if let season, item.kind == .show {
                            let seen = season.userState?.watched ?? false
                            Button { Task { await setWatched(season, !seen) } } label: {
                                Label(seen ? "item.markSeasonUnwatched" : "item.markSeasonWatched",
                                      systemImage: seen ? "checkmark.circle.fill" : "checkmark.circle")
                            }
                            .buttonStyle(SeasonPillStyle(selected: false))
                            .padding(.leading, 20)
                        }
                    }
                    .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                    .padding(.vertical, 12)
                }
                .scrollClipDisabled()
                .focusSection()
            }
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 40) {
                    ForEach(episodes) { EpisodeCard(item: $0) }
                }
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                .padding(.vertical, 24)
            }
            .scrollClipDisabled()
            .focusSection()
        }
    }

    private func load() async {
        guard let client = app.client else { return }
        detail = try? await client.get("/items/\(summary.id)")
        switch summary.kind {
        case .show:
            async let next: ItemSummary? = fetchUpNext(client)
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
            let start = seasons.first { $0.seasonNumber == upNext?.seasonNumber } ?? seasons.first { ($0.seasonNumber ?? 0) > 0 } ?? seasons.first
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
    }

    /// A show's years, status and number of seasons.
    private var showFacts: [String] {
        guard item.kind == .show, let m = detail?.metadata else { return [] }
        var out: [String] = []
        if let first = item.year {
            let last = m.endDate.flatMap { Int($0.prefix(4)) }
            out.append(last.map { $0 == first ? "\(first)" : "\(first)–\($0)" } ?? "\(first)–")
        }
        if let status = m.showStatus { out.append(localized("item.status." + status)) }
        let n = seasons.filter { ($0.seasonNumber ?? 0) > 0 }.count
        if n > 0 { out.append(tr("item.seasonCount %lld", n)) }
        return out
    }

    /// The files a movie can play, longest first.
    private var versions: [MediaFile] {
        guard item.kind == .movie else { return [] }
        return (detail?.files ?? []).filter { $0.missingSince == nil }
    }

    private func versionLabel(_ f: MediaFile) -> String {
        [f.edition, resolutionLabel(f.height), f.hdrFormat.map(hdrLabel), f.videoCodec?.uppercased()]
            .compactMap { $0 }.joined(separator: " · ")
    }

    private func setWatched(_ s: ItemSummary, _ on: Bool) async {
        guard let client = app.client else { return }
        _ = try? await client.send(on ? "PUT" : "DELETE", "/items/\(s.id)/watched", body: Optional<Int>.none)
        // Fresh watch marks for the season and its episodes.
        if let lib = item.libraryId, let page: ItemPage = try? await client.get("/libraries/\(lib)/items", query: [
            .init(name: "parentId", value: summary.id), .init(name: "limit", value: "200"),
        ]) {
            seasons = page.items.sorted { a, b in
                let x = a.seasonNumber ?? 0, y = b.seasonNumber ?? 0
                return (x == 0 ? Int.max : x) < (y == 0 ? Int.max : y)
            }
            season = seasons.first { $0.id == s.id } ?? season
        }
        if let season { await loadEpisodes(season) }
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

    private func toggleWatched(_ isWatched: Bool) async {
        guard let client = app.client else { return }
        do {
            try await client.send(isWatched ? "DELETE" : "PUT", "/items/\(item.id)/watched", body: Optional<Int>.none)
            watched = !isWatched
            if let s = season { await loadEpisodes(s) }
        } catch {}
    }
}

struct SeasonPillStyle: ButtonStyle {
    let selected: Bool

    func makeBody(configuration: Configuration) -> some View {
        Pill(configuration: configuration, selected: selected)
    }

    private struct Pill: View {
        let configuration: ButtonStyleConfiguration
        let selected: Bool
        @Environment(\.isFocused) private var focused

        var body: some View {
            configuration.label
                .font(Theme.sans(26, .semibold))
                .padding(.horizontal, 32)
                .frame(height: 60)
                .foregroundStyle(focused ? Theme.onAction : Theme.textPrimary)
                .background(Capsule().fill(focused ? Theme.action : selected ? Color.white.opacity(0.32) : Theme.control))
                .scaleEffect(focused ? 1.06 : 1)
                .animation(.smooth(duration: 0.25), value: focused)
        }
    }
}

struct EpisodeCard: View {
    let item: ItemSummary
    @Environment(\.play) private var play

    var body: some View {
        Button { play(item, from: item.userState?.positionMs ?? 0) } label: {
            VStack(alignment: .leading, spacing: 10) {
                CardArt(art: item.still, width: 780, fallback: item.name)
                    .frame(width: 480, height: 270)
                    .overlay(alignment: .topTrailing) { WatchBadge(item: item) }
                    .overlay(alignment: .bottom) {
                        if let p = item.progress { ProgressThread(value: p).padding(16) }
                    }
                Text("card.episode \(item.episodeNumber ?? 0)")
                    .font(Theme.sans(22, .semibold))
                    .foregroundStyle(Theme.textTertiary)
                Text(verbatim: item.name)
                    .font(Theme.sans(28, .semibold))
                    .lineLimit(1)
                if let o = item.overview {
                    Text(verbatim: o)
                        .font(Theme.sans(22))
                        .foregroundStyle(Theme.textSecondary)
                        .lineLimit(3)
                }
            }
            .frame(width: 480, alignment: .leading)
        }
        .buttonStyle(CardButtonStyle())
    }
}

struct CastRow: View {
    let credits: [Credit]

    var body: some View {
        let cast = Array(credits.filter { $0.role == "cast" }.prefix(20)) + credits.filter { $0.role != "cast" }
        VStack(alignment: .leading, spacing: 8) {
            Text("item.castAndCrew")
                .font(Theme.sans(34, .semibold))
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 40) {
                    ForEach(Array(cast.enumerated()), id: \.offset) { _, c in
                        NavigationLink(value: PersonRoute(id: c.personId, name: c.name)) {
                            VStack(spacing: 12) {
                                ZStack {
                                    Circle().fill(Theme.surface)
                                    Text(verbatim: c.name.split(separator: " ").compactMap(\.first).prefix(2).map(String.init).joined())
                                        .font(Theme.sans(44, .semibold))
                                        .foregroundStyle(Theme.textTertiary)
                                    RemoteImage(art: c.portrait, width: 342)
                                }
                                .frame(width: 180, height: 180)
                                .clipShape(Circle())
                                Text(verbatim: c.name).font(Theme.sans(22, .semibold)).lineLimit(1)
                                Text(verbatim: c.character ?? localized("person.role." + c.role))
                                    .font(Theme.sans(20, .medium))
                                    .foregroundStyle(Theme.textTertiary)
                                    .lineLimit(1)
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
