#if DEBUG
import Foundation

/// Debug builds only: `PLAID_DEV_SERVER` and `PLAID_DEV_TOKEN` in the launch
/// environment (`SIMCTL_CHILD_…` with `simctl launch`) sign the app in
/// without typing, for simulator runs.
enum DevSession {
    static func apply() {
        let env = ProcessInfo.processInfo.environment
        guard let server = env["PLAID_DEV_SERVER"], let token = env["PLAID_DEV_TOKEN"] else { return }
        UserDefaults.standard.set(server, forKey: "server.url")
        Keychain.save(token, account: server)
    }
}
#endif
