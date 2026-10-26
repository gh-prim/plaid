# PLAID — Project plan

> Plaid & chill. Your cinema under the blanket.

A self-hosted alternative to Plex: a server that indexes your movies/shows, streams them everywhere, and makes watching **together** (even remotely) native rather than hacked together.

---

## 1. The applications

| # | App | Proposed stack | Why |
|---|-----|----------------|-----|
| 1 | **Server** | Go + FFmpeg + SQLite | Single binary, easy cross-compile (NAS, Raspberry, Docker), low RAM, very good at concurrency (streams + WebSocket). |
| 2 | **Web** (served by the server) | React + Vite + TypeScript, embedded in the Go binary | Admin + player. Zero extra install: `http://server:32500`. |
| 3 | **Apple**: iOS, iPadOS, tvOS (+ macOS for free) | Multiplatform SwiftUI, a single Xcode project | One codebase, views adapted per platform. macOS via the same SwiftUI target (no dedicated code). |
| 4 | **Android** + **Android TV** | Kotlin, Jetpack Compose + Compose for TV, Media3/ExoPlayer | A single Gradle project, shared `core` modules, separate mobile and TV UIs. |

**Language:** all development is in English (code, commits, API). Every app is internationalized (i18n) from day one: English as the source locale, French as the first translation.

**Shared contract:** `api/openapi.yaml` is the source of truth. We generate the TypeScript, Swift and Kotlin clients from this file, so no app reinvents the API.

### Monorepo

```
plaid/
├── api/            # openapi.yaml + WebSocket event schema
├── server/         # Go
├── web/            # React (build copied into server/ via go:embed)
├── apple/          # Xcode, SwiftUI (iOS/iPadOS/tvOS/macOS)
├── android/        # Gradle (mobile + tv)
├── design/         # tokens (colors, type, spacing), logo, patterns
└── docs/
```

---

## 2. Features

### Plex parity (the foundation)
- **Libraries**: movies, shows (seasons/episodes). Later: music, photos.
- **Scanning** of folders + file watching (fsnotify), name parsing (`Movie (2019).mkv`, `S01E02`).
- **Metadata**: TMDB (posters, synopses, cast, trailers), OpenSubtitles subtitles.
- **Playback**: Direct Play → Direct Stream (remux) → HLS Transcode decision.
- FFmpeg **transcoding** with hardware acceleration (VAAPI, Intel QSV, NVENC, VideoToolbox).
- **Subtitles**: SRT/VTT as text, ASS/PGS burned in if the client can't render them.
- Resume playback, "Continue watching", watched/unwatched, sync across devices.
- Search, collections, filters, sorting.
- Timeline preview thumbnails (trickplay), credits / intro detection ("Skip").
- Users, profiles, parental controls.

### What we do better

**🛋️ Plaid Party (multi-watch)** — the flagship feature
- A room = one media item + a host + guests (server accounts or guests via link).
- Play/pause/seek/speed sync driven by the server over WebSocket, server clock as reference, drift correction (slight speed-up/slow-down rather than jumps).
- Each participant gets their own stream adapted to their device and connection (one in Direct Play on Apple TV, another in 720p transcode on mobile).
- Reactions, chat, "bathroom break" (anyone can request a pause), "everyone controls" or "host only controls" mode.
- Cross-platform from the start: web ↔ iOS ↔ Android TV in the same room.

**📚 Library sharing**
- Share per library (or per collection) with friends, fine-grained rights: read-only, downloads allowed, max quality.
- Invitations by link, without a mandatory central account.
- Per-friend quotas for simultaneous streams and bandwidth.

---

## 3. The architecture call to make early: remote access

Plex relies on plex.tv (accounts, relay, discovery). For a Party between friends at a distance, the server has to be reachable:

1. **v1 — Pure self-host**: local accounts, access via public IP / reverse proxy / Tailscale / Cloudflare Tunnel. Simple, zero infra on our side.
2. **v2 — Plaid Hub (optional)**: a small central service for federated identity (one account for several servers), discovery of shared servers and a last-resort relay.

From v1 we design accounts and invitations so the Hub can plug in later without a painful migration.

---

## 4. Roadmap

### Phase 0 — Foundations (≈ 1 week)
- Monorepo, CI (lint + tests per app), Docker.
- `openapi.yaml` v0 + Party WebSocket event schema.
- Design tokens + logo + key screens (home, movie page, player, Party room).

### Phase 1 — Server MVP
- Config, SQLite + migrations, auth (admin account + JWT/sessions).
- Movie/show libraries: scanning, parsing, TMDB metadata, cached images.
- Streaming: Direct Play (range requests) then HLS transcoded via FFmpeg.
- **End of phase**: we play a movie from `curl`/VLC through the API.

### Phase 2 — Web app
- Onboarding (first launch: create the admin, add folders).
- Home, libraries, detail page, search, player (hls.js), resume.
- Admin: users, libraries, transcoding, logs.

### Phase 3 — Multi-user, sharing, Plaid Party
- Users/profiles, per-library sharing, invitations.
- Plaid Party on server + web. We validate all the sync mechanics here before porting them elsewhere.

### Phase 4 — Apple (iOS, iPadOS, tvOS)
- Player: AVPlayer (native HLS); KSPlayer/VLCKit as an option to direct-play more codecs.
- Focus navigation on tvOS, Party, SharePlay as a bonus.

### Phase 5 — Android + Android TV
- Media3/ExoPlayer, Compose for TV, Party.

### Phase 6 — The rest
- Offline downloads (mobile), Plaid Hub, music, photos, Live TV/DVR, advanced Chromecast/AirPlay.

---

## 5. Art direction

- **Concept**: the couch blanket, movie night, the cocoon. Warm, soft, never "cold tech".
- **Dark mode by default** (darkened theater), "Sunday morning" light mode.
- **Palette**: deep midnight blue (background), tartan burgundy, amber / lamplight, woolly cream (text), fir green as secondary accent.
- Very subtle **tartan pattern** as texture (headers, empty states, loading), generous rounded corners, soft shadows.
- **Tone**: "Under the plaid" ("Sous le plaid") for now playing, "Plaid Party", empty states like "Nothing to watch? Add a folder and come back to snuggle in." ("Rien à regarder ? Ajoute un dossier et reviens te caler.").

---

## 6. Open decisions
- [ ] Go for the server (recommended) or TypeScript/Bun (to share the language with the web)?
- [ ] MVP limited to movies + shows? (recommended)
- [ ] Remote access v1: do we document Tailscale/Cloudflare Tunnel, or build a tunnel in?
- [ ] License: open source (AGPL/MIT) or closed?
