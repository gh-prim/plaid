import SwiftUI
import UIKit

/// Join with a code (Party07): for last-minute joiners outside the
/// invitations (R3). The code your friend sent, what it leads to, where you
/// watch, then in.
struct JoinCodeSheet: View {
    let request: PartyNav.CodeRequest
    @Environment(AppState.self) private var app
    @Environment(PartyCenter.self) private var center
    @Environment(PartyNav.self) private var nav
    @Environment(\.dismiss) private var dismiss
    @State private var code = ""
    @State private var preview: PartyPreview?
    @State private var checking = false
    @State private var error: String?
    @State private var screens: [PartyScreenOption] = []
    @State private var tv: PartyScreenOption?
    @FocusState private var focused: Bool

    private var raw: String { code.uppercased().filter { $0.isLetter || $0.isNumber } }

    var body: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: 18) {
                Text("party.join.title").displayTitle(30)
                Text("party.join.hint").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                TextField(text: $code) { Text(verbatim: "K7P-4QX") }
                    .font(.system(size: 34, weight: .bold, design: .monospaced))
                    .multilineTextAlignment(.center)
                    .textInputAutocapitalization(.characters)
                    .autocorrectionDisabled()
                    .focused($focused)
                    .padding(.vertical, 14)
                    .background(RoundedRectangle(cornerRadius: 18).fill(Color.white.opacity(0.06)))
                    .onChange(of: code) { _, v in
                        let r = v.uppercased().filter { $0.isLetter || $0.isNumber }.prefix(6)
                        let f = r.count > 3 ? "\(r.prefix(3))-\(r.dropFirst(3))" : String(r)
                        if f != v { code = f }
                        preview = nil
                        error = nil
                        if r.count == 6 { Task { await check() } }
                    }
                if checking {
                    HStack { Spacer(); ProgressView().tint(Theme.textSecondary); Spacer() }
                } else if let error {
                    Text(verbatim: error).font(Theme.sans(13, .medium)).foregroundStyle(Theme.error)
                } else if let preview {
                    previewCard(preview)
                }
                Spacer()
                Button("party.join.cta", action: join)
                    .buttonStyle(CreamButtonStyle())
                    .disabled(preview == nil)
            }
            .padding(.horizontal, Theme.gutter)
            .padding(.vertical, 20)
            .background(Theme.bgBase)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button { dismiss() } label: { Image(systemName: "xmark") }.accessibilityLabel(Text("common.close"))
                }
            }
        }
        .presentationDetents([.large])
        .presentationBackground(Theme.bgBase)
        .onAppear {
            code = request.code
            focused = request.code.isEmpty
        }
        .task { screens = await center.screens() }
    }

    private func previewCard(_ p: PartyPreview) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("party.guest.invite \(p.hostName)").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
            Text(verbatim: p.item?.label ?? tr("vote.tbd")).font(Theme.display(22)).lineLimit(2)
            if let started = p.startedAt {
                Text("party.guest.started \(partyRelative(started)) \(p.watching)").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            } else if let at = p.scheduledAt {
                Text(verbatim: partyDate(at)).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
            }
            if p.full {
                Text("party.full.queue").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textSecondary)
            }
            if !screens.isEmpty {
                PartySectionTitle("party.start.whereTitle").padding(.top, 4)
                ForEach(screens) { s in
                    PartyChoiceRow(icon: "appletv", title: Text(verbatim: "Apple TV · \(s.name)"), hint: Text("party.start.tvHint"),
                                   on: tv?.id == s.id) { tv = s }
                }
                PartyChoiceRow(icon: UIDevice.current.userInterfaceIdiom == .pad ? "ipad" : "iphone",
                               title: Text(UIDevice.current.userInterfaceIdiom == .pad ? "party.start.thisIPad" : "party.start.thisIPhone"),
                               hint: Text("party.start.thisDevice"), on: tv == nil) { tv = nil }
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 20))
    }

    private func check() async {
        guard let client = app.client else { return }
        checking = true
        defer { checking = false }
        do {
            preview = try await client.partyPreview(code: raw)
        } catch let e as APIError where e.status == 404 {
            error = tr("party.error.party_not_found")
        } catch let e as APIError {
            error = partyError(e.code)
        } catch {
            self.error = tr("party.error.generic")
        }
    }

    private func join() {
        let entry = PartyEntry(partyId: nil, code: raw, role: tv == nil ? .screen : .companion, tvSession: tv?.sessionId, screenName: tv?.name)
        dismiss()
        nav.live = entry
    }
}
