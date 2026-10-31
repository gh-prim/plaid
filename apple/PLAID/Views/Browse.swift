import SwiftUI

/// A person and what they're in on this server, by role (F008 R22).
struct PersonView: View {
    let route: PersonRoute
    @Environment(AppState.self) private var app
    @State private var person: Person?
    @State private var failed = false

    private static let roles = ["cast", "director", "writer", "creator"]

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 40) {
                HStack(spacing: 48) {
                    ZStack {
                        Circle().fill(Theme.surface)
                        Text(verbatim: initials(route.name))
                            .font(Theme.sans(72, .semibold))
                            .foregroundStyle(Theme.textTertiary)
                        RemoteImage(art: person?.portrait, width: 500)
                    }
                    .frame(width: 240, height: 240)
                    .clipShape(Circle())
                    VStack(alignment: .leading, spacing: 12) {
                        Text(verbatim: route.name).font(Theme.sans(72, .bold)).tracking(-2)
                        if let person {
                            Text("person.summary \(Set(person.credits.map(\.item.id)).count) \(app.serverName ?? "PLAID")")
                                .font(Theme.sans(28))
                                .foregroundStyle(Theme.textSecondary)
                        }
                    }
                }
                .padding(.leading, Theme.gutter)
                if let person {
                    ForEach(Self.roles, id: \.self) { role in
                        let items = unique(person.credits.filter { $0.role == role }.map(\.item))
                        if !items.isEmpty {
                            ItemRow(title: LocalizedStringKey(stringLiteral: "person.role." + role), items: items)
                        }
                    }
                } else if failed {
                    ErrorState { Task { await load() } }
                } else {
                    HeroPlaceholder()
                }
            }
            .padding(.vertical, 80)
            .padding(.leading, Theme.rail)
        }
        .scrollClipDisabled()
        .ignoresSafeArea(edges: .horizontal)
        .task { await load() }
    }

    private func load() async {
        guard let client = app.client else { return }
        failed = false
        do { person = try await client.get("/people/\(route.id)") } catch { failed = true }
    }
}

/// A collection: its backdrop and overview, then its movies by release.
struct CollectionView: View {
    let route: CollectionRoute
    @Environment(AppState.self) private var app
    @State private var collection: CollectionDetail?

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 40) {
                ZStack(alignment: .bottomLeading) {
                    Color.clear
                    VStack(alignment: .leading, spacing: 16) {
                        Text(verbatim: collection?.summary.name ?? route.name)
                            .font(Theme.sans(80, .bold)).tracking(-2)
                        if let overview = collection?.summary.overview {
                            Text(verbatim: overview)
                                .font(Theme.sans(28)).lineSpacing(6).lineLimit(3)
                                .foregroundStyle(Theme.textSecondary)
                                .frame(maxWidth: 1200, alignment: .leading)
                        }
                    }
                    .padding(.leading, Theme.gutter)
                }
                .frame(height: 480)
                .background {
                    HeroBackdrop(art: collection?.summary.backdrop)
                        .padding(.leading, -Theme.rail)
                        .padding(.bottom, -Theme.heroFadeBelow)
                }
                if let items = collection?.items {
                    PosterGrid(items: items)
                }
            }
            .padding(.bottom, 80)
            .padding(.leading, Theme.rail)
        }
        .scrollClipDisabled()
        .ignoresSafeArea(edges: [.horizontal, .top])
        .task {
            if let client = app.client { collection = try? await client.get("/collections/\(route.id)") }
        }
    }
}

/// A movie library's collections (F005 R23).
struct CollectionsView: View {
    let library: Library
    @Environment(AppState.self) private var app
    @State private var collections: [CollectionSummary]?

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 32) {
                Text("library.collections")
                    .font(Theme.sans(56, .bold))
                    .padding(.leading, Theme.gutter)
                if let collections {
                    if collections.isEmpty {
                        Text("library.collections.empty").font(Theme.sans(30)).foregroundStyle(Theme.textTertiary)
                            .padding(.leading, Theme.gutter)
                    }
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 240, maximum: 300), spacing: 40)], spacing: 56) {
                        ForEach(collections) { c in
                            NavigationLink(value: CollectionRoute(id: c.id, name: c.name)) {
                                VStack(alignment: .leading, spacing: 10) {
                                    CardArt(art: c.poster, width: 500, fallback: c.name)
                                        .frame(width: 260, height: 390)
                                    Text(verbatim: c.name).font(Theme.sans(24, .semibold)).lineLimit(1)
                                    Text("library.collections.count \(c.movieCount)")
                                        .font(Theme.sans(20)).foregroundStyle(Theme.textTertiary)
                                }
                                .frame(width: 260, alignment: .leading)
                            }
                            .buttonStyle(CardButtonStyle())
                        }
                    }
                    .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                } else {
                    LoadingView().frame(height: 400)
                }
            }
            .padding(.vertical, 60)
            .padding(.leading, Theme.rail)
        }
        .scrollClipDisabled()
        .ignoresSafeArea(edges: .horizontal)
        .task {
            if let client = app.client {
                let list: CollectionList? = try? await client.get("/collections", query: [.init(name: "libraryId", value: library.id)])
                collections = list?.collections ?? []
            }
        }
    }
}

/// Posters in a grid, as in a library.
struct PosterGrid: View {
    let items: [ItemSummary]

    var body: some View {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 240, maximum: 300), spacing: 40)], spacing: 64) {
            ForEach(items) { PosterCard(item: $0) }
        }
        .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
        .padding(.vertical, 20)
        .focusSection()
    }
}

/// "About": genres (links to the library by genre), then the facts and
/// the file's languages, as on the web.
struct AboutSection: View {
    let detail: ItemDetail
    let seasonCount: Int

    private var item: ItemSummary { detail.summary }
    private var meta: ItemMetadata? { detail.metadata }

    var body: some View {
        VStack(alignment: .leading, spacing: 28) {
            Text("item.about.title").font(Theme.sans(34, .semibold))
                .padding(.leading, Theme.gutter)
            if let lib = item.libraryId, let genres = meta?.genres, !genres.isEmpty {
                ScrollView(.horizontal) {
                    HStack(spacing: 20) {
                        ForEach(genres.filter { $0.id > 0 }, id: \.id) { g in
                            NavigationLink(value: GenreRoute(libraryId: lib, genre: g)) { Text(verbatim: g.name) }
                                .buttonStyle(SeasonPillStyle(selected: false))
                        }
                    }
                    .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
                    .padding(.vertical, 12)
                }
                .scrollClipDisabled()
                .focusSection()
            }
            if let overview = meta?.overview {
                Text(verbatim: overview)
                    .font(Theme.sans(26)).lineSpacing(6)
                    .foregroundStyle(Theme.textSecondary)
                    .frame(maxWidth: 1400, alignment: .leading)
                    .padding(.leading, Theme.gutter)
            }
            LazyVGrid(columns: [GridItem(.adaptive(minimum: 520), spacing: 48, alignment: .topLeading)], alignment: .leading, spacing: 24) {
                ForEach(Array(facts.enumerated()), id: \.offset) { _, fact in
                    let (label, value) = fact
                    VStack(alignment: .leading, spacing: 4) {
                        Text(label).font(Theme.sans(22, .semibold)).foregroundStyle(Theme.textTertiary)
                        Text(verbatim: value).font(Theme.sans(26)).lineLimit(3)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            .padding(.leading, Theme.gutter).padding(.trailing, Theme.trailing)
        }
    }

    private var facts: [(LocalizedStringKey, String)] {
        var out: [(LocalizedStringKey, String)] = []
        if let date = item.releaseDate.flatMap(parseDay) {
            out.append((item.kind == .show ? "item.about.firstAired" : "item.about.released", date.formatted(Date.FormatStyle(date: .long, time: .omitted, locale: AppLanguage.locale))))
        }
        if item.kind != .show, let minutes = meta?.runtimeMinutes ?? item.runtimeMinutes {
            out.append(("item.about.runtime", Duration.seconds(minutes * 60).formatted(.units(allowed: [.hours, .minutes], width: .wide).locale(AppLanguage.locale))))
        }
        if let cert = item.certification { out.append(("item.about.rated", cert)) }
        if item.kind == .show {
            if let status = meta?.showStatus { out.append(("item.about.status", localized("item.status." + status))) }
            if seasonCount > 0 { out.append(("item.about.seasons", "\(seasonCount)")) }
        }
        if let studios = meta?.studios, !studios.isEmpty { out.append(("item.about.studios", studios.joined(separator: ", "))) }
        for (role, key) in [("director", "item.about.directedBy"), ("writer", "item.about.writtenBy"), ("creator", "item.about.createdBy")] {
            let names = unique(meta?.credits.filter { $0.role == role }.map(\.name) ?? [])
            if !names.isEmpty { out.append((LocalizedStringKey(key), names.formatted(.list(type: .and).locale(AppLanguage.locale)))) }
        }
        if let file = detail.files.first(where: { $0.missingSince == nil }) {
            let size = ByteCountFormatter.string(fromByteCount: file.size, countStyle: .file)
            let video = [resolutionLabel(file.height), file.hdrFormat.map(hdrLabel), file.videoCodec?.uppercased()].compactMap { $0 }
            out.append(("item.about.file", (video + [size]).joined(separator: " · ")))
            let audio = unique(file.streams.filter { $0.type == "audio" }.map { languageName($0.language) })
            if !audio.isEmpty { out.append(("item.about.audio", audio.joined(separator: ", "))) }
            let subs = unique(file.streams.filter { $0.type == "subtitle" }.map { languageName($0.language) })
            if !subs.isEmpty { out.append(("item.about.subtitles", subs.joined(separator: ", "))) }
        }
        return out
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
