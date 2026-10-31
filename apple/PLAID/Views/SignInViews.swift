import SwiftUI

/// Servers found over Bonjour, plus an address typed by hand.
struct ServerPickerView: View {
    @Environment(AppState.self) private var app
    @State private var discovery = ServerDiscovery()
    @State private var address = ""
    @State private var connecting = false
    @State private var error: LocalizedStringKey?

    var body: some View {
        VStack(alignment: .leading, spacing: 40) {
            Wordmark(size: 72)
            Text("server.title")
                .font(Theme.sans(48, .semibold))
            HStack(alignment: .top, spacing: 80) {
                VStack(alignment: .leading, spacing: 24) {
                    Text("server.found")
                        .font(Theme.sans(28, .semibold))
                        .foregroundStyle(Theme.textTertiary)
                    if discovery.servers.isEmpty {
                        HStack(spacing: 20) {
                            ProgressView()
                            Text("server.searching").foregroundStyle(Theme.textSecondary)
                        }
                        .font(Theme.sans(28))
                    }
                    ForEach(discovery.servers) { s in
                        Button {
                            connect(s.url)
                        } label: {
                            VStack(alignment: .leading, spacing: 4) {
                                Text(verbatim: s.name).font(Theme.sans(32, .semibold))
                                Text(verbatim: s.url.host() ?? "").font(Theme.sans(22))
                                    .opacity(0.7)
                            }
                        }
                        .buttonStyle(RowButtonStyle())
                    }
                }
                .frame(width: 720, alignment: .leading)
                VStack(alignment: .leading, spacing: 24) {
                    Text("server.manual")
                        .font(Theme.sans(28, .semibold))
                        .foregroundStyle(Theme.textTertiary)
                    TextField(tr("server.addressPlaceholder"), text: $address)
                        .keyboardType(.URL)
                        .textContentType(.URL)
                        .autocorrectionDisabled()
                        .frame(width: 640)
                        .onSubmit { connectTyped() }
                    Button("server.connect") { connectTyped() }
                        .buttonStyle(PillButtonStyle())
                        .disabled(address.isEmpty || connecting)
                    if let error {
                        Text(error).foregroundStyle(Theme.textSecondary).font(Theme.sans(26))
                    }
                }
            }
            Spacer()
        }
        .padding(Theme.gutter)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .onAppear { discovery.start() }
        .onDisappear { discovery.stop() }
    }

    private func connectTyped() {
        var s = address.trimmingCharacters(in: .whitespaces)
        if !s.contains("://") { s = "http://" + s }
        guard var c = URLComponents(string: s), c.host != nil else { error = "server.invalidAddress"; return }
        if c.port == nil && c.scheme == "http" { c.port = 32500 }
        guard let url = c.url else { error = "server.invalidAddress"; return }
        connect(url)
    }

    private func connect(_ url: URL) {
        connecting = true
        error = nil
        Task {
            let probe = APIClient(baseURL: url)
            do {
                let _: ServerInfo = try await probe.get("/server")
                await app.useServer(url)
            } catch {
                self.error = "server.unreachable"
            }
            connecting = false
        }
    }
}

/// Shows the pairing code and waits for an admin to claim it on the web.
struct PairingView: View {
    @Environment(AppState.self) private var app
    @State private var pairing: PairingStarted?
    @State private var failed = false

    var body: some View {
        VStack(spacing: 48) {
            Wordmark(size: 72)
            if let name = app.serverName {
                Text("pair.server \(name)")
                    .font(Theme.sans(28))
                    .foregroundStyle(Theme.textTertiary)
            }
            Text("pair.title").font(Theme.sans(48, .semibold))
            Group {
                if let pairing {
                    Text(verbatim: pairing.code)
                        .font(Theme.sans(120, .bold))
                        .monospaced()
                        .tracking(12)
                        .foregroundStyle(Theme.textPrimary)
                } else if failed {
                    Text("pair.failed").font(Theme.sans(32)).foregroundStyle(Theme.textSecondary)
                } else {
                    ProgressView().controlSize(.large)
                }
            }
            .frame(height: 160)
            if let host = app.client?.baseURL {
                Text("pair.instructions \(host.appending(path: "pair").absoluteString)")
                    .font(Theme.sans(30))
                    .foregroundStyle(Theme.textSecondary)
                    .multilineTextAlignment(.center)
                    .frame(maxWidth: 1200)
            }
            Button("pair.changeServer") { app.forgetServer() }
                .buttonStyle(PillButtonStyle(primary: false))
        }
        .padding(Theme.gutter)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .task { await run() }
    }

    /// Starts a pairing, polls it every 2 s and starts again when it expires.
    private func run() async {
        guard let client = app.client else { return }
        while !Task.isCancelled {
            do {
                let started: PairingStarted = try await client.post("/pairing", StartPairingRequest(device: AppState.device))
                pairing = started
                failed = false
                while !Task.isCancelled && started.expiresAt > .now {
                    try await Task.sleep(for: .seconds(2))
                    let (data, status) = try await client.send(
                        "GET", "/pairing/\(started.id)", body: Optional<Int>.none,
                        headers: ["X-Pairing-Secret": started.pollSecret]
                    )
                    if status == 200 {
                        app.signedIn(try client.decode(data))
                        return
                    }
                }
            } catch is CancellationError {
                return
            } catch {
                pairing = nil
                failed = true
                try? await Task.sleep(for: .seconds(5))
            }
        }
    }
}

/// Who's watching: the profiles of the account, a PIN when one is set.
struct ProfilePickerView: View {
    @Environment(AppState.self) private var app
    @State private var pinFor: Profile?
    @State private var pin = ""
    @State private var error: LocalizedStringKey?

    var body: some View {
        VStack(spacing: 64) {
            Text("profiles.title").font(Theme.sans(56, .semibold))
            HStack(spacing: 56) {
                ForEach(app.me?.profiles ?? []) { p in
                    Button {
                        if p.hasPin { pinFor = p; pin = "" } else { select(p, pin: nil) }
                    } label: {
                        VStack(spacing: 20) {
                            Circle()
                                .fill(Theme.party[p.avatarColor % Theme.party.count])
                                .frame(width: 200, height: 200)
                                .overlay {
                                    Text(verbatim: String(p.name.prefix(1)).uppercased())
                                        .font(Theme.sans(88, .semibold))
                                        .foregroundStyle(Theme.bgBase)
                                }
                            Text(verbatim: p.name).font(Theme.sans(30, .semibold))
                        }
                    }
                    .buttonStyle(.borderless)
                }
            }
            if let error {
                Text(error).foregroundStyle(Theme.textSecondary).font(Theme.sans(28))
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .alert(Text("profiles.pin"), isPresented: Binding(get: { pinFor != nil }, set: { if !$0 { pinFor = nil } })) {
            SecureField(tr("profiles.pin"), text: $pin)
                .keyboardType(.numberPad)
            Button("common.ok") { if let p = pinFor { select(p, pin: pin) } }
            Button("common.cancel", role: .cancel) {}
        }
    }

    private func select(_ p: Profile, pin: String?) {
        error = nil
        Task {
            do {
                try await app.selectProfile(p, pin: pin)
            } catch let e as APIError {
                error = e.code.contains("pin") ? "profiles.wrongPin" : "error.generic"
            } catch {
                self.error = "error.generic"
            }
        }
    }
}
