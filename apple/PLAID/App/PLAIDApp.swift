import SwiftUI

@main
struct PLAIDApp: App {
    @State private var app = AppState()
    /// The launch intro plays once per cold start, over the app loading behind it.
    @State private var showSplash = true

    var body: some Scene {
        WindowGroup {
            ZStack {
                RootView()
                    .environment(app)
                    // Nothing behind the intro takes focus or remote presses.
                    .disabled(showSplash)
                if showSplash {
                    SplashView { showSplash = false }
                }
            }
            .preferredColorScheme(.dark)
        }
    }
}

struct RootView: View {
    @Environment(AppState.self) private var app

    var body: some View {
        ZStack {
            Theme.bgBase.ignoresSafeArea()
            switch app.phase {
            case .loading: LoadingView()
            case .chooseServer: ServerPickerView()
            case .pairing: PairingView()
            case .chooseProfile: ProfilePickerView()
            case .ready: MainView()
            }
        }
        .animation(.smooth, value: app.phase)
        // A new language redraws everything, with its formats too.
        .id(app.language ?? "system")
        .environment(\.locale, app.language.map(Locale.init(identifier:)) ?? .autoupdatingCurrent)
    }
}
