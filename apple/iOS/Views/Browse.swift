import AVFoundation
import SwiftUI

/// "About" (F008): genres linking to the library by genre, then the facts
/// and the file's languages, as on the web and the TV.
struct AboutSection: View {
    let detail: ItemDetail
    let seasonCount: Int
    @Environment(\.isWide) private var wide

    private var item: ItemSummary { detail.summary }
    private var meta: ItemMetadata? { detail.metadata }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            RowHeader(title: tr("item.about.title"))
            if let lib = item.libraryId, let genres = meta?.genres.filter({ $0.id > 0 }), !genres.isEmpty {
                ScrollView(.horizontal) {
                    HStack(spacing: 8) {
                        ForEach(genres, id: \.id) { g in
                            NavigationLink(value: GenreRoute(libraryId: lib, genre: g)) {
                                Text(verbatim: g.name).font(Theme.sans(13, .semibold))
                            }
                            .buttonStyle(.glass)
                        }
                    }
                }
                .scrollIndicators(.hidden)
            }
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 24, alignment: .topLeading), count: wide ? 2 : 1),
                      alignment: .leading, spacing: 0) {
                ForEach(Array(facts.enumerated()), id: \.offset) { _, fact in
                    HStack(alignment: .firstTextBaseline, spacing: 12) {
                        Text(fact.0).font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary)
                            .frame(width: 110, alignment: .leading)
                        Text(verbatim: fact.1).font(Theme.sans(13, .medium)).lineLimit(3)
                        Spacer(minLength: 0)
                    }
                    .padding(.vertical, 10)
                    .overlay(alignment: .bottom) { Theme.hairline.frame(height: 1) }
                }
            }
        }
    }

    private var facts: [(LocalizedStringKey, String)] {
        var out: [(LocalizedStringKey, String)] = []
        if let original = meta?.originalTitle, original != item.name { out.append(("item.about.originalTitle", original)) }
        if let date = item.releaseDate.flatMap(parseDay) {
            out.append((item.kind == .show ? "item.about.firstAired" : "item.about.released",
                        date.formatted(Date.FormatStyle(date: .long, time: .omitted, locale: AppLanguage.locale))))
        }
        if item.kind != .show, let minutes = meta?.runtimeMinutes ?? item.runtimeMinutes {
            out.append(("item.about.runtime", duration(minutes: minutes)))
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
            let audio = unique(file.streams.filter { $0.type == "audio" }.map { s in
                [languageName(s.language), (s.channels ?? 2) > 2 ? "\((s.channels ?? 2) - 1).1" : nil].compactMap { $0 }.joined(separator: " ")
            })
            if !audio.isEmpty { out.append(("item.about.audio", audio.joined(separator: ", "))) }
            let subs = unique(file.streams.filter { $0.type == "subtitle" }.map { languageName($0.language) })
            if !subs.isEmpty { out.append(("item.about.subtitles", subs.joined(separator: ", "))) }
        }
        return out
    }
}

/// A collection: its backdrop and overview, then its movies by release.
struct CollectionView: View {
    let route: CollectionRoute
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @State private var collection: CollectionDetail?
    @State private var failed = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                ZStack(alignment: .bottomLeading) {
                    RemoteImage(art: collection?.summary.backdrop, width: wide ? 2400 : 1200)
                        .frame(height: wide ? 420 : 360)
                        .mask(LinearGradient(stops: [.init(color: .black, location: 0.45), .init(color: .clear, location: 1)],
                                             startPoint: .top, endPoint: .bottom))
                        .accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: 10) {
                        Text(verbatim: collection?.summary.name ?? route.name).displayTitle(wide ? 44 : 36).lineLimit(2)
                        if let c = collection {
                            Text("library.collections.count \(c.items.count)").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                        }
                    }
                    .padding(.horizontal, gutter)
                }
                if let o = collection?.summary.overview {
                    Text(verbatim: o).font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary).lineSpacing(4)
                        .frame(maxWidth: 640, alignment: .leading)
                        .padding(.horizontal, gutter).padding(.top, 16)
                }
                if let items = collection?.items {
                    PosterGrid(items: items.sorted { ($0.year ?? 0) < ($1.year ?? 0) }).padding(.top, 22)
                } else if failed {
                    ErrorState { failed = false; Task { await load() } }.frame(height: 240)
                } else {
                    LoadingView().frame(height: 240)
                }
            }
            .padding(.bottom, 120)
        }
        .background(alignment: .top) {
            AmbientGlow(art: collection?.summary.backdrop, brightness: -0.6).frame(height: 900).ignoresSafeArea()
        }
        .background(Theme.bgBase)
        .ignoresSafeArea(edges: .top)
        .toolbarBackgroundVisibility(.hidden, for: .navigationBar)
        .navigationBarTitleDisplayMode(.inline)
        .task { await load() }
    }

    private var gutter: CGFloat { wide ? Theme.wideGutter : Theme.gutter }

    private func load() async {
        guard let client = app.client else { return }
        do { collection = try await client.get("/collections/\(route.id)") } catch { failed = true }
    }
}

/// A movie library's collections (F005 R23).
struct CollectionsView: View {
    let library: Library
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @State private var collections: [CollectionSummary]?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                Text("library.collections").displayTitle(36).padding(.top, 10)
                if let collections {
                    if collections.isEmpty {
                        Text("library.collections.empty").font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary)
                            .padding(.top, 30)
                    }
                    let width: CGFloat = wide ? 132 : 108
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: width, maximum: wide ? 160 : 124), spacing: wide ? 18 : 12, alignment: .top)],
                              alignment: .leading, spacing: 20) {
                        ForEach(collections) { c in
                            NavigationLink(value: CollectionRoute(id: c.id, name: c.name)) {
                                VStack(alignment: .leading, spacing: 0) {
                                    ZStack {
                                        Theme.surface
                                        RemoteImage(art: c.poster, width: Int(width * 3))
                                        if c.poster == nil {
                                            Text(verbatim: c.name).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                                                .multilineTextAlignment(.center).padding(8)
                                        }
                                    }
                                    .frame(width: width, height: width * 1.5)
                                    .clipShape(RoundedRectangle(cornerRadius: 9))
                                    Text(verbatim: c.name).font(Theme.sans(13, .semibold)).lineLimit(1).padding(.top, 8)
                                    Text("library.collections.count \(c.movieCount)")
                                        .font(Theme.sans(12, .medium)).foregroundStyle(Theme.textTertiary).padding(.top, 3)
                                }
                                .frame(width: width, alignment: .leading)
                            }
                            .buttonStyle(.plain)
                        }
                    }
                    .padding(.top, 20)
                } else {
                    LoadingView().frame(height: 300)
                }
            }
            .padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
            .padding(.bottom, 120)
        }
        .background(Theme.bgBase)
        .task {
            let list: CollectionList? = try? await app.client?.get("/collections", query: [.init(name: "libraryId", value: library.id)])
            collections = list?.collections ?? []
        }
    }
}

/// Posters in a grid, as in a library.
struct PosterGrid: View {
    let items: [ItemSummary]
    @Environment(\.isWide) private var wide

    var body: some View {
        let width: CGFloat = wide ? 132 : 108
        LazyVGrid(columns: [GridItem(.adaptive(minimum: width, maximum: wide ? 160 : 124), spacing: wide ? 18 : 12, alignment: .top)],
                  alignment: .leading, spacing: 20) {
            ForEach(items) { item in
                NavigationLink(value: item) { PosterCard(item: item, width: width, showTitle: true) }
                    .buttonStyle(.plain)
                    .cardMenu(item)
            }
        }
        .padding(.horizontal, wide ? Theme.wideGutter : Theme.gutter)
    }
}

/// Opens a title's trailer in YouTube: the app when it's installed, the
/// website otherwise.
struct TrailerButton: View {
    let trailer: Trailer
    @Environment(\.openURL) private var openURL

    var body: some View {
        Button {
            guard let app = URL(string: "youtube://www.youtube.com/watch?v=\(trailer.key)"),
                  let web = URL(string: "https://www.youtube.com/watch?v=\(trailer.key)") else { return }
            openURL(app) { opened in if !opened { openURL(web) } }
        } label: {
            Label("item.trailer", systemImage: "play.rectangle").font(Theme.sans(13, .semibold))
        }
        .buttonStyle(.glass)
    }
}

/// The preview over a hero's backdrop: the backdrop shows first, then the
/// scenes fade in once they play, looping. Paused while `active` is false.
struct HeroPreview: View {
    let itemId: String
    let active: Bool

    @Environment(AppState.self) private var app
    @Environment(\.scenePhase) private var scenePhase
    @AppStorage(previewSoundKey) private var soundOn = false
    @State private var looper: PreviewLooper?
    @State private var playing = false

    /// The backdrop shows alone this long before the scenes fade in.
    private static let backdropFirst: Duration = .seconds(2)

    private var running: Bool { active && scenePhase == .active }

    var body: some View {
        ZStack {
            if let looper {
                PlayerLayer(player: looper.player)
                    .opacity(playing ? 1 : 0)
                    .animation(.easeInOut(duration: 1.2), value: playing)
            }
        }
        .task(id: itemId) {
            looper = nil
            playing = false
            guard let client = app.client else { return }
            async let wait: Void = (try? await Task.sleep(for: Self.backdropFirst)) ?? ()
            let file = await PreviewStore.file(for: itemId, client: client)
            await wait
            guard let file, !Task.isCancelled else { return }
            // Previews never stop the viewer's music, unless they unmute them.
            try? AVAudioSession.sharedInstance().setCategory(.ambient)
            let l = PreviewLooper(url: file) { playing = true }
            l.player.isMuted = !soundOn
            looper = l
            if running { l.player.play() }
        }
        .onChange(of: running) { _, on in
            if on { looper?.player.play() } else { looper?.player.pause() }
        }
        .onChange(of: soundOn) { _, on in looper?.player.isMuted = !on }
        .onDisappear { looper?.player.pause() }
        .allowsHitTesting(false)
    }
}

/// The hero preview's sound, on or off for every title.
struct PreviewSoundButton: View {
    @AppStorage(previewSoundKey) private var soundOn = false

    var body: some View {
        Button { soundOn.toggle() } label: {
            Image(systemName: soundOn ? "speaker.wave.2.fill" : "speaker.slash.fill")
                .font(.system(size: 14, weight: .semibold))
                .frame(width: 26, height: 26)
        }
        .buttonStyle(.glass)
        .buttonBorderShape(.circle)
        .accessibilityLabel(Text(soundOn ? "item.preview.mute" : "item.preview.unmute"))
    }
}

extension EnvironmentValues {
    /// Whether hero previews may play: false while the main player is open.
    @Entry var previewsActive = true
}

/// A title the server no longer has, or this profile can't see.
struct Unavailable: View {
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: "eye.slash").font(.system(size: 34, weight: .medium)).foregroundStyle(Theme.textTertiary)
            Text("item.unavailable.title").font(Theme.display(22)).multilineTextAlignment(.center)
            Text("item.unavailable.body").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                .multilineTextAlignment(.center)
            Button("common.back") { dismiss() }.buttonStyle(.glass).padding(.top, 8)
        }
        .padding(32)
        .frame(maxWidth: 420)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Theme.bgBase)
    }
}
