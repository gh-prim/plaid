import AVFoundation
import SwiftUI

/// The scrubber: elapsed time, the track, and what's left (IPh07). Dragging
/// shows where it will land; letting go seeks there.
struct Scrubber: View {
    let clock: PlaybackClock
    let seek: (Double) -> Void
    @State private var dragging: Double?

    var body: some View {
        let d = max(clock.duration, 1)
        let at = dragging ?? clock.seconds
        HStack(spacing: 12) {
            Text(verbatim: clockTime(at)).frame(minWidth: 40, alignment: .leading)
            GeometryReader { g in
                ZStack(alignment: .leading) {
                    Capsule().fill(Color.white.opacity(0.28)).frame(height: dragging == nil ? 4 : 8)
                    Capsule().fill(Theme.textPrimary).frame(width: max(4, g.size.width * min(1, at / d)), height: dragging == nil ? 4 : 8)
                }
                .frame(maxHeight: .infinity)
                .contentShape(Rectangle())
                .gesture(DragGesture(minimumDistance: 0)
                    .onChanged { v in dragging = min(max(0, v.location.x / g.size.width), 1) * d }
                    .onEnded { _ in
                        if let dragging { seek(dragging) }
                        dragging = nil
                    })
                .animation(.smooth(duration: 0.15), value: dragging == nil)
            }
            .frame(height: 24)
            Text(verbatim: clockTime(at - d)).frame(minWidth: 44, alignment: .trailing)
        }
        .font(Theme.sans(12, .semibold).monospacedDigit())
        .foregroundStyle(Theme.textSecondary)
        .accessibilityElement()
        .accessibilityLabel(Text("player.position"))
        .accessibilityValue(Text(verbatim: "\(clockTime(clock.seconds)) / \(clockTime(d))"))
        .accessibilityAdjustableAction { dir in seek(clock.seconds + (dir == .increment ? 10 : -10)) }
    }
}

/// -10 s, play or pause, +10 s.
struct TransportButtons: View {
    let player: AVPlayer
    let clock: PlaybackClock
    var big: CGFloat = 70

    var body: some View {
        HStack(spacing: big * 0.6) {
            Button { player.seek(by: -10) } label: {
                Image(systemName: "gobackward.10").font(.system(size: big * 0.36, weight: .medium)).frame(width: big * 0.7, height: big * 0.7)
            }
            .accessibilityLabel(Text("player.back10"))
            Button { clock.playing ? player.pause() : player.play() } label: {
                Image(systemName: clock.playing ? "pause.fill" : "play.fill")
                    .font(.system(size: big * 0.38, weight: .bold))
                    .contentTransition(.symbolEffect(.replace))
                    .frame(width: big, height: big)
            }
            .buttonStyle(.glass).buttonBorderShape(.circle)
            .accessibilityLabel(Text(clock.playing ? "player.pause" : "player.play"))
            Button { player.seek(by: 10) } label: {
                Image(systemName: "goforward.10").font(.system(size: big * 0.36, weight: .medium)).frame(width: big * 0.7, height: big * 0.7)
            }
            .accessibilityLabel(Text("player.forward10"))
        }
        .foregroundStyle(Theme.textPrimary)
    }
}

/// Subtitles and audio side by side in a glass panel (IPh08): text
/// subtitles from the stream, image ones drawn by the server, and the
/// file's audio tracks, which the server switches.
struct TracksPanel: View {
    let model: PlayerModel
    let close: () -> Void
    @State private var group: AVMediaSelectionGroup?
    @State private var selected: AVMediaSelectionOption?

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            column("player.subtitles") {
                row(Text("player.subtitles.off"), detail: nil, on: selected == nil && burned == nil && model.drawnIndex == nil) {
                    selectText(nil)
                }
                ForEach(textOptions, id: \.self) { o in
                    row(Text(verbatim: languageName(o.extendedLanguageTag ?? "und")),
                        detail: o.hasMediaCharacteristic(.describesMusicAndSoundForAccessibility) ? Text("player.subtitles.sdh") : nil,
                        on: selected == o && burned == nil && model.drawnIndex == nil) { selectText(o) }
                }
                // Blu-ray subtitles the app draws: the picture stays the original.
                ForEach(model.drawnSubtitles, id: \.index) { s in
                    row(Text(verbatim: model.subtitleLabel(s)), detail: nil, on: model.drawnIndex == s.index) { selectDrawn(s) }
                }
                ForEach(model.imageSubtitles, id: \.index) { s in
                    row(Text(verbatim: model.subtitleLabel(s)), detail: Text("player.subtitles.image"),
                        on: burned?.index == s.index) { Task { await model.burnSubtitles(s); close() } }
                }
            }
            if let tracks = model.session?.audioTracks, !tracks.isEmpty {
                column("player.audio") {
                    ForEach(tracks, id: \.index) { t in
                        row(Text(verbatim: languageName(t.language)), detail: Text(verbatim: channelsLabel(t)), on: t.selected) {
                            Task { await model.switchAudio(to: t); close() }
                        }
                    }
                }
            }
        }
        .padding(8)
        .frame(maxWidth: 440)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 24))
        .task(id: model.player?.currentItem) { await loadGroup() }
    }

    private var burned: PlaybackSubtitle? {
        model.imageSubtitles.first { $0.selected && $0.delivery == "burn_in" }
    }

    /// The stream's subtitles, without the forced ones the player shows on
    /// its own nor the closed captions it finds in the video.
    private var textOptions: [AVMediaSelectionOption] {
        (group?.options ?? []).filter {
            $0.mediaType == .subtitle && !$0.hasMediaCharacteristic(.containsOnlyForcedSubtitles)
        }
    }

    private func channelsLabel(_ t: PlaybackAudioTrack) -> String {
        let ch = t.channels > 2 ? "\(t.channels - 1).1" : (t.channels == 1 ? tr("player.audio.mono") : tr("player.audio.stereo"))
        return [ch, t.title].compactMap { $0 }.joined(separator: " · ")
    }

    private func column<C: View>(_ title: LocalizedStringKey, @ViewBuilder _ rows: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(title).textCase(.uppercase).font(Theme.sans(11, .semibold)).tracking(0.6)
                .foregroundStyle(Theme.textTertiary).padding(.horizontal, 10).padding(.top, 8).padding(.bottom, 4)
            // As tall as its rows; it scrolls only when they don't fit.
            ViewThatFits(in: .vertical) {
                VStack(alignment: .leading, spacing: 2) { rows() }
                ScrollView { VStack(alignment: .leading, spacing: 2) { rows() } }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func row(_ title: Text, detail: Text?, on: Bool, _ action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Image(systemName: "checkmark").font(.system(size: 11, weight: .bold)).opacity(on ? 1 : 0)
                VStack(alignment: .leading, spacing: 2) {
                    title.font(Theme.sans(14, .semibold))
                    if let detail { detail.font(Theme.sans(11, .medium)).foregroundStyle(Theme.textTertiary) }
                }
                Spacer(minLength: 0)
            }
            .padding(.horizontal, 10).padding(.vertical, 8)
            .background { if on { RoundedRectangle(cornerRadius: 12).fill(Color.white.opacity(0.14)) } }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(on ? .isSelected : [])
    }

    private func loadGroup() async {
        guard let item = model.player?.currentItem,
              let g = try? await item.asset.loadMediaSelectionGroup(for: .legible) else { group = nil; return }
        group = g
        selected = item.currentMediaSelection.selectedMediaOption(in: g)
    }

    private func selectText(_ o: AVMediaSelectionOption?) {
        guard let item = model.player?.currentItem else { return }
        if burned != nil { Task { await model.burnSubtitles(nil) } }
        if model.drawnIndex != nil { model.showDrawn(nil) }
        if let group { item.select(o, in: group) }
        selected = o
        close()
    }

    /// A drawn track replaces any text or burned one.
    private func selectDrawn(_ s: PlaybackSubtitle) {
        model.showDrawn(s)
        if let item = model.player?.currentItem, let group {
            item.select(nil, in: group)
            selected = nil
        }
        if burned != nil { Task { await model.burnSubtitles(nil) } }
        close()
    }
}

/// The next episode over the credits (IPh09): its still, "Next episode ·
/// in 6 s", the title, and play now inside the countdown ring; under it,
/// "Watch the credits", which stays on this one.
struct NextEpisodeCard: View {
    let card: NextCard
    let playNow: () -> Void
    let cancel: () -> Void

    var body: some View {
        VStack(alignment: .trailing, spacing: 10) {
            HStack(spacing: 14) {
                RemoteImage(art: card.next.still ?? card.next.backdrop, width: 400)
                    .frame(width: 112, height: 63)
                    .background(Theme.surface)
                    .clipShape(RoundedRectangle(cornerRadius: 10))
                VStack(alignment: .leading, spacing: 3) {
                    Text(verbatim: card.left.map { tr("player.next.in %lld", $0) } ?? tr("player.next.label"))
                        .font(Theme.sans(11, .semibold)).foregroundStyle(Theme.textTertiary)
                    Text(verbatim: card.next.name).font(Theme.display(17)).lineLimit(1)
                    Text(verbatim: [episodeCode(card.next), card.next.runtimeMinutes.map { duration(minutes: $0) }]
                        .compactMap { $0 }.joined(separator: " · "))
                        .font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
                }
                .frame(maxWidth: 160, alignment: .leading)
                Button(action: playNow) {
                    ZStack {
                        Circle().fill(Theme.cream)
                        if let left = card.left {
                            Circle().trim(from: 0, to: CGFloat(left) / 10)
                                .stroke(Theme.onCream.opacity(0.35), style: StrokeStyle(lineWidth: 3, lineCap: .round))
                                .rotationEffect(.degrees(-90))
                                .padding(3)
                                .animation(.linear(duration: 1), value: left)
                        }
                        Image(systemName: "play.fill").font(.system(size: 17, weight: .bold)).foregroundStyle(Theme.onCream)
                    }
                    .frame(width: 50, height: 50)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(Text("player.next.playNow"))
            }
            .padding(10)
            .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 20))
            Button("player.next.watchCredits", action: cancel)
                .font(Theme.sans(13, .semibold))
                .buttonStyle(.glass)
        }
    }
}
