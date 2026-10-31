import Foundation
import Observation
import Security
import UIKit

/// Where the app is: no server yet, signing in, or signed in.
@Observable
final class AppState {
    enum Phase: Equatable {
        case loading
        case chooseServer
        case pairing
        case chooseProfile
        case ready
    }

    private(set) var phase: Phase = .loading
    private(set) var client: APIClient?
    private(set) var serverName: String?
    private(set) var server: ServerInfo?
    private(set) var me: Me? {
        didSet { profileChanged() }
    }
    /// The UI language: the profile's, else the account's, else the TV's.
    private(set) var language: String?
    /// "Play previews automatically", for the active profile on this TV.
    var previewsOn = true {
        didSet { UserDefaults.standard.set(previewsOn, forKey: "previews.\(profileKey)") }
    }

    private var profileKey: String { me?.activeProfile?.id ?? "default" }

    private func profileChanged() {
        previewsOn = UserDefaults.standard.object(forKey: "previews.\(profileKey)") as? Bool ?? true
        let wanted = (me?.activeProfile?.locale ?? me?.user.locale).map { String($0.prefix(2)).lowercased() }
        let lang = wanted.flatMap { Bundle.main.path(forResource: $0, ofType: "lproj") != nil ? $0 : nil }
        if lang != language {
            language = lang
            AppLanguage.set(lang)
        }
    }

    private let serverKey = "server.url"

    init() {
        if let s = UserDefaults.standard.string(forKey: serverKey), let url = URL(string: s) {
            client = APIClient(baseURL: url, token: Keychain.read(account: s))
        }
        Task { await refresh() }
    }

    /// Settles the phase from what is stored and what the server says.
    func refresh() async {
        guard let client else { phase = .chooseServer; return }
        if let info: ServerInfo = try? await client.get("/server") { server = info; serverName = info.name }
        guard client.token != nil else { phase = .pairing; return }
        do {
            let me: Me = try await client.get("/me")
            self.me = me
            phase = me.activeProfile == nil ? .chooseProfile : .ready
        } catch let e as APIError where e.status == 401 {
            signOutLocally()
        } catch {
            // Offline or server down: keep the session, show the app; screens retry.
            phase = .ready
        }
    }

    func useServer(_ url: URL) async {
        UserDefaults.standard.set(url.absoluteString, forKey: serverKey)
        client = APIClient(baseURL: url, token: Keychain.read(account: url.absoluteString))
        phase = .loading
        await refresh()
    }

    func forgetServer() {
        signOutLocally()
        UserDefaults.standard.removeObject(forKey: serverKey)
        client = nil
        serverName = nil
        phase = .chooseServer
    }

    func signedIn(_ result: AuthResult) {
        guard let client, let token = result.token else { return }
        client.token = token
        Keychain.save(token, account: client.baseURL.absoluteString)
        me = Me(user: result.user, profiles: result.profiles, activeProfile: result.activeProfile)
        phase = result.activeProfile == nil ? .chooseProfile : .ready
    }

    func selectProfile(_ profile: Profile, pin: String?) async throws {
        guard let client else { return }
        me = try await client.post("/session/profile", SelectProfileRequest(profileId: profile.id, pin: pin))
        phase = .ready
    }

    func switchProfile() {
        phase = .chooseProfile
    }

    func signOut() async {
        if let client { try? await client.send("POST", "/auth/logout", body: Optional<Int>.none) }
        signOutLocally()
    }

    private func signOutLocally() {
        if let client {
            Keychain.delete(account: client.baseURL.absoluteString)
            client.token = nil
        }
        me = nil
        phase = client == nil ? .chooseServer : .pairing
    }

    static var device: Device {
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String
        return Device(name: UIDevice.current.name, type: "tvos", clientVersion: version)
    }
}

/// The session token, kept in the Keychain per server.
enum Keychain {
    private static let service = "app.plaid.tv.session"

    static func read(account: String) -> String? {
        let q: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
        ]
        var out: AnyObject?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let data = out as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    static func save(_ value: String, account: String) {
        delete(account: account)
        let q: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: Data(value.utf8),
        ]
        SecItemAdd(q as CFDictionary, nil)
    }

    static func delete(account: String) {
        let q: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
        SecItemDelete(q as CFDictionary)
    }
}
