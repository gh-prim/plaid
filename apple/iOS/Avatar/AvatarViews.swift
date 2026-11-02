import PhotosUI
import SwiftUI
import UIKit

/// The server's drawing of avatar options (F009), loaded with the bearer
/// token; the previous drawing stays until the next one is in.
struct RenderedAvatar: View {
    let options: AvatarOptions
    let size: CGFloat
    var color: Int?

    @Environment(AppState.self) private var app
    @State private var image: UIImage?

    var body: some View {
        ZStack {
            if let image {
                Image(uiImage: image).resizable().scaledToFill()
            } else {
                Shimmer()
            }
        }
        .frame(width: size, height: size)
        .task(id: [options.hashValue, color ?? -1]) { await load() }
    }

    private func load() async {
        guard let client = app.client else { return }
        let url = client.avatarRenderURL(options, size: Int(size * UIScreen.main.scale), color: color)
        if let hit = ImageCache.shared.image(for: url) { image = hit; return }
        guard let data = try? await client.data(for: url), let img = UIImage(data: data) else { return }
        ImageCache.shared.store(img, for: url)
        image = img
    }
}

/// The builder's tabs, in the order of board Avatar01.
private let builderTabs = ["head", "hair", "eyes", "glasses", "beard", "background", "eyebrows", "mouth", "nose", "earrings", "freckles"]

private func randomOptions(_ b: AvatarBuilderInfo) -> AvatarOptions {
    var o = b.defaults
    for p in b.parts {
        o[p.name] = p.optional && Double.random(in: 0..<1) < 0.6 ? nil : p.variants.randomElement()
    }
    o.hairColor = b.hairColors.randomElement() ?? o.hairColor
    o.skinColor = b.skinColors.randomElement() ?? o.skinColor
    return o
}

/// The avatar builder (board Avatar01): the avatar in big, Surprise me, a
/// sheet with a tab per part, its choices as drawn tiles, its colors.
struct AvatarBuilderView: View {
    let profile: Profile
    let done: () -> Void

    @Environment(AppState.self) private var app
    @Environment(\.dismiss) private var dismiss
    @State private var info: AvatarBuilderInfo?
    @State private var options: AvatarOptions?
    @State private var color: Int
    @State private var tab = "hair"
    @State private var saving = false
    @State private var failed = false

    init(profile: Profile, done: @escaping () -> Void) {
        self.profile = profile
        self.done = done
        _color = State(initialValue: profile.avatarColor)
    }

    var body: some View {
        ZStack(alignment: .bottom) {
            Theme.bgBase.ignoresSafeArea()
            if let info, let options {
                VStack(spacing: 16) {
                    RenderedAvatar(options: options, size: 170, color: color)
                        .clipShape(Circle())
                        .padding(3)
                        .overlay(Circle().strokeBorder(Theme.textPrimary, lineWidth: 3))
                        .shadow(color: .black.opacity(0.6), radius: 30, y: 20)
                    Button { self.options = randomOptions(info) } label: {
                        Label("avatar.random", systemImage: "arrow.triangle.2.circlepath")
                            .font(Theme.sans(13.5, .semibold))
                    }
                    .buttonStyle(.glass)
                    Spacer()
                }
                .padding(.top, 20)
                sheet(info, options)
            } else {
                VStack(spacing: 12) {
                    ProgressView().tint(Theme.textSecondary)
                    Text("avatar.loading").font(Theme.sans(14, .medium)).foregroundStyle(Theme.textSecondary)
                }
                .frame(maxHeight: .infinity)
            }
        }
        .navigationTitle(Text("avatar.builder.title"))
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .confirmationAction) {
                Button(action: save) {
                    if saving { ProgressView() } else { Text("avatar.ok") }
                }
                .disabled(options == nil || saving)
            }
        }
        .task { await load() }
    }

    private func sheet(_ info: AvatarBuilderInfo, _ options: AvatarOptions) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 4) {
                    ForEach(builderTabs, id: \.self) { name in
                        Button { tab = name } label: {
                            Text(verbatim: tr("avatar.tab.\(name)"))
                                .font(Theme.sans(14, .semibold))
                                .padding(.horizontal, 14).frame(height: 36)
                                .foregroundStyle(tab == name ? Theme.onCream : Theme.textSecondary)
                                .background(Capsule().fill(tab == name ? Theme.cream : .clear))
                        }
                        .buttonStyle(.plain)
                        .accessibilityAddTraits(tab == name ? .isSelected : [])
                    }
                }
            }
            if tab == "background" {
                swatches(Theme.party.indices.map { Theme.party[$0] }, selected: color) { color = $0 }
                    .accessibilityLabel(Text("avatar.background"))
            } else if let part = info.parts.first(where: { $0.name == tab }) {
                ScrollView {
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 92), spacing: 12)], spacing: 12) {
                        if part.optional {
                            tile(selected: options[part.name] == nil) {
                                Text("avatar.none").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                            } action: { self.options?[part.name] = nil }
                        }
                        ForEach(Array(part.variants.enumerated()), id: \.element) { i, v in
                            tile(selected: options[part.name] == v) {
                                RenderedAvatar(options: with(options, part.name, v), size: 96)
                            } action: { self.options?[part.name] = v }
                            .accessibilityLabel(Text("avatar.choice \(i + 1)"))
                        }
                    }
                    .padding(6)
                }
                .frame(maxHeight: 230)
                if let palette = palette(info) {
                    Text(palette.label).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.textSecondary)
                    swatches(palette.colors.map { Color(hex: $0) },
                             selected: palette.colors.firstIndex(of: options[keyPath: palette.key]) ?? -1) {
                        self.options?[keyPath: palette.key] = palette.colors[$0]
                    }
                }
            }
            if failed {
                Text("error.generic").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassEffect(.regular, in: UnevenRoundedRectangle(topLeadingRadius: 34, topTrailingRadius: 34))
        .ignoresSafeArea(edges: .bottom)
    }

    private func palette(_ info: AvatarBuilderInfo) -> (label: LocalizedStringKey, colors: [String], key: WritableKeyPath<AvatarOptions, String>)? {
        switch tab {
        case "hair", "eyebrows", "beard": ("avatar.color", info.hairColors, \.hairColor)
        case "head": ("avatar.skin", info.skinColors, \.skinColor)
        default: nil
        }
    }

    private func tile<Content: View>(selected: Bool, @ViewBuilder content: () -> Content, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            content()
                .frame(maxWidth: .infinity)
                .aspectRatio(1, contentMode: .fit)
                .background(Theme.surface)
                .clipShape(RoundedRectangle(cornerRadius: 22))
                .overlay {
                    if selected { RoundedRectangle(cornerRadius: 24).strokeBorder(Theme.textPrimary, lineWidth: 2).padding(-4) }
                }
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }

    private func swatches(_ colors: [Color], selected: Int, pick: @escaping (Int) -> Void) -> some View {
        HStack {
            ForEach(colors.indices, id: \.self) { i in
                Button { pick(i) } label: {
                    Circle().fill(colors[i]).frame(width: 34, height: 34)
                        .overlay { if i == selected { Circle().strokeBorder(Theme.textPrimary, lineWidth: 2).padding(-5) } }
                        .padding(5)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(Text("avatar.choice \(i + 1)"))
                .accessibilityAddTraits(i == selected ? .isSelected : [])
                if i < colors.count - 1 { Spacer(minLength: 0) }
            }
        }
    }

    private func with(_ o: AvatarOptions, _ part: String, _ v: String) -> AvatarOptions {
        var c = o
        c[part] = v
        return c
    }

    private func load() async {
        guard info == nil, let client = app.client, let b: AvatarBuilderInfo = try? await client.get("/avatar-builder") else { return }
        info = b
        options = profile.avatar?.options ?? randomOptions(b)
    }

    private func save() {
        guard let client = app.client, let options else { return }
        saving = true
        failed = false
        Task {
            defer { saving = false }
            do {
                if color != profile.avatarColor {
                    let _: Profile = try await client.patch("/me/profiles/\(profile.id)", ProfileColorUpdate(avatarColor: color))
                }
                let _: Profile = try await client.put("/me/profiles/\(profile.id)/avatar", AvatarUpdate(kind: "builder", options: options))
                await app.reloadMe()
                done()
            } catch {
                failed = true
            }
        }
    }
}

/// A photo, framed in a circle (board Avatar02): drag to place it, pinch
/// to zoom, Use sends a 512 px JPEG.
struct PhotoCropView: View {
    let profile: Profile
    let image: UIImage
    let done: () -> Void

    @Environment(AppState.self) private var app
    @State private var scale: CGFloat = 1
    @State private var lastScale: CGFloat = 1
    @State private var offset: CGSize = .zero
    @State private var lastOffset: CGSize = .zero
    @State private var saving = false
    @State private var failed = false

    private let side: CGFloat = 300

    var body: some View {
        VStack(spacing: 18) {
            Spacer()
            let cover = side / min(image.size.width, image.size.height)
            Image(uiImage: image)
                .resizable()
                .frame(width: image.size.width * cover * scale, height: image.size.height * cover * scale)
                .offset(clamped(offset, cover))
                .frame(width: side, height: side)
                .clipShape(Circle())
                .overlay(Circle().strokeBorder(Theme.textPrimary, lineWidth: 3))
                .contentShape(Circle())
                .gesture(
                    DragGesture()
                        .onChanged { g in
                            offset = CGSize(width: lastOffset.width + g.translation.width, height: lastOffset.height + g.translation.height)
                        }
                        .onEnded { _ in offset = clamped(offset, cover); lastOffset = offset }
                        .simultaneously(with: MagnifyGesture()
                            .onChanged { g in scale = min(4, max(1, lastScale * g.magnification)) }
                            .onEnded { _ in lastScale = scale; offset = clamped(offset, cover); lastOffset = offset })
                )
            Text("avatar.crop.help").font(Theme.sans(13.5, .medium)).foregroundStyle(Theme.textSecondary)
            if failed {
                Text("avatar.crop.error").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error)
            }
            Spacer()
        }
        .frame(maxWidth: .infinity)
        .background(Theme.bgBase.ignoresSafeArea())
        .navigationTitle(Text("avatar.crop.title"))
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .confirmationAction) {
                Button(action: use) {
                    if saving { ProgressView() } else { Text("avatar.crop.use") }
                }
                .disabled(saving)
            }
        }
    }

    /// Keeps the picture covering the circle.
    private func clamped(_ o: CGSize, _ cover: CGFloat) -> CGSize {
        let w = image.size.width * cover * scale, h = image.size.height * cover * scale
        let mx = max(0, (w - side) / 2), my = max(0, (h - side) / 2)
        return CGSize(width: min(mx, max(-mx, o.width)), height: min(my, max(-my, o.height)))
    }

    private func use() {
        guard let client = app.client else { return }
        let cover = side / min(image.size.width, image.size.height)
        let k = cover * scale
        let o = clamped(offset, cover)
        // The circle's square, in the photo's points.
        let crop = CGRect(x: (image.size.width * k - side) / 2 - o.width, y: (image.size.height * k - side) / 2 - o.height,
                          width: side, height: side)
        let format = UIGraphicsImageRendererFormat()
        format.scale = 1
        let out = UIGraphicsImageRenderer(size: CGSize(width: 512, height: 512), format: format).image { _ in
            let f = 512 / side
            image.draw(in: CGRect(x: -crop.minX * f, y: -crop.minY * f, width: image.size.width * k * f, height: image.size.height * k * f))
        }
        guard let jpeg = out.jpegData(compressionQuality: 0.9) else { failed = true; return }
        saving = true
        failed = false
        Task {
            defer { saving = false }
            do {
                let _: Profile = try await client.upload("PUT", "/me/profiles/\(profile.id)/avatar/photo", data: jpeg, contentType: "image/jpeg")
                await app.reloadMe()
                done()
            } catch {
                failed = true
            }
        }
    }
}

/// The camera, for a photo taken now.
struct CameraPicker: UIViewControllerRepresentable {
    let picked: (UIImage) -> Void
    @Environment(\.dismiss) private var dismiss

    func makeUIViewController(context: Context) -> UIImagePickerController {
        let c = UIImagePickerController()
        c.sourceType = .camera
        c.cameraDevice = .front
        c.delegate = context.coordinator
        return c
    }

    func updateUIViewController(_ c: UIImagePickerController, context: Context) {}

    func makeCoordinator() -> Coordinator { Coordinator(self) }

    final class Coordinator: NSObject, UIImagePickerControllerDelegate, UINavigationControllerDelegate {
        let parent: CameraPicker
        init(_ parent: CameraPicker) { self.parent = parent }

        func imagePickerController(_ picker: UIImagePickerController, didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]) {
            if let img = info[.originalImage] as? UIImage { parent.picked(img) }
            parent.dismiss()
        }

        func imagePickerControllerDidCancel(_ picker: UIImagePickerController) { parent.dismiss() }
    }
}

/// "Your face in PLAID" (board Invite15): build an avatar, a photo, or the
/// initial on a color. Pushes the builder and the crop on its own stack.
struct AvatarChoiceView: View {
    let profile: Profile
    let doneLabel: LocalizedStringKey
    let done: () -> Void

    @Environment(AppState.self) private var app
    @State private var path: [Step] = []
    @State private var color: Int
    @State private var item: PhotosPickerItem?
    @State private var photo: UIImage?
    @State private var choosingPhoto = false
    @State private var camera = false
    @State private var saving = false
    @State private var failed = false

    private enum Step: Hashable { case builder, crop }

    init(profile: Profile, doneLabel: LocalizedStringKey, done: @escaping () -> Void) {
        self.profile = profile
        self.doneLabel = doneLabel
        self.done = done
        _color = State(initialValue: profile.avatarColor)
    }

    var body: some View {
        NavigationStack(path: $path) {
            ScrollView {
                VStack(spacing: 14) {
                    Text("avatar.title").displayTitle(30).multilineTextAlignment(.center).padding(.top, 30)
                    Text("avatar.lead").font(Theme.sans(15, .medium)).foregroundStyle(Theme.textSecondary)
                        .multilineTextAlignment(.center)
                    VStack(spacing: 10) {
                        choice("avatar.build", "avatar.build.help") {
                            Circle().fill(Theme.party[2]).overlay(Image(systemName: "face.smiling").font(.system(size: 22)))
                                .foregroundStyle(Theme.onCream)
                        } action: { path.append(.builder) }
                        Menu {
                            Button { choosingPhoto = true } label: { Label("avatar.photo.library", systemImage: "photo.on.rectangle") }
                            if UIImagePickerController.isSourceTypeAvailable(.camera) {
                                Button { camera = true } label: { Label("avatar.photo.camera", systemImage: "camera") }
                            }
                        } label: {
                            row("avatar.photo", "avatar.photo.help") {
                                Circle().fill(Theme.surfaceHover).overlay(Image(systemName: "camera").font(.system(size: 20)))
                            }
                        }
                        .buttonStyle(.plain)
                        HStack(spacing: 14) {
                            Circle().fill(Theme.party[color % Theme.party.count]).frame(width: 48, height: 48)
                                .overlay(Text(verbatim: String(profile.name.prefix(1)).uppercased())
                                    .font(Theme.display(22)).foregroundStyle(Theme.onCream))
                            VStack(alignment: .leading, spacing: 2) {
                                Text("avatar.initial").font(Theme.sans(16, .semibold))
                                Text("avatar.initial.help").font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
                            }
                            Spacer()
                        }
                        .padding(14)
                        .background(RoundedRectangle(cornerRadius: 18).fill(Theme.surface))
                    }
                    .padding(.top, 10)
                    HStack {
                        ForEach(Theme.party.indices, id: \.self) { i in
                            Button { color = i } label: {
                                Circle().fill(Theme.party[i]).frame(width: 32, height: 32)
                                    .overlay { if i == color { Circle().strokeBorder(Theme.textPrimary, lineWidth: 2).padding(-5) } }
                                    .padding(5)
                            }
                            .buttonStyle(.plain)
                            .accessibilityLabel(Text("avatar.choice \(i + 1)"))
                            .accessibilityAddTraits(i == color ? .isSelected : [])
                            if i < Theme.party.count - 1 { Spacer(minLength: 0) }
                        }
                    }
                    if failed {
                        Text("error.generic").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error)
                    }
                    Button(action: keepInitial) {
                        if saving { ProgressView().tint(Theme.onCream) } else { Text(doneLabel) }
                    }
                    .buttonStyle(CreamButtonStyle())
                    .disabled(saving)
                    .padding(.top, 8)
                }
                .padding(.horizontal, 24)
                .frame(maxWidth: 440)
                .frame(maxWidth: .infinity)
            }
            .background(WarmGround())
            .navigationDestination(for: Step.self) { step in
                switch step {
                case .builder: AvatarBuilderView(profile: profile, done: done)
                case .crop:
                    if let photo { PhotoCropView(profile: profile, image: photo, done: done) }
                }
            }
            .photosPicker(isPresented: $choosingPhoto, selection: $item, matching: .images)
            .fullScreenCover(isPresented: $camera) {
                CameraPicker { img in
                    photo = img
                    path.append(.crop)
                }
                .ignoresSafeArea()
            }
            .onChange(of: item) { _, item in
                guard let item else { return }
                Task {
                    if let data = try? await item.loadTransferable(type: Data.self), let img = UIImage(data: data) {
                        photo = img
                        path.append(.crop)
                    }
                    self.item = nil
                }
            }
        }
    }

    private func row<Art: View>(_ title: LocalizedStringKey, _ help: LocalizedStringKey, @ViewBuilder art: () -> Art) -> some View {
        HStack(spacing: 14) {
            art().frame(width: 48, height: 48)
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(Theme.sans(16, .semibold))
                Text(help).font(Theme.sans(13, .medium)).foregroundStyle(Theme.textSecondary)
            }
            Spacer()
            Image(systemName: "chevron.right").font(.system(size: 14, weight: .semibold)).foregroundStyle(Theme.textTertiary)
        }
        .padding(14)
        .background(RoundedRectangle(cornerRadius: 18).fill(Theme.surface))
        .contentShape(RoundedRectangle(cornerRadius: 18))
    }

    private func choice<Art: View>(_ title: LocalizedStringKey, _ help: LocalizedStringKey, @ViewBuilder art: () -> Art,
                                   action: @escaping () -> Void) -> some View {
        Button(action: action) { row(title, help, art: art) }.buttonStyle(.plain)
    }

    private func keepInitial() {
        guard let client = app.client else { return }
        saving = true
        failed = false
        Task {
            defer { saving = false }
            do {
                if color != profile.avatarColor {
                    let _: Profile = try await client.patch("/me/profiles/\(profile.id)", ProfileColorUpdate(avatarColor: color))
                }
                let _: Profile = try await client.put("/me/profiles/\(profile.id)/avatar", AvatarUpdate(kind: "initial", options: nil))
                await app.reloadMe()
                done()
            } catch {
                failed = true
            }
        }
    }
}
