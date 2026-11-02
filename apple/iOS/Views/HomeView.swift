import SwiftUI

/// Home (IPh01 / IPa01): a full-bleed hero that pages through a few titles,
/// then the server's rows, "Continue watching" as stills and the rest as
/// posters.
struct HomeView: View {
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @State private var home: Home?
    @State private var failed = false
    @State private var heroIndex = 0
    /// The status bar's height: the page runs under it, the top bar doesn't.
    @Environment(\.topInset) private var topInset

    /// The ambient title first, then the next ones with a backdrop.
    private var heroes: [ItemSummary] {
        guard let home else { return [] }
        let candidates = [home.ambient].compactMap { $0 } + home.rows.flatMap(\.items).filter { $0.kind != .episode }
        return Array(unique(candidates.filter { $0.backdrop != nil }).prefix(4))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                hero
                // Live and upcoming Plaid Parties, only when there are some (F010-R40).
                HomePartiesRow()
                if let home {
                    ForEach(home.rows) { row in
                        if !row.items.isEmpty { HomeRowView(row: row).padding(.top, 26) }
                    }
                    if heroes.isEmpty && home.rows.allSatisfy(\.items.isEmpty) {
                        EmptyHome { Task { await load() } }
                    }
                } else if failed {
                    ErrorState { failed = false; Task { await load() } }.frame(height: 200)
                } else {
                    VStack(alignment: .leading, spacing: 30) {
                        RowPlaceholder(landscape: true)
                        RowPlaceholder()
                    }
                    .padding(.leading, wide ? Theme.wideGutter : Theme.gutter)
                    .padding(.top, 26)
                }
            }
            .padding(.bottom, 120)
        }
        .background(alignment: .top) {
            AmbientGlow(art: heroes[safe: heroIndex]?.backdrop, brightness: -0.58)
                .frame(height: 1300)
                .animation(.smooth(duration: 0.8), value: heroIndex)
                .ignoresSafeArea()
        }
        .background(Theme.bgBase)
        .ignoresSafeArea(edges: .top)
        .toolbarVisibility(.hidden, for: .navigationBar)
        .refreshable { await load() }
        .task { if home == nil { await load() } }
        .onReceive(NotificationCenter.default.publisher(for: .watchChanged)) { _ in Task { await load() } }
    }

    private var hero: some View {
        let height: CGFloat = wide ? 520 : 540
        return ZStack(alignment: .top) {
            TabView(selection: $heroIndex) {
                ForEach(Array(heroes.enumerated()), id: \.element.id) { i, item in
                    HeroPage(item: item, height: height, current: i == heroIndex).tag(i)
                }
            }
            .tabViewStyle(.page(indexDisplayMode: .never))
            .frame(height: height)
            .overlay {
                if heroes.isEmpty { Shimmer().opacity(home == nil ? 0.5 : 0).allowsHitTesting(false) }
            }
            HStack {
                if !wide {
                    HStack(spacing: 9) {
                        LogoMark(size: 30)
                        Wordmark(height: 21)
                    }
                }
                Spacer()
                ProfileMenu()
            }
            .padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
            .padding(.top, topInset + (wide ? 8 : 4))
        }
        .overlay(alignment: .bottom) {
            if heroes.count > 1 {
                HStack(spacing: 6) {
                    ForEach(heroes.indices, id: \.self) { i in
                        Capsule().fill(Theme.textPrimary.opacity(i == heroIndex ? 1 : 0.35))
                            .frame(width: i == heroIndex ? 18 : 6, height: 6)
                    }
                }
                .animation(.smooth, value: heroIndex)
                .frame(maxWidth: .infinity, alignment: wide ? .trailing : .center)
                .padding(.horizontal, wide ? Theme.wideGutter : 0)
                .padding(.bottom, wide ? 22 : 26)
                .accessibilityHidden(true)
            }
        }
    }

    private func load() async {
        guard let client = app.client else { return }
        do {
            let h: Home = try await client.get("/home")
            withAnimation(.smooth) { home = h }
            failed = false
        } catch {
            if home == nil { failed = true }
        }
    }
}

/// One title of the hero: its backdrop fading into the page, the facts, the
/// title, then Play or Resume and the way to its page.
private struct HeroPage: View {
    let item: ItemSummary
    let height: CGFloat
    /// The page shown: only its preview plays.
    let current: Bool
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @Environment(\.play) private var play
    @Environment(\.previewsActive) private var previewsActive

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            RemoteImage(art: item.backdrop, width: wide ? 2400 : 1200)
                .frame(height: height)
                .overlay {
                    if app.previewsOn, let id = previewId(item) { HeroPreview(itemId: id, active: current && previewsActive) }
                }
                .mask(LinearGradient(stops: [.init(color: .black, location: 0.55), .init(color: .clear, location: 1)],
                                     startPoint: .top, endPoint: .bottom))
                .overlay {
                    LinearGradient(stops: [
                        .init(color: Theme.bgBase.opacity(0.35), location: 0), .init(color: .clear, location: 0.22),
                        .init(color: .clear, location: 0.5), .init(color: Theme.bgBase.opacity(0.45), location: 0.84),
                        .init(color: .clear, location: 1),
                    ], startPoint: .top, endPoint: .bottom)
                }
                .overlay {
                    if wide {
                        LinearGradient(colors: [Theme.bgBase.opacity(0.6), .clear], startPoint: .leading, endPoint: .center)
                    }
                }
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 0) {
                Text(verbatim: heroFacts(item)).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                Text(verbatim: item.name)
                    .displayTitle(44)
                    .lineLimit(2).minimumScaleFactor(0.7)
                    .padding(.top, 10)
                if wide, let o = item.overview {
                    Text(verbatim: o).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                        .lineLimit(2).frame(maxWidth: 420, alignment: .leading).padding(.top, 12)
                }
                HStack(spacing: 10) {
                    Button { play(item, from: item.userState?.positionMs ?? 0) } label: {
                        Label { Text(verbatim: playLabel(item)) } icon: { Image(systemName: "play.fill") }
                    }
                    .buttonStyle(CreamButtonStyle())
                    .frame(maxWidth: wide ? 200 : .infinity)
                    NavigationLink(value: item) {
                        Image(systemName: "info.circle").font(.system(size: 19, weight: .medium)).frame(width: 50, height: 50)
                    }
                    .buttonStyle(.glass).buttonBorderShape(.circle)
                    .accessibilityLabel(Text("card.moreInfo"))
                    if app.previewsOn { PreviewSoundButton() }
                }
                .padding(.top, 18)
            }
            .padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
            .padding(.bottom, wide ? 56 : 46)
        }
        .frame(height: height)
    }
}

/// "Film · 2012 · Science-fiction" over a hero.
func heroFacts(_ item: ItemSummary) -> String {
    let kind = item.kind == .show ? tr("kind.show") : tr("kind.movie")
    var parts = [kind] + [item.year.map(String.init)].compactMap { $0 } + item.genres.prefix(1)
    if let m = item.runtimeMinutes, item.kind == .movie { parts.append(duration(minutes: m)) }
    return parts.joined(separator: " · ")
}

/// "Resume · 12 min" when started, else "Play".
func playLabel(_ item: ItemSummary) -> String {
    if let left = timeLeft(item) { return tr("item.resumeLeft %@", left) }
    return tr("item.play")
}

/// A home row: stills for "Continue watching", posters otherwise; its
/// chevron opens the whole list when the server offers one.
struct HomeRowView: View {
    let row: HomeRow
    @Environment(\.isWide) private var wide

    private var title: String {
        switch row.type {
        case .continueWatching: tr("home.row.continueWatching")
        case .recentlyAdded: row.libraryName.map { tr("home.row.recentlyAdded %@", $0) } ?? tr("home.row.recentlyAddedAll")
        case .genre: row.genre?.name ?? ""
        case .unknown: ""
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Group {
                if let seeAll = row.seeAll {
                    NavigationLink(value: SeeAllRoute(seeAll: seeAll, title: title)) { RowHeader(title: title, more: true) }
                        .buttonStyle(.plain)
                } else {
                    RowHeader(title: title)
                }
            }
            .padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: row.type == .continueWatching ? 12 : 10) {
                    ForEach(row.items) { item in
                        NavigationLink(value: item) {
                            if row.type == .continueWatching {
                                LandscapeCard(item: item, width: wide ? 300 : 236)
                            } else {
                                PosterCard(item: item, width: wide ? 140 : 112, showTitle: wide)
                            }
                        }
                        .buttonStyle(.plain)
                        .cardMenu(item)
                    }
                }
                .scrollTargetLayout()
            }
            .scrollIndicators(.hidden)
            .scrollTargetBehavior(.viewAligned)
            .contentMargins(.horizontal, wide ? Theme.wideGutter : Theme.gutter, for: .scrollContent)
        }
    }
}

/// A home row's whole list: the library grid with the row's sort and genre.
struct SeeAllRoute: Hashable {
    let seeAll: SeeAll
    let title: String
}

extension Array {
    subscript(safe i: Int) -> Element? { indices.contains(i) ? self[i] : nil }
}

/// Previews are cut for movies and shows; an episode plays its show's.
func previewId(_ item: ItemSummary) -> String? {
    switch item.kind {
    case .movie, .show: item.id
    case .episode, .season: item.show?.id
    }
}

/// Nothing to watch yet: the libraries are being scanned (with what was
/// found so far), or there are none and an admin has to add one.
private struct EmptyHome: View {
    let reload: () -> Void
    @Environment(AppState.self) private var app
    @State private var found: Int?

    var body: some View {
        VStack(spacing: 10) {
            Image(systemName: found == nil ? "film.stack" : "sparkles.tv")
                .font(.system(size: 34, weight: .medium)).foregroundStyle(Theme.textTertiary)
            if let found {
                Text("home.empty.scanning.title").font(Theme.display(22))
                Text("home.empty.scanning.body \(found)").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                ProgressView().tint(Theme.textSecondary).padding(.top, 6)
            } else {
                Text("home.empty.title").font(Theme.display(22))
                Text("home.empty.member.body").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
            }
        }
        .multilineTextAlignment(.center)
        .padding(32)
        .frame(maxWidth: .infinity)
        .padding(.top, 20)
        // While a scan runs, the home is asked again every few seconds.
        .task {
            while !Task.isCancelled {
                guard let libs: [Library] = try? await app.client?.get("/libraries") else { return }
                let busy = libs.compactMap(\.scan).filter { $0.state != "idle" }
                found = busy.isEmpty ? nil : busy.reduce(0) { $0 + $1.counters.found }
                if busy.isEmpty { return }
                try? await Task.sleep(for: .seconds(5))
                reload()
            }
        }
    }
}
