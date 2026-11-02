import SwiftUI

@main
struct PLAIDApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @State private var app: AppState
    /// The realtime socket and the party this device is in (F010).
    @State private var party = PartyCenter()
    @State private var partyNav = PartyNav()

    init() {
        #if DEBUG
        DevSession.apply()
        #endif
        _app = State(initialValue: AppState())
    }

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(app)
                .environment(party)
                .environment(partyNav)
                .preferredColorScheme(.dark)
                .tint(Theme.textPrimary)
                .foregroundStyle(Theme.textPrimary)
        }
    }
}

struct RootView: View {
    @Environment(AppState.self) private var app
    @State private var topInset: CGFloat = 0

    var body: some View {
        ZStack {
            Theme.bgBase.ignoresSafeArea()
            if let invite = app.invite {
                // An invitation covers everything until the account is set up
                // (F009). It stays out of the language's id below: the new
                // account's language must not restart it half way.
                InviteView(invite: invite).id(invite.token).transition(.opacity)
            } else {
                Group {
                    switch app.phase {
                    case .loading: LoadingView()
                    case .chooseServer: ServerPickerView()
                    case .pairing: SignInView()
                    case .chooseProfile: ProfilePickerView()
                    case .ready: MainTabs()
                    }
                }
                // A new language redraws everything, with its formats too.
                .id(app.language ?? "system")
            }
        }
        // plaid://invite?server=…&token=… from the invitation web page.
        .onOpenURL { url in app.openInvite(url) }
        .onGeometryChange(for: CGFloat.self) { $0.safeAreaInsets.top } action: { topInset = $0 }
        .environment(\.topInset, topInset)
        .animation(.smooth, value: app.phase)
        .animation(.smooth, value: app.invite)
        .environment(\.locale, app.language.map(Locale.init(identifier:)) ?? .autoupdatingCurrent)
    }
}
