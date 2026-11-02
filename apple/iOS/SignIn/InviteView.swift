import SwiftUI

/// An invitation link on iPhone and iPad (F009, canvas step 20, boards
/// Invite11–Invite16): who invites you, your account, your code (required),
/// your face, welcome. The link works once.
struct InviteView: View {
    let invite: PendingInvite

    @Environment(AppState.self) private var app
    @State private var preview: InvitationPreview?
    @State private var expired = false
    @State private var phase: Phase = .open
    @State private var displayName = ""
    @State private var username = ""
    @State private var password = ""
    @State private var pin = ""
    @State private var confirm = ""
    @State private var mismatch = false
    @State private var busy = false
    @State private var error: LocalizedStringKey?
    @State private var profile: Profile?
    @FocusState private var focused: Bool

    private enum Phase { case open, account, pin, confirm, face, welcome }

    private var client: APIClient { APIClient(baseURL: invite.server) }

    var body: some View {
        Group {
            if phase == .face, let profile {
                AvatarChoiceView(profile: profile, doneLabel: "invite.next") {
                    withAnimation(.smooth) { phase = .welcome }
                }
            } else {
                SignInScaffold { card.transition(.blurReplace) }
                    .overlay(alignment: .topLeading) { backButton }
            }
        }
        .animation(.smooth, value: phase)
        .task { await check() }
    }

    @ViewBuilder
    private var backButton: some View {
        let back: Phase? = switch phase {
        case .open: nil
        case .account: .open
        case .pin: .account
        case .confirm: .pin
        case .face, .welcome: nil
        }
        if phase == .open || expired {
            Button { app.invite = nil } label: {
                Image(systemName: "xmark").font(.system(size: 13, weight: .bold)).frame(width: 30, height: 30)
            }
            .buttonStyle(.glass).buttonBorderShape(.circle)
            .accessibilityLabel(Text("common.close"))
            .padding(.horizontal, 16)
        } else if let back {
            Button { withAnimation(.smooth) { phase = back } } label: {
                Image(systemName: "chevron.left").font(.system(size: 13, weight: .bold)).frame(width: 30, height: 30)
            }
            .buttonStyle(.glass).buttonBorderShape(.circle)
            .accessibilityLabel(Text("common.back"))
            .padding(.horizontal, 16)
        }
    }

    @ViewBuilder
    private var card: some View {
        if expired {
            GlassCard {
                LogoMark(size: 54)
                title("invite.expired.title")
                lead("invite.expired.body")
                Button("invite.close") { app.invite = nil }.buttonStyle(CreamButtonStyle(height: 46)).padding(.top, 18)
            }
        } else if let preview {
            switch phase {
            case .open: open(preview)
            case .account: account(preview)
            case .pin, .confirm: code
            case .welcome: welcome(preview)
            case .face: EmptyView()
            }
        } else {
            GlassCard { ProgressView().tint(Theme.textSecondary).frame(maxWidth: .infinity, minHeight: 160) }
        }
    }

    private func title(_ key: LocalizedStringKey) -> some View {
        Text(key).font(Theme.display(24)).tracking(-0.4).multilineTextAlignment(.center).padding(.top, 14)
    }

    private func lead(_ key: LocalizedStringKey) -> some View {
        Text(key).font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
            .multilineTextAlignment(.center).padding(.top, 8)
    }

    /// Invite11: the link opened.
    private func open(_ p: InvitationPreview) -> some View {
        GlassCard {
            LogoMark(size: 64)
            Text("invite.title \(p.invitedBy.isEmpty ? p.serverName : p.invitedBy) \(p.serverName)")
                .font(Theme.display(24)).tracking(-0.4).multilineTextAlignment(.center).padding(.top, 14)
            lead("invite.lead")
            Button("invite.start") { withAnimation(.smooth) { phase = .account } }
                .buttonStyle(CreamButtonStyle(height: 46)).padding(.top, 18)
        }
    }

    /// Invite12: the account.
    private func account(_ p: InvitationPreview) -> some View {
        GlassCard {
            title("invite.account.title")
            Text("invite.account.lead \(p.serverName)").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                .multilineTextAlignment(.center).padding(.top, 8)
            VStack(spacing: 10) {
                GlassField(systemImage: "person.crop.circle") {
                    TextField(text: $displayName, prompt: Text("invite.displayName").foregroundStyle(Theme.textTertiary)) {
                        Text("invite.displayName")
                    }
                    .textContentType(.name)
                }
                GlassField(systemImage: "person") {
                    TextField(text: $username, prompt: Text("signIn.username").foregroundStyle(Theme.textTertiary)) {
                        Text("signIn.username")
                    }
                    .textContentType(.username)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                }
                GlassField(systemImage: "lock") {
                    SecureField(text: $password, prompt: Text("signIn.password").foregroundStyle(Theme.textTertiary)) {
                        Text("signIn.password")
                    }
                    .textContentType(.newPassword)
                }
            }
            .padding(.top, 18)
            Text("invite.password.help").font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                .frame(maxWidth: .infinity, alignment: .leading).padding(.top, 8)
            if let error {
                Text(error).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error).padding(.top, 10)
            }
            Button("invite.next") {
                error = nil
                withAnimation(.smooth) { phase = .pin }
            }
            .buttonStyle(CreamButtonStyle(height: 46))
            .disabled(displayName.trimmingCharacters(in: .whitespaces).isEmpty || username.count < 3 || password.count < 8)
            .padding(.top, 16)
        }
    }

    /// Invite13 and Invite14: the code, then once more.
    private var code: some View {
        let confirming = phase == .confirm
        let value = confirming ? $confirm : $pin
        return GlassCard {
            title(confirming ? "invite.pin.confirm" : "invite.pin.title")
            lead("invite.pin.lead")
            HStack(spacing: 12) {
                ForEach(0..<4, id: \.self) { i in
                    let chars = Array(value.wrappedValue)
                    Text(verbatim: i < chars.count ? "•" : "")
                        .font(Theme.sans(28, .bold))
                        .frame(width: 56, height: 64)
                        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 16))
                        .overlay {
                            if i == chars.count { RoundedRectangle(cornerRadius: 16).strokeBorder(Theme.textPrimary.opacity(0.6)) }
                        }
                }
            }
            .padding(.top, 20)
            .overlay {
                TextField("", text: value)
                    .keyboardType(.numberPad)
                    .textContentType(.oneTimeCode)
                    .focused($focused)
                    .opacity(0.02)
                    .accessibilityLabel(Text("invite.pin.label"))
                    .onChange(of: value.wrappedValue) { _, v in
                        let digits = String(v.filter(\.isNumber).prefix(4))
                        if digits != v { value.wrappedValue = digits }
                        if digits.count == 4 { codeTyped() }
                    }
            }
            .onAppear { focused = true }
            .onTapGesture { focused = true }
            if mismatch && !confirming {
                Text("invite.pin.mismatch").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error)
                    .multilineTextAlignment(.center).padding(.top, 12)
            }
            if let error {
                Text(error).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error).padding(.top, 12)
            }
            if busy { ProgressView().tint(Theme.textSecondary).padding(.top, 14) }
        }
        .id(phase)
    }

    /// Invite16: welcome.
    private func welcome(_ p: InvitationPreview) -> some View {
        GlassCard {
            if let profile { ProfileFace(profile: profile, size: 96) }
            Text("invite.welcome.title \(displayName.trimmingCharacters(in: .whitespaces))")
                .font(Theme.display(26)).tracking(-0.4).multilineTextAlignment(.center).padding(.top, 14)
            Text("invite.welcome.lead \(p.serverName)").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                .multilineTextAlignment(.center).padding(.top, 8)
            Button("invite.welcome.go") { app.invite = nil }
                .buttonStyle(CreamButtonStyle(height: 46)).padding(.top, 18)
        }
    }

    private func check() async {
        do {
            let p: InvitationPreview = try await client.post("/invite/check", InvitationToken(token: invite.token))
            preview = p
            if displayName.isEmpty { displayName = p.name }
        } catch {
            expired = true
        }
    }

    private func codeTyped() {
        if phase == .pin {
            mismatch = false
            confirm = ""
            withAnimation(.smooth) { phase = .confirm }
            return
        }
        guard confirm == pin else {
            mismatch = true
            pin = ""
            withAnimation(.smooth) { phase = .pin }
            return
        }
        accept()
    }

    private func accept() {
        guard !busy else { return }
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                let result: AuthResult = try await client.post("/invite/accept", AcceptInvitationRequest(
                    token: invite.token, displayName: displayName.trimmingCharacters(in: .whitespaces),
                    username: username.lowercased(), password: password, pin: pin,
                    locale: Locale.current.language.languageCode?.identifier, device: AppState.device))
                app.acceptedInvitation(result, on: invite.server)
                profile = result.activeProfile ?? result.profiles.first
                withAnimation(.smooth) { phase = .face }
            } catch let e as APIError {
                switch e.code {
                case "invitation_expired", "invitation_not_found": expired = true
                case "username_taken": error = "invite.error.usernameTaken"; phase = .account
                case "weak_password": error = "invite.error.weakPassword"; phase = .account
                case "validation_failed": error = "invite.error.invalid"; phase = .account
                case "network": error = "server.unreachable"
                default: error = "error.generic"
                }
            } catch {
                self.error = "error.generic"
            }
        }
    }
}

/// Paste an invitation link, when it didn't open the app by itself.
struct InviteLinkCard: View {
    let back: () -> Void
    @Environment(AppState.self) private var app
    @State private var link = ""
    @State private var invalid = false

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
            Text("invite.link.title").font(Theme.display(22)).tracking(-0.4).padding(.top, 14)
            Text("invite.link.hint").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
                .multilineTextAlignment(.center).padding(.top, 6)
            GlassField(systemImage: "link") {
                TextField(text: $link, prompt: Text(verbatim: "https://…/invite/…").foregroundStyle(Theme.textTertiary)) {
                    Text("invite.link.title")
                }
                .keyboardType(.URL)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
            }
            .padding(.top, 16)
            if invalid {
                Text("invite.link.invalid").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error).padding(.top, 10)
            }
            Button("invite.next") {
                let url = URL(string: link.trimmingCharacters(in: .whitespacesAndNewlines))
                invalid = url.map { !app.openInvite($0) } ?? true
            }
            .buttonStyle(CreamButtonStyle(height: 46))
            .disabled(link.isEmpty)
            .padding(.top, 14)
        }
    }
}
