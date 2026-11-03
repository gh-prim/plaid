import SwiftUI

/// The group votes for the movie (VotePh, R31–R34): everyone proposes up to
/// two titles (search, or "Nobody has seen it" suggestions), then ranks a
/// top 3 (3, 2, 1 points) and may veto one. Ballots are anonymous: the
/// results show the points, the vetoed titles and who voted, never for what.
struct VoteView: View {
    let ref: PartyNav.PartyRef
    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(\.dismiss) private var dismiss
    @State private var vote: PartyVote?
    @State private var party: Party?
    @State private var failed = false
    @State private var error: String?
    // Proposals
    @State private var query = ""
    @State private var results: [ItemSummary] = []
    @State private var suggestions: [PartyItem] = []
    // Ranking
    @State private var ranking: [String] = []
    @State private var veto: String?
    @State private var editing = false

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    if let party {
                        Text(verbatim: header(party)).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                    }
                    if let vote {
                        switch vote.phase {
                        case "proposals": proposals(vote)
                        case "ranking": if editing || vote.myRanking.isEmpty { rank(vote) } else { results(vote) }
                        default: results(vote)
                        }
                    } else if failed {
                        ErrorState { failed = false; Task { await load() } }.frame(height: 260)
                    } else {
                        ForEach(0..<5, id: \.self) { _ in Shimmer().frame(height: 60).clipShape(RoundedRectangle(cornerRadius: 14)) }
                    }
                    if let error { Text(verbatim: error).font(Theme.sans(13, .medium)).foregroundStyle(Theme.error) }
                }
                .padding(.horizontal, Theme.gutter)
                .padding(.bottom, 40)
            }
            .background(Theme.bgBase)
            .navigationTitle(Text("vote.title"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button { dismiss() } label: { Image(systemName: "xmark") }.accessibilityLabel(Text("common.close"))
                }
            }
        }
        .presentationBackground(Theme.bgBase)
        .task { await load() }
        .onChange(of: center.changes) { Task { await load() } }
    }

    private func header(_ p: Party) -> String {
        let when = p.scheduledAt.map { $0.formatted(.dateTime.weekday(.wide).hour().minute().locale(AppLanguage.locale)) } ?? ""
        return tr("vote.partyOf %@ %@", p.organizer.name, when)
    }

    // MARK: Proposals

    @ViewBuilder
    private func proposals(_ v: PartyVote) -> some View {
        let mine = v.proposals.filter(\.mine)
        VStack(alignment: .leading, spacing: 6) {
            Text("vote.propose.title").displayTitle(28)
            Text("vote.until \(v.proposalsUntil.formatted(.dateTime.weekday(.wide).hour().minute().locale(AppLanguage.locale)))")
                .font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
        }
        TextField("vote.propose.search", text: $query)
            .font(Theme.sans(15, .medium))
            .padding(.horizontal, 16).frame(height: 44)
            .glassEffect(.regular, in: Capsule())
            .task(id: query) { await search() }
        ForEach(results.prefix(6)) { item in
            proposalRow(title: item.name, detail: item.year.map(String.init) ?? "", id: item.id, v: v, full: mine.count >= 2)
        }
        if !mine.isEmpty {
            PartySectionTitle("vote.propose.mine")
            ForEach(mine, id: \.item.id) { p in
                proposalRow(title: p.item.label, detail: tr("vote.propose.yours"), id: p.item.id, v: v, full: false)
            }
            if mine.count < 2 { Text("vote.propose.oneMore").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary) }
        }
        if !suggestions.isEmpty {
            PartySectionTitle("vote.suggestions")
            ForEach(suggestions, id: \.id) { s in
                proposalRow(title: s.label, detail: tr("vote.nobodySaw"), id: s.id, v: v, full: mine.count >= 2)
            }
        }
        PartySectionTitle("vote.proposed \(v.proposals.count)")
        ForEach(v.proposals, id: \.item.id) { p in
            HStack {
                Text(verbatim: p.item.label).font(Theme.sans(15, .semibold))
                Spacer()
                Text("vote.by \(p.mine ? tr("party.people.you") : p.proposedBy)").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            }
        }
        voters(v)
    }

    private func proposalRow(title: String, detail: String, id: String, v: PartyVote, full: Bool) -> some View {
        let proposed = v.proposals.contains { $0.item.id == id }
        let mine = v.proposals.contains { $0.item.id == id && $0.mine }
        return HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(verbatim: title).font(Theme.sans(15, .semibold)).lineLimit(1)
                Text(verbatim: detail).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            }
            Spacer()
            if mine {
                Button("vote.propose.withdraw") { Task { await propose(id, false) } }.font(Theme.sans(13, .semibold)).buttonStyle(.glass)
            } else if !proposed {
                Button("vote.propose.cta") { Task { await propose(id, true) } }.font(Theme.sans(13, .semibold)).buttonStyle(.glass)
                    .disabled(full)
            } else {
                Image(systemName: "checkmark").foregroundStyle(Theme.textTertiary)
            }
        }
        .padding(12)
        .background(RoundedRectangle(cornerRadius: 14).fill(Color.white.opacity(0.05)))
    }

    // MARK: Ranking

    @ViewBuilder
    private func rank(_ v: PartyVote) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("vote.rank.title").displayTitle(28)
            Text("vote.rank.hint").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
            Text("vote.until \(v.rankingUntil.formatted(.dateTime.weekday(.wide).hour().minute().locale(AppLanguage.locale)))")
                .font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
        }
        ForEach(v.proposals, id: \.item.id) { p in
            let id = p.item.id
            let place = ranking.firstIndex(of: id)
            HStack(spacing: 8) {
            Button {
                if let place { ranking.remove(at: place) } else if ranking.count < 3, veto != id { ranking.append(id) }
            } label: {
                HStack(spacing: 12) {
                    ZStack {
                        Circle().fill(place != nil ? Theme.cream : Color.white.opacity(0.08))
                        if let place {
                            Text(verbatim: "\(place + 1)").font(Theme.display(16)).foregroundStyle(Theme.onCream)
                        } else if veto == id {
                            Image(systemName: "nosign").foregroundStyle(Theme.error)
                        }
                    }
                    .frame(width: 34, height: 34)
                    VStack(alignment: .leading, spacing: 2) {
                        Text(verbatim: p.item.label).font(Theme.sans(15, .semibold)).strikethrough(veto == id)
                        Text("vote.proposedBy \(p.mine ? tr("party.people.you") : p.proposedBy)").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                    }
                    Spacer()
                    if let place { Text("vote.points \(3 - place)").font(Theme.sans(12, .semibold)).foregroundStyle(Theme.textSecondary) }
                    if veto == id { Text("vote.veto").font(Theme.sans(12, .semibold)).foregroundStyle(Theme.error) }
                }
                .padding(12)
                .background(RoundedRectangle(cornerRadius: 14).fill(Color.white.opacity(place != nil ? 0.1 : 0.05)))
            }
            .buttonStyle(.plain)
            Button {
                veto = veto == id ? nil : id
                ranking.removeAll { $0 == id }
            } label: {
                Image(systemName: "nosign").font(.system(size: 14, weight: .semibold)).frame(width: 36, height: 36)
                    .foregroundStyle(veto == id ? Theme.error : Theme.textTertiary)
            }
            .buttonStyle(.glass).buttonBorderShape(.circle)
            .accessibilityLabel(Text(veto == id ? "vote.veto.remove" : "vote.veto.give"))
            }
        }
        Text("vote.veto.hint").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
        HStack(spacing: 10) {
            Button { ranking = []; veto = nil } label: {
                Text("vote.reset").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
            }
            .buttonStyle(.glass)
            Button("vote.rank.cta") { Task { await submit() } }
                .buttonStyle(CreamButtonStyle())
                .disabled(ranking.isEmpty)
        }
        Text("vote.anonymous").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
    }

    // MARK: Results

    @ViewBuilder
    private func results(_ v: PartyVote) -> some View {
        let scores = (v.result?.scores ?? []).sorted { $0.points > $1.points }
        VStack(alignment: .leading, spacing: 6) {
            if v.phase == "closed", let w = v.result?.winner, let item = v.item(w) {
                Text("vote.closed").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                Text(verbatim: item.label).displayTitle(32)
                if let pts = scores.first(where: { $0.itemId == w })?.points {
                    Text("vote.winner.points \(pts)").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
                }
            } else {
                Text("vote.results").displayTitle(28)
                let voted = v.voters.filter(\.voted).count
                Text("vote.votedCount \(voted) \(v.voters.count)").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
            }
        }
        if let tied = v.result?.tied, tied.count > 1, let pts = scores.first?.points {
            VStack(alignment: .leading, spacing: 4) {
                Text("vote.tie \(pts)").font(Theme.sans(14, .semibold))
                Text(verbatim: tied.compactMap { v.item($0)?.label }.joined(separator: " · ")).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
            }
            .padding(14)
            .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 18))
        }
        ForEach(scores.filter { !$0.vetoed }, id: \.itemId) { s in
            HStack {
                Text(verbatim: v.item(s.itemId)?.label ?? "").font(Theme.sans(15, .semibold))
                Spacer()
                Text("vote.points \(s.points)").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
            }
            .padding(12)
            .background(RoundedRectangle(cornerRadius: 14).fill(Color.white.opacity(s.itemId == v.result?.winner ? 0.14 : 0.05)))
        }
        let vetoed = scores.filter(\.vetoed)
        if !vetoed.isEmpty {
            PartySectionTitle("vote.vetoed")
            ForEach(vetoed, id: \.itemId) { s in
                Text(verbatim: v.item(s.itemId)?.label ?? "").font(Theme.sans(14, .medium)).strikethrough().foregroundStyle(Theme.textTertiary)
            }
        }
        if scores.isEmpty && v.phase != "closed" {
            Text("vote.resultsLater").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textTertiary)
        }
        voters(v)
        Text("vote.anonymous").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
        if v.phase == "ranking" {
            Button { ranking = v.myRanking; veto = v.myVeto; editing = true } label: {
                Text("vote.rank.edit").font(Theme.sans(15, .semibold)).frame(maxWidth: .infinity).frame(height: 38)
            }
            .buttonStyle(.glass)
        }
    }

    private func voters(_ v: PartyVote) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            PartySectionTitle("vote.voters")
            ForEach(v.voters, id: \.self) { voter in
                HStack {
                    Text(verbatim: voter.name).font(Theme.sans(14, .semibold))
                    Spacer()
                    Text(voter.voted ? "vote.voted" : "vote.notYet").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                }
            }
        }
        .padding(.top, 6)
    }

    // MARK: Loading

    private func load() async {
        guard let client = app.client else { return }
        do {
            async let p = try? client.party(ref.id)
            let v = try await client.partyVote(ref.id)
            party = await p
            vote = v
            if ranking.isEmpty { ranking = v.myRanking; veto = v.myVeto }
            if v.phase == "proposals", suggestions.isEmpty { suggestions = (try? await client.voteSuggestions(ref.id)) ?? [] }
        } catch {
            if vote == nil { failed = true }
        }
    }

    private func search() async {
        let q = query.trimmingCharacters(in: .whitespaces)
        guard q.count >= 2, let client = app.client else { results = []; return }
        try? await Task.sleep(for: .milliseconds(300))
        guard !Task.isCancelled else { return }
        let r: SearchResults? = try? await client.get("/search", query: [.init(name: "q", value: q), .init(name: "limit", value: "10")])
        results = r?.movies ?? []
    }

    private func propose(_ id: String, _ on: Bool) async {
        guard let client = app.client else { return }
        do {
            vote = try await client.propose(id, in: ref.id, on)
            error = nil
        } catch let e as APIError {
            error = partyError(e.code)
        } catch {}
    }

    private func submit() async {
        guard let client = app.client else { return }
        do {
            vote = try await client.castBallot(Ballot(ranking: ranking, veto: veto), in: ref.id)
            editing = false
            error = nil
        } catch let e as APIError {
            error = partyError(e.code)
        } catch {}
    }
}
