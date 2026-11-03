import SwiftUI

/// The host manages the people (Party17, R14, R16): who controls playback,
/// then each person with where they watch, to hand the host role over, give
/// or take control, or remove them. Others see the list only.
struct HostPeopleSheet: View {
    let room: PartyRoom
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @State private var inviting = false

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    Group {
                        if room.isHost { Text("party.people.youHost \(room.presence.count)") } else { Text("party.count \(room.presence.count)") }
                    }
                    .font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                    if room.isHost {
                        VStack(alignment: .leading, spacing: 10) {
                            PartySectionTitle("party.start.controlTitle")
                            Picker(selection: Binding(get: { room.mode }, set: { m in Task { await room.setMode(m) } })) {
                                Text("party.control.host").tag("host")
                                Text("party.control.everyone").tag("everyone")
                            } label: { EmptyView() }
                            .pickerStyle(.segmented)
                        }
                    }
                    VStack(spacing: 8) {
                        ForEach(room.presence.people) { p in row(p) }
                    }
                    if room.isHost {
                        Button { inviting = true } label: {
                            Label("party.people.invite", systemImage: "person.badge.plus").font(Theme.sans(15, .semibold))
                                .frame(maxWidth: .infinity).frame(height: 36)
                        }
                        .buttonStyle(.glass)
                    }
                }
                .padding(Theme.gutter)
            }
            .background(Theme.bgBase)
            .navigationTitle(Text("party.people.title"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) { Button("common.done") { dismiss() } }
            }
            .sheet(isPresented: $inviting) {
                if let id = room.partyId { InvitePeopleSheet(partyId: id, already: Set(room.presence.people.map(\.id))) }
            }
        }
        .presentationDetents([.medium, .large])
        .presentationBackground(Theme.bgBase)
    }

    private func row(_ p: RoomPerson) -> some View {
        HStack(spacing: 12) {
            PartyFace(p, client: app.client, size: 40)
            VStack(alignment: .leading, spacing: 2) {
                Text(verbatim: p.id == room.me ? tr("party.people.you") : p.name).font(Theme.sans(15, .semibold))
                Text(verbatim: personLine(p, me: room.me)).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
            }
            Spacer()
            if p.canControl && !p.host {
                Image(systemName: "playpause.fill").font(.system(size: 12)).foregroundStyle(Theme.textTertiary)
                    .accessibilityLabel(Text("party.people.hasControl"))
            }
            if room.isHost && p.id != room.me {
                Menu {
                    if !p.guest {
                        Button { Task { await room.makeHost(p) } } label: { Label("party.host.handTo \(p.name)", systemImage: "crown") }
                    }
                    if room.mode == "host" {
                        Button { Task { await room.grant(p, !p.canControl) } } label: {
                            Label(p.canControl ? "party.host.takeControl" : "party.host.giveControl", systemImage: "playpause")
                        }
                    }
                    Button(role: .destructive) { Task { await room.remove(p) } } label: {
                        Label("party.host.remove", systemImage: "person.fill.xmark")
                    }
                } label: {
                    Image(systemName: "ellipsis").font(.system(size: 15, weight: .bold)).frame(width: 36, height: 36)
                }
                .buttonStyle(.glass).buttonBorderShape(.circle)
                .accessibilityLabel(Text("party.people.actions \(p.name)"))
            }
        }
        .padding(12)
        .background(RoundedRectangle(cornerRadius: 16).fill(Color.white.opacity(0.05)))
    }
}

/// Invite more during a live party (R3): they get a notification with Join.
struct InvitePeopleSheet: View {
    let partyId: String
    let already: Set<String>
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @State private var people: [PartyPerson]?
    @State private var picked: Set<String> = []
    @State private var busy = false

    var body: some View {
        NavigationStack {
            List {
                if let people {
                    ForEach(people.filter { !already.contains("u:\($0.userId)") }) { p in
                        Button {
                            if picked.contains(p.userId) { picked.remove(p.userId) } else { picked.insert(p.userId) }
                        } label: {
                            HStack(spacing: 12) {
                                PartyFace(p, client: app.client, size: 34)
                                Text(verbatim: p.name).font(Theme.sans(15, .semibold))
                                Spacer()
                                Image(systemName: picked.contains(p.userId) ? "checkmark.circle.fill" : "circle")
                            }
                        }
                        .buttonStyle(.plain)
                    }
                } else {
                    ForEach(0..<4, id: \.self) { _ in Shimmer().frame(height: 34).clipShape(Capsule()) }
                }
            }
            .scrollContentBackground(.hidden)
            .background(Theme.bgBase)
            .navigationTitle(Text("party.people.invite"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) { Button("common.cancel") { dismiss() } }
                ToolbarItem(placement: .topBarTrailing) {
                    Button("party.people.inviteCta") {
                        busy = true
                        Task {
                            try? await app.client?.invite(Array(picked), to: partyId)
                            dismiss()
                        }
                    }
                    .disabled(picked.isEmpty || busy)
                }
            }
            .task { people = (try? await app.client?.partyPeople()) ?? [] }
        }
        .presentationBackground(Theme.bgBase)
    }
}

/// The host leaves (Party18, R17): hand over to the first one, pick
/// someone else, or end for everyone.
struct HostLeaveSheet: View {
    let room: PartyRoom
    let left: () -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var picking = false

    private var candidates: [RoomPerson] { room.others.filter { !$0.guest } }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("party.host.leaveTitle").displayTitle(26)
            Text("party.host.leaveHint").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
            if picking {
                ForEach(candidates) { p in
                    Button { hand(to: p) } label: {
                        HStack(spacing: 12) {
                            PartyFaceFromEnv(person: p, size: 32)
                            Text(verbatim: p.name).font(Theme.sans(15, .semibold))
                            Spacer()
                        }
                        .padding(12)
                        .background(RoundedRectangle(cornerRadius: 14).fill(Color.white.opacity(0.06)))
                    }
                    .buttonStyle(.plain)
                }
            } else {
                if let first = candidates.first {
                    Button("party.host.handTo \(first.name)") { hand(to: first) }.buttonStyle(CreamButtonStyle())
                }
                if candidates.count > 1 {
                    Button { picking = true } label: {
                        Text("party.host.pickOther").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
                    }
                    .buttonStyle(.glass)
                }
            }
            Button(role: .destructive) {
                Task {
                    await room.end()
                    dismiss()
                    left()
                }
            } label: {
                Text("party.host.endAll").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
            }
            .buttonStyle(.glass)
            .tint(Theme.error)
        }
        .padding(Theme.gutter)
        .presentationDetents([.medium])
        .presentationBackground(Theme.bgBase)
    }

    private func hand(to p: RoomPerson) {
        Task {
            await room.makeHost(p)
            dismiss()
            left()
        }
    }
}

/// The server can't convert for everyone (Party19, R26): one shared
/// quality for all who need it, one first and the others wait, or cancel.
struct TranscodeLimitSheet: View {
    let room: PartyRoom
    let limit: TranscodeLimit

    var body: some View {
        let names = ListFormatter.localizedString(byJoining: limit.waiting.map(\.name))
        VStack(alignment: .leading, spacing: 14) {
            Image(systemName: "cpu").font(.system(size: 28, weight: .medium)).foregroundStyle(Theme.textSecondary)
            Text("party.limit.title").displayTitle(26)
            Text("party.limit.hint \(names)").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
            if limit.options.contains("shared"), let q = limit.sharedQuality {
                Button("party.limit.shared \(partyQuality(q))") {
                    Task { await room.chooseTranscode("shared", quality: q) }
                }
                .buttonStyle(CreamButtonStyle())
            }
            if limit.options.contains("order"), let first = limit.waiting.first {
                let others = ListFormatter.localizedString(byJoining: limit.waiting.dropFirst().map(\.name))
                Button { Task { await room.chooseTranscode("order", first: first.id) } } label: {
                    Text("party.limit.order \(first.name) \(others)").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
                }
                .buttonStyle(.glass)
            }
            Button { Task { await room.chooseTranscode("cancel") } } label: {
                Text("common.cancel").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
            }
            .buttonStyle(.glass)
        }
        .padding(Theme.gutter)
        .presentationDetents([.medium])
        .presentationBackground(Theme.bgBase)
        .interactiveDismissDisabled()
    }
}
