---
id: F011
title: Apple TV app
owner: gh-prim
reviewer: gh-prim
status: approved
intent: ./intent.md
decisions: [0003, 0005, 0008]
---

# F011 — Spec: Apple TV app

## Summary
A native SwiftUI tvOS app that finds the PLAID server on the home network over Bonjour, pairs with the existing TV code flow, and offers the web app's browsing and playback (F008) with the Siri Remote: full-bleed home with hero previews, libraries, search, title, person and collection pages, settings, and a player with skip intro, next-episode card, track memory and quality choice. The server gains a Bonjour announcement and an endpoint giving each movie or episode's intro, recap, credits and preview (ADR 0008). Continue watching is shown in the Apple TV's Top Shelf.

## User journeys
1. **First launch.** The app lists the servers announcing `_plaid._tcp` on the network (or takes an address typed by hand). The member picks one, the TV shows a `XXX-XXX` code, the member claims it on the web `/pair` page with a profile; the TV signs in and lands on the home.
2. **Browsing.** The home opens on the hero (its preview plays muted after a moment) with focus on Play; rows below; a folded side menu on the left (profile, search, home, ☰) opens when focus moves left, with the libraries and settings. Back at a page's root goes to the menu; back again on Home leaves the app.
3. **Watching a show.** On an episode, the member plays; their audio and subtitle choices from last time on this show are applied. Skip intro shows during a known intro. When the credits start, a card offers the next episode with a 10 s countdown; Play now or Cancel. After the last episode, a warm end-of-show screen.
4. **From the Top Shelf.** On the Apple TV home screen, PLAID's Top Shelf lists what the member is watching; picking one opens its page (or plays it).

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F011-R1 | The server announces itself over Bonjour as `_plaid._tcp` on its HTTP port, with TXT `id`, `name`, `version`, `path=/api/v1`. A failure to announce is logged, never fatal. | must |
| F011-R2 | The app lists the servers found on the network, and accepts an address typed by hand. | must |
| F011-R3 | The app pairs with the TV code flow of F003-R20 and keeps the session token in the Keychain per server; a 401 signs it out. | must |
| F011-R4 | Profile picker with PIN, profile switch, sign out, forget server. | must |
| F011-R5 | The UI follows the active profile's language, else the account's, else the TV's (en, fr), for strings, dates, durations and language names. System views follow from the next launch. | must |
| F011-R6 | Home: full-bleed hero (F008 home hero) with its preview (muted by default, mute toggle, "Play previews automatically" per profile on this TV), then the home rows with paging and "See all". | must |
| F011-R7 | A folded side menu (profile, search, home, ☰), vertically centered; opened, full height with Search, Home, the libraries and Settings. It never starts open, including when the app is opened by a link. | must |
| F011-R8 | Libraries: poster grid with offset paging, sort, filters (genre, year, etc. from `/facets`), a letter index that jumps to a letter, and the collections of a movie library. | must |
| F011-R9 | Search with titles and people, recent searches. | must |
| F011-R10 | Title page: hero, Play/Resume (version choice when several files), trailer button opening the YouTube app, watched toggle, seasons with episodes (specials last, mark a season watched), collection row, cast linking to person pages, an "About" section (genres linking to the library by genre, facts, file and languages). | must |
| F011-R11 | Person page (credits by role) and collection page (movies by release). | must |
| F011-R12 | Card context menu (long press): mark watched / unwatched, go to the show. | should |
| F011-R13 | Player: HLS per F006 with the TV's capability profile; resume; progress reported; quality menu; image subtitles burned in via the session update; on error, a retry at a lower quality. | must |
| F011-R14 | The player remembers the audio and subtitle choices per show and profile on this TV and applies them to the next episodes. | should |
| F011-R15 | `GET /api/v1/items/{itemId}/segments` returns the intro, recap, credits and preview of a movie or episode, in time order (ADR 0008): TheIntroDB, cached in `item_segments`; for episodes, end credits detected once per file with FFmpeg at the lowest priority win over TheIntroDB's. An item the profile can't see is a 404; nothing known is an empty list. | must |
| F011-R16 | Skip intro / skip recap button while inside a known intro or recap. | should |
| F011-R17 | Next-episode card (web `pl-next--tv` design): shown when the credits start (or 30 s before the end when none are known), 10 s countdown, Play now (focused) and Cancel; "still watching?" after 3 episodes in a row; end-of-show screen after the last. | must |
| F011-R18 | Settings: previews toggle, language shown, server and account, About with versions and the TMDB / fanart.tv / TheIntroDB attributions. | must |
| F011-R19 | Top Shelf extension: Continue watching from the app group, opening `plaid://item/<id>` or `plaid://play/<id>`. | should |
| F011-R20 | A link to an item that can't be loaded shows an error with Retry, never an endless spinner. | must |

## Design

### Data model
Migrations `0010_segments.sql` and `0011_detected_credits.sql`:

`item_segments` — `item_id TEXT PK → items(id) ON DELETE CASCADE`, `segments TEXT` (JSON array of `{kind,startMs,endMs}`), `fetched_at TEXT`, `credits_ms INTEGER NULL`, `detected_at TEXT NULL`.

### API
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/items/{itemId}/segments` | `ItemSegments { segments: Segment[] }`, `Segment { kind: intro\|recap\|credits\|preview, startMs?, endMs? }` |

Everything else the app uses already exists (F003, F005, F006, F008).

### Behavior per app
- **server**: `internal/discovery` (Bonjour, `libp2p/zeroconf`), `internal/segments` (TheIntroDB client on the metadata HTTP client with host allow-list, 2 req/s; detection queue running one FFmpeg at a time at `nice 19` over the last 10 minutes, 2 fps `blackframe`), `handlers/segments.go`.
- **web**: generated `schema.d.ts` only; no UI change.
- **apple**: new `apple/` project generated by xcodegen from `apple/project.yml`: target `PLAID-tvOS` (`app.plaid.tv`) and the `PLAID-TopShelf` extension, sharing `group.app.plaid.tv`. The Xcode project is not committed (`PLAID_TEAM=… xcodegen generate`).

### UI / Art direction
Matches the web app's V2 look (dark, white focus, no orange accent on TV), Liquid Glass on tvOS 26 for the side menu. Every page has a loading state (spinner or shimmer) and an error state with Retry.

### i18n
`apple/PLAID/Resources/Localizable.xcstrings` and `apple/TopShelf/Localizable.xcstrings`, en source + fr, with plurals for counts. Keys are semantic (`player.next.countdown`, `home.continueWatching`…).

## Non-functional requirements
- Segments: the endpoint answers from the cache; the first TheIntroDB lookup is bounded by the HTTP client timeout; detection never blocks a request.
- Detection costs a few seconds of one core per episode, once, at the lowest priority.
- No token, password or PIN in logs (F003-R23).

## ⚠️ Watch-outs
- [x] TheIntroDB is a new outbound dependency → ADR 0008, degrades to "no intro".
- [x] Credits over pictures may not be detected → the card falls back to 30 s before the end.
- [x] YouTube trailers can't be embedded → the button opens the YouTube app (out of scope in the intent).
- [ ] Signing needs a team and the App Group registered on it; documented in `apple/project.yml`.
- [ ] Not yet covered by CI (no macOS runner); the app is built and checked by hand.

## Acceptance criteria
- [ ] Given the server running on the LAN, when the app starts unpaired, then the server is listed by name without typing an address.
- [ ] Given a code shown on the TV, when it is claimed on the web with a profile, then the TV lands on that profile's home.
- [ ] Given an episode with credits detected at 57:16, when playback reaches 57:16, then the next-episode card shows with Play now focused.
- [ ] Given an item the profile can't see, when its segments are asked, then the answer is a 404.
- [ ] Given an episode never looked at, when its segments are asked, then the answer comes from TheIntroDB (or is empty) and a detection is queued, run once.
- [ ] Given a French profile, when the app opens, then the UI and dates are in French.
- [ ] Given a `plaid://item/<id>` link to a missing item, when the app opens it, then an error with Retry is focused and the side menu stays folded.

## Revisions
- 2026-10-31: written after the code, from what was built (the team chose to prototype first); reviewed and approved by the owner.
