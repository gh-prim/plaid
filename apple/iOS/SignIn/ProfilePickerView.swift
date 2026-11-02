import SwiftUI

/// Who's watching (IPh13 / IPa13): the account's profiles as faces, a lock
/// on those with a PIN, which then asks for it on a keypad (IPh14 / IPa14).
struct ProfilePickerView: View {
    @Environment(AppState.self) private var app
    @Environment(\.isWide) private var wide
    @Environment(\.openURL) private var openURL
    @State private var pinFor: Profile?
    @State private var error: LocalizedStringKey?

    private var profiles: [Profile] { app.me?.profiles ?? [] }

    var body: some View {
        ZStack {
            WarmGround()
            AmbientGlow(art: profiles.lazy.compactMap(\.ambient).first, brightness: -0.55)
                .ignoresSafeArea()
            if let p = pinFor {
                PinPad(profile: p) { pinFor = nil }
                    .transition(.blurReplace)
            } else {
                picker.transition(.blurReplace)
            }
        }
        .animation(.smooth(duration: 0.35), value: pinFor?.id)
    }

    private var picker: some View {
        VStack(spacing: 0) {
            Text("profiles.whoIsWatching").displayTitle(wide ? 44 : 36)
            let columns = Array(repeating: GridItem(.fixed(wide ? 104 : 128), spacing: wide ? 24 : 0), count: wide ? max(1, min(profiles.count, 5)) : 2)
            LazyVGrid(columns: columns, spacing: 28) {
                ForEach(profiles) { p in
                    Button { pick(p) } label: {
                        VStack(spacing: 10) {
                            ProfileFace(profile: p, size: 88)
                                .overlay(alignment: .bottomTrailing) {
                                    if p.hasPin {
                                        Image(systemName: "lock.fill")
                                            .font(.system(size: 10, weight: .bold))
                                            .foregroundStyle(Theme.textPrimary)
                                            .frame(width: 24, height: 24)
                                            .background(Circle().fill(.black.opacity(0.55)))
                                            .accessibilityLabel(Text("profiles.locked"))
                                    }
                                }
                                .padding(4)
                                .overlay {
                                    if p.id == app.me?.activeProfile?.id {
                                        Circle().strokeBorder(Theme.textPrimary, lineWidth: 3)
                                    }
                                }
                            Text(verbatim: p.name)
                                .font(Theme.sans(14, p.id == app.me?.activeProfile?.id ? .semibold : .medium))
                                .foregroundStyle(p.id == app.me?.activeProfile?.id ? Theme.textPrimary : Theme.textSecondary)
                        }
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(.top, 44)
            if let error {
                Text(error).font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error).padding(.top, 20)
            }
            if let base = app.client?.baseURL {
                Button("profiles.manage") { openURL(base.appending(path: "profiles")) }
                    .font(Theme.sans(13, .semibold))
                    .buttonStyle(.glass)
                    .padding(.top, 40)
            }
        }
        .padding(.horizontal, Theme.gutter)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func pick(_ p: Profile) {
        error = nil
        if p.hasPin { pinFor = p; return }
        Task {
            do { try await app.selectProfile(p, pin: nil) } catch { self.error = "error.generic" }
        }
    }
}

/// The four-digit PIN of a profile, on a round glass keypad; sent once the
/// fourth digit is in.
struct PinPad: View {
    let profile: Profile
    let cancel: () -> Void
    @Environment(AppState.self) private var app
    @State private var pin = ""
    @State private var error: LocalizedStringKey?
    @State private var shake = 0
    @State private var busy = false

    var body: some View {
        VStack(spacing: 0) {
            ProfileFace(profile: profile, size: 88)
                .padding(4)
                .overlay(Circle().strokeBorder(Theme.textPrimary, lineWidth: 3))
            Text("profiles.pinFor \(profile.name)").displayTitle(26).padding(.top, 14)
            HStack(spacing: 14) {
                ForEach(0..<4, id: \.self) { i in
                    Circle()
                        .fill(i < pin.count ? Theme.textPrimary : .clear)
                        .strokeBorder(Theme.textPrimary.opacity(i < pin.count ? 1 : 0.5), lineWidth: 1.5)
                        .frame(width: 12, height: 12)
                }
            }
            .padding(.top, 16)
            .keyframeAnimator(initialValue: 0.0, trigger: shake) { v, x in v.offset(x: x) } keyframes: { _ in
                KeyframeTrack { for x in [-10.0, 10, -7, 7, -3, 0] { LinearKeyframe(x, duration: 0.06) } }
            }
            .accessibilityElement()
            .accessibilityLabel(Text("profiles.pin"))
            .accessibilityValue(Text(verbatim: "\(pin.count)/4"))
            Text(error ?? " ").font(Theme.sans(13, .semibold)).foregroundStyle(Theme.error).frame(height: 20).padding(.top, 10)

            Grid(horizontalSpacing: 20, verticalSpacing: 14) {
                ForEach([[1, 2, 3], [4, 5, 6], [7, 8, 9]], id: \.self) { row in
                    GridRow { ForEach(row, id: \.self) { key($0) } }
                }
                GridRow {
                    Color.clear.frame(width: 64, height: 64)
                    key(0)
                    Button { if !pin.isEmpty { pin.removeLast() } } label: {
                        Image(systemName: "delete.left").font(.system(size: 20, weight: .medium))
                            .foregroundStyle(Theme.textSecondary)
                            .frame(width: 64, height: 64)
                    }
                    .accessibilityLabel(Text("profiles.pin.delete"))
                }
            }
            .padding(.top, 8)
            .disabled(busy)

            Button("common.cancel", action: cancel)
                .font(Theme.sans(14, .semibold))
                .foregroundStyle(Theme.textSecondary)
                .padding(.top, 22)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .sensoryFeedback(.error, trigger: shake)
    }

    private func key(_ n: Int) -> some View {
        Button {
            guard pin.count < 4 else { return }
            error = nil
            pin.append(String(n))
            if pin.count == 4 { submit() }
        } label: {
            Text(verbatim: "\(n)").font(Theme.display(26, .semibold)).frame(width: 64, height: 64)
        }
        .buttonStyle(.glass)
        .buttonBorderShape(.circle)
    }

    private func submit() {
        busy = true
        Task {
            defer { busy = false }
            do {
                try await app.selectProfile(profile, pin: pin)
            } catch let e as APIError {
                error = e.code.contains("lock") ? "profiles.pinLocked" : (e.code.contains("pin") ? "profiles.wrongPin" : "error.generic")
                pin = ""
                shake += 1
            } catch {
                self.error = "error.generic"
                pin = ""
            }
        }
    }
}
