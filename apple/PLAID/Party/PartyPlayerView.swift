import AVFoundation
import SwiftUI
import UIKit

/// What the Apple TV plays as a party screen: one party at a time, opened
/// from a title (Party23), a cast from the phone (`tv.cast`, R22) or a vote
/// winner. MainView presents it full screen.
@Observable
final class TVPartyNav {
    struct Open: Identifiable, Equatable {
        let id = UUID()
        let partyId: String
    }

    var open: Open?
    /// A closed vote whose winner the TV reveals (VoteBig).
    var reveal: Open?
}

/// A Plaid Party on the Apple TV (F010-R24): the picture kept on the room
/// by PartySync, and over it the room (Party03): faces top right (loading
/// ones dimmed), reactions floating up from the left, the last message
/// bottom left, a pause request at the top ("answer on your iPhone"), the
/// waiting line, the next episode together (Party22). Play/pause plays or
/// pauses for everyone with control, asks for a pause otherwise; a long
/// press opens the six reactions (Party24). Before it starts: the code and
/// who's arriving (Party23).
struct PartyPlayerView: View {
    let partyId: String
    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(\.dismiss) private var dismiss
    @State private var room: PartyRoom?
    @State private var model = PlayerModel()
    @State private var sync: PartySync?
    @State private var picker = false
    @State private var info = false
    @State private var leaving = false
    @State private var infoHide: Task<Void, Never>?

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            if let player = model.player {
                PartyVideo(player: player).ignoresSafeArea()
            }
            if let room {
                RemoteCatcher(enabled: catcherEnabled(room),
                              playPause: { playPause(room) },
                              longPlayPause: { withAnimation(.smooth) { picker = true } },
                              select: { showInfo() },
                              arrow: { left in if room.canControl { Task { await room.skip(left ? -10 : 10) } } })
                    .ignoresSafeArea()
                overlay(room)
            } else {
                LoadingView()
            }
        }
        .onAppear {
            if room == nil { room = center.enter(partyId: partyId, role: .screen) }
        }
        .task(id: room?.itemId) { await startIfNeeded() }
        .onExitCommand {
            if picker { withAnimation(.smooth) { picker = false } } else { leaving = true }
        }
        .confirmationDialog(Text("party.tv.leaveTitle"), isPresented: $leaving) {
            if room?.isHost == true, room?.others.isEmpty == false {
                if let next = room?.others.first(where: { !$0.guest }) {
                    Button("party.host.handTo \(next.name)") {
                        Task { await room?.makeHost(next); close() }
                    }
                }
                Button("party.host.endAll", role: .destructive) { Task { await room?.end(); close() } }
            }
            Button("party.leave") { close() }
            Button("party.tv.stay", role: .cancel) {}
        }
        .onDisappear {
            sync?.stop()
            model.stop()
        }
    }

    private func close() {
        center.leave()
        dismiss()
    }

    private func startIfNeeded() async {
        guard let room, let id = room.itemId, model.item == nil, let client = app.client, let pid = room.partyId else { return }
        let s = PartySync(room: room, model: model)
        sync = s
        s.start()
        await model.start(itemId: id, partyId: pid, client: client, profileId: app.me?.activeProfile?.id ?? "default") {}
    }

    /// The remote's presses go to the picture, except when a panel has
    /// buttons to focus.
    private func catcherEnabled(_ room: PartyRoom) -> Bool {
        !picker && !leaving && room.joined && room.started && !(room.isHost && room.next != nil) && !room.ended
    }

    private func playPause(_ room: PartyRoom) {
        let playing = room.playback.state == "playing"
        let at = room.playback.position(atServerMs: room.serverNowMs)
        Task { await room.control(playing ? "pause" : "play", at: at) }
        showInfo()
    }

    private func showInfo() {
        withAnimation(.smooth) { info = true }
        infoHide?.cancel()
        infoHide = Task {
            try? await Task.sleep(for: .seconds(5))
            guard !Task.isCancelled else { return }
            withAnimation(.smooth) { info = false }
        }
    }

    // MARK: Overlay

    @ViewBuilder
    private func overlay(_ room: PartyRoom) -> some View {
        if room.ended {
            TVPartyEnd(room: room) { close() }
        } else if room.removed {
            TVPartyMessage(title: "party.removed.title", text: tr("party.removed.hint")) { close() }
        } else if let failure = room.failure {
            TVPartyMessage(title: "party.join.failed", text: partyError(failure)) { close() }
        } else if !room.joined {
            LoadingView()
        } else {
            ZStack {
                TVFloatingReactions(room: room)
                    .padding(.leading, 80).padding(.bottom, 200)
                VStack(alignment: .leading, spacing: 0) {
                    HStack(alignment: .top) {
                        topCenter(room).frame(maxWidth: .infinity, alignment: .leading)
                        TVPresence(room: room)
                    }
                    Spacer()
                    HStack(alignment: .bottom) {
                        if let line = room.lastMessage { TVLastMessage(room: room, line: line).id(line.id) }
                        Spacer()
                        if let next = room.next { TVNextEpisode(room: room, next: next) }
                    }
                    if info { TVPartyInfo(room: room, model: model).padding(.top, 24).transition(.opacity) }
                }
                .padding(.horizontal, 80)
                .padding(.vertical, 60)
                if !room.started {
                    TVPartyOpen(room: room).transition(.opacity)
                } else if model.blocked {
                    TVPartyMessage(title: "party.limit.title", text: tr(room.isHost ? "party.limit.hostDecidesPhone" : "party.limit.waiting"), close: nil)
                } else if let error = model.error {
                    TVPartyMessage(title: "party.tv.playbackError", text: localized(error)) { model.retry(lower: false) }
                }
                if picker {
                    TVReactionPicker(room: room) { withAnimation(.smooth) { picker = false } }
                        .frame(maxHeight: .infinity, alignment: .bottom)
                        .padding(.bottom, 80)
                        .transition(.move(edge: .bottom).combined(with: .opacity))
                }
            }
        }
    }

    /// Pause request, waiting, the pause countdown, the short sync lines.
    @ViewBuilder
    private func topCenter(_ room: PartyRoom) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            if let ask = room.pauseRequests.first {
                HStack(spacing: 14) {
                    if let p = room.presence.people.first(where: { $0.id == ask.by }) { RoomFace(person: p, size: 44) }
                    Text(verbatim: room.isHost ? "\(tr("party.pause.requested %@", ask.name)) · \(tr("party.pause.answerOnPhone"))"
                         : tr("party.pause.requested %@", ask.name))
                        .font(Theme.sans(26, .semibold))
                }
                .tvPill()
            }
            if let waiting = room.waiting {
                VStack(alignment: .leading, spacing: 6) {
                    HStack(spacing: 14) {
                        ProgressView()
                        Text(verbatim: tr("party.sync.waiting %@", ListFormatter.localizedString(byJoining: waiting.names))).font(Theme.sans(28, .semibold))
                    }
                    Text(verbatim: tr("party.sync.waitingHint %@", partyClock(waiting.positionMs))).font(Theme.sans(22)).foregroundStyle(Theme.textSecondary)
                }
                .tvPill()
            }
            if let at = room.pauseAtServerMs {
                TimelineView(.periodic(from: .now, by: 1)) { _ in
                    Text(verbatim: tr("party.pause.countdown %@", partyClock(max(0, at - room.serverNowMs)))).font(Theme.sans(24, .semibold)).tvPill()
                }
            }
            if room.reconnecting { Text("party.reconnecting").font(Theme.sans(24, .semibold)).tvPill() }
            if let e = room.event { Text(verbatim: e.text).font(Theme.sans(24, .semibold)).tvPill().id(e.id).transition(.opacity) }
        }
        .animation(.smooth, value: room.event)
    }
}

// MARK: Pieces

extension View {
    /// A dark glass pill over the picture.
    func tvPill() -> some View {
        padding(.horizontal, 28).padding(.vertical, 16)
            .background(Capsule().fill(.ultraThinMaterial))
            .background(Capsule().fill(Theme.bgBase.opacity(0.35)))
    }
}

/// The picture, without the system's transport bar.
private struct PartyVideo: UIViewRepresentable {
    let player: AVPlayer

    final class LayerView: UIView {
        override class var layerClass: AnyClass { AVPlayerLayer.self }
        var playerLayer: AVPlayerLayer { layer as! AVPlayerLayer }
    }

    func makeUIView(context: Context) -> LayerView {
        let v = LayerView()
        v.backgroundColor = .black
        v.playerLayer.player = player
        v.playerLayer.videoGravity = .resizeAspect
        return v
    }

    func updateUIView(_ v: LayerView, context: Context) {
        if v.playerLayer.player !== player { v.playerLayer.player = player }
    }
}

/// Takes the remote's presses over the picture: play/pause (a tap, or a
/// hold for the reactions), click, and the clickpad's left and right edges.
/// Menu goes on to SwiftUI.
private struct RemoteCatcher: UIViewRepresentable {
    let enabled: Bool
    let playPause: () -> Void
    let longPlayPause: () -> Void
    let select: () -> Void
    let arrow: (_ left: Bool) -> Void

    final class CatcherView: UIView {
        var enabled = true
        var playPause: () -> Void = {}
        var longPlayPause: () -> Void = {}
        var select: () -> Void = {}
        var arrow: (Bool) -> Void = { _ in }
        private var hold: Task<Void, Never>?
        private var held = false

        override var canBecomeFocused: Bool { enabled }

        override func pressesBegan(_ presses: Set<UIPress>, with event: UIPressesEvent?) {
            guard enabled, let p = presses.first else { return super.pressesBegan(presses, with: event) }
            switch p.type {
            case .playPause:
                held = false
                hold = Task { @MainActor [weak self] in
                    try? await Task.sleep(for: .milliseconds(500))
                    guard let self, !Task.isCancelled else { return }
                    self.held = true
                    self.longPlayPause()
                }
            case .select, .leftArrow, .rightArrow:
                break
            default:
                super.pressesBegan(presses, with: event)
            }
        }

        override func pressesEnded(_ presses: Set<UIPress>, with event: UIPressesEvent?) {
            guard enabled, let p = presses.first else { return super.pressesEnded(presses, with: event) }
            switch p.type {
            case .playPause:
                hold?.cancel()
                if !held { playPause() }
            case .select: select()
            case .leftArrow: arrow(true)
            case .rightArrow: arrow(false)
            default: super.pressesEnded(presses, with: event)
            }
        }

        override func pressesCancelled(_ presses: Set<UIPress>, with event: UIPressesEvent?) {
            hold?.cancel()
            super.pressesCancelled(presses, with: event)
        }
    }

    func makeUIView(context: Context) -> CatcherView {
        let v = CatcherView()
        v.backgroundColor = .clear
        update(v)
        return v
    }

    func updateUIView(_ v: CatcherView, context: Context) {
        let was = v.enabled
        update(v)
        if was != enabled { v.setNeedsFocusUpdate() }
    }

    private func update(_ v: CatcherView) {
        v.enabled = enabled
        v.playPause = playPause
        v.longPlayPause = longPlayPause
        v.select = select
        v.arrow = arrow
    }
}

/// A person's face in the party: picture, else initial on their color.
struct RoomFace: View {
    let person: RoomPerson
    var size: CGFloat = 56
    @Environment(AppState.self) private var app
    @State private var image: UIImage?

    var body: some View {
        Text(verbatim: String(person.name.prefix(1)).uppercased())
            .font(Theme.sans(size * 0.46, .semibold))
            .foregroundStyle(Theme.bgBase)
            .frame(width: size, height: size)
            .background(Circle().fill(Theme.party[abs(person.avatarColor) % Theme.party.count]))
            .overlay {
                if let image { Image(uiImage: image).resizable().scaledToFill().frame(width: size, height: size).clipShape(Circle()) }
            }
            .opacity(person.state == "loading" || person.state == "away" ? 0.4 : 1)
            .task(id: person.avatarUrl) {
                guard let path = person.avatarUrl, let client = app.client else { return }
                var c = URLComponents(url: client.absolute(path), resolvingAgainstBaseURL: false)!
                c.queryItems = [URLQueryItem(name: "size", value: String(Int(size * 2)))]
                guard let url = c.url else { return }
                if let hit = ImageCache.shared.image(for: url) { image = hit; return }
                guard let data = try? await client.data(for: url), let img = UIImage(data: data) else { return }
                ImageCache.shared.store(img, for: url)
                image = img
            }
    }
}

/// Top right: everyone's face and first name, loading ones dimmed.
private struct TVPresence: View {
    let room: PartyRoom

    var body: some View {
        HStack(spacing: 18) {
            ForEach(room.presence.people.prefix(8)) { p in
                VStack(spacing: 6) {
                    RoomFace(person: p, size: 56)
                        .overlay(alignment: .bottomTrailing) {
                            if p.host { Image(systemName: "crown.fill").font(.system(size: 14)).foregroundStyle(Theme.tan) }
                        }
                    Text(verbatim: p.name).font(Theme.sans(18, .semibold)).lineLimit(1)
                        .opacity(p.state == "loading" ? 0.5 : 1)
                }
                .frame(width: 80)
            }
        }
        .accessibilityElement(children: .combine)
    }
}

/// Reactions rising from the left, with who sent them; still with reduced motion.
private struct TVFloatingReactions: View {
    let room: PartyRoom
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            ForEach(Array(room.floating.enumerated()), id: \.element.id) { i, f in
                Bubble(floating: f, lane: i % 3, still: reduceMotion)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomLeading)
        .allowsHitTesting(false)
    }

    private struct Bubble: View {
        let floating: PartyRoom.Floating
        let lane: Int
        let still: Bool
        @State private var up = false

        var body: some View {
            VStack(spacing: 4) {
                Text(verbatim: floating.emoji).font(.system(size: 64))
                Text(verbatim: floating.name).font(Theme.sans(18, .semibold)).foregroundStyle(Theme.textSecondary)
            }
            .offset(x: CGFloat(lane) * 90, y: up && !still ? -460 : 0)
            .opacity(up ? 0 : 1)
            .onAppear { withAnimation(.easeOut(duration: 3.8)) { up = true } }
        }
    }
}

/// Bottom left: the last message, read-only on the TV (R24), fading after a while.
private struct TVLastMessage: View {
    let room: PartyRoom
    let line: ChatLine
    @State private var shown = true

    var body: some View {
        HStack(alignment: .top, spacing: 16) {
            if let p = room.presence.people.first(where: { $0.id == line.from }) { RoomFace(person: p, size: 48) }
            VStack(alignment: .leading, spacing: 4) {
                Text(verbatim: line.name ?? "").font(Theme.sans(20, .semibold)).foregroundStyle(Theme.textSecondary)
                Text(verbatim: line.text ?? "").font(Theme.sans(26, .medium)).lineLimit(3)
            }
        }
        .padding(20)
        .frame(maxWidth: 760, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 24).fill(.ultraThinMaterial))
        .opacity(shown ? 1 : 0)
        .task {
            try? await Task.sleep(for: .seconds(10))
            withAnimation(.smooth(duration: 0.8)) { shown = false }
        }
    }
}

/// The title and where the room is, after a press.
private struct TVPartyInfo: View {
    let room: PartyRoom
    let model: PlayerModel

    var body: some View {
        TimelineView(.periodic(from: .now, by: 1)) { _ in
            let at = room.playback.position(atServerMs: room.serverNowMs)
            let length = model.session?.durationMs ?? room.party?.item?.runtimeMs ?? 0
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text(verbatim: room.party?.item?.label ?? model.item?.name ?? "").font(Theme.sans(34, .semibold))
                    Spacer()
                    Text(verbatim: "\(partyClock(at)) / \(partyClock(length))").font(Theme.sans(24, .medium)).monospacedDigit()
                        .foregroundStyle(Theme.textSecondary)
                }
                ProgressThread(value: length > 0 ? min(1, Double(at) / Double(length)) : 0)
                Text(verbatim: room.canControl ? tr("party.tv.controlHint") : tr("party.tv.askHint"))
                    .font(Theme.sans(20)).foregroundStyle(Theme.textTertiary)
            }
        }
        .padding(28)
        .background(RoundedRectangle(cornerRadius: 24).fill(.ultraThinMaterial))
    }
}

/// The six reactions (Party24): swipe to choose, click to send.
private struct TVReactionPicker: View {
    let room: PartyRoom
    let close: () -> Void
    @FocusState private var focus: String?
    @State private var idle: Task<Void, Never>?

    var body: some View {
        VStack(spacing: 18) {
            HStack(spacing: 20) {
                ForEach(room.emojis, id: \.self) { e in
                    Button {
                        room.react(e)
                        keepOpen()
                    } label: {
                        Text(verbatim: e).font(.system(size: 60))
                    }
                    .buttonStyle(EmojiButtonStyle())
                    .focused($focus, equals: e)
                }
            }
            .padding(.horizontal, 30).padding(.vertical, 18)
            .background(Capsule().fill(.ultraThinMaterial))
            .focusSection()
            Text("party.tv.reactHint").font(Theme.sans(20)).foregroundStyle(Theme.textSecondary)
        }
        .defaultFocus($focus, room.emojis.first)
        .onAppear { focus = room.emojis.first; keepOpen() }
    }

    /// Closes after 6 s without a reaction.
    private func keepOpen() {
        idle?.cancel()
        idle = Task {
            try? await Task.sleep(for: .seconds(6))
            guard !Task.isCancelled else { return }
            close()
        }
    }

    private struct EmojiButtonStyle: ButtonStyle {
        func makeBody(configuration: Configuration) -> some View {
            EmojiBody(configuration: configuration)
        }

        private struct EmojiBody: View {
            let configuration: ButtonStyleConfiguration
            @Environment(\.isFocused) private var focused

            var body: some View {
                configuration.label
                    .frame(width: 100, height: 100)
                    .background(Circle().fill(Color.white.opacity(focused ? 0.3 : 0)))
                    .scaleEffect(focused ? 1.3 : 1)
                    .scaleEffect(configuration.isPressed ? 0.9 : 1)
                    .animation(.spring(response: 0.3, dampingFraction: 0.6), value: focused)
            }
        }
    }
}

/// The next episode together (Party22, R30).
private struct TVNextEpisode: View {
    let room: PartyRoom
    let next: RoomNext
    @FocusState private var stopFocused: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("party.next.title").font(Theme.sans(20, .semibold)).foregroundStyle(Theme.textSecondary)
            if let s = next.season, let e = next.episode {
                let meta = [tr("card.episodeShort %lld %lld", s, e),
                            next.runtimeMs.map { Duration.milliseconds($0).formatted(.units(allowed: [.hours, .minutes], width: .narrow).locale(AppLanguage.locale)) }]
                Text(verbatim: meta.compactMap { $0 }.joined(separator: " · ")).font(Theme.sans(22, .semibold))
            }
            Text(verbatim: next.title ?? "").font(Theme.sans(38, .bold)).lineLimit(1)
            if let r = next.ready, let of = next.of {
                Text(verbatim: tr("party.next.ready %lld %lld", r, of)).font(Theme.sans(22)).foregroundStyle(Theme.textSecondary)
            }
            HStack(spacing: 20) {
                TimelineView(.periodic(from: .now, by: 1)) { _ in
                    let left = max(0, Int(((next.startsAtServerMs ?? 0) - room.serverNowMs) / 1000))
                    Text(verbatim: tr("party.next.startsIn %lld", left)).font(Theme.sans(24, .semibold)).monospacedDigit()
                        .padding(.horizontal, 28).frame(height: 60)
                        .background(Capsule().fill(Theme.action)).foregroundStyle(Theme.onAction)
                }
                if room.isHost {
                    Button("party.next.stop") { Task { await room.stopNext() } }
                        .buttonStyle(PillButtonStyle(primary: false))
                        .focused($stopFocused)
                }
            }
            .padding(.top, 8)
        }
        .padding(28)
        .frame(width: 720, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 28).fill(.ultraThinMaterial))
        .focusSection()
        .onAppear { stopFocused = room.isHost }
    }
}

/// Before the party plays (Party23, and a scheduled party's lobby): the
/// code, the hint that the chat is on the iPhone, who's here and who's
/// arriving, the countdown, and Start for the host.
private struct TVPartyOpen: View {
    let room: PartyRoom
    @FocusState private var startFocused: Bool

    var body: some View {
        HStack(alignment: .center, spacing: 80) {
            VStack(alignment: .leading, spacing: 20) {
                Text(verbatim: "\(tr("party.title")) · \(room.party?.item?.label ?? "")").font(Theme.sans(26, .semibold))
                    .foregroundStyle(Theme.textSecondary).lineLimit(1)
                if let at = room.party?.scheduledAt, at > .now {
                    TimelineView(.periodic(from: .now, by: 1)) { ctx in
                        Text(verbatim: partyClock(Int64(max(0, at.timeIntervalSince(ctx.date))) * 1000))
                            .font(Theme.sans(120, .bold)).monospacedDigit()
                    }
                    Text(verbatim: tr("party.lobby.before %@", at.formatted(.dateTime.hour().minute().locale(AppLanguage.locale))))
                        .font(Theme.sans(28)).foregroundStyle(Theme.textSecondary)
                } else {
                    Text("party.tv.open").font(Theme.sans(64, .bold))
                    Text("party.tv.openHint").font(Theme.sans(28)).foregroundStyle(Theme.textSecondary).frame(maxWidth: 900, alignment: .leading)
                }
                HStack(spacing: 24) {
                    ForEach(room.presence.people.prefix(8)) { p in
                        VStack(spacing: 8) {
                            RoomFace(person: p, size: 72)
                            Text(verbatim: p.id == room.me ? tr("party.people.you") : p.name).font(Theme.sans(20, .semibold)).lineLimit(1)
                            Text(p.state == "loading" ? "party.tv.arriving" : "party.lobby.ready").font(Theme.sans(16)).foregroundStyle(Theme.textTertiary)
                        }
                        .frame(width: 110)
                    }
                }
                if room.isHost {
                    Button { Task { await room.start() } } label: { Label("party.lobby.start", systemImage: "play.fill") }
                        .buttonStyle(PillButtonStyle())
                        .focused($startFocused)
                        .padding(.top, 10)
                } else if let host = room.hostPerson?.name {
                    Text(verbatim: tr("party.lobby.waitingHost %@", host)).font(Theme.sans(24)).foregroundStyle(Theme.textTertiary)
                }
            }
            if let code = room.party?.code {
                VStack(spacing: 12) {
                    Text("party.tv.codeLabel").font(Theme.sans(22, .semibold)).foregroundStyle(Theme.textSecondary)
                    Text(verbatim: tvPartyCode(code)).font(.system(size: 76, weight: .bold, design: .monospaced)).tracking(8)
                }
                .padding(40)
                .background(RoundedRectangle(cornerRadius: 32).fill(.ultraThinMaterial))
            }
        }
        .padding(80)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Theme.bgBase.opacity(0.82))
        .focusSection()
        .onAppear { startFocused = room.isHost }
    }
}

func tvPartyCode(_ code: String) -> String {
    let raw = code.uppercased().filter { $0.isLetter || $0.isNumber }
    return raw.count == 6 ? "\(raw.prefix(3))-\(raw.suffix(3))" : code
}

/// One line over a dim picture and a button: removed, an error, the limit.
private struct TVPartyMessage: View {
    let title: LocalizedStringKey
    let text: String
    let close: (() -> Void)?

    var body: some View {
        VStack(spacing: 24) {
            Text(title).font(Theme.sans(48, .bold))
            Text(verbatim: text).font(Theme.sans(28)).foregroundStyle(Theme.textSecondary).multilineTextAlignment(.center).frame(maxWidth: 1100)
            if let close { Button("common.close", action: close).buttonStyle(PillButtonStyle()) }
        }
        .padding(60)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Theme.bgBase.opacity(0.85))
    }
}

/// The end of the party on the TV (R21): who, how long, the reactions.
private struct TVPartyEnd: View {
    let room: PartyRoom
    let close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 28) {
            Text("party.end.title").font(Theme.sans(64, .bold))
            if let s = room.party?.summary {
                Text(verbatim: tr("party.end.with %@ %@", room.party?.item?.label ?? "", ListFormatter.localizedString(byJoining: s.people)))
                    .font(Theme.sans(30)).foregroundStyle(Theme.textSecondary)
                HStack(spacing: 60) {
                    let together = Duration.milliseconds(s.togetherMs).formatted(.units(allowed: [.hours, .minutes], width: .narrow).locale(AppLanguage.locale))
                    stat(tr("party.end.together %@", together))
                    stat(tr("party.end.reactions %lld", s.reactions))
                    stat(tr("party.end.pauses %lld", s.pauseRequests))
                }
                if let peak = s.peakPositionMs, let emoji = s.peakEmoji {
                    Text(verbatim: "\(tr("party.end.peak %@", partyClock(peak))) \(emoji)").font(Theme.sans(28, .semibold))
                }
            } else {
                ProgressView()
            }
            Button("party.end.leave", action: close).buttonStyle(PillButtonStyle()).padding(.top, 20)
        }
        .padding(120)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
        .background(Theme.bgBase.opacity(0.92))
        .task { if room.party?.summary == nil { await room.refresh() } }
    }

    private func stat(_ text: String) -> some View {
        Text(verbatim: text).font(Theme.sans(40, .bold))
    }
}
