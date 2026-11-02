import SwiftUI

/// A Plaid Party, full screen: joins the room, then shows the companion
/// (the TV plays, Party01, 02) or the party player (this device plays,
/// Party10–12), and the ends: queue (Party25), removed, ended (Party16).
/// The host's sheets live here: people (Party17), leaving (Party18) and
/// the transcode limit (Party19).
struct PartyScreen: View {
    let entry: PartyEntry
    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(\.dismiss) private var dismiss
    @State private var room: PartyRoom?
    @State private var people = false
    @State private var leaving = false

    var body: some View {
        ZStack {
            Theme.bgBase.ignoresSafeArea()
            if let room {
                content(room)
            } else {
                LoadingView()
            }
        }
        .onAppear {
            if room == nil {
                room = center.enter(partyId: entry.partyId, code: entry.code, role: entry.role,
                                    tvSession: entry.tvSession, screenName: entry.screenName)
            }
        }
        .sheet(isPresented: $people) {
            if let room { HostPeopleSheet(room: room) }
        }
        .sheet(isPresented: $leaving) {
            if let room {
                HostLeaveSheet(room: room) { close(force: true) }
            }
        }
        .sheet(isPresented: Binding(get: { room?.isHost == true && room?.transcodeLimit != nil }, set: { _ in })) {
            if let room, let limit = room.transcodeLimit { TranscodeLimitSheet(room: room, limit: limit) }
        }
    }

    @ViewBuilder
    private func content(_ room: PartyRoom) -> some View {
        if room.ended {
            PartyEndView(room: room) { close(force: true) }
        } else if room.removed {
            PartyMessage(icon: "door.left.hand.open", title: "party.removed.title", hint: "party.removed.hint") { close(force: true) }
        } else if let pos = room.queuedAt {
            QueueView(room: room, position: pos) { close(force: true) }
        } else if let failure = room.failure {
            PartyMessage(icon: "exclamationmark.bubble", title: "party.join.failed", text: partyError(failure)) { close(force: true) }
        } else if !room.joined {
            PartyJoining()
        } else if room.role == .companion {
            CompanionView(room: room, people: { people = true }, close: { close() })
        } else {
            PartyPlayerScreen(room: room, people: { people = true }, close: { close() })
        }
    }

    /// The host of a party others are in picks who takes over first (R17).
    private func close(force: Bool = false) {
        if !force, let room, room.isHost, room.role == .screen, !room.others.isEmpty {
            leaving = true
            return
        }
        center.leave()
        dismiss()
    }
}

/// While joining: the shapes of the party, shimmering.
private struct PartyJoining: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Shimmer().frame(height: 220).clipShape(RoundedRectangle(cornerRadius: 20))
            Shimmer().frame(width: 220, height: 26).clipShape(Capsule())
            HStack(spacing: -8) {
                ForEach(0..<4, id: \.self) { _ in Shimmer().frame(width: 32, height: 32).clipShape(Circle()) }
            }
            ForEach(0..<3, id: \.self) { _ in Shimmer().frame(height: 44).clipShape(RoundedRectangle(cornerRadius: 14)) }
            Spacer()
        }
        .padding(Theme.gutter)
        .accessibilityLabel(Text("party.joining"))
    }
}

/// One line and a way out: removed, can't join.
struct PartyMessage: View {
    let icon: String
    let title: LocalizedStringKey
    var hint: LocalizedStringKey?
    var text: String?
    let close: () -> Void

    var body: some View {
        VStack(spacing: 14) {
            Image(systemName: icon).font(.system(size: 38, weight: .medium)).foregroundStyle(Theme.textTertiary)
            Text(title).displayTitle(26).multilineTextAlignment(.center)
            if let hint { Text(hint).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary).multilineTextAlignment(.center) }
            if let text { Text(verbatim: text).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary).multilineTextAlignment(.center) }
            Button("common.close", action: close).buttonStyle(CreamButtonStyle()).frame(maxWidth: 260).padding(.top, 10)
        }
        .padding(32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// A full party (Party25, R4): in the queue until a seat frees up.
private struct QueueView: View {
    let room: PartyRoom
    let position: Int
    let close: () -> Void

    var body: some View {
        VStack(spacing: 14) {
            Image(systemName: "person.3.sequence").font(.system(size: 38, weight: .medium)).foregroundStyle(Theme.textTertiary)
            if let title = room.party?.item?.label {
                Text(verbatim: title).font(Theme.sans(14, .semibold)).foregroundStyle(Theme.textSecondary)
            }
            Text("party.full.title").displayTitle(28)
            if let host = room.party?.organizer.name {
                Text("party.full.hostKnows \(host)").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                    .multilineTextAlignment(.center)
            }
            HStack(spacing: 8) {
                ProgressView().tint(Theme.textSecondary)
                Text("party.full.queue").font(Theme.sans(13, .semibold))
            }
            .padding(.top, 6)
            Text("party.full.position \(position)").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            Button("party.full.leave") {
                room.leaveQueue()
                close()
            }
            .buttonStyle(.glass)
            .padding(.top, 10)
        }
        .padding(32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
