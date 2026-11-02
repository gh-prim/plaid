import SwiftUI

@main
struct PLAIDApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @State private var app: AppState

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
            switch app.phase {
            case .loading: LoadingView()
            case .chooseServer: ServerPickerView()
            case .pairing: SignInView()
            case .chooseProfile: ProfilePickerView()
            case .ready: MainTabs()
            }
        }
        .onGeometryChange(for: CGFloat.self) { $0.safeAreaInsets.top } action: { topInset = $0 }
        .environment(\.topInset, topInset)
        .animation(.smooth, value: app.phase)
        // A new language redraws everything, with its formats too.
        .id(app.language ?? "system")
        .environment(\.locale, app.language.map(Locale.init(identifier:)) ?? .autoupdatingCurrent)
    }
}
