import AVFoundation
import SwiftUI

/// Watching the party on this device (Party10 iPhone portrait, Party11
/// landscape with the chat, Party12 iPad split): the picture kept on the
/// room by PartySync, controls that go through the room, the chat and the
/// reactions, the lobby and its countdown before it starts (Party09), the
/// next episode together (R30).
struct PartyPlayerScreen: View {
    let room: PartyRoom
    let people: () -> Void
    let close: () -> Void
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @Environment(\.verticalSizeClass) private var vertical
    @State private var model = PlayerModel()
    @State private var clock = PlaybackClock()
    @State private var pip = PictureInPicture()
    @State private var sync: PartySync?
    @State private var controls = true
    @State private var chatOpen = false
    @State private var hide: Task<Void, Never>?

    private var landscape: Bool { vertical == .compact }

    var body: some View {
        Group {
            if wide {
                HStack(spacing: 0) {
                    VStack(spacing: 0) {
                        PartyTopBar(room: room, people: people, close: close)
                        picture.aspectRatio(16 / 9, contentMode: .fit).clipShape(RoundedRectangle(cornerRadius: 18))
                            .padding(.horizontal, Theme.wideGutter)
                        details.padding(.horizontal, Theme.wideGutter).padding(.top, 16)
                        Spacer(minLength: 0)
                    }
                    sidePanel.frame(width: 360).padding(.trailing, 20).padding(.vertical, 16)
                }
            } else if landscape {
                ZStack(alignment: .trailing) {
                    picture.ignoresSafeArea()
                    if chatOpen {
                        VStack(spacing: 10) {
                            HStack {
                                Text("party.chat.title").font(Theme.sans(15, .bold))
                                Spacer()
                                GlassIconButton(systemImage: "xmark", label: "common.close", size: 32) { chatOpen = false }
                            }
                            ChatList(room: room)
                            ChatComposer(room: room)
                        }
                        .padding(14)
                        .frame(width: 320)
                        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 24))
                        .padding(10)
                        .transition(.move(edge: .trailing).combined(with: .opacity))
                    }
                }
            } else {
                VStack(spacing: 0) {
                    PartyTopBar(room: room, people: people, close: close)
                    picture.aspectRatio(16 / 9, contentMode: .fit)
                    VStack(alignment: .leading, spacing: 12) {
                        details
                        ChatList(room: room)
                        ChatComposer(room: room)
                    }
                    .padding(.horizontal, Theme.gutter)
                    .padding(.top, 12)
                    .padding(.bottom, 8)
                }
            }
        }
        .background(Theme.bgBase)
        .background(alignment: .top) { PartyAmbient(room: room).frame(height: 700).ignoresSafeArea() }
        .animation(.smooth(duration: 0.3), value: chatOpen)
        .animation(.smooth(duration: 0.3), value: controls)
        .task(id: room.itemId) { await startIfNeeded() }
        .onChange(of: model.player) { _, p in
            guard let p else { return }
            clock.attach(p, fallbackDuration: Double(model.session?.durationMs ?? 0) / 1000)
            scheduleHide()
        }
        .onChange(of: model.session?.id) { clock.setFallbackDuration(Double(model.session?.durationMs ?? 0) / 1000) }
        .onDisappear {
            sync?.stop()
            model.stop()
            clock.detach()
        }
    }

    private func startIfNeeded() async {
        guard let id = room.itemId, model.player == nil, model.item == nil, let client = app.client, let partyId = room.partyId else { return }
        let profile = app.me?.activeProfile?.id ?? "default"
        let s = PartySync(room: room, model: model)
        sync = s
        s.start()
        await model.start(itemId: id, partyId: partyId, client: client, profileId: profile) {}
    }

    // MARK: Picture

    private var picture: some View {
        ZStack {
            Color.black
            if let player = model.player {
                VideoSurface(player: player, pip: pip)
                    .allowsHitTesting(false)
                if let sub = model.drawnSubtitle, let path = sub.url, let client = app.client {
                    PGSOverlay(url: client.absolute(path), player: player, fill: false)
                }
            } else if !model.blocked && model.error == nil {
                ProgressView().controlSize(.large).tint(Theme.textPrimary)
            }
            if clock.buffering { ProgressView().controlSize(.large).tint(Theme.textPrimary) }
            Color.clear.contentShape(Rectangle()).onTapGesture { controls.toggle(); if controls { scheduleHide() } }
            FloatingReactions(room: room, size: landscape || wide ? 32 : 24)
                .padding(.leading, 16).padding(.bottom, 50)
            if landscape && controls { landscapeControls.transition(.opacity) }
            overlays
        }
        .overlay(alignment: .top) { PartyToasts(room: room).padding(.top, 10) }
    }

    @ViewBuilder
    private var overlays: some View {
        if !room.started {
            LobbyPanel(room: room)
        } else if model.blocked {
            blockedPanel
        } else if let error = model.error {
            VStack(spacing: 12) {
                Text(LocalizedStringKey(error)).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary).multilineTextAlignment(.center)
                Button { model.retry(lower: false) } label: { Label("common.retry", systemImage: "arrow.clockwise") }.buttonStyle(.glass)
            }
            .padding(20)
        }
        VStack(alignment: .trailing, spacing: 10) {
            if let waiting = room.waiting { WaitingBanner(room: room, waiting: waiting) }
            if let at = room.pauseAtServerMs { PauseCountdown(room: room, at: at) }
            if landscape, room.isHost, let ask = room.pauseRequests.first { PauseRequestCard(room: room, ask: ask).frame(maxWidth: 360) }
            if let next = room.next { NextTogetherCard(room: room, next: next) }
        }
        .padding(12)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomTrailing)
    }

    /// Waiting for the host's choice when the server can't convert (R26).
    private var blockedPanel: some View {
        VStack(spacing: 8) {
            Image(systemName: "hourglass").font(.system(size: 28, weight: .medium))
            Text("party.limit.title").font(Theme.sans(16, .bold))
            Text(room.isHost ? "party.limit.hostDecides" : "party.limit.waiting").font(Theme.sans(13, .medium))
                .foregroundStyle(Theme.textSecondary).multilineTextAlignment(.center)
        }
        .padding(20)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 22))
        .padding(20)
    }

    /// Landscape on iPhone (Party11): the controls over the picture, with
    /// the chat button.
    private var landscapeControls: some View {
        ZStack {
            LinearGradient(stops: [.init(color: .black.opacity(0.5), location: 0), .init(color: .clear, location: 0.3),
                                   .init(color: .clear, location: 0.65), .init(color: .black.opacity(0.6), location: 1)],
                           startPoint: .top, endPoint: .bottom)
                .ignoresSafeArea().allowsHitTesting(false)
            VStack {
                HStack(spacing: 10) {
                    GlassIconButton(systemImage: "xmark", label: "party.leave", size: 40, action: close)
                    Text(verbatim: tr("party.count.short %lld", room.presence.count))
                        .font(Theme.sans(13, .bold)).padding(.horizontal, 12).frame(height: 34)
                        .glassEffect(.regular, in: Capsule())
                    Spacer()
                    GlassIconButton(systemImage: "person.2", label: "party.people.title", size: 40, action: people)
                    GlassIconButton(systemImage: "bubble.left.and.bubble.right", label: "party.chat.title", size: 40) { chatOpen.toggle() }
                }
                Spacer()
                Scrubber(clock: clock) { s in Task { await room.control("seek", at: Int64(s * 1000)) } }
                    .disabled(!room.canControl)
            }
            .padding(.horizontal, 20).padding(.vertical, 12)
            PartyTransport(room: room, big: 64)
        }
    }

    // MARK: Details

    /// "Plaid Party · you're watching on this iPhone", the title, the
    /// scrubber and controls, who's here, the host's pause requests.
    private var details: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(UIDevice.current.userInterfaceIdiom == .pad ? "party.watching.iPad" : "party.watching.iPhone")
                .font(Theme.sans(12, .semibold)).foregroundStyle(Theme.textTertiary)
            Text(verbatim: room.party?.item?.label ?? model.item?.name ?? "").font(Theme.display(wide ? 26 : 21)).lineLimit(1)
            Scrubber(clock: clock) { s in Task { await room.control("seek", at: Int64(s * 1000)) } }
                .disabled(!room.canControl)
            PartyTransport(room: room, big: 52)
            if !wide { PresenceLine(room: room) }
            if room.isHost, let ask = room.pauseRequests.first { PauseRequestCard(room: room, ask: ask) }
        }
    }

    /// iPad (Party12): the people, then the chat.
    private var sidePanel: some View {
        VStack(alignment: .leading, spacing: 14) {
            ScrollView { PeopleList(room: room) }.frame(maxHeight: 260).scrollIndicators(.hidden)
            Divider().overlay(Theme.hairline)
            PartySectionTitle("party.chat.title")
            ChatList(room: room)
            ChatComposer(room: room)
        }
        .padding(16)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 26))
    }

    /// The landscape controls fade out after 4 s of playing untouched.
    private func scheduleHide() {
        hide?.cancel()
        hide = Task {
            try? await Task.sleep(for: .seconds(4))
            guard !Task.isCancelled, clock.playing, !chatOpen else { return }
            controls = false
        }
    }
}

/// Before the party plays (Party09, Party23 on a phone): for a scheduled
/// party, the countdown to its time and who is ready; the host can start
/// without waiting. A party just opened waits for its host to start it.
struct LobbyPanel: View {
    let room: PartyRoom
    @Environment(AppState.self) private var app
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(spacing: 12) {
            Text(verbatim: "\(tr("party.title")) · \(room.party?.item?.label ?? "")").font(Theme.sans(13, .semibold))
                .foregroundStyle(Theme.textSecondary).lineLimit(1)
            if let at = room.party?.scheduledAt, at > .now {
                TimelineView(.periodic(from: .now, by: 1)) { ctx in
                    let left = max(0, Int(at.timeIntervalSince(ctx.date)))
                    Text(verbatim: partyClock(Int64(left) * 1000))
                        .font(Theme.display(54)).monospacedDigit()
                        .contentTransition(reduceMotion ? .identity : .numericText(countsDown: true))
                        .animation(.smooth, value: left)
                }
                Text("party.lobby.before \(at.formatted(.dateTime.hour().minute().locale(AppLanguage.locale)))")
                    .font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
            } else if room.presence.people.allSatisfy({ $0.state == "here" }) && room.presence.count > 1 {
                Text("party.lobby.allHere").font(Theme.display(26))
                Text("party.lobby.enjoy").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
            } else {
                Text("party.tv.open").font(Theme.display(24))
            }
            HStack(spacing: 14) {
                ForEach(room.presence.people.prefix(6)) { p in
                    VStack(spacing: 4) {
                        PartyFace(p, client: app.client, size: 36)
                        Text(verbatim: p.id == room.me ? tr("party.people.you") : p.name).font(Theme.sans(11, .semibold)).lineLimit(1)
                        Text(p.state == "loading" ? "party.lobby.loading" : "party.lobby.ready")
                            .font(Theme.sans(10, .medium)).foregroundStyle(Theme.textTertiary)
                    }
                    .frame(width: 56)
                }
            }
            if room.isHost {
                Button(room.party?.scheduledAt.map { $0 > .now } == true ? "party.lobby.startNow" : "party.lobby.start") {
                    Task { await room.start() }
                }
                .buttonStyle(CreamButtonStyle(height: 44))
                .frame(maxWidth: 280)
            } else if let host = room.hostPerson?.name {
                Text("party.lobby.waitingHost \(host)").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            }
        }
        .padding(20)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 26))
        .padding(16)
    }
}
