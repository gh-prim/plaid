import SwiftUI

/// A titled row of cards that scrolls sideways.
struct ItemRow: View {
    let title: LocalizedStringKey
    let items: [ItemSummary]
    var landscape = false

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(Theme.sans(34, .semibold))
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 40) {
                    ForEach(items) { item in
                        if landscape {
                            LandscapeCard(item: item)
                        } else {
                            PosterCard(item: item)
                        }
                    }
                }
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                .padding(.vertical, 36)
            }
            .scrollClipDisabled()
        }
        .focusSection()
    }
}

/// Lifts the focused card with the amber ring of the web.
struct CardButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        CardBody(configuration: configuration)
    }
}

private struct CardBody: View {
    let configuration: ButtonStyleConfiguration
    @Environment(\.isFocused) private var focused

    var body: some View {
        configuration.label
            .scaleEffect(focused ? 1.08 : 1)
            .brightness(focused ? 0 : -0.06)
            .animation(.timingCurve(0.32, 0.72, 0, 1, duration: 0.35), value: focused)
    }
}

struct CardArt: View {
    let art: Artwork?
    let width: Int
    let fallback: String
    @Environment(\.isFocused) private var focused

    var body: some View {
        ZStack {
            Theme.surface
            if art == nil {
                Text(verbatim: fallback)
                    .font(Theme.sans(26, .semibold))
                    .multilineTextAlignment(.center)
                    .foregroundStyle(Theme.textTertiary)
                    .padding(20)
            }
            RemoteImage(art: art, width: width)
        }
        .clipShape(RoundedRectangle(cornerRadius: Theme.radius))
        .shadow(color: .black.opacity(focused ? 0.6 : 0.35), radius: focused ? 30 : 10, y: focused ? 24 : 6)
    }
}

struct PosterCard: View {
    let item: ItemSummary
    var width: CGFloat = 260

    var body: some View {
        NavigationLink(value: item) {
            CardArt(art: item.poster, width: 500, fallback: item.name)
                .frame(width: width, height: width * 1.5)
                .overlay(alignment: .topTrailing) { WatchBadge(item: item) }
                .overlay(alignment: .bottom) {
                    if let p = item.progress {
                        ProgressThread(value: p).padding(14)
                    }
                }
        }
        .buttonStyle(CardButtonStyle())
        .cardMenu(item)
    }
}

/// Continue watching: a still, its progress, and plays at once.
struct LandscapeCard: View {
    let item: ItemSummary
    @Environment(\.play) private var play

    var body: some View {
        Button { play(item, from: item.userState?.positionMs ?? 0) } label: {
            VStack(alignment: .leading, spacing: 10) {
                CardArt(art: item.still ?? item.backdrop, width: 780, fallback: item.name)
                    .frame(width: 440, height: 248)
                    .overlay(alignment: .bottom) {
                        if let p = item.progress { ProgressThread(value: p).padding(16) }
                    }
                Text(verbatim: item.show?.displayTitle ?? item.name)
                    .font(Theme.sans(26, .semibold))
                    .lineLimit(1)
                Text(verbatim: subtitle)
                    .font(Theme.sans(22))
                    .foregroundStyle(Theme.textTertiary)
                    .lineLimit(1)
            }
            .frame(width: 440, alignment: .leading)
        }
        .buttonStyle(CardButtonStyle())
        .cardMenu(item)
    }

    private var subtitle: String {
        var parts: [String] = []
        if item.kind == .episode {
            if let code = episodeCode(item) { parts.append(code) }
            if !item.name.isEmpty { parts.append(item.name) }
        }
        if let s = item.userState, let d = s.durationMs, d > s.positionMs {
            let left = Duration.milliseconds(d - s.positionMs).formatted(.units(allowed: [.hours, .minutes], width: .narrow).locale(AppLanguage.locale))
            parts.append(tr("card.timeLeft %@", left))
        }
        return parts.joined(separator: " · ")
    }
}

struct WatchBadge: View {
    let item: ItemSummary

    var body: some View {
        Group {
            if let s = item.userState {
                if s.watched {
                    Image(systemName: "checkmark").font(.system(size: 20, weight: .bold))
                        .padding(10)
                } else if let total = s.totalEpisodes, let seen = s.watchedEpisodes, total > seen, item.kind == .show {
                    Text(verbatim: "\(total - seen)").font(Theme.sans(20, .bold))
                        .padding(.horizontal, 12).padding(.vertical, 6)
                }
            }
        }
        .foregroundStyle(Theme.textPrimary)
        .background(Theme.tint.opacity(0.78), in: Capsule())
        .padding(12)
    }
}

/// Sent when a card's menu changed what was watched: pages reload.
extension Notification.Name {
    static let watchChanged = Notification.Name("plaid.watchChanged")
}

/// A long press on a card (F008 R8): mark watched or unwatched, and for an
/// episode, go to its show.
private struct CardMenu: ViewModifier {
    let item: ItemSummary
    @Environment(AppState.self) private var app

    func body(content: Content) -> some View {
        content.contextMenu {
            let watched = item.userState?.watched ?? false
            Button {
                Task {
                    guard let client = app.client else { return }
                    _ = try? await client.send(watched ? "DELETE" : "PUT", "/items/\(item.id)/watched", body: Optional<Int>.none)
                    NotificationCenter.default.post(name: .watchChanged, object: nil)
                }
            } label: {
                Label(watched ? "item.markUnwatched" : "item.markWatched",
                      systemImage: watched ? "checkmark.circle.fill" : "checkmark.circle")
            }
            if let show = item.show {
                NavigationLink(value: ItemRoute(id: show.id)) {
                    Label("card.goToShow", systemImage: "tv")
                }
            }
        }
    }
}

extension View {
    func cardMenu(_ item: ItemSummary) -> some View { modifier(CardMenu(item: item)) }
}

/// An item known only by its id (a show from an episode's menu).
struct ItemRoute: Hashable { let id: String }

struct ItemByIdView: View {
    let route: ItemRoute
    @Environment(AppState.self) private var app
    @State private var summary: ItemSummary?
    @State private var failed = false
    @Environment(\.linkReady) private var linkReady

    var body: some View {
        Group {
            if let summary {
                ItemView(summary: summary)
            } else if failed {
                ErrorState { Task { await load() } }
            } else {
                LoadingView()
            }
        }
        .task { await load() }
    }

    /// A gone or unreachable item shows the error and a retry, never an endless spinner.
    private func load() async {
        guard let client = app.client else { failed = true; return }
        failed = false
        do {
            let d: ItemDetail = try await client.get("/items/\(route.id)")
            summary = d.summary
        } catch {
            failed = true
        }
        // The page's main control takes focus first, then the menu may.
        try? await Task.sleep(for: .milliseconds(300))
        linkReady()
    }
}

/// A home row that loads its next pages as it scrolls, and ends with
/// "See all" when the server gives the grid query (F008 R31).
struct HomeRowView: View {
    let row: HomeRow
    let title: LocalizedStringKey
    @Environment(AppState.self) private var app
    @State private var items: [ItemSummary]
    @State private var cursor: String?
    @State private var loading = false

    init(row: HomeRow, title: LocalizedStringKey) {
        self.row = row
        self.title = title
        _items = State(initialValue: row.items)
        _cursor = State(initialValue: row.nextCursor)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(Theme.sans(34, .semibold))
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 40) {
                    ForEach(items) { item in
                        Group {
                            if row.type == .continueWatching { LandscapeCard(item: item) } else { PosterCard(item: item) }
                        }
                        .onAppear { if item.id == items.last?.id { Task { await more() } } }
                    }
                    if let seeAll = row.seeAll, cursor == nil || items.count >= 40 {
                        NavigationLink(value: SeeAllRoute(seeAll: seeAll, title: row.libraryName ?? "")) {
                            VStack(spacing: 16) {
                                Image(systemName: "square.grid.2x2").font(.system(size: 44, weight: .semibold))
                                Text("home.row.seeAll").font(Theme.sans(26, .semibold))
                            }
                            .frame(width: 260, height: row.type == .continueWatching ? 248 : 390)
                            .background(RoundedRectangle(cornerRadius: Theme.radius).fill(Theme.control))
                        }
                        .buttonStyle(CardButtonStyle())
                    }
                }
                .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                .padding(.vertical, 36)
            }
            .scrollClipDisabled()
        }
        .focusSection()
    }

    private func more() async {
        guard !loading, let cursor, let client = app.client else { return }
        loading = true
        defer { loading = false }
        guard let page: ItemPage = try? await client.get("/home/rows/\(row.id)", query: [
            .init(name: "cursor", value: cursor), .init(name: "limit", value: "20"),
        ]) else { return }
        items += page.items.filter { new in !items.contains { $0.id == new.id } }
        self.cursor = page.nextCursor
    }
}

/// A home row's "See all": its library's grid, sorted and filtered alike.
struct SeeAllRoute: Hashable {
    let seeAll: SeeAll
    let title: String
}
