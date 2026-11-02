import SwiftUI

/// The Upcoming tab (PartiesPh, R37–R39): invitations to answer on top,
/// then day by day the episodes of followed shows (released, file pending,
/// upcoming, with Notify me and Plan a party) and the parties (live: Join;
/// scheduled: hosting or my answer). As a list or a month calendar.
struct UpcomingView: View {
    enum Kind: String, CaseIterable { case all, episodes, parties }

    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(PartyNav.self) private var nav
    @Environment(\.isWide) private var wide
    @State private var kind: Kind = .all
    @State private var calendar = false
    @State private var data: Upcoming?
    @State private var failed = false
    @State private var month = Calendar.current.dateInterval(of: .month, for: .now)?.start ?? .now
    @State private var selected = Calendar.current.startOfDay(for: .now)

    private var gutter: CGFloat { wide ? Theme.wideGutter : Theme.gutter }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                VStack(alignment: .leading, spacing: 4) {
                    Text("upcoming.title").displayTitle(34)
                    Text("upcoming.hint").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                }
                HStack(spacing: 8) {
                    Picker(selection: $kind) {
                        ForEach(Kind.allCases, id: \.self) { k in Text(LocalizedStringKey("upcoming.filter.\(k.rawValue)")).tag(k) }
                    } label: { EmptyView() }
                    .pickerStyle(.segmented)
                    Picker(selection: $calendar) {
                        Image(systemName: "list.bullet").accessibilityLabel(Text("upcoming.view.list")).tag(false)
                        Image(systemName: "calendar").accessibilityLabel(Text("upcoming.view.calendar")).tag(true)
                    } label: { EmptyView() }
                    .pickerStyle(.segmented)
                    .frame(width: 100)
                }
                if let data {
                    if !data.invitations.isEmpty && kind != .episodes { invitations(data.invitations) }
                    if calendar {
                        CalendarMonth(month: $month, selected: $selected, marked: markedDays(data))
                        dayList(entries(data).filter { Calendar.current.isDate($0.day, inSameDayAs: selected) }, emptyKey: "upcoming.dayEmpty")
                    } else {
                        dayList(entries(data), emptyKey: "upcoming.empty")
                    }
                    Text("upcoming.followHint").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary).padding(.top, 10)
                } else if failed {
                    ErrorState { failed = false; Task { await load() } }.frame(height: 240)
                } else {
                    ForEach(0..<4, id: \.self) { _ in Shimmer().frame(height: 72).clipShape(RoundedRectangle(cornerRadius: 16)) }
                }
            }
            .padding(.horizontal, gutter)
            .padding(.top, 12)
            .padding(.bottom, 120)
        }
        .background(Theme.bgBase)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Menu {
                    Button { nav.joinCode = .init() } label: { Label("party.join.title", systemImage: "number") }
                    Button { nav.launch = PartyTarget(schedule: true, vote: true) } label: { Label("vote.open.menu", systemImage: "hand.raised") }
                } label: { Image(systemName: "plus") }
                .accessibilityLabel(Text("upcoming.add"))
            }
        }
        .refreshable { await load() }
        .task(id: "\(kind.rawValue)\(calendar)\(month.timeIntervalSince1970)\(center.changes)") { await load() }
    }

    // MARK: Data

    private struct Entry: Identifiable {
        let id: String
        let day: Date
        var episode: UpcomingEpisode?
        var party: Party?
    }

    private func entries(_ d: Upcoming) -> [Entry] {
        var out: [Entry] = []
        for e in d.episodes { if let day = e.day { out.append(Entry(id: "e" + e.id, day: day, episode: e)) } }
        for p in d.parties {
            let day = p.isOpen ? Date.now : (p.scheduledAt ?? p.createdAt)
            out.append(Entry(id: "p" + p.id, day: day, party: p))
        }
        return out.sorted { $0.day < $1.day }
    }

    private func markedDays(_ d: Upcoming) -> Set<Date> {
        Set(entries(d).map { Calendar.current.startOfDay(for: $0.day) })
    }

    private func load() async {
        guard let client = app.client else { return }
        let c = Calendar.current
        let from: Date, to: Date
        if calendar {
            from = month
            to = c.date(byAdding: DateComponents(month: 1, day: -1), to: month) ?? month
        } else {
            from = c.date(byAdding: .day, value: -7, to: .now) ?? .now
            to = c.date(byAdding: .day, value: 60, to: .now) ?? .now
        }
        do {
            let d = try await client.upcoming(from: from, to: to, kind: kind.rawValue)
            withAnimation(.smooth) { data = d }
            failed = false
        } catch {
            if data == nil { failed = true }
        }
    }

    // MARK: Sections

    private func invitations(_ list: [Party]) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("upcoming.invitations \(list.count)").font(Theme.sans(15, .bold))
            ForEach(list) { p in InvitationRow(party: p) { Task { await load() } } }
        }
    }

    @ViewBuilder
    private func dayList(_ list: [Entry], emptyKey: LocalizedStringKey) -> some View {
        if list.isEmpty {
            Text(emptyKey).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textTertiary).padding(.vertical, 20)
        }
        let days = Dictionary(grouping: list) { Calendar.current.startOfDay(for: $0.day) }
        ForEach(days.keys.sorted(), id: \.self) { day in
            VStack(alignment: .leading, spacing: 8) {
                Text(verbatim: dayTitle(day)).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                ForEach(days[day] ?? []) { e in
                    if let ep = e.episode { EpisodeRelease(episode: ep) { Task { await load() } } }
                    if let p = e.party { PartyRow(party: p) }
                }
            }
        }
    }

    /// "Today · Monday 2 November", "Thursday 5 November".
    private func dayTitle(_ d: Date) -> String {
        let c = Calendar.current
        let full = d.formatted(.dateTime.weekday(.wide).day().month(.wide).locale(AppLanguage.locale))
        if c.isDateInToday(d) { return "\(tr("upcoming.today")) · \(full)" }
        if c.isDateInYesterday(d) { return "\(tr("upcoming.yesterday")) · \(full)" }
        if c.isDateInTomorrow(d) { return "\(tr("upcoming.tomorrow")) · \(full)" }
        return full.prefix(1).uppercased() + full.dropFirst()
    }
}

/// An episode of a followed show: its status, Notify me, Plan a party.
private struct EpisodeRelease: View {
    let episode: UpcomingEpisode
    let changed: () -> Void
    @Environment(AppState.self) private var app
    @Environment(PartyNav.self) private var nav
    @State private var alert: Bool?

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(verbatim: "\(episode.showTitle) · \(tr("card.episodeShort %lld %lld", episode.season, episode.episode))")
                .font(Theme.sans(12, .semibold)).foregroundStyle(Theme.textSecondary)
            Text(verbatim: episode.title ?? "").font(Theme.display(18)).lineLimit(1)
            Text(LocalizedStringKey("upcoming.episode.\(episode.status)")).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            HStack(spacing: 8) {
                if episode.status != "released_on_server" {
                    let on = alert ?? episode.alert
                    Button {
                        alert = !on
                        Task {
                            do { try await app.client?.setReleaseAlert(episode, !on) } catch { alert = on }
                        }
                    } label: {
                        Label(on ? "upcoming.notifyOn" : "upcoming.notifyMe", systemImage: on ? "bell.fill" : "bell")
                            .font(Theme.sans(13, .semibold))
                    }
                    .buttonStyle(.glass)
                }
                Button { nav.launch = PartyTarget(future: episode, schedule: true) } label: {
                    Label("upcoming.planParty", systemImage: "party.popper").font(Theme.sans(13, .semibold))
                }
                .buttonStyle(.glass)
            }
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 18).fill(Color.white.opacity(0.05)))
    }
}

/// A party in the list: LIVE with Join, or its time with what I do.
private struct PartyRow: View {
    let party: Party
    @Environment(AppState.self) private var app
    @Environment(PartyNav.self) private var nav

    var body: some View {
        Button {
            if party.isOpen { nav.live = PartyEntry(partyId: party.id) } else { nav.detail = .init(id: party.id) }
        } label: {
            HStack(spacing: 12) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(verbatim: header).font(Theme.sans(12, .semibold)).foregroundStyle(party.isOpen ? Theme.textPrimary : Theme.textSecondary)
                    Text(verbatim: party.item?.label ?? tr("vote.tbd")).font(Theme.display(18)).lineLimit(1)
                    Text(verbatim: status).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                }
                Spacer()
                if party.isOpen {
                    Text("party.join.cta").font(Theme.sans(14, .semibold)).padding(.horizontal, 16).frame(height: 36)
                        .background(Capsule().fill(Theme.cream)).foregroundStyle(Theme.onCream)
                } else {
                    Image(systemName: "chevron.right").foregroundStyle(Theme.textTertiary)
                }
            }
            .padding(14)
            .background(RoundedRectangle(cornerRadius: 18).fill(Color.white.opacity(party.isOpen ? 0.1 : 0.05)))
        }
        .buttonStyle(.plain)
    }

    /// "LIVE · Plaid Party · Camille", "Plaid Party · 21:00".
    private var header: String {
        if party.isOpen { return "\(tr("party.home.live").uppercased(with: AppLanguage.locale)) · \(tr("party.title")) · \(party.organizer.name)" }
        let t = party.scheduledAt?.formatted(.dateTime.hour().minute().locale(AppLanguage.locale)) ?? ""
        return "\(tr("party.title")) · \(t)"
    }

    private var status: String {
        if party.organizer.userId == app.me?.user.id { return tr("party.home.hosting") }
        if party.status == .voting { return tr("vote.cta") }
        return localized("party.answer.state.\(party.myAnswer ?? "pending")")
    }
}

/// An invitation to answer, right in the list (PartiesPh).
struct InvitationRow: View {
    let party: Party
    let answered: () -> Void
    @Environment(AppState.self) private var app
    @Environment(PartyNav.self) private var nav

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Button { nav.detail = .init(id: party.id) } label: {
                HStack(spacing: 10) {
                    PartyFace(party.organizer, client: app.client, size: 34)
                    VStack(alignment: .leading, spacing: 2) {
                        Text("party.invitation.from \(party.organizer.name)").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                        Text(verbatim: party.item?.label ?? tr("vote.tbd")).font(Theme.display(18)).lineLimit(1)
                        if let at = party.scheduledAt {
                            Text(verbatim: partyDate(at)).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                        }
                    }
                    Spacer()
                }
            }
            .buttonStyle(.plain)
            if let note = party.note, !note.isEmpty {
                Text(verbatim: "« \(note) »").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
            }
            HStack(spacing: 8) {
                ForEach(["yes", "maybe", "no"], id: \.self) { a in
                    Button {
                        Task {
                            try? await app.client?.answerParty(party.id, a)
                            if a != "no" { await PartyReminders.schedule(party) } else { PartyReminders.cancel(party.id) }
                            answered()
                        }
                    } label: {
                        Text(LocalizedStringKey("party.answer.\(a)")).font(Theme.sans(13, .semibold))
                            .frame(maxWidth: .infinity).frame(height: 36)
                            .foregroundStyle(a == "yes" ? Theme.onCream : Theme.textPrimary)
                            .background(Capsule().fill(a == "yes" ? Theme.cream : Color.white.opacity(0.08)))
                    }
                    .buttonStyle(.plain)
                }
            }
        }
        .padding(14)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 20))
    }
}

/// A month: the days with something marked, one picked.
private struct CalendarMonth: View {
    @Binding var month: Date
    @Binding var selected: Date
    let marked: Set<Date>

    private var cal: Calendar {
        var c = Calendar.current
        c.locale = AppLanguage.locale
        return c
    }

    private var days: [Date?] {
        guard let range = cal.range(of: .day, in: .month, for: month) else { return [] }
        let first = cal.component(.weekday, from: month)
        let lead = (first - cal.firstWeekday + 7) % 7
        return Array(repeating: nil, count: lead) + range.compactMap { cal.date(byAdding: .day, value: $0 - 1, to: month) }
    }

    var body: some View {
        VStack(spacing: 10) {
            HStack {
                Text(verbatim: month.formatted(.dateTime.month(.wide).year().locale(AppLanguage.locale)).capitalized(with: AppLanguage.locale))
                    .font(Theme.display(20))
                Spacer()
                Button { shift(-1) } label: { Image(systemName: "chevron.left") }.accessibilityLabel(Text("upcoming.previousMonth"))
                Button { shift(1) } label: { Image(systemName: "chevron.right") }.accessibilityLabel(Text("upcoming.nextMonth"))
            }
            .buttonStyle(.glass)
            let symbols = cal.veryShortStandaloneWeekdaySymbols
            let ordered = Array(symbols[(cal.firstWeekday - 1)...] + symbols[..<(cal.firstWeekday - 1)])
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 4), count: 7), spacing: 6) {
                ForEach(Array(ordered.enumerated()), id: \.offset) { _, s in
                    Text(verbatim: s).font(Theme.sans(11, .semibold)).foregroundStyle(Theme.textTertiary)
                }
                ForEach(Array(days.enumerated()), id: \.offset) { _, d in
                    if let d {
                        let on = cal.isDate(d, inSameDayAs: selected)
                        Button { selected = d } label: {
                            VStack(spacing: 3) {
                                Text(verbatim: "\(cal.component(.day, from: d))").font(Theme.sans(14, cal.isDateInToday(d) ? .bold : .medium))
                                Circle().fill(marked.contains(cal.startOfDay(for: d)) ? (on ? Theme.onCream : Theme.textPrimary) : .clear)
                                    .frame(width: 5, height: 5)
                            }
                            .frame(maxWidth: .infinity).frame(height: 40)
                            .foregroundStyle(on ? Theme.onCream : Theme.textPrimary)
                            .background(RoundedRectangle(cornerRadius: 10).fill(on ? Theme.cream : .clear))
                        }
                        .buttonStyle(.plain)
                        .accessibilityAddTraits(on ? .isSelected : [])
                    } else {
                        Color.clear.frame(height: 40)
                    }
                }
            }
        }
        .padding(14)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 22))
    }

    private func shift(_ n: Int) {
        if let m = cal.date(byAdding: .month, value: n, to: month) {
            month = m
            selected = m
        }
    }
}
