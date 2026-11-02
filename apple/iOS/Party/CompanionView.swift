import SwiftUI
import UIKit

/// The companion while an Apple TV plays (Party01 on iPhone, Party02 on
/// iPad, R22): the code, the TV's progress with play, pause and ±10 s for
/// whoever has control ("Pause?" for the others), who's here, the host's
/// pause requests, the chat and the reactions.
struct CompanionView: View {
    let room: PartyRoom
    let people: () -> Void
    let close: () -> Void
    @Environment(\.isWide) private var wide

    var body: some View {
        VStack(spacing: 0) {
            PartyTopBar(room: room, people: people, close: close)
            if wide {
                HStack(alignment: .top, spacing: 20) {
                    ScrollView {
                        VStack(alignment: .leading, spacing: 18) {
                            NowPlayingCard(room: room)
                            alerts
                            PeopleList(room: room)
                        }
                    }
                    .scrollIndicators(.hidden)
                    .frame(maxWidth: 440)
                    VStack(spacing: 12) {
                        PartySectionTitle("party.chat.title").frame(maxWidth: .infinity, alignment: .leading)
                        ChatList(room: room)
                        ChatComposer(room: room)
                    }
                    .padding(16)
                    .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 26))
                }
                .padding(.horizontal, Theme.wideGutter)
                .padding(.bottom, 20)
            } else {
                VStack(alignment: .leading, spacing: 14) {
                    NowPlayingCard(room: room)
                    PresenceLine(room: room)
                    alerts
                    ChatList(room: room)
                    ChatComposer(room: room)
                }
                .padding(.horizontal, Theme.gutter)
                .padding(.bottom, 10)
            }
        }
        .background(alignment: .top) { PartyAmbient(room: room).frame(height: 700).ignoresSafeArea() }
        .overlay(alignment: .top) { PartyToasts(room: room).padding(.top, 60) }
        .overlay(alignment: .bottomLeading) { FloatingReactions(room: room).padding(.leading, 24).padding(.bottom, 140) }
    }

    @ViewBuilder
    private var alerts: some View {
        if room.isHost, let ask = room.pauseRequests.first {
            PauseRequestCard(room: room, ask: ask)
        }
        if let waiting = room.waiting {
            WaitingBanner(room: room, waiting: waiting)
        }
        if let at = room.pauseAtServerMs {
            PauseCountdown(room: room, at: at)
        }
        if let next = room.next {
            NextTogetherCard(room: room, next: next)
        }
        if room.isHost, !room.queue.isEmpty {
            Label { Text("party.queue.waiting \(ListFormatter.localizedString(byJoining: room.queue))") } icon: { Image(systemName: "person.badge.clock") }
                .font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
        }
    }
}

/// Close, "Plaid Party", the code (copy, share), and the people for the host.
struct PartyTopBar: View {
    let room: PartyRoom
    let people: () -> Void
    let close: () -> Void
    @Environment(\.isWide) private var wide

    var body: some View {
        HStack(spacing: 10) {
            GlassIconButton(systemImage: "xmark", label: "party.leave", size: 40, action: close)
            VStack(alignment: .leading, spacing: 1) {
                Text("party.title").font(Theme.sans(15, .bold))
                if let name = room.screenName {
                    Text("party.companion.on \(name)").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
                }
            }
            Spacer()
            if let party = room.party {
                Menu {
                    Button { UIPasteboard.general.string = party.code } label: { Label("party.invite.copy", systemImage: "doc.on.doc") }
                    if let url = URL(string: party.link) {
                        ShareLink(item: url) { Label("party.invite.share", systemImage: "square.and.arrow.up") }
                    }
                } label: {
                    Text(verbatim: formattedPartyCode(party.code))
                        .font(.system(size: 14, weight: .bold, design: .monospaced))
                        .padding(.horizontal, 12).frame(height: 34)
                }
                .buttonStyle(.glass)
                .accessibilityLabel(Text("party.code \(party.code)"))
            }
            GlassIconButton(systemImage: "person.2", label: "party.people.title", size: 40, action: people)
        }
        .padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
        .padding(.vertical, 10)
    }
}

/// What the room plays: the title, "38:12 · 56 min left", the thread, and
/// the controls (R13), or "Pause?" without control (R15).
struct NowPlayingCard: View {
    let room: PartyRoom

    private var lengthMs: Int64 { room.party?.item?.runtimeMs ?? 0 }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let item = room.party?.item {
                Text(verbatim: item.label).font(Theme.display(24)).lineLimit(2)
            } else {
                Shimmer().frame(width: 200, height: 24).clipShape(Capsule())
            }
            TimelineView(.periodic(from: .now, by: 1)) { _ in
                let at = room.playback.position(atServerMs: room.serverNowMs)
                VStack(alignment: .leading, spacing: 8) {
                    Text(verbatim: progressLine(at)).font(Theme.sans(13, .semibold).monospacedDigit()).foregroundStyle(Theme.textSecondary)
                    ProgressThread(value: lengthMs > 0 ? min(1, Double(at) / Double(lengthMs)) : 0)
                }
            }
            PartyTransport(room: room)
        }
        .padding(16)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 24))
    }

    private func progressLine(_ at: Int64) -> String {
        guard lengthMs > 0 else { return partyClock(at) }
        let left = max(0, Int((lengthMs - at) / 60000))
        return tr("party.progress %@ %@", partyClock(at), duration(minutes: max(1, left)))
    }
}

/// −10 s, play or pause, +10 s for the room; "Pause?" without control.
struct PartyTransport: View {
    let room: PartyRoom
    var big: CGFloat = 56

    var body: some View {
        if room.canControl {
            HStack(spacing: big * 0.5) {
                Spacer()
                Button { Task { await room.skip(-10) } } label: {
                    Image(systemName: "gobackward.10").font(.system(size: big * 0.38, weight: .medium)).frame(width: big * 0.8, height: big * 0.8)
                }
                .accessibilityLabel(Text("player.back10"))
                let playing = room.playback.state == "playing"
                Button {
                    let at = room.playback.position(atServerMs: room.serverNowMs)
                    Task { await room.control(playing ? "pause" : "play", at: at) }
                } label: {
                    Image(systemName: playing ? "pause.fill" : "play.fill")
                        .font(.system(size: big * 0.4, weight: .bold))
                        .contentTransition(.symbolEffect(.replace))
                        .frame(width: big, height: big)
                }
                .buttonStyle(.glass).buttonBorderShape(.circle)
                .accessibilityLabel(Text(playing ? "player.pause" : "player.play"))
                Button { Task { await room.skip(10) } } label: {
                    Image(systemName: "goforward.10").font(.system(size: big * 0.38, weight: .medium)).frame(width: big * 0.8, height: big * 0.8)
                }
                .accessibilityLabel(Text("player.forward10"))
                Spacer()
            }
            .foregroundStyle(Theme.textPrimary)
        } else {
            Button { Task { await room.askPause() } } label: {
                Label("party.pause.ask", systemImage: "pause.circle").font(Theme.sans(15, .semibold))
                    .frame(maxWidth: .infinity).frame(height: 34)
            }
            .buttonStyle(.glass)
        }
    }
}

/// Everyone in the room: face, name, role, where they watch (device and
/// quality, R20), loading.
struct PeopleList: View {
    let room: PartyRoom
    @Environment(AppState.self) private var app

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            PartySectionTitle("party.people.title")
            Text(verbatim: presenceText(room)).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
            ForEach(room.presence.people) { p in
                HStack(spacing: 12) {
                    PartyFace(p, client: app.client, size: 36)
                    VStack(alignment: .leading, spacing: 2) {
                        Text(verbatim: p.id == room.me ? tr("party.people.you") : p.name).font(Theme.sans(15, .semibold))
                        Text(verbatim: personLine(p, me: room.me)).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
                    }
                    Spacer()
                    if p.state == "loading" { ProgressView().controlSize(.small).tint(Theme.textSecondary) }
                }
            }
        }
    }
}

/// "Host · iPad · 1080p", "Guest · browser · Loading…".
func personLine(_ p: RoomPerson, me: String) -> String {
    var parts: [String] = []
    if p.host { parts.append(tr("party.people.host")) }
    if p.guest { parts.append(tr("party.people.guest")) }
    if let s = p.screen { parts.append(partyDevice(s)) } else if p.companion { parts.append(tr("party.people.companionOnly")) }
    switch p.state {
    case "loading": parts.append(tr("party.people.loading"))
    case "away": parts.append(tr("party.people.away"))
    case "waiting": parts.append(tr("party.people.waiting"))
    default: if let q = p.quality, !q.isEmpty { parts.append(partyQuality(q)) }
    }
    return parts.joined(separator: " · ")
}

/// The title's backdrop, blurred wide behind a party screen.
struct PartyAmbient: View {
    let room: PartyRoom
    @Environment(AppState.self) private var app
    @State private var art: Artwork?

    var body: some View {
        AmbientGlow(art: art, brightness: -0.6)
            .task(id: room.itemId) {
                guard let id = room.itemId, let client = app.client,
                      let d: ItemDetail = try? await client.get("/items/\(id)") else { return }
                art = d.summary.backdrop ?? d.summary.still ?? d.summary.poster
            }
    }
}
