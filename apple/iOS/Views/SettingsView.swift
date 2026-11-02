import SwiftUI

/// Settings, from the profile's face: who's watching, playback choices,
/// the account (password, pairing a TV), the server and About PLAID with
/// the metadata providers' notices (F005 R37).
struct SettingsView: View {
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @Environment(\.openURL) private var openURL
    @State private var confirmSignOut = false
    @State private var changingPicture = false

    private var profileId: String { app.me?.activeProfile?.id ?? "default" }

    var body: some View {
        @Bindable var app = app
        NavigationStack {
            List {
                Section {
                    if let p = app.me?.activeProfile {
                        HStack(spacing: 14) {
                            ProfileFace(profile: p, size: 44)
                            VStack(alignment: .leading, spacing: 3) {
                                Text(verbatim: p.name).font(Theme.sans(17, .semibold))
                                if let u = app.me?.user.displayName ?? app.me?.user.username {
                                    Text(verbatim: u).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textTertiary)
                                }
                            }
                        }
                        .padding(.vertical, 4)
                    }
                    if let p = app.me?.activeProfile {
                        Button { changingPicture = true } label: { Label("avatar.change", systemImage: "face.smiling") }
                            .sheet(isPresented: $changingPicture) {
                                AvatarChoiceView(profile: p, doneLabel: "common.done") { changingPicture = false }
                                    .presentationBackground(Theme.bgBase)
                            }
                    }
                    Button { dismiss(); app.switchProfile() } label: { Label("settings.switchProfile", systemImage: "person.2") }
                    if let base = app.client?.baseURL {
                        Button { openURL(base.appending(path: "profiles")) } label: {
                            Label("profiles.manage", systemImage: "person.crop.circle.badge.plus")
                        }
                    }
                }

                Section("settings.playback") {
                    Toggle(isOn: Binding(
                        get: { UserDefaults.standard.object(forKey: autoplayKey(profileId)) as? Bool ?? true },
                        set: { UserDefaults.standard.set($0, forKey: autoplayKey(profileId)) }
                    )) { Text("settings.autoplayNext") }
                    Toggle(isOn: $app.previewsOn) { Text("settings.previews") }
                }

                Section("settings.account") {
                    NavigationLink { ChangePasswordView() } label: { Label("settings.changePassword", systemImage: "key") }
                    NavigationLink { PairTVView() } label: { Label("pairing.title", systemImage: "appletv") }
                    Button(role: .destructive) { confirmSignOut = true } label: {
                        Label("settings.signOut", systemImage: "rectangle.portrait.and.arrow.right")
                    }
                }

                Section("settings.serverSection") {
                    if let name = app.serverName {
                        LabeledContent { Text(verbatim: app.client?.baseURL.host() ?? "") } label: { Text(verbatim: name) }
                    }
                    Button { dismiss(); app.forgetServer() } label: { Label("pair.changeServer", systemImage: "server.rack") }
                }

                Section("settings.about") {
                    LabeledContent("settings.versionsLabel") {
                        Text(verbatim: tr("settings.versions %@ %@", appVersion, app.server?.version ?? "?"))
                    }
                    ForEach(app.server?.attributions ?? [], id: \.provider) { a in
                        VStack(alignment: .leading, spacing: 6) {
                            Text(verbatim: a.name).font(Theme.sans(15, .semibold))
                            Text(verbatim: attributionText(a)).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
                            if let url = URL(string: a.url) {
                                Link(destination: url) { Text(verbatim: url.host() ?? a.url).font(Theme.sans(13, .semibold)) }
                            }
                        }
                        .padding(.vertical, 4)
                    }
                }
            }
            .scrollContentBackground(.hidden)
            .background(Theme.bgBase)
            .navigationTitle(Text("nav.settings"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(role: .close) { dismiss() }
                }
            }
            .confirmationDialog(Text("settings.signOut.confirm"), isPresented: $confirmSignOut, titleVisibility: .visible) {
                Button("settings.signOut", role: .destructive) { Task { await app.signOut() } }
            }
        }
        .font(Theme.sans(16, .medium))
    }

    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "?"
    }

    /// The notice in the app's language when the catalog has it, else the server's.
    private func attributionText(_ a: Attribution) -> String {
        let key = "about.attribution." + a.provider
        let text = localized(key)
        return text == key ? a.notice : text
    }
}

/// Change the account's password: the current one, then the new one twice.
/// Every other session of the account ends; this one keeps working.
struct ChangePasswordView: View {
    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @State private var current = ""
    @State private var new = ""
    @State private var again = ""
    @State private var busy = false
    @State private var error: LocalizedStringKey?
    @State private var done = false

    private var mismatch: Bool { !again.isEmpty && again != new }

    var body: some View {
        Form {
            Section {
                SecureField(text: $current) { Text("password.current") }.textContentType(.password)
            }
            Section {
                SecureField(text: $new) { Text("password.new") }.textContentType(.newPassword)
                SecureField(text: $again) { Text("password.again") }.textContentType(.newPassword)
            } footer: {
                if mismatch {
                    Text("password.mismatch").foregroundStyle(Theme.error)
                } else {
                    Text("password.othersSignedOut")
                }
            }
            if let error {
                Text(error).foregroundStyle(Theme.error)
            }
            Section {
                Button {
                    Task { await submit() }
                } label: {
                    if busy { ProgressView() } else { Text("password.submit") }
                }
                .disabled(current.isEmpty || new.isEmpty || new != again || busy)
            }
        }
        .scrollContentBackground(.hidden)
        .background(Theme.bgBase)
        .navigationTitle(Text("settings.changePassword"))
        .alert(Text("password.changed"), isPresented: $done) {
            Button("common.ok") { dismiss() }
        }
    }

    private func submit() async {
        guard let client = app.client else { return }
        busy = true
        defer { busy = false }
        error = nil
        do {
            try await client.postNoContent("/me/password", ChangePasswordRequest(currentPassword: current, newPassword: new))
            done = true
        } catch let e as APIError {
            switch e.code {
            case "invalid_credentials": error = "password.error.current"
            case "weak_password": error = "password.error.weak"
            case "too_many_attempts": error = "signIn.error.tooMany"
            default: error = "error.generic"
            }
        } catch {
            self.error = "error.generic"
        }
    }
}
