import SwiftUI

/// The next episode on the card, and the seconds left when counting down.
struct NextCard: Equatable {
    let next: ItemSummary
    var left: Int?
}

/// The web's TV next-episode card (`pl-next--tv`): the still with the
/// countdown ring, "Next episode", the episode, then Play now and Cancel.
/// Play now has focus as soon as the card shows.
struct NextEpisodeCard: View {
    let card: NextCard
    let playNow: () -> Void
    let cancel: () -> Void

    private enum Control { case play, cancel }
    @FocusState private var focus: Control?

    private static let seconds = 10.0

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            HStack(spacing: 20) {
                ZStack {
                    Theme.surface
                    RemoteImage(art: card.next.still ?? card.next.backdrop, width: 500)
                    if let left = card.left {
                        Color.black.opacity(0.45)
                        Circle()
                            .stroke(Color.white.opacity(0.25), lineWidth: 5)
                            .frame(width: 96, height: 96)
                        Circle()
                            .trim(from: 0, to: Double(left) / Self.seconds)
                            .stroke(Color.white, style: StrokeStyle(lineWidth: 5, lineCap: .round))
                            .rotationEffect(.degrees(-90))
                            .frame(width: 96, height: 96)
                            .animation(.linear(duration: 1), value: left)
                        Text(verbatim: "\(left)")
                            .font(Theme.sans(44, .semibold))
                            .monospacedDigit()
                    }
                }
                .frame(width: 288, height: 162)
                .clipShape(RoundedRectangle(cornerRadius: 10))

                VStack(alignment: .leading, spacing: 4) {
                    Group {
                        if let left = card.left {
                            Text("player.next.countdown \(left)")
                        } else {
                            Text("player.next.label")
                        }
                    }
                    .font(Theme.sans(22, .semibold))
                    .monospacedDigit()
                    .foregroundStyle(Theme.textSecondary)
                    Text(verbatim: episodeLine)
                        .font(Theme.sans(30, .semibold))
                        .lineLimit(2)
                    if !meta.isEmpty {
                        Text(verbatim: meta)
                            .font(Theme.sans(22, .semibold))
                            .foregroundStyle(Theme.textSecondary)
                            .lineLimit(1)
                    }
                }
            }
            HStack(spacing: 16) {
                Button(action: playNow) {
                    Label("player.next.playNow", systemImage: "play.fill").frame(maxWidth: .infinity)
                }
                .buttonStyle(PillButtonStyle())
                .focused($focus, equals: .play)
                Button(action: cancel) {
                    Text("common.cancel").frame(maxWidth: .infinity)
                }
                .buttonStyle(PillButtonStyle(primary: false))
                .focused($focus, equals: .cancel)
            }
        }
        .padding(20)
        .frame(width: 760)
        .background(RoundedRectangle(cornerRadius: Theme.radius).fill(Theme.bgRaised))
        .overlay(RoundedRectangle(cornerRadius: Theme.radius).strokeBorder(Color.white.opacity(0.08), lineWidth: 1))
        .shadow(color: .black.opacity(0.5), radius: 40, y: 20)
        .focusSection()
        .defaultFocus($focus, .play)
        .onAppear { focus = .play }
    }

    private var episodeLine: String {
        guard let code = episodeCode(card.next) else { return card.next.name }
        return card.next.name.isEmpty ? code : "\(code) · \(card.next.name)"
    }

    private var meta: String {
        [card.next.show?.displayTitle,
         card.next.runtimeMinutes.map { Duration.seconds($0 * 60).formatted(.units(allowed: [.hours, .minutes], width: .narrow).locale(AppLanguage.locale)) }]
            .compactMap { $0 }.joined(separator: " · ")
    }
}

/// The last episode ended: one warm line, nothing plays (F008 R43).
struct EndOfShow: View {
    let show: String
    let back: () -> Void

    var body: some View {
        VStack(spacing: 32) {
            Text("player.endOfShow.title \(show)")
                .font(Theme.sans(48, .bold))
                .multilineTextAlignment(.center)
                .frame(maxWidth: 1200)
            Button(action: back) { Text("player.endOfShow.back \(show)") }
                .buttonStyle(PillButtonStyle(primary: false))
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Theme.bgBase.opacity(0.92))
    }
}
