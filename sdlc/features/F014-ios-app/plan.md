---
id: F014
title: "iPhone and iPad app"
owner: gh-prim
reviewer: gh-prim
status: in-progress
spec: ./spec.md
branch: feat/F014-ios-app
---

# F014 — Plan: iPhone and iPad app

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `apple/project.yml` | edit | `PLAID-iOS` target (iOS 26, iPhone + iPad, Info.plist keys, fonts); `Common` added to the TV target |
| `apple/Common/API/*`, `Common/App/AppState.swift`, `Common/App/AppLanguage.swift`, `Common/RemoteImage.swift` | move from `PLAID/` | Shared by the TV and the phone; `AppState` gets the device type per platform and the splash on tvOS only |
| `apple/Common/API/Models.swift` | edit | `LoginRequest`, `ClaimPairingRequest`, `SessionInfo`, `ChangePasswordRequest`, `Profile.ambient`, `Library.scan` |
| `apple/Common/Player/PlayerModel.swift` | split from `PLAID/Player/PlayerView.swift` | The session logic for both; `session`, `imageSubtitles`, `switchAudio`, `burnSubtitles`, `setQuality` opened up; tvOS-only metadata behind `#if os(tvOS)` |
| `apple/Common/Player/Preview.swift` | split from `PLAID/Player/HeroPreview.swift` | `PreviewStore`, `PreviewLooper`, `PlayerLayer` |
| `apple/Common/Helpers.swift` | new | Routes, `PlayAction`, `tr`, `localized`, `languageName`, `episodeCode`, `serverURL(typed:)`, `Color(hex:)`, moved from the TV views |
| `apple/Common/Resources/` | move + edit | Strings catalog (new keys en + fr), fonts, Bricolage Grotesque converted from `web/public/fonts` |
| `apple/PLAID/**` | edit | Use the shared code; `SignInViews.swift` shows the pairing QR code |
| `apple/iOS/App/*` | new | `PLAIDApp`, `RootView`, debug-only `DevSession` |
| `apple/iOS/SignIn/*` | new | Server picker and code, sign in, profiles and PIN pad (R1–R3) |
| `apple/iOS/Views/*` | new | Theme, tabs, home, grid, search, title, person, collections, About, settings, pair a TV, QR scanner (R4–R11, R16–R18, R20) |
| `apple/iOS/Player/*` | new | Player screen, controls, tracks panel, next episode, AirPlay remote, PiP, Now Playing, orientation (R12–R15); `PGS.swift`, the PGS decoder and overlay ported from `web/src/player/pgs.ts` (R21) |
| `apple/iOS/Resources/*` | new | App icon, launch color, `InfoPlist.xcstrings` |
| `apple/scripts/make-icons.swift` | edit | Also draws the iOS icon |
| `web/src/pages/Pair.tsx`, `Pair.test.tsx` | edit | `?code=` prefill and its test (R19) |
| `.claude/launch.json` | edit | `server-ios`, a local server for simulator runs |

## Steps, in order
1. [x] Move the shared code to `apple/Common/` with `git mv`; `PLAID-iOS` target; both targets build.
2. [x] Theme from the boards (colors, Bricolage, glass buttons, logo and wordmark drawn in SwiftUI); icon and launch color.
3. [x] Sign-in: server picker with Bonjour, getplaid.direct code boxes, address field, sign in, profiles, PIN pad (R1–R3).
4. [x] Tabs (`TabView`, `Tab(role: .search)`, `.tabBarOnly` so the iPad gets the floating bar), home with heroes and rows (R4, R5).
5. [x] Library grid, search, title page, person page (R7, R9–R11 first pass).
6. [x] Player on `PlayerModel`: video layer, glass controls, scrubber, tracks panel, speed and quality, skip and next episode, AirPlay remote, PiP, Now Playing, landscape lock on iPhone (R12–R15).
7. [x] Parity with TV and web: Settings with About and attributions, change password; collections, trailer, version, About, season watched, unavailable; previews; all grid filters, letter index with `offset` paging, empty states; search scopes (R5–R11, R16).
8. [x] Pair a TV with the QR code: TV screen QR (`CIFilter.qrCodeGenerator`), app scanner (`DataScannerViewController`), `plaid://pair` link, web prefill (R17–R20).
9. [x] Strings en + fr, permission prompts; simulator runs on iPhone 17 Pro and iPad Pro 11" against `server-ios`.
9b. [x] PGS drawn by the app (R21): `separate: [pgs]` on iOS only, `PlaybackSubtitle.url`, `PlayerModel.drawnSubtitles` / `drawnIndex` / `showDrawn`, `PGSOverlay` over the video; the search page keeps its navigation bar on iPad.
10. [ ] `make test` (done, green), then try on a real iPhone and a real Apple TV (QR scan, AirPlay, PiP, previews of real titles).

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Moving files breaks the TV app | Medium | Both targets built after each move; the TV runs the same functions, only their files moved |
| Taps on the video's UIKit layer never reach SwiftUI | Seen once | The video layer ignores hits; a clear SwiftUI layer above it takes the taps |
| `ignoresSafeArea` hides the status bar's height from the home's top bar | Seen once | The root measures it and passes `topInset` down |
| Big fixed-size background glows widen the sign-in page | Seen once | Glows in an overlay, clipped |
| The QR code holds a LAN address unreachable from another network | Low | The typed code still works |
| Untested on hardware: camera, AirPlay, PiP, lock screen | Medium | Step 10 on a real iPhone before merging |

**Riskiest step:** step 6, the player: the TV relied on AVPlayerViewController, the phone draws its own controls over the shared `PlayerModel`.

## Rejected options
- AVPlayerViewController on iOS: its controls don't match the boards (IPh07–IPh10) and its subtitle menu can't list the server's image subtitles or audio tracks.
- A separate Xcode project for iOS: two copies of the API and player code.
- The sidebar on iPad (`.sidebarAdaptable`): the board shows the floating tab bar at the top.
- A `plaid://` QR code on the TV: a phone without the app couldn't do anything with it; the web URL works for both.
- Administration in the app: the owner's call, the web does it.

## Expected evidence
- [x] iOS and tvOS targets build with no warnings in the new code
- [x] Web: `make lint-web`, `make test-web` green (235 tests, `?code=` prefill test added)
- [x] `make test` green (2026-11-02)
- [x] Simulator screenshots compared to the boards: sign-in (IPh11–IPh15), profiles and PIN (IPh13–IPh14), home iPhone and iPad (IPh01, IPa01), grid (IPh02), search (IPh03), title (IPh04, IPa04), show (IPh05), player and tracks panel (IPh07, IPh08, IPa07, IPa08), next episode (IPh09)
- [x] Manual: French subtitles shown and audio tracks listed on a two-track MKV; a test TV paired on a profile through `plaid://pair`; the TV's QR code decoded back to its `/pair?code=` URL
- [x] Manual: an MKV with a hand-made PGS track plays as `direct_stream` / original, the track is fetched as `.sup` and drawn over the picture, no switch to transcode
- [ ] Real iPhone: QR scan, AirPlay to an Apple TV, Picture in Picture, lock screen controls, a preview playing

## Log
- 2026-11-02: the code was written first, at the owner's request, then this intent, spec and plan; all ship in one PR.
- 2026-11-02: the owner's TestFlight build converted to 1080p when Blu-ray subtitles were on (server log: `subtitle_burn_in`); iOS now draws PGS itself (R21). The search field was missing on iPad (hidden navigation bar).
- 2026-11-02: administration dropped from the app; QR code pairing added (TV, app, web) at the owner's suggestion.
