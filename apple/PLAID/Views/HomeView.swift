import SwiftUI

struct HomeView: View {
    @Environment(AppState.self) private var app
    @Environment(\.play) private var play
    @State private var home: Home?
    @State private var heroDetail: ItemDetail?
    @State private var failed = false
    @Environment(\.pageReady) private var pageReady
    /// The hero's play button, focused when the home first loads.
    @FocusState private var playFocused: Bool
    @State private var scroll = ScrollPosition(edge: .top)
    @Environment(\.pageFocusTick) private var pageFocusTick

    var body: some View {
        ScrollView(.vertical) {
            LazyVStack(alignment: .leading, spacing: 32) {
                if let home {
                    if let hero = home.ambient {
                        HomeHero(item: hero, detail: heroDetail, kicker: kicker(hero, home), playFocused: $playFocused)
                            .environment(\.onHeroFocus, { withAnimation(.smooth(duration: 0.35)) { scroll.scrollTo(edge: .top) } })
                    }
                    ForEach(home.rows) { row in
                        HomeRowView(row: row, title: title(row))
                    }
                } else if failed {
                    ErrorState { Task { await load() } }
                } else {
                    HeroPlaceholder()
                }
            }
            .padding(.bottom, 80)
            // Content starts right of the rail, so focus moves between them;
            // the hero's backdrop still runs under it.
            .padding(.leading, Theme.rail)
        }
        .scrollClipDisabled()
        .scrollPosition($scroll)
        .onChange(of: pageFocusTick) { playFocused = true }
        .onReceive(NotificationCenter.default.publisher(for: .watchChanged)) { _ in Task { await load() } }
        // The hero runs under the tab bar, edge to edge.
        .ignoresSafeArea(edges: [.horizontal, .top])
        .task { await load() }
    }

    private func load() async {
        guard let client = app.client else { return }
        failed = false
        do {
            let h: Home = try await client.get("/home")
            let firstLoad = home == nil
            withAnimation(.smooth(duration: 0.4)) { home = h }
            if let row = h.rows.first(where: { $0.type == .continueWatching }) {
                Task { await Shelf.update(row.items, client: client) }
            } else {
                Task { await Shelf.update([], client: client) }
            }
            if firstLoad {
                // Focus starts on the hero's play button, then the menu may take focus.
                try? await Task.sleep(for: .milliseconds(100))
                playFocused = true
                pageReady()
            }
            if let id = h.ambient?.id { heroDetail = try? await client.get("/items/\(id)") }
        } catch {
            failed = home == nil
        }
    }

    private func title(_ row: HomeRow) -> LocalizedStringKey {
        switch row.type {
        case .continueWatching: "home.row.continueWatching"
        case .recentlyAdded: "home.row.recentlyAdded \(row.libraryName ?? "")"
        case .genre, .unknown: "home.row.genre \(row.genre?.name ?? "")"
        }
    }

    private func kicker(_ hero: ItemSummary, _ home: Home) -> LocalizedStringKey {
        if hero.progress != nil || hero.kind == .episode { return "home.row.continueWatching" }
        let lib = home.rows.first { $0.libraryId == hero.libraryId }?.libraryName ?? ""
        return "home.hero.newIn \(lib)"
    }
}

/// The blurred backdrop that lights the top of a page, as on the web.
struct PageLight: View {
    let art: Artwork?

    var body: some View {
        RemoteImage(art: art, width: 780)
            .frame(height: 900)
            .blur(radius: 80)
            .brightness(-0.25)
            .opacity(0.55)
            .mask(LinearGradient(colors: [.black, .clear], startPoint: .top, endPoint: .bottom))
            .ignoresSafeArea()
            .allowsHitTesting(false)
    }
}

/// The hero of the home on TV: the backdrop edge to edge under the tab bar,
/// about half the screen, fading into the page at the bottom and the left.
struct HomeHero: View {
    let item: ItemSummary
    let detail: ItemDetail?
    let kicker: LocalizedStringKey
    var playFocused: FocusState<Bool>.Binding
    @Environment(\.play) private var play
    @Environment(\.previewsActive) private var previewsActive
    @State private var visible = true

    @Environment(AppState.self) private var app

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            Color.clear
            VStack(alignment: .leading, spacing: 14) {
                Text(kicker)
                    .textCase(.uppercase)
                    .font(Theme.sans(24, .semibold))
                    .tracking(2)
                    .foregroundStyle(Theme.textSecondary)
                Text(verbatim: item.show?.displayTitle ?? item.name)
                    .font(Theme.sans(76, .bold))
                    .tracking(-2)
                    .lineLimit(1)
                    .minimumScaleFactor(0.6)
                MetaLine(item: item, runtime: detail?.metadata?.runtimeMinutes)
                if let overview = detail?.metadata?.overview ?? item.overview {
                    Text(verbatim: overview)
                        .font(Theme.sans(28))
                        .lineSpacing(6)
                        .lineLimit(2)
                        .foregroundStyle(Theme.textSecondary)
                        .frame(maxWidth: 1100, alignment: .leading)
                }
                HStack(spacing: 24) {
                    Button { play(item, from: item.userState?.positionMs ?? 0) } label: {
                        Label(item.progress != nil ? "item.resume" : "item.play", systemImage: "play.fill")
                    }
                    .buttonStyle(PillButtonStyle())
                    .focused(playFocused)
                    NavigationLink(value: item.kind == .episode ? showSummary(item) : item) {
                        Label("card.moreInfo", systemImage: "info.circle")
                    }
                    .buttonStyle(PillButtonStyle(primary: false))
                    if previewId(item) != nil, app.previewsOn { PreviewSoundButton() }
                    if let p = item.progress {
                        TimeLeft(item: item, progress: p)
                    }
                }
                .fixedSize()
                .padding(.top, 8)
            }
            .frame(maxWidth: 1300, alignment: .leading)
            .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
            .padding(.bottom, 8)
        }
        .frame(height: 600)
        // Drawn under the rail too; the hero's own frame starts after it.
        .background {
            HeroBackdrop(art: item.backdrop ?? item.still, previewId: previewId(item), active: visible && previewsActive)
                .padding(.leading, -Theme.rail)
                .padding(.bottom, -Theme.heroFadeBelow)
        }
        .onScrollVisibilityChange(threshold: 0.4) { visible = $0 }
        .focusSection()
    }

    /// An episode's "More info" opens its show.
    private func showSummary(_ e: ItemSummary) -> ItemSummary { e }
}

/// A backdrop edge to edge that fades into the page at the bottom, and
/// darkens on the left under the rail and the text.
struct HeroBackdrop: View {
    let art: Artwork?
    /// The title whose scenes loop over the backdrop, if any.
    var previewId: String?
    var active = true

    @Environment(AppState.self) private var app

    var body: some View {
        ZStack {
            ZStack {
                RemoteImage(art: art, width: 1920)
                if let previewId, app.previewsOn { HeroPreview(itemId: previewId, active: active) }
            }
                // Eased fade, so the image has no visible bottom edge.
                .mask(LinearGradient(stops: [.init(color: .black, location: 0.45),
                                             .init(color: .black.opacity(0.55), location: 0.7),
                                             .init(color: .black.opacity(0.18), location: 0.87),
                                             .init(color: .clear, location: 1)],
                                     startPoint: .top, endPoint: .bottom))
            // Darkens from the bottom-left, under the text; the top of the
            // image stays clean, with no band beside the rail.
            LinearGradient(stops: [.init(color: Theme.bgBase.opacity(0.85), location: 0),
                                   .init(color: Theme.bgBase.opacity(0.45), location: 0.4),
                                   .init(color: .clear, location: 0.7)],
                           startPoint: .leading, endPoint: .trailing)
                .mask(LinearGradient(stops: [.init(color: .clear, location: 0.1),
                                             .init(color: .black, location: 0.6)],
                                     startPoint: .top, endPoint: .bottom))
        }
    }
}

/// Previews are cut for movies and shows; an episode plays its show's.
func previewId(_ item: ItemSummary) -> String? {
    switch item.kind {
    case .movie, .show: item.id
    case .episode, .season: item.show?.id
    }
}

func dominant(_ item: ItemSummary) -> Color {
    (item.backdrop?.colors?.dominant).map(Color.init(hex:)) ?? Theme.glow
}

struct HeroShade: View {
    var body: some View {
        ZStack {
            LinearGradient(
                stops: [.init(color: Theme.tint.opacity(0.85), location: 0),
                        .init(color: Theme.tint.opacity(0.45), location: 0.36),
                        .init(color: .clear, location: 0.65)],
                startPoint: .leading, endPoint: .trailing)
            LinearGradient(
                stops: [.init(color: Theme.tint.opacity(0.75), location: 0),
                        .init(color: .clear, location: 0.45)],
                startPoint: .bottom, endPoint: .top)
        }
    }
}

struct MetaLine: View {
    let item: ItemSummary
    var runtime: Int?
    /// Replaces the year (a show's years, status, seasons).
    var extra: [String] = []

    var body: some View {
        let head: [String?] = extra.isEmpty ? [item.year.map(String.init)] : extra
        let rest: [String?] = [
            item.kind == .episode ? episodeCode(item) : nil,
            item.kind == .show ? nil : (runtime ?? item.runtimeMinutes).map { Duration.seconds($0 * 60).formatted(.units(allowed: [.hours, .minutes], width: .narrow).locale(AppLanguage.locale)) },
            item.certification,
        ]
        let parts = (head + rest).compactMap { $0 } + item.genres.prefix(2)
        Text(verbatim: parts.joined(separator: " · "))
            .font(Theme.sans(26, .medium))
            .foregroundStyle(Theme.textSecondary)
    }
}

func episodeCode(_ item: ItemSummary) -> String? {
    guard let s = item.seasonNumber, let e = item.episodeNumber else { return nil }
    return tr("card.episodeShort %lld %lld", s, e)
}

struct TimeLeft: View {
    let item: ItemSummary
    let progress: Double

    var body: some View {
        HStack(spacing: 16) {
            ProgressThread(value: progress).frame(width: 160)
            if let s = item.userState, let d = s.durationMs {
                Text("card.timeLeft \(Duration.milliseconds(d - s.positionMs).formatted(.units(allowed: [.hours, .minutes], width: .narrow).locale(AppLanguage.locale)))")
                    .font(Theme.sans(24, .medium))
                    .foregroundStyle(Theme.textSecondary)
            }
        }
    }
}

struct HeroPlaceholder: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 56) {
            Shimmer().frame(height: 600)
                .mask(LinearGradient(colors: [.black, .clear], startPoint: .top, endPoint: .bottom))
                .padding(.leading, -(Theme.gutter + Theme.rail)).padding(.trailing, -Theme.trailing)
            HStack(spacing: 40) {
                ForEach(0..<6, id: \.self) { _ in
                    Shimmer().frame(width: 260, height: 390).clipShape(RoundedRectangle(cornerRadius: Theme.radius))
                }
            }
        }
        .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
    }
}

struct ErrorState: View {
    let retry: () -> Void
    @FocusState private var focused: Bool
    @Environment(\.pageFocusTick) private var pageFocusTick

    var body: some View {
        VStack(spacing: 32) {
            Text("error.generic").font(Theme.sans(32)).foregroundStyle(Theme.textSecondary)
            Button("common.retry", action: retry).buttonStyle(PillButtonStyle())
                .focused($focused)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, 200)
        .onAppear { focused = true }
        .onChange(of: pageFocusTick) { focused = true }
    }
}
