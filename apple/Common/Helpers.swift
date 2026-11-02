import Foundation
import SwiftUI

/// What the player is asked to play, and from where.
struct PlayRequest: Identifiable, Hashable {
    let id = UUID()
    let item: ItemSummary
    let startMs: Int64
    /// The version picked on the item page; nil lets the server choose.
    var fileId: String?
}

/// Pages reached from an item: a person, a collection, a library by genre
/// or a library's collections.
struct PersonRoute: Hashable { let id: String; let name: String }
struct CollectionRoute: Hashable { let id: String; let name: String }
struct GenreRoute: Hashable { let libraryId: String; let genre: GenreRef }
struct CollectionsRoute: Hashable { let library: Library }

/// An item known only by its id (a show from an episode's menu).
struct ItemRoute: Hashable { let id: String }

struct PlayAction {
    let run: (PlayRequest) -> Void
    func callAsFunction(_ item: ItemSummary, from startMs: Int64 = 0, fileId: String? = nil) {
        run(PlayRequest(item: item, startMs: startMs, fileId: fileId))
    }
}

private struct PlayKey: EnvironmentKey {
    static let defaultValue = PlayAction { _ in }
}

extension EnvironmentValues {
    var play: PlayAction {
        get { self[PlayKey.self] }
        set { self[PlayKey.self] = newValue }
    }
}

func unique<T: Hashable>(_ xs: [T]) -> [T] {
    var seen = Set<T>()
    return xs.filter { seen.insert($0).inserted }
}

func initials(_ name: String) -> String {
    name.split(separator: " ").compactMap(\.first).prefix(2).map(String.init).joined()
}

func parseDay(_ s: String) -> Date? {
    let f = DateFormatter()
    f.calendar = Calendar(identifier: .gregorian)
    f.locale = Locale(identifier: "en_US_POSIX")
    f.dateFormat = "yyyy-MM-dd"
    return f.date(from: String(s.prefix(10)))
}

func resolutionLabel(_ height: Int?) -> String? {
    guard let h = height else { return nil }
    switch h {
    case 1600...: return "4K"
    case 1000...: return "1080p"
    case 700...: return "720p"
    default: return "SD"
    }
}

func hdrLabel(_ f: String) -> String {
    ["hdr10": "HDR10", "hdr10plus": "HDR10+", "dolby_vision": "Dolby Vision", "hlg": "HLG"][f] ?? f.uppercased()
}

/// A stream's language in the viewer's language; "und" and empty stay unnamed.
func languageName(_ code: String) -> String {
    guard !code.isEmpty, code != "und" else { return tr("item.about.unknownLanguage") }
    let two = Locale.LanguageCode(code).identifier(.alpha2) ?? code
    return AppLanguage.locale.localizedString(forLanguageCode: two)?.capitalized(with: AppLanguage.locale) ?? code
}

/// The text of a key built at run time (a status, a role, a sort): string
/// interpolation in a localized literal would turn the variable part into
/// a format argument instead.
func localized(_ key: String) -> String {
    Bundle.main.localizedString(forKey: key, value: nil, table: nil)
}

/// A localized text built in code, in the app's language (AppLanguage):
/// the catalog key with its format arguments.
func tr(_ key: String, _ args: CVarArg...) -> String {
    let format = localized(key)
    return args.isEmpty ? format : String(format: format, locale: AppLanguage.locale, arguments: args)
}

func episodeCode(_ item: ItemSummary) -> String? {
    guard let s = item.seasonNumber, let e = item.episodeNumber else { return nil }
    return tr("card.episodeShort %lld %lld", s, e)
}

extension Color {
    init(hex: String) {
        let s = hex.trimmingCharacters(in: CharacterSet(charactersIn: "#"))
        var v: UInt64 = 0
        Scanner(string: s).scanHexInt64(&v)
        self.init(
            red: Double((v >> 16) & 0xff) / 255,
            green: Double((v >> 8) & 0xff) / 255,
            blue: Double(v & 0xff) / 255
        )
    }
}

/// A server address as typed: http and port 32500 unless it says otherwise.
func serverURL(typed: String) -> URL? {
    var s = typed.trimmingCharacters(in: .whitespaces)
    if !s.contains("://") { s = "http://" + s }
    guard var c = URLComponents(string: s), c.host != nil else { return nil }
    if c.port == nil && c.scheme == "http" { c.port = 32500 }
    return c.url
}
