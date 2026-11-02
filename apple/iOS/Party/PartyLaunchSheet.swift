import SwiftUI
import UIKit

/// Starting a Plaid Party from a title (Party04, 05): now or scheduled,
/// where you watch (an Apple TV of the account that is online, or this
/// device), who controls playback, and who is invited. Then the code and the
/// link to send (Party06). Without a title, it schedules a party where the
/// group votes for the movie first (VotePh step 1, R31).
struct PartyLaunchSheet: View {
    let target: PartyTarget
    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(PartyNav.self) private var nav
    @Environment(\.dismiss) private var dismiss

    @State private var schedule: Bool
    @State private var screens: [PartyScreenOption]?
    /// The TV that plays, nil for this device.
    @State private var tv: PartyScreenOption?
    @State private var mode = "host"
    @State private var day = Calendar.current.startOfDay(for: .now)
    @State private var time = PartyLaunchSheet.defaultTime
    @State private var otherTime = false
    @State private var people: [PartyPerson]?
    @State private var invited: Set<String>
    @State private var note = ""
    @State private var busy = false
    @State private var error: String?
    /// The party just started: its code and link (Party06).
    @State private var created: Party?

    init(target: PartyTarget) {
        self.target = target
        _schedule = State(initialValue: target.schedule || target.vote || target.future != nil)
        _invited = State(initialValue: Set(target.people))
    }

    /// 21:00 today.
    private static var defaultTime: Date {
        Calendar.current.date(bySettingHour: 21, minute: 0, second: 0, of: .now) ?? .now
    }

    private var when: Date {
        let c = Calendar.current
        let hm = c.dateComponents([.hour, .minute], from: time)
        return c.date(bySettingHour: hm.hour ?? 21, minute: hm.minute ?? 0, second: 0, of: day) ?? day
    }

    /// R28: 15 min to 60 days ahead.
    private var whenValid: Bool {
        when > .now.addingTimeInterval(15 * 60) && when < .now.addingTimeInterval(60 * 86400)
    }

    var body: some View {
        NavigationStack {
            Group {
                if let created {
                    InviteReady(party: created) {
                        let entry = PartyEntry(partyId: created.id, role: tv == nil ? .screen : .companion,
                                               tvSession: tv?.sessionId, screenName: tv?.name)
                        dismiss()
                        nav.live = entry
                    }
                } else {
                    form
                }
            }
            .background(Theme.bgBase)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button { dismiss() } label: { Image(systemName: "xmark") }.accessibilityLabel(Text("common.close"))
                }
            }
            .navigationBarTitleDisplayMode(.inline)
        }
        .presentationDetents([.large])
        .presentationBackground(Theme.bgBase)
    }

    private var form: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                VStack(alignment: .leading, spacing: 4) {
                    Text("party.title").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                    Text(verbatim: target.title).displayTitle(30).lineLimit(2)
                    if let item = target.item { Text(verbatim: facts(item)).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textTertiary) }
                }
                if target.vote {
                    VoteIntro(proposalsUntil: proposalsUntil, rankingUntil: rankingUntil)
                } else if target.future == nil {
                    Picker(selection: $schedule) {
                        Text("party.start.now").tag(false)
                        Text("party.start.schedule").tag(true)
                    } label: { EmptyView() }
                    .pickerStyle(.segmented)
                }
                if schedule { scheduleSection }
                if !schedule { whereSection }
                if !target.vote { controlSection }
                inviteSection
                if let error {
                    Text(verbatim: error).font(Theme.sans(13, .medium)).foregroundStyle(Theme.error)
                }
                Button(action: go) {
                    if busy { ProgressView().tint(Theme.onCream) } else { Text(verbatim: ctaTitle) }
                }
                .buttonStyle(CreamButtonStyle())
                .disabled(busy || (schedule && !whenValid))
            }
            .padding(.horizontal, Theme.gutter)
            .padding(.bottom, 30)
        }
        .task { await load() }
    }

    private var ctaTitle: String {
        if target.vote { return tr("vote.open.cta") }
        guard schedule else { return tr("party.start.cta") }
        let d = when.formatted(.dateTime.weekday(.abbreviated).day().month(.abbreviated).locale(AppLanguage.locale))
        let t = when.formatted(.dateTime.hour().minute().locale(AppLanguage.locale))
        return tr("party.schedule.cta %@ %@", d, t)
    }

    // MARK: Sections

    private var whereSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            PartySectionTitle("party.start.whereTitle")
            if let screens {
                ForEach(screens) { s in
                    PartyChoiceRow(icon: "appletv", title: Text(verbatim: "Apple TV · \(s.name)"), hint: Text("party.start.tvHint"),
                                   on: tv?.id == s.id) { tv = s }
                }
                PartyChoiceRow(icon: UIDevice.current.userInterfaceIdiom == .pad ? "ipad" : "iphone",
                               title: Text(UIDevice.current.userInterfaceIdiom == .pad ? "party.start.thisIPad" : "party.start.thisIPhone"),
                               hint: Text("party.start.thisDevice"), on: tv == nil) { tv = nil }
            } else {
                Shimmer().frame(height: 64).clipShape(RoundedRectangle(cornerRadius: 16))
                Shimmer().frame(height: 64).clipShape(RoundedRectangle(cornerRadius: 16))
            }
        }
    }

    private var controlSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            PartySectionTitle("party.start.controlTitle")
            Picker(selection: $mode) {
                Text("party.control.host").tag("host")
                Text("party.control.everyone").tag("everyone")
            } label: { EmptyView() }
            .pickerStyle(.segmented)
            Text("party.start.controlHint").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
        }
    }

    private var days: [Date] {
        let c = Calendar.current
        return (0..<14).compactMap { c.date(byAdding: .day, value: $0, to: c.startOfDay(for: .now)) }
    }

    private var scheduleSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            PartySectionTitle("party.schedule.day")
            ScrollView(.horizontal) {
                HStack(spacing: 8) {
                    ForEach(days, id: \.self) { d in
                        let on = Calendar.current.isDate(d, inSameDayAs: day)
                        Button { day = d } label: {
                            VStack(spacing: 2) {
                                Text(verbatim: Calendar.current.isDateInToday(d) ? tr("party.schedule.today")
                                     : d.formatted(.dateTime.weekday(.abbreviated).locale(AppLanguage.locale)))
                                    .font(Theme.sans(11, .semibold))
                                Text(verbatim: d.formatted(.dateTime.day().locale(AppLanguage.locale))).font(Theme.display(18))
                            }
                            .frame(width: 52, height: 58)
                            .foregroundStyle(on ? Theme.onCream : Theme.textPrimary)
                            .background(RoundedRectangle(cornerRadius: 14).fill(on ? Theme.cream : Color.white.opacity(0.06)))
                        }
                        .buttonStyle(.plain)
                        .accessibilityAddTraits(on ? .isSelected : [])
                    }
                }
            }
            .scrollIndicators(.hidden)
            PartySectionTitle("party.schedule.time")
            HStack(spacing: 8) {
                ForEach([20 * 60, 20 * 60 + 30, 21 * 60, 21 * 60 + 30], id: \.self) { m in
                    let t = Calendar.current.date(bySettingHour: m / 60, minute: m % 60, second: 0, of: .now) ?? .now
                    let on = !otherTime && Calendar.current.dateComponents([.hour, .minute], from: time) == Calendar.current.dateComponents([.hour, .minute], from: t)
                    Button { time = t; otherTime = false } label: {
                        Text(verbatim: t.formatted(.dateTime.hour().minute().locale(AppLanguage.locale)))
                            .font(Theme.sans(14, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
                            .foregroundStyle(on ? Theme.onCream : Theme.textPrimary)
                            .background(Capsule().fill(on ? Theme.cream : Color.white.opacity(0.06)))
                    }
                    .buttonStyle(.plain)
                }
                Button { otherTime = true } label: {
                    Text("party.schedule.other").font(Theme.sans(14, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
                        .foregroundStyle(otherTime ? Theme.onCream : Theme.textPrimary)
                        .background(Capsule().fill(otherTime ? Theme.cream : Color.white.opacity(0.06)))
                }
                .buttonStyle(.plain)
            }
            if otherTime {
                DatePicker("party.schedule.time", selection: $time, displayedComponents: .hourAndMinute)
                    .font(Theme.sans(15, .medium))
            }
            if !whenValid {
                Text("party.schedule.tooSoon").font(Theme.sans(12, .medium)).foregroundStyle(Theme.error)
            }
            TextField("party.schedule.note", text: $note, axis: .vertical)
                .font(Theme.sans(15, .medium))
                .onChange(of: note) { if note.count > 140 { note = String(note.prefix(140)) } }
                .padding(14)
                .background(RoundedRectangle(cornerRadius: 14).fill(Color.white.opacity(0.06)))
            Label("party.schedule.reminder", systemImage: "bell.badge")
                .font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
        }
    }

    private var inviteSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            PartySectionTitle("party.invite.title")
            if let people {
                if people.isEmpty {
                    Text("party.invite.nobody").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textTertiary)
                } else {
                    ScrollView(.horizontal) {
                        HStack(spacing: 14) {
                            ForEach(people) { p in
                                let on = invited.contains(p.userId)
                                Button {
                                    if on { invited.remove(p.userId) } else { invited.insert(p.userId) }
                                } label: {
                                    VStack(spacing: 6) {
                                        PartyFace(p, client: app.client, size: 50)
                                            .overlay(Circle().strokeBorder(Theme.cream, lineWidth: on ? 3 : 0).padding(-3))
                                            .overlay(alignment: .bottomTrailing) {
                                                if on {
                                                    Image(systemName: "checkmark.circle.fill").font(.system(size: 18))
                                                        .foregroundStyle(Theme.cream, Theme.bgBase)
                                                }
                                            }
                                        Text(verbatim: p.name).font(Theme.sans(12, .semibold)).lineLimit(1)
                                    }
                                    .frame(width: 64)
                                }
                                .buttonStyle(.plain)
                                .accessibilityAddTraits(on ? .isSelected : [])
                            }
                        }
                        .padding(.vertical, 4)
                    }
                    .scrollIndicators(.hidden)
                }
            } else {
                Shimmer().frame(height: 70).clipShape(RoundedRectangle(cornerRadius: 14))
            }
            Text("party.invite.linkHint").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
        }
    }

    // MARK: Vote deadlines

    /// Ranking closes 3 h before the party; proposals half way there.
    private var rankingUntil: Date { when.addingTimeInterval(-3 * 3600) }
    private var proposalsUntil: Date { Date(timeIntervalSince1970: (Date.now.timeIntervalSince1970 + rankingUntil.timeIntervalSince1970) / 2) }

    // MARK: Actions

    private func facts(_ item: ItemSummary) -> String {
        [item.year.map(String.init), item.runtimeMinutes.map { duration(minutes: $0) }].compactMap { $0 }.joined(separator: " · ")
    }

    private func load() async {
        guard let client = app.client else { return }
        async let list = try? client.partyPeople()
        if !schedule { screens = await center.screens() }
        people = await list ?? []
        if screens == nil { screens = await center.screens() }
    }

    private func go() {
        guard let client = app.client else { return }
        var body = PartyCreate()
        body.itemId = target.item?.id
        body.controlMode = mode
        body.invite = Array(invited)
        if let f = target.future, f.itemId == nil {
            body.showId = f.showId
            body.season = f.season
            body.episode = f.episode
        } else if let id = target.future?.itemId {
            body.itemId = id
        }
        if schedule {
            body.scheduledAt = when
            body.note = note.isEmpty ? nil : note
        }
        if target.vote {
            guard rankingUntil > .now.addingTimeInterval(3600) else { error = tr("vote.open.tooSoon"); return }
            body.vote = .init(proposalsUntil: proposalsUntil, rankingUntil: rankingUntil)
        }
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                let party = try await client.createParty(body)
                if schedule {
                    await PartyReminders.ask()
                    dismiss()
                    nav.detail = .init(id: party.id)
                } else {
                    withAnimation(.smooth) { created = party }
                }
            } catch let e as APIError {
                error = partyError(e.code)
            } catch {
                self.error = tr("party.error.generic")
            }
        }
    }
}

/// VotePh step 1: the group chooses; everyone proposes until one time,
/// ranks until another.
struct VoteIntro: View {
    let proposalsUntil: Date
    let rankingUntil: Date

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("vote.open.hint").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
            HStack(spacing: 10) {
                step("vote.step1", "vote.step1.title", proposalsUntil)
                step("vote.step2", "vote.step2.title", rankingUntil)
            }
            Text("vote.open.fair").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
        }
    }

    private func step(_ label: LocalizedStringKey, _ title: LocalizedStringKey, _ until: Date) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(label).font(Theme.sans(10, .bold)).textCase(.uppercase).foregroundStyle(Theme.textTertiary)
            Text(title).font(Theme.sans(15, .bold))
            Text("vote.until \(until.formatted(.dateTime.weekday(.abbreviated).hour().minute().locale(AppLanguage.locale)))")
                .font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 14).fill(Color.white.opacity(0.06)))
    }
}

/// Party06: the party is ready; the code and the link to send, valid for
/// the whole party, then into the party.
struct InviteReady: View {
    let party: Party
    let go: () -> Void
    @State private var copied = false

    var body: some View {
        VStack(spacing: 18) {
            Spacer(minLength: 10)
            Image(systemName: "party.popper").font(.system(size: 40, weight: .medium)).foregroundStyle(Theme.textSecondary)
            Text("party.invite.ready").displayTitle(30)
            Text("party.invite.hint").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary).multilineTextAlignment(.center)
            PartyCodeView(code: party.code, size: 44)
                .padding(.vertical, 8)
            HStack(spacing: 10) {
                Button {
                    UIPasteboard.general.string = party.code
                    copied = true
                } label: {
                    Label(copied ? "party.invite.copied" : "party.invite.copy", systemImage: copied ? "checkmark" : "doc.on.doc")
                        .font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 36)
                }
                .buttonStyle(.glass)
                if let url = URL(string: party.link) {
                    ShareLink(item: url, message: Text("party.invite.message \(party.item?.label ?? "") \(party.code)")) {
                        Label("party.invite.share", systemImage: "square.and.arrow.up")
                            .font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 36)
                    }
                    .buttonStyle(.glass)
                }
            }
            Text("party.invite.valid").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            Spacer()
            Button("party.invite.go", action: go).buttonStyle(CreamButtonStyle())
        }
        .padding(.horizontal, Theme.gutter)
        .padding(.bottom, 24)
    }
}

/// K7P-4QX in big separated letters.
struct PartyCodeView: View {
    let code: String
    var size: CGFloat = 34

    var body: some View {
        Text(verbatim: formattedPartyCode(code))
            .font(.system(size: size, weight: .bold, design: .monospaced))
            .tracking(size * 0.12)
            .accessibilityLabel(Text("party.code \(code.map(String.init).joined(separator: " "))"))
    }
}

/// XXX-XXX from whatever the server or the person gave.
func formattedPartyCode(_ code: String) -> String {
    let raw = code.uppercased().filter { $0.isLetter || $0.isNumber }
    guard raw.count == 6 else { return code.uppercased() }
    return "\(raw.prefix(3))-\(raw.suffix(3))"
}
