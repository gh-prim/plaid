import SwiftUI

/// Find the server (board IPh11 / IPa11): the servers seen on the network,
/// one picked, and a way in by getplaid.direct code for a server elsewhere.
struct ServerPickerView: View {
    @Environment(AppState.self) private var app
    @State private var discovery = ServerDiscovery()
    @State private var picked: DiscoveredServer?
    @State private var byCode = false
    @State private var connecting = false
    @State private var error: LocalizedStringKey?

    var body: some View {
        SignInScaffold {
            if byCode {
                ServerCodeCard(back: { withAnimation(.smooth) { byCode = false } })
                    .transition(.blurReplace)
            } else {
                card.transition(.blurReplace)
            }
        }
        .onAppear { discovery.start() }
        .onDisappear { discovery.stop() }
        .onChange(of: discovery.servers) { _, servers in
            if picked == nil || !servers.contains(where: { $0.id == picked?.id }) { picked = servers.first }
        }
    }

    private var card: some View {
        GlassCard {
            LogoMark(size: 54)
            Group {
                if let picked {
                    Text("server.tonight \(picked.name)")
                } else {
                    Text("server.find.title")
                }
            }
            .font(Theme.display(22))
            .tracking(-0.4)
            .multilineTextAlignment(.center)
            .padding(.top, 14)

            VStack(spacing: 6) {
                if discovery.servers.isEmpty {
                    HStack(spacing: 10) {
                        ProgressView().tint(Theme.textSecondary)
                        Text("server.searching").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                    }
                    .frame(maxWidth: .infinity, minHeight: 56)
                }
                ForEach(discovery.servers) { s in
                    Button { picked = s } label: {
                        VStack(alignment: .leading, spacing: 5) {
                            Text(verbatim: s.name).font(Theme.sans(14, .semibold))
                            Text(verbatim: [s.url.host(), s.version.map { "PLAID \($0)" }, tr("server.onNetwork")]
                                .compactMap { $0 }.joined(separator: " · "))
                                .font(Theme.sans(11, .medium))
                                .foregroundStyle(Theme.textTertiary)
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, 14).padding(.vertical, 12)
                        .background {
                            if picked?.id == s.id {
                                RoundedRectangle(cornerRadius: 14).fill(Color.white.opacity(0.08))
                                    .strokeBorder(Color.white.opacity(0.14))
                            }
                        }
                        .contentShape(RoundedRectangle(cornerRadius: 14))
                    }
                    .buttonStyle(.plain)
                    .accessibilityAddTraits(picked?.id == s.id ? .isSelected : [])
                }
            }
            .padding(.top, 18)

            if let error {
                Text(error).font(Theme.sans(13, .medium)).foregroundStyle(Theme.error)
                    .multilineTextAlignment(.center).padding(.top, 10)
            }

            Button {
                guard let picked else { return }
                connecting = true
                Task {
                    defer { connecting = false }
                    if await serverAnswers(picked.url) { await app.useServer(picked.url) } else { error = "server.unreachable" }
                }
            } label: {
                if connecting { ProgressView().tint(Theme.onCream) } else if let picked {
                    Text("server.continueWith \(picked.name)")
                } else {
                    Text("server.continue")
                }
            }
            .buttonStyle(CreamButtonStyle(height: 46))
            .disabled(picked == nil || connecting)
            .padding(.top, 16)

            VStack(spacing: 3) {
                Text("server.away").foregroundStyle(Theme.textTertiary)
                Button("server.enterCode") { withAnimation(.smooth) { byCode = true } }
                    .foregroundStyle(Theme.textPrimary)
            }
            .font(Theme.sans(12, .semibold))
            .padding(.top, 16)
        }
    }
}

/// The server's six-character getplaid.direct code (IPh15, IPh16): one box per
/// character, the server it names once found, or "no server with this code".
struct ServerCodeCard: View {
    let back: () -> Void
    @Environment(AppState.self) private var app
    @State private var code = ""
    @State private var found: (url: URL, info: ServerInfo)?
    @State private var looking = false
    @State private var error: LocalizedStringKey?
    @FocusState private var typing: Bool
    /// A server with neither Bonjour nor getplaid.direct: its address, typed.
    @State private var byAddress = false
    @State private var address = ""

    private static let alphabet = Set("abcdefghijklmnopqrstuvwxyz234567")

    var body: some View {
        GlassCard {
            HStack {
                Button(action: back) {
                    Image(systemName: "chevron.left").font(.system(size: 13, weight: .bold)).frame(width: 30, height: 30)
                }
                .buttonStyle(.glass).buttonBorderShape(.circle)
                .accessibilityLabel(Text("common.back"))
                Spacer()
            }
            .padding(.bottom, -18)
            LogoMark(size: 54)
            Text("server.code.title").font(Theme.display(22)).tracking(-0.4).padding(.top, 14)
            Text("server.code.hint").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary).padding(.top, 6)

            ZStack {
                // The real field, out of sight: the boxes draw what it holds.
                TextField("", text: $code)
                    .focused($typing)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .keyboardType(.asciiCapable)
                    .textContentType(.oneTimeCode)
                    .opacity(0.02)
                    .accessibilityLabel(Text("server.code.title"))
                HStack(spacing: 7) {
                    ForEach(0..<6, id: \.self) { i in
                        let chars = Array(code)
                        Text(verbatim: i < chars.count ? String(chars[i]) : "")
                            .font(Theme.display(20, .semibold))
                            .frame(width: 40, height: 50)
                            .background(RoundedRectangle(cornerRadius: 11).fill(Color.white.opacity(0.08)))
                            .overlay {
                                RoundedRectangle(cornerRadius: 11).strokeBorder(
                                    error != nil ? Theme.error.opacity(0.8)
                                        : (typing && i == min(chars.count, 5) ? Theme.textPrimary.opacity(0.8) : Color.white.opacity(0.14)),
                                    lineWidth: error != nil || (typing && i == min(chars.count, 5)) ? 1.5 : 1)
                            }
                    }
                }
                .allowsHitTesting(false)
                .accessibilityHidden(true)
            }
            .contentShape(Rectangle())
            .onTapGesture { typing = true }
            .padding(.top, 20)

            if let error {
                Text(error).font(Theme.sans(12, .semibold)).foregroundStyle(Theme.error)
                    .multilineTextAlignment(.center).padding(.top, 12)
            } else if let found {
                VStack(alignment: .leading, spacing: 4) {
                    Text(verbatim: found.info.name).font(Theme.sans(13, .semibold))
                    Text(verbatim: "PLAID \(found.info.version)").font(Theme.sans(11, .medium)).foregroundStyle(Theme.textTertiary)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 14).padding(.vertical, 12)
                .background(RoundedRectangle(cornerRadius: 12).fill(Color.white.opacity(0.08)).strokeBorder(Color.white.opacity(0.14)))
                .padding(.top, 14)
            } else if looking {
                ProgressView().tint(Theme.textSecondary).padding(.top, 14)
            }

            Button {
                if let found { Task { await app.useServer(found.url) } }
            } label: {
                if let found { Text("server.continueWith \(found.info.name)") } else { Text("server.continue") }
            }
            .buttonStyle(CreamButtonStyle(height: 46))
            .disabled(found == nil)
            .padding(.top, 14)

            if byAddress {
                GlassField(systemImage: "network") {
                    TextField(text: $address, prompt: Text(verbatim: "192.168.1.10:32500").foregroundStyle(Theme.textTertiary)) {
                        Text("server.address")
                    }
                    .keyboardType(.URL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .submitLabel(.go)
                    .onSubmit { Task { await lookAddress() } }
                }
                .padding(.top, 14)
            } else {
                Button("server.typeAddress") { withAnimation(.smooth) { byAddress = true } }
                    .font(Theme.sans(12, .semibold))
                    .foregroundStyle(Theme.textSecondary)
                    .padding(.top, 14)
            }
        }
        .onAppear { typing = true }
        .onChange(of: code) { _, typed in
            // Lower case, the code's alphabet only (no 0, 1, 8, 9), six at most.
            let clean = String(typed.lowercased().filter { Self.alphabet.contains($0) }.prefix(6))
            if clean != typed { code = clean; return }
            found = nil
            error = nil
        }
        .task(id: code) {
            guard code.count == 6 else { return }
            await look(code)
        }
    }

    private func lookAddress() async {
        guard let url = serverURL(typed: address) else { error = "server.invalidAddress"; return }
        looking = true
        defer { looking = false }
        error = nil
        if let info = await serverInfo(url) { found = (url, info) } else { error = "server.unreachable" }
    }

    /// Asks getplaid.direct where the server is and keeps the first address
    /// that answers, the local one first.
    private func look(_ code: String) async {
        looking = true
        defer { looking = false }
        let urls: [URL]
        do {
            urls = try await DirectLink.addresses(for: code)
        } catch DirectLink.Failure.unknownCode {
            error = "server.code.unknown"
            return
        } catch {
            self.error = "server.code.directUnreachable"
            return
        }
        for (i, url) in urls.enumerated() {
            if let info = await serverInfo(url, within: i < urls.count - 1 ? 4 : 15), !Task.isCancelled {
                found = (url, info)
                return
            }
        }
        if !Task.isCancelled { error = "server.unreachable" }
    }
}

/// The sign-in screens' ground: the board's dark warm blur, the lettering
/// on top, the content centered and kept narrow on iPad.
struct SignInScaffold<Content: View>: View {
    var showWordmark = true
    @ViewBuilder let content: Content

    var body: some View {
        ZStack {
            WarmGround()
            ScrollView {
                VStack(spacing: 0) {
                    if showWordmark { Wordmark(height: 18).padding(.top, 14) }
                    Spacer(minLength: 40)
                    content.frame(maxWidth: 400)
                    Spacer(minLength: 40)
                }
                .padding(.horizontal, 16)
                .frame(maxWidth: .infinity)
                .containerRelativeFrame(.vertical, alignment: .top) { h, _ in h }
            }
            .scrollBounceBehavior(.basedOnSize)
        }
    }
}

/// The sign-in screens' background: soft warm and cool glows on black.
struct WarmGround: View {
    var body: some View {
        // The glows sit in an overlay: their size never widens the page.
        Theme.bgBase
            .overlay {
                ZStack {
                    Circle().fill(Color(hex: "#6b4a1e")).frame(width: 520).blur(radius: 140).offset(x: -160, y: -260).opacity(0.55)
                    Circle().fill(Color(hex: "#1f2742")).frame(width: 480).blur(radius: 140).offset(x: 180, y: 260).opacity(0.7)
                }
            }
            .clipped()
            .ignoresSafeArea()
        .accessibilityHidden(true)
    }
}

/// A rounded glass panel with its content centered.
struct GlassCard<Content: View>: View {
    @ViewBuilder let content: Content

    var body: some View {
        VStack(spacing: 0) { content }
            .padding(18)
            .padding(.vertical, 6)
            .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 30))
    }
}

/// The server's identity at `url`, or nil when nothing answers within `seconds`.
func serverInfo(_ url: URL, within seconds: Double = 15) async -> ServerInfo? {
    await withTaskGroup(of: ServerInfo?.self) { group in
        group.addTask { try? await APIClient(baseURL: url).get("/server") }
        group.addTask {
            try? await Task.sleep(for: .seconds(seconds))
            return nil
        }
        let first = await group.next() ?? nil
        group.cancelAll()
        return first
    }
}

func serverAnswers(_ url: URL) async -> Bool { await serverInfo(url) != nil }
