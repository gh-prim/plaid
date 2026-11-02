import SwiftUI
import UIKit

// The pieces every Plaid Party screen shares on iPhone and iPad (F010):
// faces, the presence line, chat, reactions, the host's pause request card,
// the waiting banner and the short sync lines.

/// Where the party screens open from: the item page, the home row,
/// Upcoming, an invitation, a link or a TV's offer.
@Observable
final class PartyNav {
    /// The launch sheet (Party04, 05) for a title, or a vote (VotePh step 1).
    var launch: PartyTarget?
    /// Join with a code (Party07), with the code from a link when there is one.
    var joinCode: CodeRequest?
    /// A scheduled party or an invitation (Party08, 20).
    var detail: PartyRef?
    /// The vote of a party (VotePh).
    var vote: PartyRef?
    /// The party itself, full screen.
    var live: PartyEntry?

    struct CodeRequest: Identifiable { let id = UUID(); var code = "" }
    struct PartyRef: Identifiable, Hashable { let id: String }
}

/// What a party is planned on: a title, an episode not on the server yet
/// (R38), or nothing while the group votes (R31).
struct PartyTarget: Identifiable {
    let id = UUID()
    var item: ItemSummary?
    var future: UpcomingEpisode?
    var schedule = false
    var vote = false
    /// The people of the last party, for "Schedule the next one" (R21).
    var people: [String] = []

    var title: String {
        if let item { return item.kind == .episode ? [item.show?.displayTitle, episodeCode(item), item.name].compactMap { $0 }.joined(separator: " · ") : item.name }
        if let future { return "\(future.showTitle) · \(tr("card.episodeShort %lld %lld", future.season, future.episode))" }
        return tr("vote.open.title")
    }
}

/// How this device enters a party: by id (invitees) or code, as its screen
/// or as the companion of a TV.
struct PartyEntry: Identifiable {
    let id = UUID()
    var partyId: String?
    var code: String?
    var role: PartyRoom.Role = .screen
    var tvSession: String?
    var screenName: String?
}

/// A person's round face in a party: their picture, else their initial on
/// their party color. Dimmed while their screen loads (Party01, 03).
struct PartyFace: View {
    let name: String
    let color: Int
    let url: URL?
    var size: CGFloat = 32
    var dimmed = false
    @State private var image: UIImage?
    @Environment(AppState.self) private var app

    var body: some View {
        Text(verbatim: String(name.prefix(1)).uppercased())
            .font(Theme.display(size * 0.44))
            .foregroundStyle(Theme.onCream)
            .frame(width: size, height: size)
            .background(Circle().fill(Theme.party[abs(color) % Theme.party.count]))
            .overlay {
                if let image {
                    Image(uiImage: image).resizable().scaledToFill().frame(width: size, height: size).clipShape(Circle())
                }
            }
            .opacity(dimmed ? 0.45 : 1)
            .task(id: url) {
                guard let url, let client = app.client else { return }
                if let hit = ImageCache.shared.image(for: url) { image = hit; return }
                guard let data = try? await client.data(for: url), let img = UIImage(data: data) else { return }
                ImageCache.shared.store(img, for: url)
                image = img
            }
            .accessibilityLabel(Text(verbatim: name))
    }
}

extension PartyFace {
    init(_ p: RoomPerson, client: APIClient?, size: CGFloat = 32) {
        let url = p.avatarUrl.flatMap { path in client.map { c -> URL in
            var comps = URLComponents(url: c.absolute(path), resolvingAgainstBaseURL: false)!
            comps.queryItems = [URLQueryItem(name: "size", value: String(Int(size * 2)))]
            return comps.url!
        } }
        self.init(name: p.name, color: p.avatarColor, url: url, size: size, dimmed: p.state == "loading" || p.state == "away")
    }

    init(_ p: PartyPerson, client: APIClient?, size: CGFloat = 32) {
        let url = p.avatar?.url.flatMap { path in client?.avatarURL(path, size: Int(size * 2)) }
        self.init(name: p.name, color: p.avatarColor, url: url, size: size)
    }
}

/// Faces overlapping, then "4 in the party · Sam is loading".
struct PresenceLine: View {
    let room: PartyRoom
    var size: CGFloat = 26
    @Environment(AppState.self) private var app

    var body: some View {
        HStack(spacing: 10) {
            HStack(spacing: -size * 0.3) {
                ForEach(room.presence.people.prefix(5)) { p in
                    PartyFace(p, client: app.client, size: size)
                        .overlay(Circle().strokeBorder(Theme.bgBase, lineWidth: 2))
                }
            }
            Text(verbatim: presenceText(room)).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary).lineLimit(1)
        }
        .accessibilityElement(children: .combine)
    }
}

/// "4 in the party · Sam is loading".
func presenceText(_ room: PartyRoom) -> String {
    var parts = [tr("party.count %lld", room.presence.count)]
    if let loading = room.loadingPeople.first { parts.append(tr("party.presence.loading %@", loading.name)) }
    return parts.joined(separator: " · ")
}

/// The chat (R18): people's lines with their position ("Mia · 38:02"),
/// system lines in grey, my own on the right.
struct ChatList: View {
    let room: PartyRoom
    @Environment(AppState.self) private var app

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 10) {
                    ForEach(room.chat) { line in
                        row(line).id(line.id)
                    }
                }
                .padding(.vertical, 8)
            }
            .scrollIndicators(.hidden)
            .defaultScrollAnchor(.bottom)
            .onChange(of: room.chat.last?.id) { _, id in
                guard let id else { return }
                withAnimation(.smooth) { proxy.scrollTo(id, anchor: .bottom) }
            }
        }
    }

    @ViewBuilder
    private func row(_ line: ChatLine) -> some View {
        if line.system != nil {
            Text(verbatim: partySystemLine(line))
                .font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                .frame(maxWidth: .infinity)
        } else if line.from == room.me {
            Text(verbatim: line.text ?? "")
                .font(Theme.sans(14, .medium)).foregroundStyle(Theme.onCream)
                .padding(.horizontal, 12).padding(.vertical, 8)
                .background(RoundedRectangle(cornerRadius: 16).fill(Theme.cream))
                .frame(maxWidth: .infinity, alignment: .trailing)
                .padding(.leading, 40)
        } else {
            let person = room.presence.people.first { $0.id == line.from }
            HStack(alignment: .top, spacing: 8) {
                if let person {
                    PartyFace(person, client: app.client, size: 26)
                } else {
                    PartyFace(name: line.name ?? "", color: 0, url: nil, size: 26)
                }
                VStack(alignment: .leading, spacing: 2) {
                    Text(verbatim: "\(line.name ?? "") · \(partyClock(line.positionMs))")
                        .font(Theme.sans(11, .semibold)).foregroundStyle(Theme.textTertiary)
                    Text(verbatim: line.text ?? "").font(Theme.sans(14, .medium))
                }
                Spacer(minLength: 30)
            }
        }
    }
}

/// The six reactions (R19), then the message box.
struct ChatComposer: View {
    let room: PartyRoom
    var reactions = true
    @State private var text = ""
    @FocusState private var focused: Bool

    var body: some View {
        VStack(spacing: 10) {
            if reactions { ReactionBar(room: room) }
            HStack(spacing: 8) {
                TextField("party.chat.placeholder", text: $text)
                    .font(Theme.sans(15, .medium))
                    .focused($focused)
                    .submitLabel(.send)
                    .onSubmit(send)
                    .padding(.horizontal, 16).frame(height: 44)
                    .glassEffect(.regular, in: Capsule())
                Button(action: send) {
                    Image(systemName: "arrow.up").font(.system(size: 16, weight: .bold)).frame(width: 44, height: 44)
                }
                .buttonStyle(.glass).buttonBorderShape(.circle)
                .disabled(text.trimmingCharacters(in: .whitespaces).isEmpty)
                .accessibilityLabel(Text("party.chat.send"))
            }
        }
    }

    private func send() {
        let t = text
        text = ""
        Task { await room.send(chat: t) }
    }
}

struct ReactionBar: View {
    let room: PartyRoom

    var body: some View {
        HStack(spacing: 0) {
            ForEach(room.emojis, id: \.self) { e in
                Button {
                    room.react(e)
                    UIImpactFeedbackGenerator(style: .light).impactOccurred()
                } label: {
                    Text(verbatim: e).font(.system(size: 24)).frame(maxWidth: .infinity, minHeight: 40)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(Text("party.react.send \(e)"))
            }
        }
        .padding(.horizontal, 6)
        .glassEffect(.regular, in: Capsule())
    }
}

/// Reactions floating up from the bottom left (Party01, 03), each with who
/// sent it. Still, just fading, with reduced motion.
struct FloatingReactions: View {
    let room: PartyRoom
    var size: CGFloat = 30
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            ForEach(Array(room.floating.enumerated()), id: \.element.id) { i, f in
                Bubble(floating: f, size: size, lane: i % 4, still: reduceMotion)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomLeading)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }

    private struct Bubble: View {
        let floating: PartyRoom.Floating
        let size: CGFloat
        let lane: Int
        let still: Bool
        @State private var up = false

        var body: some View {
            VStack(spacing: 2) {
                Text(verbatim: floating.emoji).font(.system(size: size))
                Text(verbatim: floating.name).font(Theme.sans(size * 0.32, .semibold)).foregroundStyle(Theme.textSecondary)
            }
            .offset(x: CGFloat(lane) * size * 1.3, y: up && !still ? -size * 7 : 0)
            .opacity(up ? 0 : 1)
            .onAppear { withAnimation(.easeOut(duration: 3.6)) { up = true } }
        }
    }
}

/// For the host: "Mia asks for a pause · You're the host, it's your call",
/// then Pause now / In 5 min (R15).
struct PauseRequestCard: View {
    let room: PartyRoom
    let ask: PauseAsk

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 10) {
                if let p = room.presence.people.first(where: { $0.id == ask.by }) { PartyFaceFromEnv(person: p, size: 30) }
                VStack(alignment: .leading, spacing: 2) {
                    Text("party.pause.requested \(ask.name)").font(Theme.sans(15, .bold))
                    Text("party.pause.hostHint").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
                }
                Spacer(minLength: 0)
                Button { Task { await room.answerPause(ask, "no") } } label: {
                    Image(systemName: "xmark").font(.system(size: 12, weight: .bold)).frame(width: 28, height: 28)
                }
                .buttonStyle(.glass).buttonBorderShape(.circle)
                .accessibilityLabel(Text("party.pause.no"))
            }
            HStack(spacing: 8) {
                Button("party.pause.now") { Task { await room.answerPause(ask, "now") } }
                    .buttonStyle(CreamButtonStyle(height: 40))
                Button { Task { await room.answerPause(ask, "in5") } } label: {
                    Text("party.pause.in5").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 30)
                }
                .buttonStyle(.glass)
            }
        }
        .padding(14)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 20))
    }
}

struct PartyFaceFromEnv: View {
    let person: RoomPerson
    var size: CGFloat = 32
    @Environment(AppState.self) private var app

    var body: some View { PartyFace(person, client: app.client, size: size) }
}

/// "Waiting for Sam · Their movie is loading. Everyone resumes together, at
/// 38:12", and after 20 s "Resume without Sam" for who may (R11).
struct WaitingBanner: View {
    let room: PartyRoom
    let waiting: RoomWaiting

    var body: some View {
        let names = ListFormatter.localizedString(byJoining: waiting.names)
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
                ProgressView().tint(Theme.textPrimary)
                Text("party.sync.waiting \(names)").font(Theme.sans(15, .bold))
            }
            Text("party.sync.waitingHint \(partyClock(waiting.positionMs))").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
            TimelineView(.periodic(from: .now, by: 1)) { _ in
                if room.canControl, let at = waiting.resumeWithoutAtMs, room.serverNowMs >= at {
                    Button("party.sync.resumeWithout \(names)") { Task { await room.resumeWithout() } }
                        .font(Theme.sans(13, .semibold))
                        .buttonStyle(.glass)
                        .padding(.top, 4)
                }
            }
        }
        .padding(14)
        .frame(maxWidth: 420, alignment: .leading)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 20))
    }
}

/// "Pause in 4:32" after the host's "In 5 min" (R15).
struct PauseCountdown: View {
    let room: PartyRoom
    let at: Int64

    var body: some View {
        TimelineView(.periodic(from: .now, by: 1)) { _ in
            let left = max(0, at - room.serverNowMs)
            Label { Text("party.pause.countdown \(partyClock(left))") } icon: { Image(systemName: "pause.circle") }
                .font(Theme.sans(13, .semibold))
                .padding(.horizontal, 14).frame(height: 34)
                .glassEffect(.regular, in: Capsule())
        }
    }
}

/// The short lines: "Mia paused", "You were 1.2 s behind…", "Reconnecting…".
struct PartyToasts: View {
    let room: PartyRoom

    var body: some View {
        VStack(spacing: 8) {
            if room.reconnecting {
                Label("party.reconnecting", systemImage: "wifi.exclamationmark").toastStyle()
            }
            if let e = room.event {
                Text(verbatim: e.text).toastStyle().id(e.id).transition(.move(edge: .top).combined(with: .opacity))
            }
        }
        .animation(.smooth, value: room.event)
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.updatesFrequently)
    }
}

extension View {
    func toastStyle() -> some View {
        font(Theme.sans(13, .semibold))
            .padding(.horizontal, 16).padding(.vertical, 10)
            .glassEffect(.regular, in: Capsule())
    }
}

/// The next episode together (Party22 on a phone): ready count, the
/// countdown, and "Let's stop here" for the host (R30).
struct NextTogetherCard: View {
    let room: PartyRoom
    let next: RoomNext

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("party.next.title").font(Theme.sans(11, .semibold)).textCase(.uppercase).foregroundStyle(Theme.textTertiary)
            if let s = next.season, let e = next.episode {
                Text(verbatim: tr("card.episodeShort %lld %lld", s, e)).font(Theme.sans(12, .semibold)).foregroundStyle(Theme.textSecondary)
            }
            Text(verbatim: next.title ?? "").font(Theme.display(19)).lineLimit(1)
            if let r = next.ready, let of = next.of {
                Text("party.next.ready \(r) \(of)").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
            }
            HStack(spacing: 8) {
                TimelineView(.periodic(from: .now, by: 1)) { _ in
                    let left = max(0, Int(((next.startsAtServerMs ?? 0) - room.serverNowMs) / 1000))
                    Text("party.next.startsIn \(left)").font(Theme.sans(14, .semibold)).monospacedDigit()
                        .padding(.horizontal, 14).frame(height: 36)
                        .background(Capsule().fill(Theme.cream)).foregroundStyle(Theme.onCream)
                }
                if room.isHost {
                    Button("party.next.stop") { Task { await room.stopNext() } }
                        .font(Theme.sans(14, .semibold)).buttonStyle(.glass)
                }
            }
            .padding(.top, 4)
        }
        .padding(14)
        .frame(maxWidth: 340, alignment: .leading)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 20))
    }
}

/// A section title in a party sheet.
struct PartySectionTitle: View {
    let key: LocalizedStringKey

    init(_ key: LocalizedStringKey) { self.key = key }

    var body: some View {
        Text(key).font(Theme.sans(12, .semibold)).textCase(.uppercase).tracking(0.6).foregroundStyle(Theme.textTertiary)
    }
}

/// A row to pick among options in a sheet: icon, title, hint, check.
struct PartyChoiceRow: View {
    let icon: String
    let title: Text
    var hint: Text?
    let on: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 12) {
                Image(systemName: icon).font(.system(size: 18, weight: .medium)).frame(width: 30)
                VStack(alignment: .leading, spacing: 2) {
                    title.font(Theme.sans(15, .semibold))
                    if let hint { hint.font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary) }
                }
                Spacer(minLength: 0)
                Image(systemName: on ? "checkmark.circle.fill" : "circle")
                    .font(.system(size: 20)).foregroundStyle(on ? Theme.textPrimary : Theme.textTertiary)
            }
            .padding(14)
            .background(RoundedRectangle(cornerRadius: 16).fill(Color.white.opacity(on ? 0.12 : 0.05)))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(on ? .isSelected : [])
    }
}

/// "Saturday 7 November, 21:00" in the app's language.
func partyDate(_ d: Date, weekday: Bool = true) -> String {
    d.formatted(.dateTime.weekday(weekday ? .wide : .omitted).day().month(.wide).hour().minute().locale(AppLanguage.locale))
}

/// "in 6 days", "in 2 hours".
func partyRelative(_ d: Date) -> String {
    let f = RelativeDateTimeFormatter()
    f.locale = AppLanguage.locale
    f.unitsStyle = .full
    return f.localizedString(for: d, relativeTo: .now)
}
