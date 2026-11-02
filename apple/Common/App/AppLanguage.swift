import Foundation
import ObjectiveC

/// Shows the app in the profile's language rather than the TV's (F008 R3):
/// every localized lookup (SwiftUI Text, String(localized:)) goes through
/// Bundle.main, which reads the chosen language's strings when one is set.
nonisolated enum AppLanguage {
    nonisolated(unsafe) fileprivate static var key: UInt8 = 0
    /// Dates, durations and language names follow the app's language too.
    nonisolated(unsafe) static var locale: Locale = .autoupdatingCurrent

    static func set(_ code: String?) {
        object_setClass(Bundle.main, LanguageBundle.self)
        let bundle = code.flatMap { Bundle.main.path(forResource: $0, ofType: "lproj") }.flatMap(Bundle.init(path:))
        objc_setAssociatedObject(Bundle.main, &key, bundle, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)
        locale = code.map { Locale(identifier: $0) } ?? .autoupdatingCurrent
        // System views (the player's Info panel, alerts) read AppleLanguages
        // when the app starts, so they follow the profile from the next launch.
        if let code {
            UserDefaults.standard.set([code], forKey: "AppleLanguages")
        } else {
            UserDefaults.standard.removeObject(forKey: "AppleLanguages")
        }
    }
}

nonisolated private final class LanguageBundle: Bundle, @unchecked Sendable {
    override func localizedString(forKey key: String, value: String?, table tableName: String?) -> String {
        if let chosen = objc_getAssociatedObject(self, &AppLanguage.key) as? Bundle {
            return chosen.localizedString(forKey: key, value: value, table: tableName)
        }
        return super.localizedString(forKey: key, value: value, table: tableName)
    }
}
