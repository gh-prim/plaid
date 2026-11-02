import SwiftUI

/// A poster (2:3) with, when asked, its title and a line under it.
struct PosterCard: View {
    let item: ItemSummary
    var width: CGFloat = 112
    var showTitle = false
    var subtitle: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ZStack {
                Theme.surface
                RemoteImage(art: item.poster, width: Int(width * 3))
                if item.poster == nil {
                    Text(verbatim: item.name).font(Theme.sans(13, .semibold)).multilineTextAlignment(.center)
                        .foregroundStyle(Theme.textSecondary).padding(8)
                }
            }
            .frame(width: width, height: width * 1.5)
            .clipShape(RoundedRectangle(cornerRadius: width > 120 ? 11 : 9))
            .overlay(RoundedRectangle(cornerRadius: width > 120 ? 11 : 9).strokeBorder(Color.white.opacity(0.07)))
            .overlay(alignment: .topTrailing) { WatchBadge(item: item).padding(6) }
            .shadow(color: .black.opacity(0.6), radius: 17, y: 18)
            if showTitle {
                Text(verbatim: item.name)
                    .font(Theme.sans(13, .semibold)).lineLimit(1)
                    .padding(.top, 8)
                if let sub = subtitle ?? item.year.map(String.init) {
                    Text(verbatim: sub).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary).lineLimit(1)
                        .padding(.top, 3)
                }
            }
        }
        .frame(width: width, alignment: .leading)
        .contentShape(Rectangle())
    }
}

/// A 16:9 still with its progress thread, title and what's left (Reprendre).
struct LandscapeCard: View {
    let item: ItemSummary
    var width: CGFloat = 236

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ZStack(alignment: .bottom) {
                Theme.surface
                RemoteImage(art: item.still ?? item.backdrop, width: Int(width * 3))
                if let p = item.progress {
                    ProgressThread(value: p).padding(10)
                }
            }
            .frame(width: width, height: width * 9 / 16)
            .clipShape(RoundedRectangle(cornerRadius: 14))
            .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(Color.white.opacity(0.07)))
            .shadow(color: .black.opacity(0.6), radius: 17, y: 18)
            Text(verbatim: item.show?.displayTitle ?? item.name)
                .font(Theme.sans(14, .semibold)).lineLimit(1)
                .padding(.top, 10)
            Text(verbatim: cardSubtitle(item))
                .font(Theme.sans(12.5, .medium)).foregroundStyle(Theme.textTertiary).lineLimit(1)
                .padding(.top, 4)
        }
        .frame(width: width, alignment: .leading)
        .contentShape(Rectangle())
    }
}

/// "S1 É2 · Gran Dillama" for an episode, "4 min left" for a film started.
func cardSubtitle(_ item: ItemSummary) -> String {
    if item.kind == .episode {
        return [episodeCode(item), item.name.isEmpty ? nil : item.name].compactMap { $0 }.joined(separator: " · ")
    }
    if let left = timeLeft(item) { return tr("card.timeLeft %@", left) }
    return item.year.map(String.init) ?? ""
}

/// What's left to watch, "12 min" or "1 h 02".
func timeLeft(_ item: ItemSummary) -> String? {
    guard let s = item.userState, s.positionMs > 0, let d = s.durationMs, d > s.positionMs else { return nil }
    return duration(minutes: Int((d - s.positionMs) / 60_000))
}

func duration(minutes: Int) -> String {
    let f = DateComponentsFormatter()
    f.unitsStyle = .short
    f.allowedUnits = minutes >= 60 ? [.hour, .minute] : [.minute]
    var cal = Calendar.current
    cal.locale = AppLanguage.locale
    f.calendar = cal
    return f.string(from: TimeInterval(max(1, minutes) * 60)) ?? "\(minutes) min"
}

/// A check on what's been watched; for a show, the episodes left.
struct WatchBadge: View {
    let item: ItemSummary

    var body: some View {
        if item.userState?.watched == true {
            Image(systemName: "checkmark")
                .font(.system(size: 10, weight: .bold))
                .foregroundStyle(Theme.onCream)
                .frame(width: 22, height: 22)
                .background(Circle().fill(Theme.textPrimary.opacity(0.92)))
                .accessibilityLabel(Text("card.watched"))
        } else if let s = item.userState, let w = s.watchedEpisodes, let t = s.totalEpisodes, t > w, w > 0 {
            Text(verbatim: "\(t - w)")
                .font(Theme.sans(11, .bold))
                .foregroundStyle(Theme.onCream)
                .padding(.horizontal, 7).frame(height: 22)
                .background(Capsule().fill(Theme.textPrimary.opacity(0.92)))
                .accessibilityLabel(Text("card.episodesLeft \(t - w)"))
        }
    }
}

/// Sent when a card's menu changed what was watched: pages reload.
extension Notification.Name {
    static let watchChanged = Notification.Name("plaid.watchChanged")
}

/// A long press on a card: mark watched or unwatched, and for an episode,
/// go to its show.
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
                NavigationLink(value: ItemRoute(id: show.id)) { Label("card.goToShow", systemImage: "tv") }
            }
        }
    }
}

extension View {
    func cardMenu(_ item: ItemSummary) -> some View { modifier(CardMenu(item: item)) }
}

/// A row's header: its title in Bricolage and a chevron when it opens more.
struct RowHeader: View {
    let title: String
    var more = false

    var body: some View {
        HStack(spacing: 6) {
            Text(verbatim: title).font(Theme.display(21)).tracking(-0.4)
            if more {
                Image(systemName: "chevron.right").font(.system(size: 13, weight: .heavy)).foregroundStyle(Theme.textTertiary)
            }
        }
    }
}

/// An item known only by its id (a show from an episode's menu).
struct ItemByIdView: View {
    let route: ItemRoute
    @Environment(AppState.self) private var app
    @State private var summary: ItemSummary?
    @State private var failed = false

    var body: some View {
        Group {
            if let summary { ItemView(summary: summary) } else if failed { ErrorState { failed = false } } else { LoadingView() }
        }
        .task(id: failed) {
            guard !failed, summary == nil, let client = app.client else { return }
            do {
                let d: ItemDetail = try await client.get("/items/\(route.id)")
                summary = d.summary
            } catch { failed = true }
        }
    }
}

/// The shimmering shapes of a row still loading.
struct RowPlaceholder: View {
    var landscape = false

    var body: some View {
        HStack(spacing: 12) {
            ForEach(0..<4, id: \.self) { _ in
                Shimmer()
                    .frame(width: landscape ? 236 : 112, height: landscape ? 133 : 168)
                    .clipShape(RoundedRectangle(cornerRadius: landscape ? 14 : 9))
            }
        }
        .accessibilityHidden(true)
    }
}
