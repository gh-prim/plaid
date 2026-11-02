import CoreImage.CIFilterBuiltins
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
        if let code = DirectLink.code(in: address) {
            connect(code: code)
        } else if let url = typedURL() {
            connect(url)
        } else {
            error = "server.invalidAddress"
        }
    }

    /// The typed address as a URL: plain http on PLAID's port unless it says otherwise.
    private func typedURL() -> URL? { serverURL(typed: address) }

    private func connect(_ url: URL) {
        connecting = true
        error = nil
        Task {
            if await Self.answers(url) {
                await app.useServer(url)
            } else {
                self.error = "server.unreachable"
            }
            connecting = false
        }
    }

    /// A getplaid.direct code: asks the service where the server is and keeps
    /// the first address that answers, the local one first (a TV at home).
    private func connect(code: String) {
        connecting = true
        error = nil
        Task {
            defer { connecting = false }
            let urls: [URL]
            do {
                urls = try await DirectLink.addresses(for: code)
            } catch DirectLink.Failure.unknownCode {
                // A six-letter host name on the network looks like a code too.
                if let url = typedURL(), await Self.answers(url) { await app.useServer(url) } else { error = "server.unknownCode" }
                return
            } catch {
                self.error = "server.directUnreachable"
                return
            }
            for (i, url) in urls.enumerated() where await Self.answers(url, within: i < urls.count - 1 ? 4 : 15) {
                await app.useServer(url)
                return
            }
            error = "server.unreachable"
        }
    }

    /// Whether a PLAID server answers at `url` within `seconds`.
    private static func answers(_ url: URL, within seconds: Double = 15) async -> Bool {
        await withTaskGroup(of: Bool.self) { group in
            group.addTask {
                let info: ServerInfo? = try? await APIClient(baseURL: url).get("/server")
                return info != nil
            }
            group.addTask {
                try? await Task.sleep(for: .seconds(seconds))
                return false
            }
            let first = await group.next() ?? false
            group.cancelAll()
            return first
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
                    HStack(spacing: 72) {
                        Text(verbatim: pairing.code)
                            .font(Theme.sans(120, .bold))
                            .monospaced()
                            .tracking(12)
                            .foregroundStyle(Theme.textPrimary)
                        // The same code for a phone: the PLAID app scans it, or
                        // the camera opens the web's /pair with it typed in.
                        if let url = pairURL(pairing.code) {
                            VStack(spacing: 12) {
                                QRCodeImage(text: url.absoluteString)
                                    .frame(width: 200, height: 200)
                                    .padding(14)
                                    .background(RoundedRectangle(cornerRadius: 18).fill(Color.white))
                                Text("pair.scan").font(Theme.sans(22)).foregroundStyle(Theme.textTertiary)
                            }
                        }
                    }
                } else if failed {
                    Text("pair.failed").font(Theme.sans(32)).foregroundStyle(Theme.textSecondary)
                } else {
                    ProgressView().controlSize(.large)
                }
            }
            .frame(height: 280)
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

    /// The web's /pair with the code typed in: what the QR code holds.
    private func pairURL(_ code: String) -> URL? {
        guard let base = app.client?.baseURL,
              var c = URLComponents(url: base.appending(path: "pair"), resolvingAgainstBaseURL: false) else { return nil }
        c.queryItems = [URLQueryItem(name: "code", value: code.filter { $0 != "-" })]
        return c.url
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
                                .focusHalo(Circle())
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

/// A QR code drawn sharp at any size: Core Image's generator, scaled
/// without smoothing.
struct QRCodeImage: View {
    let text: String

    var body: some View {
        if let image = Self.render(text) {
            Image(decorative: image, scale: 1)
                .interpolation(.none)
                .resizable()
                .aspectRatio(1, contentMode: .fit)
        }
    }

    private static func render(_ text: String) -> CGImage? {
        let f = CIFilter.qrCodeGenerator()
        f.message = Data(text.utf8)
        f.correctionLevel = "M"
        guard let out = f.outputImage else { return nil }
        return CIContext().createCGImage(out, from: out.extent)
    }
}
