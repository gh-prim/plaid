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
        /// Signing in: a code on the TV, a password on iPhone and iPad.
        case pairing
        case chooseProfile
        case ready
    }

    private(set) var phase: Phase = .loading
    /// An invitation link being answered (F009): its flow covers everything
    /// else until the new account is set up.
    var invite: PendingInvite?
    /// The launch intro plays once per cold start, over the app loading
    /// behind it; nothing behind it takes focus meanwhile. Apple TV only.
    #if os(tvOS)
    var splashing = true
    #else
    var splashing = false
    #endif
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

    /// Opens an invitation: plaid://invite?server=…&token=… from the web
    /// page, or the link itself (https://server/invite/<token>). False when
    /// url isn't one.
    @discardableResult
    func openInvite(_ url: URL) -> Bool {
        guard let invite = PendingInvite(url) else { return false }
        self.invite = invite
        return true
    }

    /// The invited account exists: this server becomes the app's, signed in.
    func acceptedInvitation(_ result: AuthResult, on server: URL) {
        UserDefaults.standard.set(server.absoluteString, forKey: serverKey)
        client = APIClient(baseURL: server)
        signedIn(result)
        Task { await refresh() }
    }

    /// The active profile changed (its picture): read the account again.
    func reloadMe() async {
        guard let client, let me: Me = try? await client.get("/me") else { return }
        self.me = me
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
        #if os(tvOS)
        let type = "tvos"
        #else
        let type = UIDevice.current.userInterfaceIdiom == .pad ? "ipados" : "ios"
        #endif
        return Device(name: UIDevice.current.name, type: type, clientVersion: version)
    }
}

/// An invitation link (F009): the server it comes from and its token.
struct PendingInvite: Equatable {
    let server: URL
    let token: String

    init?(_ url: URL) {
        let c = URLComponents(url: url, resolvingAgainstBaseURL: false)
        if url.scheme == "plaid", url.host() == "invite" {
            guard let s = c?.queryItems?.first(where: { $0.name == "server" })?.value, let server = URL(string: s),
                  let token = c?.queryItems?.first(where: { $0.name == "token" })?.value, !token.isEmpty
            else { return nil }
            self.server = server
            self.token = token
            return
        }
        // https://server[/prefix]/invite/<token>
        let parts = url.path().split(separator: "/")
        guard url.scheme == "http" || url.scheme == "https", parts.count >= 2, parts[parts.count - 2] == "invite",
              var base = c
        else { return nil }
        token = String(parts[parts.count - 1])
        base.path = "/" + parts.dropLast(2).joined(separator: "/")
        base.query = nil
        base.fragment = nil
        guard let server = base.url else { return nil }
        self.server = server
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
