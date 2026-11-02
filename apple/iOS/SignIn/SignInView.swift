import SwiftUI

/// Sign in to the chosen server (IPh12 / IPa12): its name large, the
/// username and password in glass fields, then the cream button.
struct SignInView: View {
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @State private var username = ""
    @State private var password = ""
    @State private var showPassword = false
    @State private var busy = false
    @State private var error: LocalizedStringKey?
    @FocusState private var field: Field?

    private enum Field { case username, password }

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            WarmGround()
            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    Spacer(minLength: 120)
                    form.frame(maxWidth: wide ? 420 : .infinity, alignment: .leading)
                }
                .padding(.horizontal, wide ? 56 : 28)
                .padding(.bottom, 24)
                .frame(maxWidth: .infinity, alignment: .leading)
                .containerRelativeFrame(.vertical, alignment: .bottom) { h, _ in h }
            }
            .scrollBounceBehavior(.basedOnSize)
            .scrollDismissesKeyboard(.interactively)
        }
        .overlay(alignment: .topLeading) {
            Button { app.forgetServer() } label: {
                Label("pair.changeServer", systemImage: "chevron.left").font(Theme.sans(13, .semibold))
            }
            .buttonStyle(.glass)
            .padding(.horizontal, 16)
        }
    }

    private var form: some View {
        VStack(alignment: .leading, spacing: 0) {
            if let host = app.client?.baseURL.host() {
                Text(verbatim: host)
                    .font(Theme.sans(12, .semibold))
                    .foregroundStyle(Theme.textSecondary)
            }
            Text(verbatim: app.serverName ?? "PLAID")
                .displayTitle(44)
                .lineLimit(2)
                .minimumScaleFactor(0.6)
                .padding(.top, 8)
            Text("signIn.subtitle")
                .font(Theme.sans(15, .medium))
                .foregroundStyle(Theme.textSecondary)
                .padding(.top, 10)

            VStack(spacing: 10) {
                GlassField(systemImage: "person") {
                    TextField(text: $username, prompt: Text("signIn.username").foregroundStyle(Theme.textTertiary)) {
                        Text("signIn.username")
                    }
                    .textContentType(.username)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .focused($field, equals: .username)
                    .submitLabel(.next)
                    .onSubmit { field = .password }
                }
                GlassField(systemImage: "lock") {
                    Group {
                        if showPassword {
                            TextField(text: $password, prompt: Text("signIn.password").foregroundStyle(Theme.textTertiary)) { Text("signIn.password") }
                        } else {
                            SecureField(text: $password, prompt: Text("signIn.password").foregroundStyle(Theme.textTertiary)) { Text("signIn.password") }
                        }
                    }
                    .textContentType(.password)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .focused($field, equals: .password)
                    .submitLabel(.go)
                    .onSubmit(signIn)
                    Button { showPassword.toggle() } label: {
                        Image(systemName: showPassword ? "eye.slash" : "eye").foregroundStyle(Theme.textTertiary)
                    }
                    .accessibilityLabel(Text(showPassword ? "signIn.hidePassword" : "signIn.showPassword"))
                }
            }
            .padding(.top, 22)

            if let error {
                Text(error).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error).padding(.top, 12)
            }

            Button(action: signIn) {
                if busy { ProgressView().tint(Theme.onCream) } else { Text("signIn.submit") }
            }
            .buttonStyle(CreamButtonStyle())
            .disabled(username.isEmpty || password.isEmpty || busy)
            .padding(.top, 14)

            Text("signIn.forgot")
                .font(Theme.sans(12, .medium))
                .foregroundStyle(Theme.textTertiary)
                .frame(maxWidth: .infinity)
                .multilineTextAlignment(.center)
                .padding(.top, 16)
        }
    }

    private func signIn() {
        guard let client = app.client, !username.isEmpty, !password.isEmpty, !busy else { return }
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                let result: AuthResult = try await client.post(
                    "/auth/login", LoginRequest(username: username, password: password, device: AppState.device))
                app.signedIn(result)
            } catch let e as APIError {
                switch e.code {
                case "invalid_credentials": error = "signIn.error.invalid"
                case "too_many_attempts": error = "signIn.error.tooMany"
                case "network": error = "server.unreachable"
                default: error = "error.generic"
                }
            } catch {
                self.error = "error.generic"
            }
        }
    }
}

/// A glass capsule holding an icon and a text field.
struct GlassField<Content: View>: View {
    let systemImage: String
    @ViewBuilder let content: Content

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: systemImage).font(.system(size: 15, weight: .medium)).foregroundStyle(Theme.textSecondary)
                .frame(width: 20)
            content
        }
        .font(Theme.sans(16, .medium))
        .padding(.horizontal, 18)
        .frame(height: 50)
        .glassEffect(.regular.interactive(), in: Capsule())
    }
}
