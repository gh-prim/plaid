import SwiftUI

/// Pair a TV (F003 R20, the web's /pair): the code the TV shows, the profile
/// it opens on and that profile's PIN when it has one; then "connected".
struct PairTVView: View {
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @State private var code: String
    @State private var profileId = ""
    @State private var pin = ""
    @State private var busy = false
    @State private var codeError: LocalizedStringKey?
    @State private var pinError: LocalizedStringKey?
    @State private var paired: (device: String, profile: Profile?)?
    @FocusState private var focus: Field?
    @State private var scanning = false

    private enum Field { case code, pin }

    /// The TV's code alphabet (server/internal/auth/token.go): no 0, O, 1, I or L.
    private static let alphabet = Set("ABCDEFGHJKMNPQRSTUVWXYZ23456789")

    /// `code`: already known, from a TV's QR code or a plaid://pair link.
    init(code: String = "") {
        _code = State(initialValue: code)
    }

    private var profiles: [Profile] { app.me?.profiles ?? [] }
    private var profile: Profile? { profiles.first { $0.id == profileId } }
    private var ready: Bool { code.count == 6 && (profile?.hasPin != true || pin.count == 4) }

    var body: some View {
        ScrollView {
            VStack(spacing: 0) {
                if let paired {
                    done(paired)
                } else {
                    form
                }
            }
            .frame(maxWidth: 440)
            .padding(.horizontal, Theme.gutter)
            .padding(.vertical, 24)
            .frame(maxWidth: .infinity)
        }
        .scrollDismissesKeyboard(.interactively)
        .background(Theme.bgBase)
        .navigationTitle(Text("pairing.title"))
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            if profileId.isEmpty { profileId = app.me?.activeProfile?.id ?? profiles.first?.id ?? "" }
            focus = .code
        }
        .sheet(isPresented: $scanning) {
            QRScannerSheet { found in
                code = found
                focus = profile?.hasPin == true ? .pin : nil
            }
        }
    }

    private var form: some View {
        VStack(spacing: 0) {
            TVDrawing(code: code)
            Text("pairing.lead").font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary)
                .multilineTextAlignment(.center).padding(.top, 20)

            // The real field sits under the boxes, which draw what it holds.
            ZStack {
                TextField("", text: $code)
                    .focused($focus, equals: .code)
                    .textInputAutocapitalization(.characters)
                    .autocorrectionDisabled()
                    .keyboardType(.asciiCapable)
                    .opacity(0.02)
                    .accessibilityLabel(Text("pairing.codeLabel"))
                HStack(spacing: 7) {
                    ForEach(0..<6, id: \.self) { i in
                        let chars = Array(code)
                        let current = focus == .code && i == min(chars.count, 5)
                        Text(verbatim: i < chars.count ? String(chars[i]) : "")
                            .font(Theme.display(22, .semibold))
                            .frame(width: 44, height: 54)
                            .background(RoundedRectangle(cornerRadius: 12).fill(Color.white.opacity(0.08)))
                            .overlay {
                                RoundedRectangle(cornerRadius: 12).strokeBorder(
                                    codeError != nil ? Theme.error.opacity(0.8) : (current ? Theme.textPrimary.opacity(0.8) : Color.white.opacity(0.14)),
                                    lineWidth: codeError != nil || current ? 1.5 : 1)
                            }
                    }
                }
                .allowsHitTesting(false)
                .accessibilityHidden(true)
            }
            .contentShape(Rectangle())
            .onTapGesture { focus = .code }
            .padding(.top, 20)
            if QRScannerSheet.available {
                Button { scanning = true } label: { Label("pairing.scan", systemImage: "qrcode.viewfinder") }
                    .font(Theme.sans(14, .semibold))
                    .buttonStyle(.glass)
                    .padding(.top, 14)
            }
            Text(codeError ?? "pairing.codeHint")
                .font(Theme.sans(12, .medium))
                .foregroundStyle(codeError == nil ? Theme.textTertiary : Theme.error)
                .multilineTextAlignment(.center)
                .padding(.top, 10)

            if profiles.count > 1 {
                Text("pairing.startProfile").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                    .frame(maxWidth: .infinity, alignment: .leading).padding(.top, 28)
                ScrollView(.horizontal) {
                    HStack(spacing: 14) {
                        ForEach(profiles) { p in
                            Button {
                                profileId = p.id
                                pin = ""
                                pinError = nil
                                if p.hasPin { focus = .pin }
                            } label: {
                                VStack(spacing: 6) {
                                    ProfileFace(profile: p, size: 52)
                                        .padding(3)
                                        .overlay { if p.id == profileId { Circle().strokeBorder(Theme.textPrimary, lineWidth: 2.5) } }
                                    Text(verbatim: p.name).font(Theme.sans(12, p.id == profileId ? .semibold : .medium))
                                        .foregroundStyle(p.id == profileId ? Theme.textPrimary : Theme.textSecondary)
                                }
                            }
                            .buttonStyle(.plain)
                            .accessibilityAddTraits(p.id == profileId ? .isSelected : [])
                        }
                    }
                    .padding(.vertical, 4)
                }
                .scrollIndicators(.hidden)
                .padding(.top, 10)
            }

            if let profile, profile.hasPin {
                GlassField(systemImage: "lock") {
                    SecureField(text: $pin, prompt: Text("pairing.pin \(profile.name)").foregroundStyle(Theme.textTertiary)) {
                        Text("pairing.pin \(profile.name)")
                    }
                    .keyboardType(.numberPad)
                    .textContentType(.oneTimeCode)
                    .focused($focus, equals: .pin)
                }
                .padding(.top, 16)
                if let pinError {
                    Text(pinError).font(Theme.sans(12, .semibold)).foregroundStyle(Theme.error).padding(.top, 8)
                }
            }

            Button { Task { await claim() } } label: {
                if busy { ProgressView().tint(Theme.onCream) } else { Text("pairing.confirm") }
            }
            .buttonStyle(CreamButtonStyle())
            .disabled(!ready || busy)
            .padding(.top, 24)
        }
        .onChange(of: code) { _, typed in
            let clean = String(typed.uppercased().filter { Self.alphabet.contains($0) }.prefix(6))
            if clean != typed { code = clean; return }
            codeError = nil
            if clean.count == 6, profile?.hasPin == true { focus = .pin }
        }
        .onChange(of: pin) { _, typed in
            let clean = String(typed.filter(\.isNumber).prefix(4))
            if clean != typed { pin = clean }
            pinError = nil
        }
    }

    private func done(_ paired: (device: String, profile: Profile?)) -> some View {
        VStack(spacing: 0) {
            TVDrawing(code: nil, lit: true)
            Text("pairing.done.title \(paired.device)").displayTitle(26).multilineTextAlignment(.center).padding(.top, 24)
            if let p = paired.profile {
                Text("pairing.done.profile \(p.name)").font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary)
                    .padding(.top, 8)
            }
            Button("pairing.done.finish") { dismiss() }
                .buttonStyle(CreamButtonStyle())
                .padding(.top, 28)
            Button("pairing.done.another") {
                self.paired = nil
                focus = .code
            }
            .font(Theme.sans(14, .semibold))
            .buttonStyle(.glass)
            .padding(.top, 12)
        }
    }

    private func claim() async {
        guard let client = app.client, ready, !busy else { return }
        busy = true
        defer { busy = false }
        codeError = nil
        pinError = nil
        do {
            let s: SessionInfo = try await client.post(
                "/pairing/claim", ClaimPairingRequest(code: code, profileId: profileId, pin: profile?.hasPin == true ? pin : nil))
            withAnimation(.smooth) { paired = (s.deviceName, profile) }
            code = ""
            pin = ""
        } catch let e as APIError {
            switch e.code {
            case "invalid_pin": pinError = "profiles.wrongPin"; pin = ""
            case "profile_locked": pinError = "profiles.pinLocked"; pin = ""
            case "pairing_not_found": codeError = "pairing.error.notFound"
            case "pairing_expired": codeError = "pairing.error.expired"
            case "too_many_attempts": codeError = "signIn.error.tooMany"
            default: codeError = "error.generic"
            }
        } catch {
            codeError = "error.generic"
        }
    }
}

/// A drawn TV echoing the code as it's typed; lit once connected.
private struct TVDrawing: View {
    let code: String?
    var lit = false

    var body: some View {
        VStack(spacing: 0) {
            ZStack {
                RoundedRectangle(cornerRadius: 14)
                    .fill(lit ? AnyShapeStyle(LinearGradient(colors: [Color(hex: "#6b4a1e"), Color(hex: "#1f2742")],
                                                             startPoint: .topLeading, endPoint: .bottomTrailing))
                                : AnyShapeStyle(Color(hex: "#111113")))
                RoundedRectangle(cornerRadius: 14).strokeBorder(Color.white.opacity(0.14), lineWidth: 1)
                if lit {
                    Image(systemName: "checkmark").font(.system(size: 34, weight: .bold)).foregroundStyle(Theme.textPrimary)
                } else {
                    VStack(spacing: 10) {
                        LogoMark(size: 26)
                        Text(verbatim: displayed).font(Theme.display(24, .semibold)).tracking(5).monospaced()
                            .foregroundStyle(Theme.textPrimary.opacity(code?.isEmpty == false ? 1 : 0.3))
                            .contentTransition(.numericText())
                            .animation(.smooth(duration: 0.2), value: code)
                    }
                }
            }
            .frame(width: 240, height: 140)
            .shadow(color: lit ? Color(hex: "#6b4a1e").opacity(0.5) : .clear, radius: 30)
            Capsule().fill(Color.white.opacity(0.18)).frame(width: 70, height: 5).padding(.top, 8)
        }
        .accessibilityHidden(true)
        .animation(.smooth(duration: 0.4), value: lit)
    }

    private var displayed: String {
        let c = Array(code ?? "")
        return (0..<6).map { $0 < c.count ? String(c[$0]) : "·" }.joined()
    }
}
