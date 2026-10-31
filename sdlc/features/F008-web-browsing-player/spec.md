---
id: F008
title: Web — browsing and player
owner: gh-prim
reviewer: TBD
status: approved
intent: ./intent.md
decisions: [0002, 0005]
---

# F008 — Spec: Web — browsing and player

## Summary
F008 turns PLAID's web app into the place where you watch. After the profile picker ("Who's under the plaid?"), the **home** opens on shelves under the ambient backdrop of what you were last watching: Continue watching, Recently added per library, then a few genres. Each **library** is a poster grid that stays smooth with thousands of titles, with sorting and filters. **Movie and show pages** sit on the title's backdrop and ambient colors, with the cast, the trailer, seasons and episodes. **Search** answers while you type, across libraries and people. The **player** plays what F006 decides (Direct Play in `<video>`, HLS through the browser or hls.js), resumes where you stopped, switches quality, audio and subtitles, has keyboard shortcuts and plays the next episode on its own. It's a responsive web app for desktop, tablet and phone browsers, not a PWA: phones and TVs get the native apps (phases 4 and 5, ADR 0005). F008 is mostly web on top of the F003–F006 API. It adds the server pieces F004 and F005 left to it (sorting, filters, search, "recently added") and two small ones the player needs (a home endpoint and "up next"), so the native apps reuse them instead of rebuilding them on the client.

## User journeys

**Member, Friday night, on the laptop**
1. They open `http://mediabox:32500`. They're signed in, the account has three profiles, so the picker shows: "Who's under the plaid?". They pick "Alex".
2. Home: the backdrop of *Dune: Part Two*, which Alex stopped at 1:02:13 yesterday, tints the top of the page, and the first shelf is Continue watching with *Dune* first (landscape card, "1h 44m left"). Below: Recently added in Movies, Recently added in Shows, Science Fiction, Comedy.
3. The *Dune* card opens the player at 1:02:13, in under 3 s on the local network. Space pauses, → skips 10 s, F goes full screen.
4. They close the tab. The position is saved.

**Member, looking for something to watch**
1. "Movies" in the header opens the grid of 1,240 posters, sorted by title. Scrolling to the bottom stays smooth.
2. Sort by "Recently added", filter "Unwatched" and genre "Thriller": the grid narrows to 37 posters and the URL keeps the choice, so Back returns to it.
3. A poster opens *Prisoners*: backdrop, year, runtime, rating, genres, "4K · HDR10 · English, French", the synopsis, the cast. "Trailer" loads the YouTube player only after the click.
4. "Play" starts at the beginning. No resume prompt: it's never been played.

**Member, binge-watching a show**
1. *Severance* → the show page: seasons as tabs, Season 1's episodes with their stills, durations, a check on the watched ones and a bar on the one in progress. The main button says "Resume S1 E4".
2. At the end of E4, a card in the corner says "Next: S1 E5 · Woe's Hollow" with a 10 s countdown. It plays on its own. At the end of the season, it goes to S2 E1.
3. After three episodes in a row with no key pressed or click, the player asks "Still watching?" instead of playing the fourth.

**Member, a movie in a language they don't speak**
1. In the player, the subtitles menu lists "English", "French (forced)", "English SDH". They pick French: it shows at once (text track, no reload).
2. The audio menu switches to "French · 5.1": the server decides again, the player reloads the new stream at the same position, and only the picture blinks.
3. The quality menu shows "Original" and the lower qualities the server offers. On the train later (outside the home network), the server picks 4 Mbps; the menu says why ("Lowered to fit your remote streaming limit.").

**Member, instant search**
1. `/` or the search field: they type "ali". After the first character, results show and update as they type: Movies (*Alien*, *Aliens*, *Alice in Wonderland*), Shows, People (*Alicia Vikander*).
2. Enter opens the first result. A person opens their page: the movies and shows on this server they're in.

**A child's profile (max age 10)**
1. Home, grids, search, people's filmographies and "up next" never show an item above 10, or an unrated item when the profile doesn't allow them. A direct link to such an item lands on "This title isn't available on this profile."

**Admin, a wrong poster**
1. On a movie page, the admin sees "Edit" next to the title: "Change artwork" opens the choices F005 found (posters, backdrops, logos) and "Fix match" opens the F007 review screen for this item.

**Admin, from the library**
1. The header shows the server strip for admins (F007-R17): "mediabox · 2 watching". It opens `/admin`.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| **Shell and profiles** | | |
| F008-R1 | Signed-in shell for every browsing route (V2). **Desktop and tablet (≥ 768 px)**: a top bar with the PLAID wordmark (to `/`), a floating glass tab bar in the center (Home, one tab per library visible to the profile in `GET /libraries` order, Search as a magnifier that becomes the search field on `/search`; a sliding pill marks the current tab), then, for admins, the activity button (what the server is doing: scans and metadata matching from `GET /libraries` and `GET /metadata/status`, polled every 5 s, 2 s while open; who watches what from `GET /playback`, every 30 s; visible tab only), and the profile button that opens the profile menu (R54). The bar turns to glass once the page scrolls. Entries are text, never icons alone, except the magnifier, which has a label. **Phones (< 768 px)**: a top bar with the wordmark and the profile button (and the admin's server pill), and a floating bottom tab bar: Home, one tab per library up to 3, Search, More (further libraries, Pair a TV, appearance, sign out). The player has no shell. | must |
| F008-R2 | Profile picker: the existing F003 screen restyled on the design system, titled "Who's under the plaid?" (`profiles.title`), avatar tiles with the profile's `avatarColor`, PIN entry for profiles that have one. It shows when the session has no active profile (F003-R17) and, when the account has two or more profiles, at the first route of each browser tab session (a `sessionStorage` flag); choosing calls `POST /session/profile`. "Manage" in the profile menu opens it at any time, and the menu switches straight to a profile without a PIN (R54). With one profile, it never shows on its own. The page is lit (R51) by the backdrop of what the profile in focus watched last (`Profile.ambient`, from `GET /me`), crossfading as the focus moves. | must |
| F008-R3 | The active profile's `locale`, when set, overrides the account locale for the UI, through the existing `createI18n` order. Switching profile re-renders in the new profile's language without a reload. | should |
| F008-R4 | Theme: dark by default, light from the profile menu ("Appearance": System, Dark, Light), remembered per browser in `localStorage`. The player is always dark. | should |
| **Home** | | |
| F008-R5 | Home (`/`) from `GET /home` (R31): the rows the server returns as shelves, in its order, each a horizontal row with a title and "See all" when the row has a destination. Rows with no items aren't shown. Each row loads its first 20 items with the page; scrolling a row to its end loads the next page. | must |
| F008-R6 | Home hero (V2, replacing the ambient top): the `ambient` item of `GET /home` (R31) in a framed hero at the top of the home: its backdrop with a parallax drift on scroll (R53), a kicker ("Continue watching" or "New in {{library}}"), the logo or title, year, runtime, certification and two genres, two lines of synopsis, one amber button ("Resume" when the hero stands for a Continue watching item, an episode standing for its show; else "Play") with "More info", and the first cast names. It plays its scenes like a detail page (R52). No ambient item: no hero, the shelves start at the top. | must |
| F008-R7 | Continue watching row: landscape cards (16:9, the episode `still` else the item backdrop) with title, `S1 E4 · Title` for episodes, a progress bar, and the time left formatted with the locale (`Intl.DurationFormat`, or a fallback formatter where it's missing). The card plays directly; its menu has "Mark as watched" (`PUT /items/{id}/watched`) and "Go to show"/"More info". Empty: the row is hidden. | must |
| F008-R8 | Other rows use poster cards (2:3): poster at `w342` (`w185` on phone), title and year under it, a progress bar when in progress, a watched mark when watched, an unwatched-episodes count on shows (`totalEpisodes − watchedEpisodes`, when above 0). Cards without a poster show the design system's missing-artwork state with the title. | must |
| F008-R9 | Empty home states: no library at all, admin → the F007 "Add a library" empty state; member → "Nothing here yet. Ask {{adminName}} to add a library." Libraries exist but nothing is matched yet (first scan) → "Your movies are on their way." with each library's scan progress line. | must |
| **Libraries** | | |
| F008-R10 | Library grid (`/library/:id`): every top-level item (movies, or shows) as poster cards in a responsive grid (column count from the width, posters 120–200 px wide), with the total count in the header ("1,240 movies"). Data from `GET /libraries/{id}/items` with `total=true` and `offset` (R26). | must |
| F008-R11 | Virtualization: the grid renders only the rows in view plus two screens of buffer; the DOM never holds more than 300 cards whatever the library size. The scroll height comes from `total`, so the scrollbar is right before every page is loaded. Pages of 200 are fetched when their rows come near the view; a fast jump to the middle fetches that page first. Cards not loaded yet show the skeleton. | must |
| F008-R12 | Sorting: Title (default, A→Z), Recently added, Release date, Rating, Recently watched (by `lastPlayedAt` for the profile), Random (stable for the visit); each with ascending/descending except Random. Shows' "Recently added" uses the date of their newest episode (R26). | must |
| F008-R13 | Filters, combinable: watch state (All, Unwatched, In progress, Watched), genre (multi-select, from R27), years (decade chips, from R27), resolution (4K, 1080p, 720p and lower), HDR. Active filters show as removable chips with "Clear all"; the count updates. No result: "Nothing matches these filters." with "Clear filters". | must |
| F008-R14 | Sort, filters and scroll position live in the URL query (`?sort=added&order=desc&genre=53&watch=unwatched`), so links can be shared, Back restores the grid at the same place, and reload keeps the choice. | must |
| F008-R15 | Letter index on the right edge (desktop and tablet) when sorted by title: tapping a letter scrolls to its first item, using R27's letter counts. Hidden on phones. | could |
| F008-R16 | Collections: a library with collections (F005-R23) gets a "Collections" view (`/library/:id/collections`, `GET /collections`), and a collection page (`/collection/:id`) with its backdrop, overview and movies by release date. | should |
| **Detail pages** | | |
| F008-R17 | Movie page (`/item/:id`, `GET /items/{id}`): the backdrop (`w1920`/`w780`) in a framed hero with a scrim from its `colors`, drifting with a parallax on scroll and clipped by its frame (R53), playing the title's scenes once they're ready (R52), logo or title, original title when different, year, runtime, certification, TMDB rating with its count, genres (each links to the library filtered by it), technical badges (resolution, HDR format, Dolby Vision, audio languages named with `Intl.DisplayNames`), tagline, overview, director and writers, studios, collection link. Fields that fell back to English (`fieldLanguages`) carry `lang="en"`. | must |
| F008-R18 | Primary actions: "Play", or "Resume from {{position}}" with "Start over" next to it when `userState.positionMs > 0`; "Mark as watched"/"Mark as unwatched"; "Trailer" when there is one. With several files (editions, versions), a "Version" selector lists them by edition, resolution and HDR; the default is the server's choice (F006-R3). An item whose files are all missing shows "This title's files can't be found right now." and no Play. | must |
| F008-R19 | Cast row: round portraits (`w185`), name and character, up to 20, each to the person page; then "Crew" lines for directors and writers. | must |
| F008-R20 | Trailer: nothing from YouTube loads until "Trailer" is pressed. Then a dialog embeds `https://www.youtube-nocookie.com/embed/{key}` with autoplay; Esc or Close ends it. Prefer an official trailer in the UI language, then an official one in English, then any. "Open on YouTube" links out. | must |
| F008-R21 | Show page (`/item/:id` on a show): the same header as R17 with first and last air year, status ("Ended", "Returning"), seasons count; the main action from R32 ("Play S1 E1", "Resume S2 E3", "Play S3 E1" when the next one is unwatched); seasons as tabs (`?season=N`, specials last), each season's episodes from `GET /libraries/{id}/items?parentId=` with still (`w342`), number, title, air date, runtime, two lines of overview, watched mark or progress bar, and Play on hover/focus. "Mark season as watched" on the season. Episodes the server knows nothing about show their parsed name. | must |
| F008-R22 | Person page (`/person/:id`, `GET /people/{id}`): portrait, name, and what they're in on this server as poster cards, grouped by role (Acting, Directing, Writing, Creator), newest first. | must |
| F008-R23 | Admin tools on detail pages (admins only, behind an "Edit" menu): "Change artwork" opens the choices from `GET /items/{id}/artwork` per kind (poster, backdrop, logo) and applies one with `PUT /items/{id}/artwork/{kind}` and "Back to automatic" with `DELETE` (F005's picker, moved here by F007); "Fix match" links to the F007 review screen for the item; "Refresh metadata" calls `POST /items/{id}/refresh`. | should |
| F008-R24 | Unavailable item: an id the profile may not see, or that doesn't exist, returns 404; the page says "This title isn't available on this profile." with "Back to home". It never says which of the two it is. | must |
| **Search** | | |
| F008-R25 | Search (`/search?q=`, V2): the field is the tab bar's magnifier, which turns into a cream field on `/search` (on phones, the field is on the page). Before typing: recent searches as chips (remembered per browser, removable) and the biggest genres of the movies library as tiles. Then results from `GET /search` (R28) after each keystroke, debounced 150 ms, from the first character; a request in flight is aborted when a new one starts, and the previous results stay visible, faded, until the new ones arrive. Scopes with their counts (All, Movies, Shows, Episodes, People); "Top results" as posters (movies and shows interleaved as the server ranked them, up to 12, each with "Movie · 2024" or "Show · 4 seasons"), then Episodes and People. Arrow keys move through results, Enter opens the highlighted one (the first by default), Esc clears. The query is in the URL. No result: "Nothing called “{{query}}” here." `/` focuses the search from anywhere outside a text field. | must |
| **Server additions** | | |
| F008-R26 | `GET /libraries/{id}/items` gains, for top-level items (no `parentId`): `sort` (`title`, `added`, `released`, `rating`, `played`, `random`), `order` (`asc`, `desc`), `seed` (for `random`), filters `watch` (`unwatched`, `in_progress`, `watched`), `genre` (repeatable TMDB genre id, any of), `yearFrom`/`yearTo`, `resolution` (repeatable: `2160p`, `1080p`, `720p`, `sd`), `hdr` (boolean), `offset` (instead of `cursor`, for random access) and `total=true` (adds `total` to `ItemPage`). `ItemSummary` gains `addedAt` (`items.created_at`; for a show, its newest episode's). Every filter applies F005-R25's age predicate before counting, so `total` never reveals hidden items. Unknown values are `400 validation_failed`. | must |
| F008-R27 | `GET /libraries/{id}/facets` returns `total` (the library's top-level items after the age predicate, for the sidebar count, R1) and, over the items the active profile may see: `genres: [{id, name, count}]` (by count; `name` as stored for this library), `decades: [{from, count}]`, `resolutions: [{value, count}]`, `hdr: count`, and `letters: [{letter, count}]` in title sort order (digits and symbols grouped as `#`). | must |
| F008-R28 | `GET /search?q=&limit=` searches every library the profile may see: movies and shows by title and original title, episodes by title, people by name. Matching is case- and accent-insensitive, on word prefixes ("ali" finds *Alien* and *Alice*, "rings" finds *The Lord of the Rings*), through an SQLite FTS5 index kept in sync by F004/F005 writes. Ranked by exact title first, then prefix, then the BM25 rank, with ties by popularity (rating count). Returns `{movies, shows, episodes, people}`, each a list of `ItemSummary` (episodes with their show's title) or `{id, name, portrait}`, applying the age predicate. `q` under 1 character or over 100 is `400`. | must |
| F008-R29 | The search index covers every library at server start (a one-time build after migration, in the background; search answers from what's indexed meanwhile) and is updated in the same transaction as item and person writes. | must |
| F008-R30 | `ItemSummary` gains `show: {id, displayTitle, logo?}` on seasons and episodes, so Continue watching cards, search results and the next-episode card name the show without another request. | must |
| F008-R31 | `GET /home` returns `{ambient, rows}` for the active profile. `ambient`: the most recent continue-watching item with a backdrop, else the most recently added matched item with a backdrop (no logo needed, no randomness). `rows`, in order: `continue_watching` (F006-R37), then per library `recently_added` (movies by `addedAt`; shows by their newest episode), then up to 4 `genre` rows: the genres (grouped by TMDB genre id, so libraries in different languages merge) with at least 10 items across the profile's libraries, the profile's most watched first, then by size, each in random order stable for the day. Each row: `{id, type, title params (libraryId, libraryName, genre: {id, name}; the name from a library whose metadata language matches the profile's UI language, else the first library's), items (first 20), nextCursor, seeAll}`; `seeAll` names the library and query the "See all" opens. `GET /home/rows/{rowId}?cursor=` pages a row. | must |
| F008-R32 | `GET /items/{id}/up-next` for the active profile: on a show, the episode to play (the one in progress, else the first unwatched after the last watched, else S1 E1; specials skipped); on a season, the same within it; on an episode, the next one by season and episode number across seasons, specials skipped. Returns `{item: ItemSummary}` or `204` when there's none. Applies the age predicate. | must |
| **Player** | | |
| F008-R33 | The player (`/watch/:itemId`, `?fileId=`, `?t=` in seconds) is full-screen in the page, always dark, lazy-loaded with hls.js in its own chunk. It starts a session with `POST /items/{id}/playback` with the R34 profile, the file chosen on the detail page, the start position (resume, `?t=`, or 0), and the profile's last audio and subtitle language choices (R38). | must |
| F008-R34 | Capability profile built once per browser from `MediaSource.isTypeSupported` (or `ManagedMediaSource` on iOS Safari) and `HTMLVideoElement.canPlayType`: containers (`mp4`, `webm`, plus `mkv` only where the browser plays it), video codecs with profiles, levels, bit depth and HDR (`hdr10`/`hlg` only when `matchMedia('(dynamic-range: high)')` and the codec string with the HDR transfer are supported), audio codecs with channels (E-AC-3 and AC-3 only where supported), `subtitles: {text: ["vtt"], separate: ["pgs"]}` (the browser draws PGS itself, R35), `protocols: ["http", "hls"]`, `canSwitchAudioTrack: false`. It's cached in `sessionStorage` and shown in "Playback info" (R41). | must |
| F008-R35 | Engines: `direct_play` plays the `url` in `<video>` with range requests. HLS (`direct_stream`, `transcode`) uses the browser's native HLS when `canPlayType('application/vnd.apple.mpegurl')` (Safari), else hls.js. Subtitles never go into the picture when the browser can draw them (V2, like Plex): text subtitles are added as `<track kind="subtitles">` from `PlaybackSubtitle.url` (WebVTT), PGS subtitles are fetched raw from `….sup` and drawn on a canvas over the picture (`object-fit: contain`, raised while the controls show). Both are fetched the quick way: `?from=` the current position returns the next 15 minutes in seconds (`X-Subtitle-Until` marks a partial answer) while the server extracts the whole track in the background, and the player asks again until it has it; "Preparing subtitles…" shows only while the track at the current time is missing. The selected track is warmed by the server when the session starts. Only other image formats (VobSub, DVB) are `burn_in` and need a new decision (R37). Segments that answer `503 segment_not_ready` are retried after `Retry-After`. | must |
| F008-R36 | Controls, shown on mouse move, tap or any key, hidden after 3 s of playback without input: back (to where the player was opened from), title and episode line, play/pause, −10 s / +10 s, the timeline (elapsed, remaining, buffered range, hover time tooltip; no preview thumbnails), volume with mute, subtitles, audio, quality, "Playback info", picture-in-picture where supported, full screen. Touch: tap toggles controls, double-tap left/right seeks 10 s. | must |
| F008-R37 | Track and quality changes: choosing a text subtitle delivered `external` or `embedded` switches at once in the client. Choosing an audio track, a quality, or an image subtitle (`burn_in`) sends `PATCH /playback/{id}` with the current `positionMs`; a changed `url` is reloaded at that position with the controls kept, an unchanged one keeps playing. `503 transcode_limit_reached` keeps the current stream and shows the error toast. | must |
| F008-R38 | Track memory per profile in `localStorage`: the last audio language and subtitle choice (language + forced/SDH, or off) chosen in the player, applied to the next items when a track in that language exists; otherwise the server's defaults (F006-R4, forced subtitles in the audio language). Volume and mute are remembered per browser. | should |
| F008-R39 | Progress: `POST /playback/{id}/progress` every 10 s while playing, and at once on pause, seek (after it settles), buffering start and end; `DELETE /playback/{id}` with `positionMs` on leaving the player, and on `pagehide` with `fetch(…, {keepalive: true})`. A failed report is not retried, the next one carries the position. A `404 playback_session_not_found` (expired, or stopped by an admin) stops the video and shows "Playback stopped." with "Start again from {{position}}". | must |
| F008-R40 | Resume: opening a movie or episode with `positionMs > 0` from a "Resume" button starts there; from a plain "Play" (or a card in a row other than Continue watching), the player asks "Resume from {{position}}" / "Start over" before starting. `?t=` skips the question. | must |
| F008-R41 | "Playback info" panel: method in words (`playback.method.*`), each reason as a sentence (`reason.*`, shared with F007), quality and bitrate, hardware or software conversion, warnings. It helps people understand "Converting" without opening the admin. | should |
| F008-R42 | Keyboard shortcuts, active in the player when no menu has focus: Space or K play/pause, ← / → 10 s, J / L 10 s, ↑ / ↓ volume 10 %, M mute, F full screen, C cycles subtitles (off → the remembered language → off), Shift+N next episode, Esc closes a menu then leaves full screen, Home/End start/end, 0–9 jump to 0–90 %. "?" lists them. Shortcuts are also on the detail pages: Enter on a focused card opens it. | must |
| F008-R43 | Next episode: in the last 30 s of an episode that has an up-next (R32, fetched when playback starts), a card shows the next episode's still, number and title with "Play now" and "Cancel"; when the video ends, a 10 s countdown plays it in the same player (a new session, the current one stopped with its position). "Cancel" lets the episode end on the details. A profile setting "Play next episode automatically" (on by default, `localStorage` per profile, a switch in the profile menu, R54) turns the countdown off. | must |
| F008-R44 | "Still watching?": after 3 episodes started by the countdown in a row with no key, click or tap in between, the countdown is replaced by "Still watching?" with "Continue" and "Stop". It saves a sleeping viewer's progress and the server's CPU. | should |
| F008-R45 | Player errors, each in words with what to do: `transcode_limit_reached` (with "Try again" after `Retry-After`, and "Lower quality" when a lower one exists), `transcode_storage_full`, `file_unavailable`, `ffmpeg_missing`, `file_not_analyzed`, `not_playable`, `profile_required` (to the picker), a media error from the browser (`MEDIA_ERR_DECODE`, `MEDIA_ERR_SRC_NOT_SUPPORTED`) → one automatic retry with `forceMethod: transcode`, then "This video can't play in this browser." with "Copy link for VLC" (an `external` session, F006). | must |
| F008-R46 | Media Session API: title, show and episode, artwork (`w500` poster or still), play/pause/seek handlers, so the OS media keys and lock screens work. | could |
| **Cross-cutting** | | |
| F008-R47 | Every string comes from the i18n catalogs (table below); plurals with `_one`/`_other`; numbers, dates, durations, runtimes and language names with `Intl` (i18next formatters, `Intl.DisplayNames`). Genres come translated from TMDB in the library's metadata language and are shown as is. | must |
| F008-R48 | Every browsing screen works from 375 px to 2560 px with no horizontal page scroll (rows scroll on their own), in dark and light, with German-length strings (+35 %). The player works on phones in portrait and landscape. | must |
| F008-R49 | Every server error code these screens can receive has a translated message (existing `errors.*`, plus the F006 codes not added yet); unknown codes fall back to `errors.generic`. | must |
| F008-R50 | Attribution: the footer of the home and of every detail page links to the About section that shows the providers' notices from `GET /server` (F005-R37). | must |
| F008-R51 | Ambient light follows the selection (TV direction, canvas page "tv"): the home, library grids and detail pages are lit by the blurred ambient colors (F005-R21) of one title. At rest it is the hero title on the home, the first title of the grid in the current sort and filter, the title itself on a detail page. When a card gets hover or focus (mouse, keyboard, remote), the light crossfades to that title's colors over `duration-backdrop` (600 ms, `ease-standard`); it settles back only when the selection moves, not when the pointer leaves. Colors come from the server (computed at scan time, stored with the item), never extracted in the browser; their chroma and lightness are clamped so the page stays dark and the text keeps its contrast. No colors: the plain base color. `prefers-reduced-motion`: the light switches without the crossfade. | should |
| F008-R52 | Hero previews (V2, like Apple TV): the hero of a movie or show page, and the home hero, play a loop of a few scenes from the title itself, with their sound, over the backdrop. The server cuts it with FFmpeg (`GET /items/{id}/preview`): four scenes of 6 s taken evenly between 8 % and 35 % of a movie's runtime (after the opening credits, before anything decisive), or from a show's first episode on disk (season 1 before specials), so it spoils nothing; each scene cut by its own FFmpeg run (one decoder in memory at a time, threads capped) and the four joined by copying. Light browser-ready H.264 (8-bit 4:2:0 SDR, up to 1080p, at most 12 Mb/s) keeps its video as it is, cut on demand in a second or two (about 30 MB, cached apart, 4 GiB); anything else is scaled to 1280 px, tone mapped from HDR when the FFmpeg build can, H.264 MP4 with one sound track in stereo AAC 128 kb/s, picked in this order: the language the profile last chose in the player (R38, sent as `?audio=`), the library's metadata language, the file's default track, its first (a commentary only when nothing else is in that language); one preview is cut per track, and the ahead-of-time cuts use the library's language; each scene's sound faded in and out over 400 ms so the cuts don't click (none for a file without sound), `faststart`. Every movie and show whose video is encoded is cut **ahead of time**, newest first, on two threads, and only while nobody transcodes; a title opened before its turn goes to the front of the queue, interrupts the cut in progress and is cut on every core. Files shorter than 5 min get none. The page shows the backdrop first; the scenes fade in after 2 s once they play, with the parallax of the backdrop. They pause off screen and in a hidden tab. A sound button mutes them (WCAG 1.4.2), remembered per profile in the browser, sound on by default; when the browser refuses sound before the viewer has interacted with the page, they play muted with the button showing "sound off" until it's pressed, without changing the profile's choice. A pause button (WCAG 2.2.2) stops them for the page. Off with `prefers-reduced-motion`, `Save-Data`, or the profile's "Play previews automatically" switch (R54, on by default). | should |
| F008-R53 | Motion (V2, canvas Step 11): pages enter with a short rise, sideways in the direction of travel between tabs; rows and grid cards stagger in; the tab bar's pill slides; buttons press with a spring; sheets, menus and toasts scale from where they hang and leave the same way (toasts wait 5 s, pause on hover); images go from blurred to sharp as they load; the hero's picture drifts at 0.6× the scroll and fades to 40 % as it leaves, clipped by its frame. Durations and easings are the design tokens (`--duration-*`, `--ease-*`). `prefers-reduced-motion`: no movement, crossfades only. | should |
| F008-R54 | Profile menu (V2, canvas Step 12, TU01/TU02): a glass panel with the active profile (avatar, name, "Admin · {{serverName}}" or "Member · {{serverName}}"); the other profiles as avatars (up to 4, a lock on those with a PIN) and "Manage" (`/profiles`): a profile without a PIN switches at once (`POST /session/profile`), one with a PIN opens the picker; "Admin" (admins) with the count of open problems (`GET /problems`) and "Pair a TV"; "Appearance" (System, Dark, Light, R4); the switches "Play next episode automatically" (R43) and "Play previews automatically" (R52); "About PLAID" (R50); "Sign out" in red. Every row has an icon and words. | should |

## Design

### Data model
Migration `0007_browsing.sql`:

```sql
-- Instant search (R28, R29): one row per searchable thing.
CREATE VIRTUAL TABLE search_index USING fts5(
  title,                      -- displayTitle, normalized (lowercase, accents removed)
  original_title,
  kind UNINDEXED,             -- movie, show, episode, person
  ref_id UNINDEXED,           -- items.id or people.id
  library_id UNINDEXED,       -- NULL for people
  tokenize = 'unicode61 remove_diacritics 2',
  prefix = '1 2 3'
);

-- Sorting and filtering top-level items (R26) without reading JSON per row.
ALTER TABLE items ADD COLUMN newest_child_added_at TEXT;  -- shows: newest episode's created_at
CREATE INDEX items_library_added ON items(library_id, kind, created_at);
CREATE TABLE item_genres (
  item_id   TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  genre_id  INTEGER NOT NULL,         -- TMDB genre id, the same in every language
  name      TEXT NOT NULL,            -- the name as stored by F005, in the library's language
  PRIMARY KEY (item_id, genre_id)
);
CREATE INDEX item_genres_genre ON item_genres(genre_id);
```

`item_genres` is filled from `item_metadata.genres` when F005 writes metadata (and once by the migration). `newest_child_added_at` is updated when the scanner adds an episode. Resolution and HDR filters reuse what the technical summary already reads from `media_files`. No table for the home rows or up-next: both are computed per request from `watch_state`, `items` and `item_metadata`.

### API
Carried into `api/openapi.yaml` first, then `make generate`. Every route needs a signed-in session with an active profile (`403 profile_required`) and applies F005-R25.

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/libraries/{id}/items` | **Changed** (R26): `sort`, `order`, `seed`, `watch`, `genre[]`, `yearFrom`, `yearTo`, `resolution[]`, `hdr`, `offset`, `total`; `ItemPage.total`; `ItemSummary.addedAt` |
| GET | `/libraries/{id}/facets` | `{total, genres, decades, resolutions, hdr, letters}` (R27) |
| GET | `/search` | `?q=&limit=` (default 8, max 50) → `{movies, shows, episodes, people}` (R28) |
| GET | `/home` | `{ambient?: ItemSummary, rows: [HomeRow]}` (R31) |
| GET | `/home/rows/{rowId}` | `?cursor=` → `ItemPage` for one row (R31) |
| GET | `/items/{id}/up-next` | `{item: ItemSummary}` or `204` (R32) |
| — | `ItemSummary` | **Changed**: `addedAt` (R26), `show` on seasons and episodes (R30) |
| GET | `/items/{id}/preview` | The hero preview (R52): `?audio=` the language to hear. `200` MP4 with `Range` (revalidated: `private, no-cache`), `202` + `Retry-After` while it's cut, `404` when there's none. Served by a plain handler (tag `preview`, excluded from the generated server) |
| — | `Profile` | **Changed**: `ambient`, the backdrop of what the profile watched last (R2) |
| — | `SubtitleCapability` | **Changed**: `separate` (formats the client draws itself, R34) |
| GET | `/stream/{token}/subtitles/{index}.sup` | Raw PGS for a client that draws it (R35) |
| GET | `/stream/{token}/subtitles/{index}.vtt` | **Changed**: `?from=` (ms) answers the next 15 minutes with `X-Subtitle-Until` while the whole track is extracted (R35) |

`HomeRow`: `{id, type: continue_watching | recently_added | genre, libraryId?, libraryName?, genre?: {id, name}, items: [ItemSummary], nextCursor?, seeAll?: {libraryId, sort, order, genre?: id}}`. Row titles are built by the client from `type` and its params (`home.row.*`), never sent as text.

Everything else F008 uses already exists: `/me`, `/session/profile`, `/libraries`, `/items/{id}`, `/items/{id}/artwork*`, `/items/{id}/refresh`, `/images/{id}`, `/collections*`, `/people/{id}`, `/items/{id}/playback`, `/playback/{id}*`, `/items/{id}/watched`, `/me/continue-watching`, `/server`, `/playback` (admin strip).

No new error code. The previews are cached under the data folder (`cache/playback/previews`, capped at 10 GB, least recently used first; nothing more is cut ahead past 90 %); no table.

### Behavior per app
- **server**: the R26 query builder (sort and filters on `items` joined with `item_metadata`, `item_genres`, `watch_state` and the files' technical summary, the age predicate first), facets, the FTS5 index and its writers in F004/F005, `GET /search`, `GET /home` and `GET /items/{id}/up-next` in a small `internal/catalog` package that calls `internal/access` and `internal/playback`'s watch state.
- **web**: the shell and routes below; home rows, grid, detail, search and player built on the design system tokens and F007's ported components; a virtualized grid; the player module (capability detection, engine choice, hls.js lazy chunk, progress reporter, track memory, shortcuts); `Home` replaced; F007's "Open the library" link becomes the header.
- **apple / android**: out of scope. The new routes are what their home, search and up-next will use.

### Web routes
| Route | Screen |
|-------|--------|
| `/profiles` | Profile picker (existing route, restyled) |
| `/` | Home |
| `/library/:id` | Library grid (`?sort=&order=&watch=&genre=&decade=&resolution=&hdr=`) |
| `/library/:id/collections`, `/collection/:id` | Collections, one collection |
| `/item/:id` | Movie or show (`?season=N` on shows) |
| `/person/:id` | Person |
| `/search` | Search results (`?q=`) |
| `/watch/:itemId` | Player (`?fileId=&t=`) |

### UI / Art direction
Screens come from the Claude Design canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz), pages **Browse** and **Player**, made with [claude-design-prompt.md](claude-design-prompt.md). Directions kept: the home as shelves under an ambient top (A, marquee, and B, a framed hero, were rejected: a hero cost a screen of scrolling before Continue watching and competed for the one amber; **V2 overrides this**: the canvas page "V2" brings back a framed hero, see below), the show page with a season picker that opens a list or a sheet (B), the player with the design system's scrim and timeline and track pills that name what plays (A). The strings below are the canvas copy decks; the catalogs win over the canvas once merged. The rules:
- **The content is the color**: warm near-black ground, the posters and backdrops bring the color, ambient colors (F005-R21) tint the detail header and the top of the home; the UI recedes. One amber element per viewport (the primary Play, the focus ring, the timeline's played part).
- **Signature**: the woven tartan lives only in the player's timeline fill and the progress bars on cards.
- **Type**: Instrument Serif only for emotional lines (the profile picker title, empty states, the end of a show); Instrument Sans everywhere else. Titles without a logo use the sans at display size.
- **Motion**: one per surface: the ambient backdrop crossfade on the home, the card focus lift, the controls fading, the next-episode card arriving. All off with `prefers-reduced-motion`. (V2: the fuller system of R53.)
- **States** for every screen: loading (skeleton cards, never a spinner over posters), empty, error (inline, with retry), offline (F007-R16 banner), missing artwork, unmatched title (parsed name, quiet "Not identified" for admins only).
- **Canvas rules**: when the page's main button already points to an item in progress, that item's progress thread is neutral; detail pages fade the backdrop into the base color and the ambient color never reaches buttons or text; more than 12 seasons turn the season picker into a number grid, Specials always last; every status has words (Watched, N new, Not identified), a mark only adds color; missing data is a plate with the title or initials; the TMDB rating is hidden under 50 votes; menus in the player are panels anchored above their pill on desktop, a bottom sheet on a portrait phone, a side sheet in landscape; a reload (audio, quality, image subtitles) shows a corner status (`player.reload.*`) over the last frame, never a spinner over the picture; subtitles are Instrument Sans 600 on the scrim color at 62 %, 3.7 % of the picture height (16 px minimum), two lines max, raised above the timeline while the controls show.
- **V2 (canvas page "V2", Steps 1–12)**: the TV direction made the default. Glass surfaces (top bar, tab bar, sheets, menus, the profile menu) over a page lit by the selection (R51); a framed hero on the home and on detail pages, with parallax and scenes from the title (R6, R17, R52); the wordmark is Instrument Sans 700 "pla·d" with Foxy over a dotless i; the app icon and web manifest follow it; motion per R53. Where V2 and the rules above disagree, V2 wins.
- **Copy rule**: warm on the profile picker, the empty states, the end of a series and "Still watching?"; plain and exact everywhere else (buttons, menus, errors, the player).

### i18n
Keys in the repo's i18next format (`{{var}}`, `{{n, number}}`, `_one`/`_other` with `count`). Durations (`{{position}}`, time left, runtime) are formatted with `Intl.DurationFormat` (fallback: a small formatter producing `1 h 02 min` / `1:02:13`), never concatenated. Keys from F006's suggested table are taken as written unless changed below. The existing `profiles.whoIsWatching` is replaced by `profiles.title`; `welcome.*` keys still used by `Home` are removed with it.

**Shell and profiles**

| Key | en | fr |
|-----|----|----|
| `profiles.title` (serif, warm) | Who’s under the plaid? | Qui est sous le plaid ? |
| `profiles.pin.title` | Enter {{name}}’s PIN | Saisis le code de {{name}} |
| `profiles.pin.wrong` | Wrong PIN. Try again. | Code incorrect. Réessaie. |
| `profiles.pin.cancel` | Choose another profile | Choisir un autre profil |
| `profiles.pin.badge` | PIN | Code |
| `profiles.manage` | Manage | Gérer |
| `profiles.switchTo` | Switch to {{name}} | Passer à {{name}} |
| `nav.home` | Home | Accueil |
| `nav.libraries` | Libraries | Bibliothèques |
| `nav.admin` | Admin | Admin |
| `nav.pairTv` | Pair a TV | Associer une TV |
| `nav.signOut` | Sign out | Se déconnecter |
| `nav.search` | Search | Rechercher |
| `nav.more` | More | Plus |
| `nav.profileMenu` | Profile menu for {{name}} | Menu du profil {{name}} |
| `profiles.switch` (existing) | Switch profile | Changer de profil |
| `nav.appearance` | Appearance | Apparence |
| `nav.appearance.system` | System | Système |
| `nav.appearance.dark` | Dark | Sombre |
| `nav.appearance.light` | Light | Clair |
| `nav.serverStrip_one` | {{serverName}} · {{count, number}} watching | {{serverName}} · {{count, number}} en lecture |
| `nav.serverStrip_other` | {{serverName}} · {{count, number}} watching | {{serverName}} · {{count, number}} en lecture |
| `nav.serverStrip.quiet` | {{serverName}} | {{serverName}} |
| `nav.serverCard.idle` | Nothing playing | Rien en lecture |
| `nav.serverCard.stream` | {{profileName}} · {{title}} | {{profileName}} · {{title}} |
| `nav.serverCard.scanning` | Scanning {{libraryName}}, {{percent, number}} % | Analyse de {{libraryName}}, {{percent, number}} % |
| `nav.about` | About PLAID | À propos de PLAID |
| `people.role.admin` (F007) | Admin | Admin |
| `people.role.user` (F007) | Member | Membre |

**Home**

| Key | en | fr |
|-----|----|----|
| `home.row.continueWatching` | Continue watching | Reprendre |
| `home.row.recentlyAdded` | Recently added in {{libraryName}} | Ajouts récents dans {{libraryName}} |
| `home.row.genre` | {{genre}} | {{genre}} |
| `home.row.seeAll` | See all | Tout voir |
| `home.empty.member.title` (warm) | Nothing here yet. | Rien ici pour l’instant. |
| `home.empty.member.body` | Ask {{adminName}} to add a library. | Demande à {{adminName}} d’ajouter une bibliothèque. |
| `home.empty.scanning.title` (warm) | Your movies are on their way. | Tes films arrivent. |
| `home.empty.scanning.body` | {{count, number}} found so far. They appear here as they’re matched. | {{count, number}} trouvés pour l’instant. Ils apparaissent ici au fil des identifications. |
| `card.timeLeft` | {{duration}} left | Encore {{duration}} |
| `card.episode` | S{{season, number}} E{{episode, number}} · {{title}} | S{{season, number}} É{{episode, number}} · {{title}} |
| `card.episodeShort` | S{{season, number}} E{{episode, number}} | S{{season, number}} É{{episode, number}} |
| `card.unwatchedEpisodes_one` | {{count, number}} new episode | {{count, number}} nouvel épisode |
| `card.unwatchedEpisodes_other` | {{count, number}} new episodes | {{count, number}} nouveaux épisodes |
| `card.unwatchedBadge` | {{count, number}} new | {{count, number}} nouveau(x) |
| `card.progressLabel` (screen reader) | {{percent, number}} % watched | Vu à {{percent, number}} % |
| `card.watched` | Watched | Vu |
| `card.menu` | More actions for {{title}} | Plus d’actions pour {{title}} |
| `card.goToShow` | Go to show | Voir la série |
| `item.markWatched` | Mark as watched | Marquer comme vu |
| `item.markUnwatched` | Mark as unwatched | Marquer comme non vu |
| `item.missingArtwork` | No artwork for {{title}} | Pas d’image pour {{title}} |
| `item.notIdentified` (admins only) | Not identified | Non identifié |
| `item.notIdentified.body` | PLAID couldn’t match this file to a movie. Only admins see this. | PLAID n’a pas pu associer ce fichier à un film. Seuls les admins voient ce message. |

**Library grid**

| Key | en | fr |
|-----|----|----|
| `grid.count.movies_one` | {{count, number}} movie | {{count, number}} film |
| `grid.count.movies_other` | {{count, number}} movies | {{count, number}} films |
| `grid.count.shows_one` | {{count, number}} show | {{count, number}} série |
| `grid.count.shows_other` | {{count, number}} shows | {{count, number}} séries |
| `grid.sort` | Sort | Trier |
| `grid.sort.title` | Title | Titre |
| `grid.sort.added` | Recently added | Ajouts récents |
| `grid.sort.released` | Release date | Date de sortie |
| `grid.sort.rating` | Rating | Note |
| `grid.sort.played` | Recently watched | Vu récemment |
| `grid.sort.random` | Random | Aléatoire |
| `grid.order.asc` | Ascending | Croissant |
| `grid.order.desc` | Descending | Décroissant |
| `grid.filters` | Filters | Filtres |
| `grid.filter.watch` | Viewing | Visionnage |
| `grid.filter.watch.all` | All | Tout |
| `grid.filter.watch.unwatched` | Not watched | Pas vus |
| `grid.filter.watch.in_progress` | In progress | En cours |
| `grid.filter.watch.watched` | Watched | Vus |
| `grid.filter.genre` | Genre | Genre |
| `grid.filter.decade` | {{decade}}s | Années {{decade}} |
| `grid.filter.resolution` | Resolution | Résolution |
| `grid.filter.resolution.sd` | 720p and lower | 720p et moins |
| `grid.filter.hdr` | HDR | HDR |
| `grid.filter.hdrOnly` | HDR only | HDR uniquement |
| `grid.filter.remove` | Remove filter {{name}} | Retirer le filtre {{name}} |
| `grid.filter.clear` | Clear all | Tout effacer |
| `grid.empty.title` | Nothing matches these filters. | Rien ne correspond à ces filtres. |
| `grid.empty.body` | Remove a filter, or clear them all. | Retire un filtre, ou efface-les tous. |
| `grid.empty.action` | Clear filters | Effacer les filtres |
| `grid.collections` | Collections | Collections |
| `grid.letters` | Jump to letter | Aller à la lettre |
| `grid.filters.apply_one` | Show {{count, number}} title | Afficher {{count, number}} titre |
| `grid.filters.apply_other` | Show {{count, number}} titles | Afficher {{count, number}} titres |
| `grid.end_one` | That’s the only title. | C’est le seul titre. |
| `grid.end_other` | That’s all {{count, number}} titles. | Voilà les {{count, number}} titres. |

**Detail pages**

| Key | en | fr |
|-----|----|----|
| `item.play` | Play | Lire |
| `playback.resume` (F006) | Resume from {{position}} | Reprendre à {{position}} |
| `playback.startOver` (F006, fr changed) | Start over | Recommencer |
| `item.playEpisode` | Play S{{season, number}} E{{episode, number}} | Lire S{{season, number}} É{{episode, number}} |
| `item.resumeEpisode` | Resume S{{season, number}} E{{episode, number}} | Reprendre S{{season, number}} É{{episode, number}} |
| `item.trailer` | Trailer | Bande-annonce |
| `item.trailer.openYouTube` | Open on YouTube | Ouvrir sur YouTube |
| `item.trailer.play` | Play trailer | Lire la bande-annonce |
| `item.trailer.notice` | Plays from YouTube when you click. | Lue depuis YouTube quand tu cliques. |
| `item.preview.autoplay` | Play previews automatically | Lire les aperçus automatiquement |
| `item.preview.pause` | Pause the preview | Mettre l’aperçu en pause |
| `item.preview.play` | Play the preview | Lire l’aperçu |
| `item.version` | Version | Version |
| `item.versions.option` | {{edition}} · {{resolution}} · {{size}} | {{edition}} · {{resolution}} · {{size}} |
| `item.tech.audio` | Audio: {{languages, list}} | Audio : {{languages, list}} |
| `item.runtime` | {{duration}} | {{duration}} |
| `item.rating` (hidden under 50 votes) | TMDB {{rating, number}} · {{count, number}} votes | TMDB {{rating, number}} · {{count, number}} votes |
| `item.originalTitle` | Original title: {{title}} | Titre original : {{title}} |
| `item.directedBy` | Directed by {{names, list}} | Réalisé par {{names, list}} |
| `item.writtenBy` | Written by {{names, list}} | Écrit par {{names, list}} |
| `item.createdBy` | Created by {{names, list}} | Créée par {{names, list}} |
| `item.cast` | Cast | Distribution |
| `item.collection` | Part of {{name}} | Fait partie de {{name}} |
| `item.collection.thisMovie` | This movie | Ce film |
| `item.seasons_one` | {{count, number}} season | {{count, number}} saison |
| `item.seasons_other` | {{count, number}} seasons | {{count, number}} saisons |
| `item.season` | Season {{number, number}} | Saison {{number, number}} |
| `item.specials` | Specials | Épisodes spéciaux |
| `item.markSeasonWatched` | Mark season as watched | Marquer la saison comme vue |
| `item.seasonPicker.title` | {{count, number}} seasons and Specials | {{count, number}} saisons et épisodes spéciaux |
| `item.seasonPicker.jump` | Type a number to jump | Tape un numéro pour y aller |
| `item.season.summary` | {{year}} · {{episodes}} · {{watched, number}} watched | {{year}} · {{episodes}} · {{watched, number}} vu(s) |
| `item.status.ended` | Ended | Terminée |
| `item.status.returning` | Returning | Renouvelée |
| `item.status.canceled` | Canceled | Annulée |
| `item.status.in_production` | In production | En production |
| `item.filesMissing` | This title’s files can’t be found right now. | Les fichiers de ce titre sont introuvables pour l’instant. |
| `item.filesMissing.body` (admins) | The drive with {{path}} is unplugged or asleep. It plays again as soon as the drive is back. | Le disque de {{path}} est débranché ou en veille. La lecture reprend dès qu’il revient. |
| `item.unavailable.title` | This title isn’t available on this profile. | Ce titre n’est pas disponible sur ce profil. |
| `item.unavailable.body` | It was removed from the library, or this profile can’t see it. | Il a été retiré de la bibliothèque, ou ce profil ne peut pas le voir. |
| `item.unavailable.back` | Back to home | Retour à l’accueil |
| `item.edit` | Edit | Modifier |
| `item.edit.artwork` | Change artwork | Changer l’image |
| `item.edit.artwork.poster` | Poster | Affiche |
| `item.edit.artwork.backdrop` | Backdrop | Arrière-plan |
| `item.edit.artwork.logo` | Logo | Logo |
| `item.edit.fixMatch` | Fix match | Corriger l’identification |
| `item.edit.artwork.automatic` | Back to automatic | Revenir au choix automatique |
| `item.edit.artwork.title` | Change artwork · {{title}} | Changer l’image · {{title}} |
| `item.edit.artwork.auto` | PLAID picks the best one | PLAID choisit la meilleure |
| `item.edit.artwork.option` | {{language}} · {{width}} × {{height}} | {{language}} · {{width}} × {{height}} |
| `item.edit.refresh` | Refresh metadata | Actualiser les métadonnées |
| `item.edit.refreshed` | Refreshing. It can take a minute. | Actualisation en cours. Ça peut prendre une minute. |
| `person.role.cast` | Acting | Interprétation |
| `person.role.director` | Directing | Réalisation |
| `person.role.writer` | Writing | Scénario |
| `person.role.creator` | Created | Création |
| `person.summary_one` | On {{serverName}} · {{count, number}} title | Sur {{serverName}} · {{count, number}} titre |
| `person.summary_other` | On {{serverName}} · {{count, number}} titles | Sur {{serverName}} · {{count, number}} titres |
| `about.attribution` | Metadata and images | Métadonnées et images |

**Search**

| Key | en | fr |
|-----|----|----|
| `search.placeholder` | Movies, shows, people | Films, séries, personnes |
| `search.label` | Search {{serverName}} | Rechercher dans {{serverName}} |
| `search.group.movies` | Movies | Films |
| `search.group.shows` | Shows | Séries |
| `search.group.episodes` | Episodes | Épisodes |
| `search.group.people` | People | Personnes |
| `search.seeAll` | See all {{count, number}} | Voir les {{count, number}} |
| `search.empty` | Nothing called “{{query}}” here. | Rien qui s’appelle « {{query}} » ici. |
| `search.episodeOf` | {{show}} · S{{season, number}} E{{episode, number}} | {{show}} · S{{season, number}} É{{episode, number}} |
| `search.originalTitle` | Original title: {{title}} | Titre original : {{title}} |

The V2 search (R25) adds keys for the scopes, recent searches and genre tiles (`search.*`); the catalogs are the reference.

**Player**

| Key | en | fr |
|-----|----|----|
| `player.back` | Back | Retour |
| `player.play` | Play | Lire |
| `player.pause` | Pause | Pause |
| `player.back10` | Back 10 seconds | Reculer de 10 secondes |
| `player.forward10` | Forward 10 seconds | Avancer de 10 secondes |
| `player.timeline` | Timeline | Barre de lecture |
| `player.timeline.valueText` | {{elapsed}} of {{duration}} | {{elapsed}} sur {{duration}} |
| `player.remaining` | {{duration}} left | Encore {{duration}} |
| `player.volume` | Volume | Volume |
| `player.volume.valueText` | Volume {{percent, number}} % | Volume {{percent, number}} % |
| `player.mute` | Mute | Couper le son |
| `player.unmute` | Unmute | Rétablir le son |
| `player.subtitles` | Subtitles | Sous-titres |
| `player.subtitles.off` | Off | Aucun |
| `player.subtitles.forced` | {{language}} (forced) | {{language}} (forcés) |
| `player.subtitles.sdh` | {{language}} SDH | {{language}} SME |
| `player.subtitles.sdh.long` | SDH: for deaf and hard of hearing viewers | SME : pour les sourds et malentendants |
| `player.subtitles.imageNote` | Image subtitles: the video reloads | Sous-titres en image : la vidéo se recharge |
| `player.subtitles.preparing` | Preparing subtitles… | Sous-titres en préparation… |
| `player.audio.reloadNote` | Changing the audio reloads the video at the same moment. | Changer l’audio recharge la vidéo au même moment. |
| `player.audio` | Audio | Audio |
| `player.audio.track` | {{language}} · {{channels}} | {{language}} · {{channels}} |
| `player.channels.mono` | Mono | Mono |
| `player.channels.stereo` | Stereo | Stéréo |
| `player.channels.surround` | {{layout}} | {{layout}} |
| `player.quality` | Quality | Qualité |
| `player.quality.original` | Original | Originale |
| `player.quality.current` | {{quality}} · {{bitrate}} | {{quality}} · {{bitrate}} |
| `player.quality.note` | Lower qualities are converted by the server and start a few seconds later. | Les qualités plus basses sont converties par le serveur et démarrent quelques secondes plus tard. |
| `player.quality.asIs` | plays the file as is | lit le fichier tel quel |
| `player.quality.converted` | converted | convertie |
| `player.reload.audio` | Switching to {{track}} | Passage à {{track}} |
| `player.reload.quality` | Switching to {{quality}} | Passage en {{quality}} |
| `player.reload.subtitles` | Adding {{language}} subtitles | Ajout des sous-titres {{language}} |
| `player.info` | Playback info | Infos de lecture |
| `player.info.method` | How it reaches you | Comment la vidéo te parvient |
| `player.info.why` | Why | Pourquoi |
| `player.info.quality` | Quality | Qualité |
| `player.info.conversion` | Conversion | Conversion |
| `player.info.copied` | {{track}}, copied as is | {{track}}, copié tel quel |
| `player.info.textSubs` | {{language}}, text | {{language}}, texte |
| `player.info.hardware` | Converted on the GPU | Converti par la carte graphique |
| `player.info.software` | Converted by the processor | Converti par le processeur |
| `player.pip` | Picture in picture | Image dans l’image |
| `player.fullscreen` | Full screen | Plein écran |
| `player.exitFullscreen` | Exit full screen | Quitter le plein écran |
| `player.shortcuts` | Keyboard shortcuts | Raccourcis clavier |
| `player.next.label` | Next episode | Épisode suivant |
| `player.next.countdown_one` | Playing in {{count, number}} second | Lecture dans {{count, number}} seconde |
| `player.next.countdown_other` | Playing in {{count, number}} seconds | Lecture dans {{count, number}} secondes |
| `player.next.playNow` | Play now | Lire maintenant |
| `player.next.cancel` | Cancel | Annuler |
| `player.next.auto` | Play next episode automatically | Lire l’épisode suivant automatiquement |
| `player.stillWatching.title` (warm) | Still watching? | Tu regardes toujours ? |
| `player.stillWatching.continue` | Continue | Continuer |
| `player.stillWatching.stop` | Stop | Arrêter |
| `player.endOfShow` (serif, warm) | That’s the last episode of {{show}} for now. | C’était le dernier épisode de {{show}} pour l’instant. |
| `player.endOfShow.back` | Back to {{show}} | Retour à {{show}} |
| `player.resume.title` | Pick up where you left off? | Reprendre là où tu en étais ? |
| `player.resume.text` | You stopped at {{position}} in {{title}}. | Tu en étais à {{position}} dans {{title}}. |
| `player.stopped.title` | Playback stopped. | Lecture arrêtée. |
| `player.stopped.body` | Your place is saved. | Ta position est enregistrée. |
| `player.stopped.restart` | Start again from {{position}} | Reprendre à {{position}} |
| `player.cantPlay` | This video can’t play in this browser. | Cette vidéo ne peut pas être lue dans ce navigateur. |
| `player.cantPlay.body` | Copy the link and open it in VLC, or in another app that plays network streams. | Copie le lien et ouvre-le dans VLC, ou dans une autre app qui lit les flux réseau. |
| `player.copyVlcLink` | Copy link for VLC | Copier le lien pour VLC |
| `player.lowerQuality` | Lower quality | Baisser la qualité |
| `player.tryAgain` | Try again | Réessayer |
| `playback.method.direct_play` (F006) | Direct play | Lecture directe |
| `playback.method.direct_stream` (F006) | Direct stream | Flux direct |
| `playback.method.transcode` (F006) | Converting | Conversion |
| `errors.transcode_limit_reached` (F006) | The server is busy converting other videos. Try again in a moment, or pick a lower quality. | Le serveur convertit déjà d’autres vidéos. Réessaie dans un instant, ou choisis une qualité plus basse. |
| `errors.transcode_limit_reached.retryIn` | Try again in {{seconds, number}} s | Réessayer dans {{seconds, number}} s |
| `errors.transcode_storage_full` (F006) | The server is out of space for video conversion. Ask the admin to free some up. | Le serveur n’a plus de place pour convertir la vidéo. Demande à l’admin d’en libérer. |
| `errors.file_unavailable` (F006, changed) | This file can’t be found right now. Its drive may be unplugged. It plays again as soon as the drive is back. | Ce fichier est introuvable pour l’instant. Son disque est peut-être débranché. La lecture reprend dès qu’il revient. |
| `errors.ffmpeg_missing` (F006) | This video needs FFmpeg, which isn’t installed on the server. | Cette vidéo a besoin de FFmpeg, qui n’est pas installé sur le serveur. |
| `errors.ffmpeg_missing.admin` | You’re the admin: install it on the server, then try again. | Tu es l’admin : installe-le sur le serveur, puis réessaie. |
| `errors.ffmpeg_missing.member` | Ask {{adminName}} to install it. | Demande à {{adminName}} de l’installer. |
| `errors.file_not_analyzed` | This file is still being analyzed. Try again in a minute. | Ce fichier est encore en cours d’analyse. Réessaie dans une minute. |
| `errors.not_playable` | This file can’t be played. | Ce fichier ne peut pas être lu. |
| `errors.profile_required` (F006) | Pick a profile first. | Choisis d’abord un profil. |

**Keyboard shortcuts dialog** (`player.key.*`): one key per R42 line (`playPause` Play or pause / Lire ou mettre en pause, `seek` Back or forward 10 seconds / Reculer ou avancer de 10 secondes, `volume` Volume up or down / Monter ou baisser le volume, `mute` Mute or unmute / Couper ou rétablir le son, `fullscreen` Full screen / Plein écran, `subtitles` Subtitles on or off / Activer ou couper les sous-titres, `next` Next episode / Épisode suivant, `jump` Jump to 0 % to 90 % / Aller de 0 % à 90 %, `startEnd` Start or end / Début ou fin, `close` Close a menu, then leave full screen / Fermer un menu, puis quitter le plein écran, `search`), plus `player.shortcuts.hint` Press ? to see this again. / Appuie sur ? pour revoir cette liste.; key names (Space, Esc, ←) use the platform's names and aren't translated.

## Non-functional requirements
- **Time to play** (local network, from pressing Play to the first frame): Direct Play < 2 s, Direct Stream < 3 s, Transcode within F006's start times + 1 s. Measured on a 1080p H.264 MP4 and a 4K HEVC MKV.
- **Home**: first meaningful paint (shell, ambient backdrop requested, first shelf of cards) under 1.5 s on a local network from a warm server; `GET /home` under 150 ms on a Raspberry Pi 4 with 2,000 movies and 200 shows.
- **Grid**: 60 fps scrolling (no long task over 50 ms while scrolling) with 5,000 posters on a 2020 mid-range laptop; at most 300 cards in the DOM; posters requested at the displayed width (`srcset` over F005's widths), lazy-loaded, never above `w500` in grids.
- **Search**: `GET /search` p95 under 100 ms on a Pi 4 with 10,000 items and 30,000 people; results shown under 300 ms after the last keystroke on a local network.
- **Hero previews** (R52): for a title already cut, the scenes play within 4 s of opening the page (2 s of backdrop, then the fade). Cutting ahead of time takes 4–8 s per 1080p title on two threads of the reference VM, a few minutes for a 4K HEVC, and yields while anyone transcodes. A preview weighs about 2 MB of video (measured on the test server: 1.6 GB for 791 titles, silent recipe) plus about 0.4 MB of sound: about 2 GB for 800 titles.
- **Server cost**: the web never polls except the admin's activity button (scans and metadata every 5 s, 2 s while open; streams every 30 s; visible tab only) and the player's progress (10 s). Home rows page on demand. Artwork responses are immutable (F005-R19), so revisits hit the browser cache.
- **Bundle**: hls.js and the player are a lazy chunk; the home's initial JS (gzip) stays under 150 kB beyond F007's shared code.
- **Accessibility**: WCAG 2.2 AA. Cards are links with the title as name, rows are lists with a heading, the grid exposes its size (`aria-rowcount`) and position, every control in the player is a labeled button reachable by keyboard, the timeline is a slider with the time in words (`aria-valuetext`), subtitles are on when the profile chose them, focus is visible on every card and control, `prefers-reduced-motion` stops crossfades and the controls' fade, touch targets ≥ 44 px.
- **Privacy**: no request to a third party before the user asks for it (the trailer is the only one, R20); images and previews come from the server, never from TMDB or YouTube.
- **Security**: everything the profile can see is decided by the server (F005-R25 on every new route); the web never filters for age on its own. Stream URLs carry their own token (F006-R25) and are never logged by the web app or put in error reports.
- **Browsers**: the last two versions of Chrome, Edge, Firefox and Safari (macOS and iOS/iPadOS 17.1+). On iOS, playback relies on native HLS.
- **i18n**: no hardcoded string (`make check-i18n`); German-length layouts hold.

## ⚠️ Watch-outs
Settled by gh-prim on 2026-10-29 (the recommendations were taken). ✅ = resolved, ⏸ = tracked elsewhere.

- [x] **No canvas for these screens** — ✅ A Claude Design prompt ships with the plan ([claude-design-prompt.md](claude-design-prompt.md)). The server steps don't depend on it; the web steps take the canvas as their input, and the PR attaches PNG exports of what it builds. The catalogs win over the canvas once merged.
- [x] **Server additions in a "web" feature** — ✅ Kept server-side (R26–R32); `apps: [server, web]`, like F007. The native apps reuse them.
- [x] **FTS5 in the SQLite driver** — ✅ Kept. The plan's first server step proves FTS5 with `remove_diacritics` and prefix indexes on amd64 and arm64 before anything relies on it; the normalized `LIKE` fallback is the plan's rejected-but-ready option.
- [x] **"Profile picker on launch"** — ✅ R2 as written: once per browser tab session when the account has two or more profiles, and whenever no profile is active.
- [x] **"Who's under the plaid?" vs the kill list** — ✅ Kept: the intent asks for it, and the profile picker is an emotional moment (serif, warm), not a functional control.
- [x] **Starting quality from measured bandwidth** — ✅ The server's default (best within the effective limit), lowered by the user. ⏸ A "lower the quality?" prompt after repeated stalls is backlog #123.
- [x] **Why a session ended (backlog #119)** — ⏸ Stays in the backlog. R39 says "Playback stopped." for every end.
- [x] **Trailers and privacy** — ✅ R20 as written: `youtube-nocookie.com`, loaded only after the click. ⏸ The `frame-src` a future Content Security Policy needs is backlog #124.
- [x] **No intro/credits detection, no trickplay, no chapters** — ✅ The next-episode card shows in the last 30 s (R43); thumbnails and chapters wait for their own features.
- [x] **Browser codec support is uneven** — ✅ R34's detection decides, R45 retries once with a transcode; the plan has a manual matrix (Chrome, Firefox, Safari macOS, Safari iOS) on F006's sample files.
- [x] **iPhone without MSE** — ✅ iOS and iPadOS 17.1+ is the floor, through native HLS.
- [x] **Genre rows across libraries** — ✅ Grouped by TMDB genre id (`item_genres.genre_id`); the name shown comes from a library in the UI's language when there is one (R31). The `genre` filter takes ids (R26).
- [x] **Random hero and rows on a Pi** — ✅ As written in R31. (2026-10-30: the hero became the ambient top, with no randomness; genre rows keep their daily order.)
- [x] **Hidden items and `total`** — ✅ Counts are always after the age predicate, everywhere.
- [x] **No PWA** — ✅ Settled in the intent.

## Acceptance criteria
**Shell and profiles**
- [ ] Given an account with three profiles and a new browser tab, when I open `/`, then the picker titled "Who's under the plaid?" shows; when I pick a profile with a PIN and type it, then `POST /session/profile` is sent and the home loads for that profile.
- [ ] Given an account with one profile, when I open `/`, then the home shows without the picker.
- [ ] Given a profile with `locale: "fr"` on an English account, when I pick it, then the UI switches to French without a reload.
- [ ] Given an admin on a 1440 px screen, then the top bar shows the activity button with who watches what, and it opens `/admin`; given a member, then it isn't there and `GET /playback` is never called.
- [ ] Given a child profile, then the grid's counts are the ones the profile can see.
- [ ] Given a 375 px wide screen, then the tabs float at the bottom, the libraries are in the bottom tab bar and nothing scrolls horizontally except the rows.

**Home**
- [ ] Given a movie stopped at 1:02:13 with a backdrop, when I open the home, then it is the hero with "Resume", and Continue watching lists it first with a progress bar and the time left.
- [ ] Given nothing in progress, then the Continue watching row isn't shown and the ambient backdrop is the most recently added item with a backdrop.
- [ ] Given two libraries, then the home shows "Recently added in Movies" and "Recently added in Shows", the latter ordered by each show's newest episode.
- [ ] Given a row with 45 items, when I scroll it to the end, then the next page is fetched with `GET /home/rows/{rowId}?cursor=`.
- [ ] Given a member on a server with no library, then the home says "Nothing here yet." and names the admin.
- [ ] Given a movie without a poster, then its card shows the missing-artwork state with its title.

**Libraries**
- [ ] Given a library of 5,000 movies, when I scroll the grid from top to bottom, then the DOM never holds more than 300 cards and the scrollbar's size is right from the first page.
- [ ] Given I drag the scrollbar to the middle, then the page at that offset is fetched first (`offset=2400`) and skeletons show until it arrives.
- [ ] Given sort "Recently added", "Unwatched" and genre "Thriller", then the request carries `sort=added&order=desc&watch=unwatched&genre=53&total=true` (53 is TMDB's Thriller), the header count is the response's `total`, and the URL holds the same query.
- [ ] Given I open a movie from the grid and press Back, then the grid is back at the same scroll position with the same filters.
- [ ] Given filters with no result, then "Nothing matches these filters." shows with "Clear filters".
- [ ] Given a profile with max age 10, when it opens the grid, then `total` and the facets' counts don't include items above 10 or unrated items (unless allowed).

**Detail pages**
- [ ] Given a movie with a logo, a backdrop and ambient colors, then the page shows the logo over the backdrop with a scrim from `colors.dark`, and the text color follows `onDominant`.
- [ ] Given a movie with `positionMs` 3,733,000, then the primary action reads "Resume from 1:02:13" and "Start over" sits next to it.
- [ ] Given a movie with two editions, then "Version" lists both and the player starts with the chosen `fileId`.
- [ ] Given the movie page, when nothing is pressed, then no request goes to any YouTube host; when I press Trailer, then the `youtube-nocookie.com` embed loads with the official trailer in my language if there is one.
- [ ] Given a show where S1 E3 is watched and E4 is at 12 min, then the main action reads "Resume S1 E4", E1–E3 have a watched mark and E4 a progress bar.
- [ ] Given a show page with `?season=2`, when I reload, then Season 2 is selected.
- [ ] Given a person in three movies on the server, then their page lists those three under Acting, newest first.
- [ ] Given an admin on a movie page, when they pick another poster in "Change artwork", then `PUT /items/{id}/artwork/poster` is sent and the new poster shows; given a member, then there's no Edit menu.
- [ ] Given a link to an item above the profile's age limit, then the page says "This title isn't available on this profile." and makes no playback or image request for it.

**Search**
- [ ] Given *Alien*, *Aliens*, *Alice in Wonderland* and the person *Alicia Vikander*, when I type "ali", then results show under Movies and People within 300 ms of the last keystroke.
- [ ] Given *Amélie*, when I type "amel", then it's found; given *The Lord of the Rings*, when I type "rings", then it's found.
- [ ] Given I type "a", "al", "ali" quickly, then at most one request is in flight and the results match "ali".
- [ ] Given results, when I press ↓ twice and Enter, then the second result opens; when I press Esc, then the dropdown closes.
- [ ] Given a child's profile, when it searches for a title above its limit, then that title isn't in the results.

**Player**
- [ ] Given Chrome and an H.264/AAC MP4, when I press Play, then the session is `direct_play`, the `<video>` plays the `url`, and the first frame shows in under 2 s on the local network.
- [ ] Given Firefox and an HEVC MKV, then the request's profile doesn't list `hevc`, the session is `transcode`, and hls.js plays it.
- [ ] Given Safari, then HLS sessions play through native HLS and hls.js isn't loaded.
- [ ] Given hls.js and an HEVC MKV transcoded by the server, when I let it play 10 minutes, then it plays through without a stall on a local network (the hls.js half of F006's criterion).
- [ ] Given playback, then `POST /playback/{id}/progress` is sent every 10 s while playing and at once on pause and after a seek; when I close the tab, then `DELETE /playback/{id}` is sent with `keepalive` and the position.
- [ ] Given I pick a French text subtitle, then it shows without a new request to `PATCH /playback/{id}`.
- [ ] Given I pick the French audio track, then `PATCH /playback/{id}` is sent with the current position, and the new `url` resumes within 2 s of that position.
- [ ] Given `PATCH` answers `503 transcode_limit_reached`, then the current stream keeps playing and the error toast shows.
- [ ] Given I chose French subtitles in one movie, when I start another movie with a French track, then French subtitles are on.
- [ ] Given the player, when I press Space, ←, F, M, C and "?", then it pauses, seeks back 10 s, goes full screen, mutes, cycles subtitles and shows the shortcuts.
- [ ] Given S1 E4 with a next episode, when 30 s are left, then the "Next: S1 E5" card shows; when the video ends, then a 10 s countdown plays E5 in the same player, and E4's session is stopped with its position.
- [ ] Given the last episode of a season, then the next one is S2 E1; given the last episode of the show, then the end-of-show line shows and nothing plays.
- [ ] Given three episodes started by the countdown with no input, then the fourth countdown is replaced by "Still watching?".
- [ ] Given an admin stops the stream, when the next progress report gets `404`, then the video stops and "Playback stopped." shows with "Start again from {{position}}".
- [ ] Given a file the browser fails to decode in Direct Play, then the player retries once with `forceMethod: transcode`; if that fails too, "This video can't play in this browser." shows with "Copy link for VLC".
- [ ] Given "Playback info" on a transcode, then the method, reasons as sentences, quality and hardware or software are shown in words.

**Server**
- [ ] Given `GET /libraries/{id}/items?sort=rating&order=desc&total=true&limit=50`, then items come by rating, highest first, with `total` and `addedAt`.
- [ ] Given `genre=35&genre=18` (Comedy, Drama), then items with either genre are returned.
- [ ] Given `sort=random&seed=42` twice, then both pages come in the same order.
- [ ] Given `GET /libraries/{id}/facets`, then genres come with counts by count and letters in title order with `#` for digits.
- [ ] Given `GET /search?q=amel` with *Amélie* in a library, then it's in `movies`; given the item is above the profile's age limit, then it isn't.
- [ ] Given a new movie scanned and matched, when I search its title, then it's found without a restart.
- [ ] Given a show with S1 E1–E3 watched, then `GET /items/{showId}/up-next` returns S1 E4; given the last episode of S1, then `up-next` on it returns S2 E1; given the last episode of the show, then `204`.
- [ ] Given `GET /home` for a profile with nothing in progress, then there's no `continue_watching` row, `ambient` is the most recently added item with a backdrop, and the genre rows keep their order on two calls the same day.

**V2**
- [ ] Given a movie whose preview is cut, when I open its page, then the backdrop shows, and the scenes fade in within 4 s with their sound (muted when the browser refuses sound or the profile turned it off), with a sound button and a pause button; when I scroll the hero out of view, then they pause.
- [ ] Given a library just scanned, when nobody plays anything, then the previews are cut ahead of time, newest first; given a transcode starts, then no preview is cut ahead until it ends.
- [ ] Given a title not cut yet, when I open its page, then `GET /items/{id}/preview` answers `202` with `Retry-After`, the title is cut next, and the scenes appear without a reload.
- [ ] Given a show, then its preview comes from S1 E1 (or the first episode on disk); given a file under 5 minutes, then there's no preview.
- [ ] Given "Play previews automatically" off, or `prefers-reduced-motion`, then no preview is requested.
- [ ] Given a movie with PGS subtitles in Chrome, when I pick them, then they're drawn over the picture without a new decision (`burn_in` is not used) and appear within 3 s.
- [ ] Given an account with two profiles without a PIN, when I pick the other one in the profile menu, then the app switches to it without opening the picker.

**Cross-cutting**
- [ ] Given every browsing screen at 375 px in French with German-length strings, then nothing overflows and there's no horizontal page scroll.
- [ ] Given the light theme, then the home, grid and detail pages keep AA contrast; the player stays dark.
- [ ] Given `make check-i18n`, then no key is missing in `fr` and no screen has a hardcoded string.
- [ ] Given keyboard only, then I can pick a profile, search, open a movie, start it, change subtitles and leave the player.

## Revisions
- 2026-10-29 (planning): the package is `internal/catalog` (`library/browse.go` already means the folder picker); `item_genres`, `newest_child_added_at` and the search index are kept in sync by SQLite triggers, and a `search_docs` table gives each FTS row a stable rowid (plan step 3).
- 2026-10-29: approved. Genres are grouped and filtered by TMDB genre id (R26, R27, R31, data model). Backlog #123 (stall prompt) and #124 (CSP `frame-src`).
- 2026-10-30: aligned on the canvas (pages Browse and Player). Desktop shell is a sidebar with library counts and the admin server card, the search in a top bar (R1); the home hero is gone: `GET /home` returns an `ambient` item whose backdrop tints the top of the home, chosen without randomness (R5, R6, R31); `LibraryFacets.total` for the sidebar counts (R27); copy from the canvas decks (14 changes, keys added for PIN, server card, grid, detail, search and player; `player.next.title` becomes `player.next.label`; `home.hero.moreInfo` removed); canvas rules in UI. Approval stands: the owner asked for this alignment.
- 2026-10-30: R51 added. In the TV direction the ambient light of home, grids and detail pages follows the selected movie or show, crossfading on hover and focus. Pending the canvas alignment of the TV direction.
- 2026-10-31: R52 preview sound in the viewer's language (the profile's last audio choice, then the library's language) instead of the file's default track, at the owner's request (multi-language files played Portuguese).
- 2026-10-31: R52 previews get their sound and a sound button, at the owner's request; the cache recipe changes version, so existing previews are cut again.
- 2026-10-31: aligned on the canvas page "V2" (Steps 1–12) and what shipped on `feat/F008-polish`. Shell: a top bar with a glass tab bar and the admin activity button instead of the sidebar and server card (R1); the profile picker lit by `Profile.ambient` (R2); a framed home hero instead of the ambient top (R6, art direction overridden by V2); detail heroes framed, with parallax (R17); the V2 search (R25); subtitles drawn by the browser, PGS included, fetched the quick way (R34, R35; F006 contract: `SubtitleCapability.separate`, the `.sup` route, `?from=`, `X-Subtitle-Until`); R52 hero previews (`GET /items/{id}/preview`, cut ahead of time), R53 motion, R54 profile menu; i18n, NFRs and acceptance criteria to match. Changes made at the same time outside F008 (the F007 admin dashboard and settings with `GET /server/stats`, the F006 software transcode cap at 1080p, the F005 artwork retries and `refreshing` count) belong to those features' specs and are not aligned here. Approval stands: the owner asked for this alignment.
