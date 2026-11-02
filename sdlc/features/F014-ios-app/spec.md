---
id: F014
title: "iPhone and iPad app"
owner: gh-prim
reviewer: gh-prim
status: approved
intent: ./intent.md
decisions: []
---

# F014 — Spec: iPhone and iPad app

## Summary
A SwiftUI app for iPhone and iPad, drawn after step 17 of the "PLAID Design" canvas (page V2) in Liquid Glass. It reuses what the Apple TV app (F011) already does against the API (session, profiles, playback sessions, progress, up next, segments, audio and subtitle memory) by moving that code to `apple/Common/`, and brings every viewer feature of the TV and the web. Pairing a TV gains a QR code: the TV shows `…/pair?code=…`, which the app scans, or the phone's camera opens on the web with the code typed in.

## User journeys
1. **First launch.** Alex opens PLAID on the iPhone. "Looking for your server" lists the servers seen on the network (Bonjour); Alex taps "Continue with Grenier". Away from home, "Enter the server code" takes the six getplaid.direct characters (or an address), shows the server found, then "Continue".
2. **Sign in.** The server's name large, username and password in glass fields, "Sign in". Then "Who's watching?": the profiles as faces; Mia's carries a lock, so tapping it opens a round keypad; the fourth digit signs in, a wrong PIN shakes the dots.
3. **Browse.** Home: a hero paging through a few titles (its scenes loop over the backdrop once cut), Play or Resume, then the server's rows. Movies and Shows: a poster grid with Genre, Decade, Watched and Picture chips, a sort menu, Collections, and letters down the side when sorted by title. Search lives in the tab bar: best result, people, the rest, scopes, and genres to browse.
4. **A title.** Backdrop full bleed, title, facts and badges (HD, HDR, 5.1, subtitle languages), Play, watched, Trailer, Version; synopsis, a show's seasons and episodes, cast and crew, the collection, About, more like this.
5. **Play.** The phone turns to landscape. Glass controls fade after 4 s of play: close, Picture in Picture, AirPlay, title; −10 s, play/pause, +10 s; speed and quality, subtitles and audio; the scrubber. Skip intro, then the next episode over the credits with a 10 s ring. With AirPlay, the phone becomes a remote for the TV.
6. **Pair a TV.** Settings ▸ Pair a TV: "Scan the TV's QR code" opens the camera; the code fills in; Alex picks the profile it opens on (its PIN if it has one) and taps Pair. The TV drawn on the page lights up: "Living room is connected."

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F014-R1 | Find a server: the `_plaid._tcp` servers on the network, one picked, "Continue with {name}"; or the six-character getplaid.direct code in six boxes (alphabet `a–z2–7`), the server it names, or "No server with this code"; or a typed address (port 32500 by default). | must |
| F014-R2 | Sign in with username and password (`POST /auth/login`, device type `ios` or `ipados`); `invalid_credentials` and `too_many_attempts` each have their message. The token is kept in the Keychain per server. | must |
| F014-R3 | Who's watching: faces in the profiles' party colors, a lock on PIN profiles, a round keypad sending the PIN at the fourth digit; wrong PIN and locked profile have their message. "Manage profiles" opens the web. | must |
| F014-R4 | Tabs Home, Movies, Shows and Search in the Liquid Glass tab bar, floating at the top on iPad; the search field is on the Search page itself. The app follows the profile's language. | must |
| F014-R5 | Home: up to four heroes (the ambient title first) with page dots, the server's rows ("Continue watching" as stills with progress, others as posters, "See all" when offered). Empty server: "Nothing to watch yet", or while a scan runs "Your films are on their way" with what was found, refreshed every 5 s. | must |
| F014-R6 | Hero previews: on the home hero and a title's page, `GET /items/{id}/preview` (202 is asked again) loops over the backdrop after 2 s, muted by default, with a sound button; paused while the player is open or the app is in the background; off with Settings ▸ "Play previews automatically". | should |
| F014-R7 | Library grid: sort (title, added, released, rating, watched, random, order), several genres at once, decade, watched ("New episodes" for shows), resolution and HDR, the count; "Clear filters" when nothing matches; the library picker when there are several; "Collections" when the library has some. | must |
| F014-R8 | Letter index down the right side of a title-sorted, unfiltered grid: a tap or a slide jumps to the letter, loading the pages up to it (`offset`). | should |
| F014-R9 | Search: best result with the typed part underlined, people, the other titles, scopes (All, Movies, Shows, Episodes, People) when more than one found something, recent searches, genres to browse, and a hint when nothing is found. | must |
| F014-R10 | Title page: facts and file badges, Play/Resume (a show's next episode), watched, Trailer (YouTube app, else the website), Version when a movie has several files (the server picks by default), share, start over; synopsis; seasons menu with "Mark season as watched"; episodes (list on iPhone, grid on iPad); cast and crew, one card per person; the collection row; About (genres, original title, release, runtime, rating, status, seasons, studios, directed/written/created by, file, audio, subtitles); more like this. A title the profile can't see shows "This title isn't available". | must |
| F014-R11 | Person page: portrait or initials, main role and titles here, their titles. Collection page: backdrop, overview, movies by year. | must |
| F014-R12 | Player: the TV's `PlayerModel` (session, progress every 10 s, stop on leave, up next, segments, audio and subtitle memory per profile, image subtitles burned in, still watching after three episodes, end of show). Landscape on iPhone; glass controls hidden after 4 s of play; double tap fills the screen; scrubber with time left. | must |
| F014-R13 | Subtitles and audio panel: text subtitles of the stream (off, each language, SDH), Blu-ray (PGS) subtitles, other image subtitles (VobSub, burned in by the server), the file's audio tracks (the server switches). Speed (0.5× to 2×) and quality menu. | must |
| F014-R21 | PGS subtitles are drawn by the app, as on the web: the profile declares `subtitles.separate: [pgs]`, the server lists those tracks as `external` with a `.sup` link and never burns them in, so the picture stays the original. The app fetches the part around the position first (`from`, `X-Subtitle-Until`, asked again every 15 s), then the whole track, decodes it (BT.709 palettes, RLE objects) and draws the set showing over the picture, fitted or filled with it, raised while the controls show. A drawn track counts as the profile's subtitle choice. Apple TV keeps burning them in. | must |
| F014-R14 | Skip intro / recap during them; next episode card from the credits with its 10 s ring, "Play now" and "Watch the credits". | must |
| F014-R15 | AirPlay picker; while the picture is on another screen, a remote with the still, where it plays, scrubber, transport, subtitles and "Stop AirPlay". Picture in Picture from a button and on its own when the app goes to the background. Sound and controls on the lock screen. | must |
| F014-R16 | Settings (the profile's face): profile, switch profile, manage profiles; autoplay next episode and previews; change password (`POST /me/password`, other sessions end); Pair a TV; sign out; server and change server; About PLAID with app and server versions and each metadata provider's notice (F005 R37). | must |
| F014-R17 | Pair a TV (`POST /pairing/claim`): the code in six boxes filtered to the TV's alphabet, a drawn TV echoing it, the profile it opens on, its PIN, Pair; `pairing_not_found`, `pairing_expired`, `invalid_pin`, `profile_locked` on the right field; a "connected" screen with "Pair another TV". | must |
| F014-R18 | QR code: the Apple TV's pairing screen shows a QR code of `{server}/pair?code={code}`. The app's "Scan the TV's QR code" (camera, VisionKit) reads it, or a bare code, and fills the code in. `plaid://pair?code=…` opens Pair a TV with the code. | must |
| F014-R19 | Web: `/pair?code=…` starts with that code typed (uppercased, filtered, 6 at most), so a phone without the app pairs from its camera. Sign-in keeps the query. | must |
| F014-R20 | Links: `plaid://item/{id}` opens a title, `plaid://play/{id}` plays it. | could |

## Design

### Data model
None on the server. On the device: the server URL in `UserDefaults`, the token in the Keychain per server, per-profile autoplay, previews, preview sound and track memory in `UserDefaults` (as on the TV).

### API
None new. Used: `/server`, `/auth/login`, `/auth/logout`, `/me`, `/session/profile`, `/me/password`, `/pairing/claim`, `/libraries`, `/libraries/{id}/items` (filters, `offset`, `total`), `/libraries/{id}/facets`, `/home`, `/home/rows/{id}`, `/search`, `/items/{id}`, `/items/{id}/up-next`, `/items/{id}/segments`, `/items/{id}/watched`, `/items/{id}/preview`, `/items/{id}/playback`, `/playback/{id}` (PATCH, DELETE, progress), `/collections`, `/collections/{id}`, `/people/{id}`, `/images/{id}`; getplaid.direct's `/api/v1/servers/{code}/connections`.

### Behavior per app
- **server**: unchanged.
- **apple**: new target `PLAID-iOS` (iOS 26, iPhone and iPad) in `apple/project.yml`, sources `apple/iOS/` and `apple/Common/`. `Common/` takes from the TV app the API client and models, discovery, getplaid.direct, `AppState` (device type by platform, splash only on tvOS), `AppLanguage`, `RemoteImage`, `PlayerModel`, the preview cache and looper, helpers, strings and fonts. The TV app shows the pairing QR code.
- **web**: `pages/Pair.tsx` reads `?code=`.
- **android**: out of scope.

### UI / Art direction
Canvas "PLAID Design", page V2, row "Step 17. iOS and iPadOS app": IPh01–IPh06 and IPa01–IPa06 browsing, IPh07–IPh10 and IPa07–IPa10 the player, IPh11–IPh16 and IPa11–IPa15 sign-in. Ground `#0b0b0c`, cream `#f4f0e9`, Bricolage Grotesque titles (bundled from the web's font, renamed to its family), Instrument Sans text, system Liquid Glass (`glassEffect`, `.glass` buttons, the system tab bar). Deliberate additions to the boards: Settings, Pair a TV and its QR code, the address field under the server code, the letter index, Collections, About, Trailer and Version.

### i18n
All strings in `apple/Common/Resources/Localizable.xcstrings`, shared with the TV app, en and fr. New key groups: `nav.*` (movies, shows, profile), `kind.*`, `server.*` (find, code, continue, away), `signIn.*`, `profiles.*` (whoIsWatching, pinFor, locked, manage, pinLocked), `card.*`, `item.*` (resumeLeft, years, badge, similar, episodeCount, about.originalTitle/information/video, version, versions.auto, collection.thisMovie, unavailable), `library.*` (filters, decade, letters, picture), `search.*` (title, best, browse, alsoInLibrary, scope, emptyHint), `player.*` (controls, airplay, speed, subtitles, next.in, watchCredits), `home.empty.*`, `settings.*` (account, server, changePassword, signOut.confirm), `password.*`, `pairing.*` (as the web's, plus scan) and `pair.scan` on the TV. Permission prompts in `apple/iOS/Resources/InfoPlist.xcstrings`. Plurals through the catalog's plural variations; dates, durations and numbers through formatters in the app's locale.

## Non-functional requirements
- Accessibility: real buttons and fields everywhere; icon buttons labelled; the PIN and code boxes are one field each; the scrubber and letter index are adjustable; selected rows carry the selected trait.
- Loading: shimmers for rows, posters and images, a spinner for the player and actions, never a blank frame.
- Motion: system animations; the hero's Ken Burns is left out.
- The app keeps playing sound with the screen locked (`audio` background mode).

## ⚠️ Watch-outs
- [x] The TV app's code moves to `apple/Common/`: its behavior must not change (both targets build; the TV app runs the same code).
- [x] `AVPlayerItem.externalMetadata` exists on tvOS only: kept behind `#if os(tvOS)`; iOS fills Now Playing itself.
- [x] Previews for very short files answer 404: the backdrop stays.
- [x] The QR code holds the server's address as the TV knows it: a phone on another network can't reach a LAN address. The code under it still works.
- [x] With AirPlay, the picture goes to the TV but a drawn PGS track stays on the phone. Accepted for now.
- [x] The system search field (`.searchable`, `Tab(role: .search)`) lands in the tab bar, the navigation bar or nowhere depending on the iOS release and the device: it went missing on a real iPhone. Search is a plain tab with the page's own glass field.
- [x] Debug builds only: `PLAID_DEV_SERVER` and `PLAID_DEV_TOKEN` sign the app in on a simulator.

## Acceptance criteria
- [ ] Given a server on the network, when the app starts, then it's listed and "Continue with {name}" signs in after the password.
- [ ] Given the code `xk2b7q` that names no server, then "No server with this code." shows under red boxes.
- [ ] Given a profile with a PIN, when I type a wrong PIN, then "Wrong PIN." shows and the dots empty.
- [ ] Given a movie with French and English subtitles and two audio tracks, when I open the panel, then both lists show and picking French shows French subtitles.
- [ ] Given an episode with a next one, when the credits start, then the next episode card counts down from 10 and "Watch the credits" keeps this one.
- [ ] Given a title-sorted movie library, then letters show on the right and tapping one scrolls to its first title.
- [ ] Given a TV waiting with `CR8-F76`, when I open `plaid://pair?code=CR8-F76`, pick a profile without a PIN and tap Pair, then the TV gets its session on that profile and the page says "{device} is connected."
- [ ] Given `/pair?code=k7p4qx` on the web, then the code field holds `K7P4QX` and Pair is enabled.
- [ ] Given Settings, then About PLAID shows both versions and the TMDB notice.
- [ ] Given an MKV with a PGS track, when I pick it in the panel, then it shows over the picture and the session stays `direct_stream` with "Original" in the quality menu.
- [ ] Given the Search tab, on iPhone (iOS 26 and 27) and iPad, then the search field shows at the top of the page.

## Revisions
- 2026-11-02: R21 (PGS drawn by the app) after the owner found a 1080p conversion forced by image subtitles; the iPad search field fixed.
- 2026-11-02: written after the code, at the owner's request, and reviewed with it in one PR. Administration was dropped from the iOS app at the owner's request; the QR code pairing was added at their suggestion.
