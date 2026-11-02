import SwiftUI

/// A party not started yet, or an invitation (Party08, Party20): the title,
/// the time, the note, who's coming; the invitee answers yes, maybe or no
/// (R28), the organizer invites more, starts early or cancels. An open
/// party joins in one tap, by its id (R3).
struct PartyDetailSheet: View {
    let ref: PartyNav.PartyRef
    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(PartyNav.self) private var nav
    @Environment(\.dismiss) private var dismiss
    @State private var party: Party?
    @State private var failed = false
    @State private var inviting = false
    @State private var starting = false
    @State private var error: String?

    private var mine: Bool { party?.organizer.userId == app.me?.user.id }

    var body: some View {
        NavigationStack {
            ScrollView {
                if let party {
                    content(party).padding(.horizontal, Theme.gutter).padding(.bottom, 30)
                } else if failed {
                    ErrorState { failed = false; Task { await load() } }.frame(height: 300)
                } else {
                    VStack(alignment: .leading, spacing: 14) {
                        Shimmer().frame(width: 160, height: 18).clipShape(Capsule())
                        Shimmer().frame(height: 34).clipShape(Capsule())
                        Shimmer().frame(height: 120).clipShape(RoundedRectangle(cornerRadius: 18))
                    }
                    .padding(Theme.gutter)
                }
            }
            .background(Theme.bgBase)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button { dismiss() } label: { Image(systemName: "xmark") }.accessibilityLabel(Text("common.close"))
                }
            }
            .sheet(isPresented: $inviting) {
                if let party { InvitePeopleSheet(partyId: party.id, already: Set(party.invites.map { "u:\($0.person.userId)" })) }
            }
        }
        .presentationDetents([.large])
        .presentationBackground(Theme.bgBase)
        .task { await load() }
        .onChange(of: center.changes) { Task { await load() } }
    }

    @ViewBuilder
    private func content(_ p: Party) -> some View {
        VStack(alignment: .leading, spacing: 18) {
            if !mine, p.myAnswer != nil {
                HStack(spacing: 10) {
                    PartyFace(p.organizer, client: app.client, size: 40)
                    Text("party.invitation.from \(p.organizer.name)").font(Theme.sans(15, .bold))
                }
            } else {
                Text(p.isOpen ? "party.detail.open" : "party.detail.scheduled").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
            }
            Text(verbatim: p.item?.label ?? tr("vote.tbd")).displayTitle(30).lineLimit(3)
            if let at = p.scheduledAt {
                VStack(alignment: .leading, spacing: 4) {
                    Text(verbatim: partyDate(at)).font(Theme.sans(16, .semibold))
                    Text(verbatim: partyRelative(at)).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
                }
            }
            if let note = p.note, !note.isEmpty {
                Text(verbatim: "« \(note) »").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
            }
            if let error { Text(verbatim: error).font(Theme.sans(13, .medium)).foregroundStyle(Theme.error) }
            actions(p)
            if !mine, p.myAnswer != nil, !p.isOpen { answers(p) }
            people(p)
        }
        .padding(.top, 8)
    }

    @ViewBuilder
    private func actions(_ p: Party) -> some View {
        if p.isOpen {
            Button("party.join.cta") {
                dismiss()
                nav.live = PartyEntry(partyId: p.id)
            }
            .buttonStyle(CreamButtonStyle())
        } else if p.status == .voting || (p.vote != nil && p.vote?.closed == false) {
            Button("vote.cta") {
                dismiss()
                nav.vote = .init(id: p.id)
            }
            .buttonStyle(CreamButtonStyle())
        }
        if mine && !p.isOpen {
            HStack(spacing: 10) {
                if p.item != nil {
                    Button(action: startNow) {
                        Group {
                            if starting { ProgressView().tint(Theme.textPrimary) } else { Text("party.detail.startNow") }
                        }
                        .font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 36)
                    }
                    .buttonStyle(.glass)
                    .disabled(starting)
                }
                Button { inviting = true } label: {
                    Label("party.people.invite", systemImage: "person.badge.plus").font(Theme.sans(15, .semibold))
                        .frame(maxWidth: .infinity).frame(height: 36)
                }
                .buttonStyle(.glass)
            }
            if starting {
                Text("party.detail.starting").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            }
            Button(role: .destructive) {
                Task {
                    try? await app.client?.cancelParty(p.id)
                    PartyReminders.cancel(p.id)
                    dismiss()
                }
            } label: {
                Text("party.detail.cancel").font(Theme.sans(14, .semibold))
            }
            .buttonStyle(.plain)
            .foregroundStyle(Theme.error)
        }
    }

    /// "Are you coming?": I'll be there, Maybe, No (Party20).
    private func answers(_ p: Party) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("party.invitation.question").font(Theme.sans(15, .bold))
            HStack(spacing: 8) {
                ForEach(["yes", "maybe", "no"], id: \.self) { a in
                    let on = p.myAnswer == a
                    Button { answer(a) } label: {
                        Text(LocalizedStringKey("party.answer.\(a)")).font(Theme.sans(14, .semibold))
                            .frame(maxWidth: .infinity).frame(height: 42)
                            .foregroundStyle(on ? Theme.onCream : Theme.textPrimary)
                            .background(Capsule().fill(on ? Theme.cream : Color.white.opacity(0.08)))
                    }
                    .buttonStyle(.plain)
                    .accessibilityAddTraits(on ? .isSelected : [])
                }
            }
            Label("party.schedule.reminderShort", systemImage: "bell")
                .font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
        }
        .padding(16)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 22))
    }

    /// The organizer then each invitee with their answer.
    private func people(_ p: Party) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            PartySectionTitle("party.detail.people")
            row(p.organizer, Text("party.people.host"))
            ForEach(p.invites, id: \.person.userId) { i in
                row(i.person, Text(LocalizedStringKey("party.answer.state.\(i.answer)")))
            }
        }
    }

    private func row(_ person: PartyPerson, _ status: Text) -> some View {
        HStack(spacing: 12) {
            PartyFace(person, client: app.client, size: 36)
            Text(verbatim: person.userId == app.me?.user.id ? tr("party.people.you") : person.name).font(Theme.sans(15, .semibold))
            Spacer()
            status.font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
        }
    }

    // MARK: Actions

    private func load() async {
        guard let client = app.client else { return }
        do {
            party = try await client.party(ref.id)
        } catch {
            if party == nil { failed = true }
        }
    }

    private func answer(_ a: String) {
        guard let client = app.client, let p = party else { return }
        Task {
            do {
                try await client.answerParty(p.id, a)
                await load()
                if let party {
                    if a == "no" { PartyReminders.cancel(party.id) } else { await PartyReminders.schedule(party) }
                }
            } catch let e as APIError {
                error = partyError(e.code)
            } catch {}
        }
    }

    /// Starting early moves the time to a minute from now: the server opens
    /// the lobby and starts it then; we join as soon as it is open.
    private func startNow() {
        guard let client = app.client, let p = party else { return }
        starting = true
        Task {
            do {
                _ = try await client.updateParty(p.id, PartyUpdate(scheduledAt: .now.addingTimeInterval(60)))
                for _ in 0..<40 {
                    try? await Task.sleep(for: .seconds(3))
                    if let fresh = try? await client.party(p.id), fresh.isOpen {
                        dismiss()
                        nav.live = PartyEntry(partyId: fresh.id)
                        return
                    }
                }
                starting = false
            } catch let e as APIError {
                starting = false
                error = partyError(e.code)
            } catch {
                starting = false
            }
        }
    }
}
