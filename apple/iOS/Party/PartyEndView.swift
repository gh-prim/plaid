import SwiftUI

/// The end of the party (Party16, R21): who was there, time together,
/// reactions, pause requests and the minute with the most reactions; then
/// schedule the next one with the same people, watch the end again, leave.
struct PartyEndView: View {
    let room: PartyRoom
    let close: () -> Void
    @Environment(AppState.self) private var app
    @Environment(PartyNav.self) private var nav
    @Environment(\.play) private var play

    private var summary: PartySummary? { room.party?.summary }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                Image(systemName: "sparkles").font(.system(size: 32, weight: .medium)).foregroundStyle(Theme.textSecondary)
                    .padding(.top, 40)
                Text("party.end.title").displayTitle(36)
                if let summary {
                    Text("party.end.with \(room.party?.item?.label ?? "") \(ListFormatter.localizedString(byJoining: summary.people))")
                        .font(Theme.sans(16, .medium)).foregroundStyle(Theme.textSecondary)
                    HStack(spacing: 10) {
                        let together = Duration.milliseconds(summary.togetherMs).formatted(.units(allowed: [.hours, .minutes], width: .narrow).locale(AppLanguage.locale))
                        stat(Text("party.end.together \(together)"))
                        stat(Text("party.end.reactions \(summary.reactions)"))
                        stat(Text("party.end.pauses \(summary.pauseRequests)"))
                    }
                    if let peak = summary.peakPositionMs, let emoji = summary.peakEmoji {
                        Text(verbatim: "\(tr("party.end.peak %@", partyClock(peak))) \(emoji)")
                            .font(Theme.sans(14, .semibold))
                            .padding(.horizontal, 14).frame(height: 38)
                            .glassEffect(.regular, in: Capsule())
                    }
                } else {
                    Shimmer().frame(height: 22).clipShape(Capsule())
                    Shimmer().frame(height: 90).clipShape(RoundedRectangle(cornerRadius: 18))
                }
                Spacer(minLength: 20)
                VStack(spacing: 10) {
                    if room.party?.item != nil {
                        Button("party.end.next", action: scheduleNext).buttonStyle(CreamButtonStyle())
                        if room.role == .screen {
                            Button(action: rewatch) {
                                Text("party.end.rewatch").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
                            }
                            .buttonStyle(.glass)
                        }
                    }
                    Button(action: close) {
                        Text("party.end.leave").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
                    }
                    .buttonStyle(.glass)
                }
            }
            .padding(.horizontal, Theme.gutter)
            .frame(maxWidth: 560, alignment: .leading)
            .frame(maxWidth: .infinity)
        }
        .background(alignment: .top) { PartyAmbient(room: room).frame(height: 700).ignoresSafeArea() }
        .task { if room.party?.summary == nil { await room.refresh() } }
    }

    private func stat(_ text: Text) -> some View {
        text.font(Theme.display(20)).lineLimit(2).minimumScaleFactor(0.7)
            .padding(14)
            .frame(maxWidth: .infinity, minHeight: 76, alignment: .leading)
            .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 18))
    }

    /// The launch sheet, scheduled, with the same title and people.
    private func scheduleNext() {
        guard let id = room.party?.item?.id, let client = app.client else { return }
        let people = (room.party?.invites.map(\.person.userId) ?? []) + [room.party?.organizer.userId].compactMap { $0 }
        let mine = room.me.hasPrefix("u:") ? String(room.me.dropFirst(2)) : ""
        close()
        Task {
            guard let d: ItemDetail = try? await client.get("/items/\(id)") else { return }
            try? await Task.sleep(for: .milliseconds(600))
            nav.launch = PartyTarget(item: d.summary, schedule: true, people: unique(people.filter { $0 != mine }))
        }
    }

    /// The last ten minutes again, alone.
    private func rewatch() {
        guard let id = room.party?.item?.id, let client = app.client else { return }
        let from = max(0, room.playback.positionMs - 600_000)
        close()
        Task {
            guard let d: ItemDetail = try? await client.get("/items/\(id)") else { return }
            try? await Task.sleep(for: .milliseconds(600))
            play(d.summary, from: from)
        }
    }
}
