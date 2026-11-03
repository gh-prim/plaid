import SwiftUI
import UserNotifications

/// "Plaid Parties" on the home (Party21, R40): live parties I can join and
/// my next scheduled ones, only when there's at least one. One tap joins a
/// live party by its id; a scheduled one opens its page.
struct HomePartiesRow: View {
    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(PartyNav.self) private var nav
    @Environment(\.isWide) private var wide
    @State private var parties: [Party] = []

    var body: some View {
        Group {
            if !parties.isEmpty {
                VStack(alignment: .leading, spacing: 12) {
                    RowHeader(title: tr("party.home.title")).padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
                    ScrollView(.horizontal) {
                        LazyHStack(spacing: 12) {
                            ForEach(parties) { p in
                                Button { open(p) } label: { PartyCard(party: p, width: wide ? 300 : 250) }
                                    .buttonStyle(.plain)
                            }
                        }
                    }
                    .scrollIndicators(.hidden)
                    .contentMargins(.horizontal, wide ? Theme.wideGutter : Theme.gutter, for: .scrollContent)
                }
                .padding(.top, 26)
                .transition(.opacity)
            }
        }
        .task(id: center.changes) { await load() }
        .onReceive(NotificationCenter.default.publisher(for: .watchChanged)) { _ in Task { await load() } }
    }

    private func open(_ p: Party) {
        if p.isOpen { nav.live = PartyEntry(partyId: p.id) } else { nav.detail = .init(id: p.id) }
    }

    private func load() async {
        guard let client = app.client, let all = try? await client.parties() else { return }
        let shown = all.filter { $0.isOpen || $0.status == .scheduled || $0.status == .voting }
            .filter { $0.myAnswer != "no" }
        withAnimation(.smooth) { parties = Array(shown.prefix(8)) }
        await PartyReminders.sync(all, me: app.me?.user.id)
    }
}

/// One party: its title's backdrop, LIVE or the day and time, the title,
/// and who's in it or what I answered.
struct PartyCard: View {
    let party: Party
    let width: CGFloat
    @Environment(AppState.self) private var app
    @State private var art: Artwork?

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            RemoteImage(art: art, width: 600)
                .frame(width: width, height: width * 9 / 16)
                .background(LinearGradient(colors: [Theme.party[party.organizer.avatarColor % Theme.party.count].opacity(0.5), Theme.surface],
                                           startPoint: .topLeading, endPoint: .bottomTrailing))
                .overlay(LinearGradient(colors: [.clear, .black.opacity(0.75)], startPoint: .center, endPoint: .bottom))
            VStack(alignment: .leading, spacing: 3) {
                Text(verbatim: badge).font(Theme.sans(10, .heavy)).tracking(0.8)
                    .padding(.horizontal, 7).frame(height: 20)
                    .background(Capsule().fill(party.isOpen ? Theme.cream : Color.white.opacity(0.18)))
                    .foregroundStyle(party.isOpen ? Theme.onCream : Theme.textPrimary)
                Text(verbatim: party.item?.label ?? tr("vote.tbd")).font(Theme.display(18)).lineLimit(1)
                Text(verbatim: line).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary).lineLimit(1)
            }
            .padding(12)
        }
        .frame(width: width, height: width * 9 / 16)
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .task(id: party.item?.id) {
            guard let id = party.item?.id, let client = app.client, let d: ItemDetail = try? await client.get("/items/\(id)") else { return }
            art = d.summary.backdrop ?? d.summary.still
        }
        .accessibilityElement(children: .combine)
    }

    /// LIVE, or SAT. 21:00.
    private var badge: String {
        if party.isOpen { return tr("party.home.live").uppercased(with: AppLanguage.locale) }
        guard let at = party.scheduledAt else { return "" }
        return at.formatted(.dateTime.weekday(.abbreviated).hour().minute().locale(AppLanguage.locale)).uppercased(with: AppLanguage.locale)
    }

    /// "Alex and 2 others · join", "Organized by Mia · you're coming".
    private var line: String {
        let me = app.me?.user.id
        if party.isOpen {
            let others = max(0, (party.live?.count ?? 1) - 1)
            return others > 0 ? tr("party.home.liveWith %@ %lld", party.organizer.name, others) : tr("party.home.liveAlone %@", party.organizer.name)
        }
        if party.organizer.userId == me { return tr("party.home.hosting") }
        let answer = party.myAnswer ?? "pending"
        return tr("party.home.by %@", party.organizer.name) + " · " + localized("party.home.answer.\(answer)")
    }
}
