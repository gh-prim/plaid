import SwiftUI

/// The group voted (VoteBig, Apple TV): the top three come in one by one,
/// then "Let's go for" the winner and how many will be there (R33). Still,
/// all at once, with reduced motion. Back closes it.
struct VoteReveal: View {
    let partyId: String
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var vote: PartyVote?
    @State private var party: Party?
    @State private var step = 0

    private var top: [PartyVote.Score] {
        Array((vote?.result?.scores ?? []).filter { !$0.vetoed }.sorted { $0.points > $1.points }.prefix(3))
    }

    var body: some View {
        ZStack {
            Theme.bgBase.ignoresSafeArea()
            if let vote, let winner = vote.result?.winner, let item = vote.item(winner) {
                VStack(spacing: 48) {
                    Text("vote.reveal.title").font(Theme.sans(30, .semibold)).foregroundStyle(Theme.textSecondary)
                    HStack(alignment: .bottom, spacing: 40) {
                        // Second, first, third: a podium.
                        ForEach(podium, id: \.itemId) { s in
                            let rank = top.firstIndex(of: s) ?? 0
                            VStack(spacing: 12) {
                                Text(verbatim: vote.item(s.itemId)?.label ?? "").font(Theme.sans(rank == 0 ? 40 : 30, .bold)).lineLimit(2)
                                    .multilineTextAlignment(.center)
                                Text(verbatim: tr("vote.points %lld", s.points)).font(Theme.sans(24, .semibold)).foregroundStyle(Theme.textSecondary)
                                RoundedRectangle(cornerRadius: 16)
                                    .fill(rank == 0 ? Theme.action : Color.white.opacity(0.18))
                                    .frame(width: 320, height: CGFloat(240 - rank * 70))
                            }
                            .opacity(step > (2 - rank) ? 1 : 0)
                            .offset(y: step > (2 - rank) ? 0 : 40)
                        }
                    }
                    VStack(spacing: 10) {
                        Text("vote.reveal.goFor").font(Theme.sans(30)).foregroundStyle(Theme.textSecondary)
                        Text(verbatim: item.label).font(Theme.sans(80, .bold))
                        if let party {
                            let coming = party.invites.filter { $0.answer == "yes" }.count + 1
                            Text(verbatim: [tr("vote.reveal.people %lld", coming), party.scheduledAt.map { partyTVDate($0) }]
                                .compactMap { $0 }.joined(separator: " · "))
                                .font(Theme.sans(26)).foregroundStyle(Theme.textSecondary)
                        }
                    }
                    .opacity(step > 3 ? 1 : 0)
                    .scaleEffect(step > 3 ? 1 : 0.9)
                    Button("common.close") { dismiss() }.buttonStyle(PillButtonStyle(primary: false)).opacity(step > 3 ? 1 : 0)
                }
                .padding(80)
            } else {
                LoadingView()
            }
        }
        .task {
            guard let client = app.client else { return }
            async let p = try? client.party(partyId)
            vote = try? await client.partyVote(partyId)
            party = await p
            if vote?.result?.winner == nil { dismiss(); return }
            if reduceMotion { step = 4; return }
            for s in 1...4 {
                try? await Task.sleep(for: .milliseconds(s == 4 ? 1400 : 900))
                withAnimation(.spring(response: 0.6, dampingFraction: 0.75)) { step = s }
            }
        }
    }

    private var podium: [PartyVote.Score] {
        switch top.count {
        case 3: [top[1], top[0], top[2]]
        case 2: [top[1], top[0]]
        default: top
        }
    }
}

func partyTVDate(_ d: Date) -> String {
    d.formatted(.dateTime.weekday(.wide).day().month(.abbreviated).hour().minute().locale(AppLanguage.locale))
}
