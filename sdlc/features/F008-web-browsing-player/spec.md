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
F008 turns PLAID's web app into the place where you watch. After the profile picker ("Who's under the plaid?"), the **home** opens on a hero and rows: Continue watching, Recently added per library, then a few genres. Each **library** is a poster grid that stays smooth with thousands of titles, with sorting and filters. **Movie and show pages** sit on the title's backdrop and ambient colors, with the cast, the trailer, seasons and episodes. **Search** answers while you type, across libraries and people. The **player** plays what F006 decides (Direct Play in `<video>`, HLS through the browser or hls.js), resumes where you stopped, switches quality, audio and subtitles, has keyboard shortcuts and plays the next episode on its own. It's a responsive web app for desktop, tablet and phone browsers, not a PWA: phones and TVs get the native apps (phases 4 and 5, ADR 0005). F008 is mostly web on top of the F003–F006 API. It adds the server pieces F004 and F005 left to it (sorting, filters, search, "recently added") and two small ones the player needs (a home endpoint and "up next"), so the native apps reuse them instead of rebuilding them on the client.

## User journeys

**Member, Friday night, on the laptop**
1. They open `http://mediabox:32500`. They're signed in, the account has three profiles, so the picker shows: "Who's under the plaid?". They pick "Alex".
2. Home: the hero shows the backdrop and logo of *Dune: Part Two*, which Alex stopped at 1:02:13 yesterday, with "Resume" and "More info". Below: Continue watching (landscape cards with the time left), Recently added in Movies, Recently added in Shows, Science Fiction, Comedy.
3. "Resume" opens the player at 1:02:13, in under 3 s on the local network. Space pauses, → skips 10 s, F goes full screen.
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
| F008-R1 | Signed-in shell for every browsing route: a header with the PLAID mark (to `/`), one entry per library visible to the profile (in `GET /libraries` order), the search field, the profile menu (switch profile, pair a TV, account, sign out) and, for admins, the server strip (F007-R17) with the server name and the number of active streams (`GET /playback`, polled every 30 s while visible). On phones (< 768 px), the libraries move to a bottom tab bar (Home, one per library up to 3 then "More", Search). The player has no shell. | must |
| F008-R2 | Profile picker: the existing F003 screen restyled on the design system, titled "Who's under the plaid?" (`profiles.title`), avatar tiles with the profile's `avatarColor`, PIN entry for profiles that have one. It shows when the session has no active profile (F003-R17) and, when the account has two or more profiles, at the first route of each browser tab session (a `sessionStorage` flag); choosing calls `POST /session/profile`. "Switch profile" in the profile menu opens it at any time. With one profile, it never shows on its own. | must |
| F008-R3 | The active profile's `locale`, when set, overrides the account locale for the UI, through the existing `createI18n` order. Switching profile re-renders in the new profile's language without a reload. | should |
| F008-R4 | Theme: dark by default, light from the profile menu ("Appearance": System, Dark, Light), remembered per browser in `localStorage`. The player is always dark. | should |
| **Home** | | |
| F008-R5 | Home (`/`) from `GET /home` (R31): a hero, then the rows the server returns, in its order, each a horizontal row with a title and "See all" when the row has a destination. Rows with no items aren't shown. Each row loads its first 20 items with the page; scrolling a row to its end loads the next page. | must |
| F008-R6 | Hero: one item, chosen by the server (R31): its largest backdrop (`w1920` on desktop, `w780` on phone), its logo when there is one (else the title in the display face), year, runtime or seasons count, certification, two lines of synopsis, "Resume" (with the time left) or "Play", and "More info". The hero's text color follows the backdrop's `colors.onDominant`, over a scrim from `colors.dark`. | must |
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
| F008-R17 | Movie page (`/item/:id`, `GET /items/{id}`): full-bleed backdrop (`w1920`/`w780`) with a scrim from its `colors`, logo or title, original title when different, year, runtime, certification, TMDB rating with its count, genres (each links to the library filtered by it), technical badges (resolution, HDR format, Dolby Vision, audio languages named with `Intl.DisplayNames`), tagline, overview, director and writers, studios, collection link. Fields that fell back to English (`fieldLanguages`) carry `lang="en"`. | must |
| F008-R18 | Primary actions: "Play", or "Resume from {{position}}" with "Start over" next to it when `userState.positionMs > 0`; "Mark as watched"/"Mark as unwatched"; "Trailer" when there is one. With several files (editions, versions), a "Version" selector lists them by edition, resolution and HDR; the default is the server's choice (F006-R3). An item whose files are all missing shows "This title's files can't be found right now." and no Play. | must |
| F008-R19 | Cast row: round portraits (`w185`), name and character, up to 20, each to the person page; then "Crew" lines for directors and writers. | must |
| F008-R20 | Trailer: nothing from YouTube loads until "Trailer" is pressed. Then a dialog embeds `https://www.youtube-nocookie.com/embed/{key}` with autoplay; Esc or Close ends it. Prefer an official trailer in the UI language, then an official one in English, then any. "Open on YouTube" links out. | must |
| F008-R21 | Show page (`/item/:id` on a show): the same header as R17 with first and last air year, status ("Ended", "Returning"), seasons count; the main action from R32 ("Play S1 E1", "Resume S2 E3", "Play S3 E1" when the next one is unwatched); seasons as tabs (`?season=N`, specials last), each season's episodes from `GET /libraries/{id}/items?parentId=` with still (`w342`), number, title, air date, runtime, two lines of overview, watched mark or progress bar, and Play on hover/focus. "Mark season as watched" on the season. Episodes the server knows nothing about show their parsed name. | must |
| F008-R22 | Person page (`/person/:id`, `GET /people/{id}`): portrait, name, and what they're in on this server as poster cards, grouped by role (Acting, Directing, Writing, Creator), newest first. | must |
| F008-R23 | Admin tools on detail pages (admins only, behind an "Edit" menu): "Change artwork" opens the choices from `GET /items/{id}/artwork` per kind (poster, backdrop, logo) and applies one with `PUT /items/{id}/artwork/{kind}` and "Back to automatic" with `DELETE` (F005's picker, moved here by F007); "Fix match" links to the F007 review screen for the item; "Refresh metadata" calls `POST /items/{id}/refresh`. | should |
| F008-R24 | Unavailable item: an id the profile may not see, or that doesn't exist, returns 404; the page says "This title isn't available on this profile." with "Back to home". It never says which of the two it is. | must |
| **Search** | | |
| F008-R25 | Instant search (`/search?q=`, and a dropdown under the header field on desktop): results from `GET /search` (R28) after each keystroke, debounced 150 ms, from the first character; a request in flight is aborted when a new one starts. Groups: Movies, Shows, Episodes, People, each up to 8 with "See all" to the full list. Arrow keys move through results, Enter opens the highlighted one (the first by default), Esc closes. The query is in the URL. No result: "Nothing called “{{query}}” here." `/` focuses the search from anywhere outside a text field. | must |
| **Server additions** | | |
| F008-R26 | `GET /libraries/{id}/items` gains, for top-level items (no `parentId`): `sort` (`title`, `added`, `released`, `rating`, `played`, `random`), `order` (`asc`, `desc`), `seed` (for `random`), filters `watch` (`unwatched`, `in_progress`, `watched`), `genre` (repeatable TMDB genre id, any of), `yearFrom`/`yearTo`, `resolution` (repeatable: `2160p`, `1080p`, `720p`, `sd`), `hdr` (boolean), `offset` (instead of `cursor`, for random access) and `total=true` (adds `total` to `ItemPage`). `ItemSummary` gains `addedAt` (`items.created_at`; for a show, its newest episode's). Every filter applies F005-R25's age predicate before counting, so `total` never reveals hidden items. Unknown values are `400 validation_failed`. | must |
| F008-R27 | `GET /libraries/{id}/facets` returns, over the items the active profile may see: `genres: [{id, name, count}]` (by count; `name` as stored for this library), `decades: [{from, count}]`, `resolutions: [{value, count}]`, `hdr: count`, and `letters: [{letter, count}]` in title sort order (digits and symbols grouped as `#`). | must |
| F008-R28 | `GET /search?q=&limit=` searches every library the profile may see: movies and shows by title and original title, episodes by title, people by name. Matching is case- and accent-insensitive, on word prefixes ("ali" finds *Alien* and *Alice*, "rings" finds *The Lord of the Rings*), through an SQLite FTS5 index kept in sync by F004/F005 writes. Ranked by exact title first, then prefix, then the BM25 rank, with ties by popularity (rating count). Returns `{movies, shows, episodes, people}`, each a list of `ItemSummary` (episodes with their show's title) or `{id, name, portrait}`, applying the age predicate. `q` under 1 character or over 100 is `400`. | must |
| F008-R29 | The search index covers every library at server start (a one-time build after migration, in the background; search answers from what's indexed meanwhile) and is updated in the same transaction as item and person writes. | must |
| F008-R30 | `ItemSummary` gains `show: {id, displayTitle, logo?}` on seasons and episodes, so Continue watching cards, search results and the next-episode card name the show without another request. | must |
| F008-R31 | `GET /home` returns `{hero, rows}` for the active profile. `hero`: the most recent continue-watching item with a backdrop, else a matched item with a backdrop and a logo picked at random among the 50 most recently added, stable for the profile for the day (seeded by profile id and date). `rows`, in order: `continue_watching` (F006-R37), then per library `recently_added` (movies by `addedAt`; shows by their newest episode), then up to 4 `genre` rows: the genres (grouped by TMDB genre id, so libraries in different languages merge) with at least 10 items across the profile's libraries, the profile's most watched first, then by size, each in random order stable for the day. Each row: `{id, type, title params (libraryId, libraryName, genre: {id, name}; the name from a library whose metadata language matches the profile's UI language, else the first library's), items (first 20), nextCursor, seeAll}`; `seeAll` names the library and query the "See all" opens. `GET /home/rows/{rowId}?cursor=` pages a row. | must |
| F008-R32 | `GET /items/{id}/up-next` for the active profile: on a show, the episode to play (the one in progress, else the first unwatched after the last watched, else S1 E1; specials skipped); on a season, the same within it; on an episode, the next one by season and episode number across seasons, specials skipped. Returns `{item: ItemSummary}` or `204` when there's none. Applies the age predicate. | must |
| **Player** | | |
| F008-R33 | The player (`/watch/:itemId`, `?fileId=`, `?t=` in seconds) is full-screen in the page, always dark, lazy-loaded with hls.js in its own chunk. It starts a session with `POST /items/{id}/playback` with the R34 profile, the file chosen on the detail page, the start position (resume, `?t=`, or 0), and the profile's last audio and subtitle language choices (R38). | must |
| F008-R34 | Capability profile built once per browser from `MediaSource.isTypeSupported` (or `ManagedMediaSource` on iOS Safari) and `HTMLVideoElement.canPlayType`: containers (`mp4`, `webm`, plus `mkv` only where the browser plays it), video codecs with profiles, levels, bit depth and HDR (`hdr10`/`hlg` only when `matchMedia('(dynamic-range: high)')` and the codec string with the HDR transfer are supported), audio codecs with channels (E-AC-3 and AC-3 only where supported), `subtitles.text: ["vtt"]`, `protocols: ["http", "hls"]`, `canSwitchAudioTrack: false`. It's cached in `sessionStorage` and shown in "Playback info" (R41). | must |
| F008-R35 | Engines: `direct_play` plays the `url` in `<video>` with range requests. HLS (`direct_stream`, `transcode`) uses the browser's native HLS when `canPlayType('application/vnd.apple.mpegurl')` (Safari), else hls.js. Text subtitles are added as `<track kind="subtitles">` from `PlaybackSubtitle.url` (WebVTT), or as the HLS subtitle rendition on native HLS. `burn_in` subtitles need a new decision (R37). Segments that answer `503 segment_not_ready` are retried after `Retry-After`. | must |
| F008-R36 | Controls, shown on mouse move, tap or any key, hidden after 3 s of playback without input: back (to where the player was opened from), title and episode line, play/pause, −10 s / +10 s, the timeline (elapsed, remaining, buffered range, hover time tooltip; no preview thumbnails), volume with mute, subtitles, audio, quality, "Playback info", picture-in-picture where supported, full screen. Touch: tap toggles controls, double-tap left/right seeks 10 s. | must |
| F008-R37 | Track and quality changes: choosing a text subtitle delivered `external` or `embedded` switches at once in the client. Choosing an audio track, a quality, or an image subtitle (`burn_in`) sends `PATCH /playback/{id}` with the current `positionMs`; a changed `url` is reloaded at that position with the controls kept, an unchanged one keeps playing. `503 transcode_limit_reached` keeps the current stream and shows the error toast. | must |
| F008-R38 | Track memory per profile in `localStorage`: the last audio language and subtitle choice (language + forced/SDH, or off) chosen in the player, applied to the next items when a track in that language exists; otherwise the server's defaults (F006-R4, forced subtitles in the audio language). Volume and mute are remembered per browser. | should |
| F008-R39 | Progress: `POST /playback/{id}/progress` every 10 s while playing, and at once on pause, seek (after it settles), buffering start and end; `DELETE /playback/{id}` with `positionMs` on leaving the player, and on `pagehide` with `fetch(…, {keepalive: true})`. A failed report is not retried, the next one carries the position. A `404 playback_session_not_found` (expired, or stopped by an admin) stops the video and shows "Playback stopped." with "Start again from {{position}}". | must |
| F008-R40 | Resume: opening a movie or episode with `positionMs > 0` from a "Resume" button starts there; from a plain "Play" (or a card in a row other than Continue watching), the player asks "Resume from {{position}}" / "Start over" before starting. `?t=` skips the question. | must |
| F008-R41 | "Playback info" panel: method in words (`playback.method.*`), each reason as a sentence (`reason.*`, shared with F007), quality and bitrate, hardware or software conversion, warnings. It helps people understand "Converting" without opening the admin. | should |
| F008-R42 | Keyboard shortcuts, active in the player when no menu has focus: Space or K play/pause, ← / → 10 s, J / L 10 s, ↑ / ↓ volume 10 %, M mute, F full screen, C cycles subtitles (off → the remembered language → off), Shift+N next episode, Esc closes a menu then leaves full screen, Home/End start/end, 0–9 jump to 0–90 %. "?" lists them. Shortcuts are also on the detail pages: Enter on a focused card opens it. | must |
| F008-R43 | Next episode: in the last 30 s of an episode that has an up-next (R32, fetched when playback starts), a card shows the next episode's still, number and title with "Play now" and "Cancel"; when the video ends, a 10 s countdown plays it in the same player (a new session, the current one stopped with its position). "Cancel" lets the episode end on the details. A profile setting "Play next episode automatically" (on by default, `localStorage` per profile) turns the countdown off. | must |
| F008-R44 | "Still watching?": after 3 episodes started by the countdown in a row with no key, click or tap in between, the countdown is replaced by "Still watching?" with "Continue" and "Stop". It saves a sleeping viewer's progress and the server's CPU. | should |
| F008-R45 | Player errors, each in words with what to do: `transcode_limit_reached` (with "Try again" after `Retry-After`, and "Lower quality" when a lower one exists), `transcode_storage_full`, `file_unavailable`, `ffmpeg_missing`, `file_not_analyzed`, `not_playable`, `profile_required` (to the picker), a media error from the browser (`MEDIA_ERR_DECODE`, `MEDIA_ERR_SRC_NOT_SUPPORTED`) → one automatic retry with `forceMethod: transcode`, then "This video can't play in this browser." with "Copy link for VLC" (an `external` session, F006). | must |
| F008-R46 | Media Session API: title, show and episode, artwork (`w500` poster or still), play/pause/seek handlers, so the OS media keys and lock screens work. | could |
| **Cross-cutting** | | |
| F008-R47 | Every string comes from the i18n catalogs (table below); plurals with `_one`/`_other`; numbers, dates, durations, runtimes and language names with `Intl` (i18next formatters, `Intl.DisplayNames`). Genres come translated from TMDB in the library's metadata language and are shown as is. | must |
| F008-R48 | Every browsing screen works from 375 px to 2560 px with no horizontal page scroll (rows scroll on their own), in dark and light, with German-length strings (+35 %). The player works on phones in portrait and landscape. | must |
| F008-R49 | Every server error code these screens can receive has a translated message (existing `errors.*`, plus the F006 codes not added yet); unknown codes fall back to `errors.generic`. | must |
| F008-R50 | Attribution: the footer of the home and of every detail page links to the About section that shows the providers' notices from `GET /server` (F005-R37). | must |

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
| GET | `/libraries/{id}/facets` | `{genres, decades, resolutions, hdr, letters}` (R27) |
| GET | `/search` | `?q=&limit=` (default 8, max 50) → `{movies, shows, episodes, people}` (R28) |
| GET | `/home` | `{hero?: ItemSummary, rows: [HomeRow]}` (R31) |
| GET | `/home/rows/{rowId}` | `?cursor=` → `ItemPage` for one row (R31) |
| GET | `/items/{id}/up-next` | `{item: ItemSummary}` or `204` (R32) |
| — | `ItemSummary` | **Changed**: `addedAt` (R26), `show` on seasons and episodes (R30) |

`HomeRow`: `{id, type: continue_watching | recently_added | genre, libraryId?, libraryName?, genre?: {id, name}, items: [ItemSummary], nextCursor?, seeAll?: {libraryId, sort, order, genre?: id}}`. Row titles are built by the client from `type` and its params (`home.row.*`), never sent as text.

Everything else F008 uses already exists: `/me`, `/session/profile`, `/libraries`, `/items/{id}`, `/items/{id}/artwork*`, `/items/{id}/refresh`, `/images/{id}`, `/collections*`, `/people/{id}`, `/items/{id}/playback`, `/playback/{id}*`, `/items/{id}/watched`, `/me/continue-watching`, `/server`, `/playback` (admin strip).

No new error code.

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
Screens come from the Claude Design canvas made with [claude-design-prompt.md](claude-design-prompt.md) (pages Browse and Player); until it exists, the design system's revised rules below govern, as in F007:
- **The content is the color**: warm near-black ground, the posters and backdrops bring the color, ambient colors (F005-R21) tint the detail header and the hero's scrim; the UI recedes. One amber element per viewport (the primary Play, the focus ring, the timeline's played part).
- **Signature**: the woven tartan lives only in the player's timeline fill and the progress bars on cards.
- **Type**: Instrument Serif only for emotional lines (the profile picker title, empty states, the end of a show); Instrument Sans everywhere else. Titles without a logo use the sans at display size.
- **Motion**: one per surface: the hero backdrop crossfade, the card focus lift, the controls fading. All off with `prefers-reduced-motion`.
- **States** for every screen: loading (skeleton cards, never a spinner over posters), empty, error (inline, with retry), offline (F007-R16 banner), missing artwork, unmatched title (parsed name, quiet "Not identified" for admins only).
- **Copy rule**: warm on the profile picker, the empty states, the end of a series and "Still watching?"; plain and exact everywhere else (buttons, menus, errors, the player).

### i18n
Keys in the repo's i18next format (`{{var}}`, `{{n, number}}`, `_one`/`_other` with `count`). Durations (`{{position}}`, time left, runtime) are formatted with `Intl.DurationFormat` (fallback: a small formatter producing `1 h 02 min` / `1:02:13`), never concatenated. Keys from F006's suggested table are taken as written unless changed below. The existing `profiles.whoIsWatching` is replaced by `profiles.title`; `welcome.*` keys still used by `Home` are removed with it.

**Shell and profiles**

| Key | en | fr |
|-----|----|----|
| `profiles.title` (serif, warm) | Who’s under the plaid? | Qui est sous le plaid ? |
| `profiles.pin.title` | Enter {{name}}’s PIN | Saisis le code de {{name}} |
| `profiles.manage` | Manage profiles | Gérer les profils |
| `nav.home` | Home | Accueil |
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
| `nav.about` | About PLAID | À propos de PLAID |

**Home**

| Key | en | fr |
|-----|----|----|
| `home.hero.moreInfo` | More info | Plus d’infos |
| `home.row.continueWatching` | Continue watching | Reprendre |
| `home.row.recentlyAdded` | Recently added in {{libraryName}} | Ajouts récents dans {{libraryName}} |
| `home.row.genre` | {{genre}} | {{genre}} |
| `home.row.seeAll` | See all | Tout voir |
| `home.empty.member.title` (warm) | Nothing here yet. | Rien ici pour l’instant. |
| `home.empty.member.body` | Ask {{adminName}} to add a library. | Demande à {{adminName}} d’ajouter une bibliothèque. |
| `home.empty.scanning.title` (warm) | Your movies are on their way. | Tes films arrivent. |
| `card.timeLeft` | {{duration}} left | Encore {{duration}} |
| `card.episode` | S{{season, number}} E{{episode, number}} · {{title}} | S{{season, number}} É{{episode, number}} · {{title}} |
| `card.episodeShort` | S{{season, number}} E{{episode, number}} | S{{season, number}} É{{episode, number}} |
| `card.unwatchedEpisodes_one` | {{count, number}} new episode | {{count, number}} nouvel épisode |
| `card.unwatchedEpisodes_other` | {{count, number}} new episodes | {{count, number}} nouveaux épisodes |
| `card.watched` | Watched | Vu |
| `card.menu` | More actions for {{title}} | Plus d’actions pour {{title}} |
| `card.goToShow` | Go to show | Voir la série |
| `item.markWatched` | Mark as watched | Marquer comme vu |
| `item.markUnwatched` | Mark as unwatched | Marquer comme non vu |
| `item.missingArtwork` | No artwork for {{title}} | Pas d’image pour {{title}} |

**Library grid**

| Key | en | fr |
|-----|----|----|
| `grid.count.movies_one` | {{count, number}} movie | {{count, number}} film |
| `grid.count.movies_other` | {{count, number}} movies | {{count, number}} films |
| `grid.count.shows_one` | {{count, number}} show | {{count, number}} série |
| `grid.count.shows_other` | {{count, number}} shows | {{count, number}} séries |
| `grid.sort` | Sort | Trier |
| `grid.sort.title` | Title | Titre |
| `grid.sort.added` | Recently added | Ajout récent |
| `grid.sort.released` | Release date | Date de sortie |
| `grid.sort.rating` | Rating | Note |
| `grid.sort.played` | Recently watched | Vu récemment |
| `grid.sort.random` | Random | Aléatoire |
| `grid.order.asc` | Ascending | Croissant |
| `grid.order.desc` | Descending | Décroissant |
| `grid.filters` | Filters | Filtres |
| `grid.filter.watch` | Watch state | État |
| `grid.filter.watch.all` | All | Tout |
| `grid.filter.watch.unwatched` | Unwatched | Non vus |
| `grid.filter.watch.in_progress` | In progress | En cours |
| `grid.filter.watch.watched` | Watched | Vus |
| `grid.filter.genre` | Genre | Genre |
| `grid.filter.decade` | {{decade}}s | Années {{decade}} |
| `grid.filter.resolution` | Resolution | Résolution |
| `grid.filter.resolution.sd` | 720p and lower | 720p et moins |
| `grid.filter.hdr` | HDR | HDR |
| `grid.filter.remove` | Remove filter {{name}} | Retirer le filtre {{name}} |
| `grid.filter.clear` | Clear all | Tout effacer |
| `grid.empty.title` | Nothing matches these filters. | Rien ne correspond à ces filtres. |
| `grid.empty.action` | Clear filters | Effacer les filtres |
| `grid.collections` | Collections | Collections |
| `grid.letters` | Jump to letter | Aller à la lettre |

**Detail pages**

| Key | en | fr |
|-----|----|----|
| `item.play` | Play | Lecture |
| `playback.resume` (F006) | Resume from {{position}} | Reprendre à {{position}} |
| `playback.startOver` (F006) | Start over | Reprendre du début |
| `item.playEpisode` | Play S{{season, number}} E{{episode, number}} | Lire S{{season, number}} É{{episode, number}} |
| `item.resumeEpisode` | Resume S{{season, number}} E{{episode, number}} | Reprendre S{{season, number}} É{{episode, number}} |
| `item.trailer` | Trailer | Bande-annonce |
| `item.trailer.openYouTube` | Open on YouTube | Ouvrir sur YouTube |
| `item.version` | Version | Version |
| `item.runtime` | {{duration}} | {{duration}} |
| `item.rating` | {{rating, number}} ({{count, number}} votes) | {{rating, number}} ({{count, number}} votes) |
| `item.originalTitle` | Original title: {{title}} | Titre original : {{title}} |
| `item.directedBy` | Directed by {{names, list}} | Réalisé par {{names, list}} |
| `item.writtenBy` | Written by {{names, list}} | Écrit par {{names, list}} |
| `item.createdBy` | Created by {{names, list}} | Créée par {{names, list}} |
| `item.cast` | Cast | Distribution |
| `item.collection` | Part of {{name}} | Fait partie de {{name}} |
| `item.seasons_one` | {{count, number}} season | {{count, number}} saison |
| `item.seasons_other` | {{count, number}} seasons | {{count, number}} saisons |
| `item.season` | Season {{number, number}} | Saison {{number, number}} |
| `item.specials` | Specials | Épisodes spéciaux |
| `item.markSeasonWatched` | Mark season as watched | Marquer la saison comme vue |
| `item.status.ended` | Ended | Terminée |
| `item.status.returning` | Returning | En cours |
| `item.status.canceled` | Canceled | Annulée |
| `item.status.in_production` | In production | En production |
| `item.filesMissing` | This title’s files can’t be found right now. | Les fichiers de ce titre sont introuvables pour l’instant. |
| `item.unavailable.title` | This title isn’t available on this profile. | Ce titre n’est pas disponible sur ce profil. |
| `item.unavailable.back` | Back to home | Retour à l’accueil |
| `item.edit` | Edit | Modifier |
| `item.edit.artwork` | Change artwork | Changer l’image |
| `item.edit.artwork.poster` | Poster | Affiche |
| `item.edit.artwork.backdrop` | Backdrop | Arrière-plan |
| `item.edit.artwork.logo` | Logo | Logo |
| `item.edit.fixMatch` | Fix match | Corriger l’identification |
| `item.edit.artwork.automatic` | Back to automatic | Revenir au choix automatique |
| `item.edit.refresh` | Refresh metadata | Actualiser les métadonnées |
| `item.edit.refreshed` | Refreshing. It can take a minute. | Actualisation en cours. Ça peut prendre une minute. |
| `person.role.cast` | Acting | Interprétation |
| `person.role.director` | Directing | Réalisation |
| `person.role.writer` | Writing | Scénario |
| `person.role.creator` | Created | Création |
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

**Player**

| Key | en | fr |
|-----|----|----|
| `player.back` | Back | Retour |
| `player.play` | Play | Lecture |
| `player.pause` | Pause | Pause |
| `player.back10` | Back 10 seconds | Reculer de 10 secondes |
| `player.forward10` | Forward 10 seconds | Avancer de 10 secondes |
| `player.timeline` | Timeline | Barre de lecture |
| `player.remaining` | {{duration}} left | Encore {{duration}} |
| `player.volume` | Volume | Volume |
| `player.mute` | Mute | Couper le son |
| `player.unmute` | Unmute | Rétablir le son |
| `player.subtitles` | Subtitles | Sous-titres |
| `player.subtitles.off` | Off | Désactivés |
| `player.subtitles.forced` | {{language}} (forced) | {{language}} (forcés) |
| `player.subtitles.sdh` | {{language}} SDH | {{language}} SME |
| `player.audio` | Audio | Audio |
| `player.audio.track` | {{language}} · {{channels}} | {{language}} · {{channels}} |
| `player.channels.mono` | Mono | Mono |
| `player.channels.stereo` | Stereo | Stéréo |
| `player.channels.surround` | {{layout}} | {{layout}} |
| `player.quality` | Quality | Qualité |
| `player.quality.original` | Original | Originale |
| `player.quality.current` | {{quality}} · {{bitrate}} | {{quality}} · {{bitrate}} |
| `player.info` | Playback info | Infos de lecture |
| `player.info.hardware` | Converted on the GPU | Converti par la carte graphique |
| `player.info.software` | Converted by the processor | Converti par le processeur |
| `player.pip` | Picture in picture | Image dans l’image |
| `player.fullscreen` | Full screen | Plein écran |
| `player.exitFullscreen` | Exit full screen | Quitter le plein écran |
| `player.shortcuts` | Keyboard shortcuts | Raccourcis clavier |
| `player.next.title` | Next: {{episode}} | Ensuite : {{episode}} |
| `player.next.countdown_one` | Playing in {{count, number}} second | Lecture dans {{count, number}} seconde |
| `player.next.countdown_other` | Playing in {{count, number}} seconds | Lecture dans {{count, number}} secondes |
| `player.next.playNow` | Play now | Lire maintenant |
| `player.next.cancel` | Cancel | Annuler |
| `player.next.auto` | Play next episode automatically | Lire l’épisode suivant automatiquement |
| `player.stillWatching.title` (warm) | Still watching? | Tu regardes toujours ? |
| `player.stillWatching.continue` | Continue | Continuer |
| `player.stillWatching.stop` | Stop | Arrêter |
| `player.endOfShow` (serif, warm) | That’s the last episode of {{show}} for now. | C’était le dernier épisode de {{show}} pour l’instant. |
| `player.resume.title` | Pick up where you left off? | Reprendre là où tu t’étais arrêté ? |
| `player.stopped.title` | Playback stopped. | Lecture arrêtée. |
| `player.stopped.restart` | Start again from {{position}} | Reprendre à {{position}} |
| `player.cantPlay` | This video can’t play in this browser. | Cette vidéo ne peut pas être lue dans ce navigateur. |
| `player.copyVlcLink` | Copy link for VLC | Copier le lien pour VLC |
| `player.lowerQuality` | Lower quality | Baisser la qualité |
| `player.tryAgain` | Try again | Réessayer |
| `playback.method.direct_play` (F006) | Direct play | Lecture directe |
| `playback.method.direct_stream` (F006) | Direct stream | Flux direct |
| `playback.method.transcode` (F006) | Converting | Conversion |
| `errors.transcode_limit_reached` (F006) | The server is busy converting other videos. Try again in a moment, or pick a lower quality. | Le serveur convertit déjà d’autres vidéos. Réessaie dans un instant, ou choisis une qualité plus basse. |
| `errors.transcode_storage_full` (F006) | The server is out of space for video conversion. Ask the admin to free some up. | Le serveur n’a plus de place pour convertir la vidéo. Demande à l’admin d’en libérer. |
| `errors.file_unavailable` (F006) | This file can’t be found right now. Its drive may be unplugged. | Ce fichier est introuvable pour l’instant. Son disque est peut-être débranché. |
| `errors.ffmpeg_missing` (F006) | This video needs FFmpeg, which isn’t installed on the server. | Cette vidéo a besoin de FFmpeg, qui n’est pas installé sur le serveur. |
| `errors.file_not_analyzed` | This file is still being analyzed. Try again in a minute. | Ce fichier est encore en cours d’analyse. Réessaie dans une minute. |
| `errors.not_playable` | This file can’t be played. | Ce fichier ne peut pas être lu. |
| `errors.profile_required` (F006) | Pick a profile first. | Choisis d’abord un profil. |

**Keyboard shortcuts dialog** (`player.key.*`): one key per R42 line (`playPause`, `seek`, `volume`, `mute`, `fullscreen`, `subtitles`, `next`, `jump`, `search`), plain verbs, translated; key names (Space, Esc, ←) use the platform's names and aren't translated.

## Non-functional requirements
- **Time to play** (local network, from pressing Play to the first frame): Direct Play < 2 s, Direct Stream < 3 s, Transcode within F006's start times + 1 s. Measured on a 1080p H.264 MP4 and a 4K HEVC MKV.
- **Home**: first meaningful paint (header, hero image requested, first row of posters) under 1.5 s on a local network from a warm server; `GET /home` under 150 ms on a Raspberry Pi 4 with 2,000 movies and 200 shows.
- **Grid**: 60 fps scrolling (no long task over 50 ms while scrolling) with 5,000 posters on a 2020 mid-range laptop; at most 300 cards in the DOM; posters requested at the displayed width (`srcset` over F005's widths), lazy-loaded, never above `w500` in grids.
- **Search**: `GET /search` p95 under 100 ms on a Pi 4 with 10,000 items and 30,000 people; results shown under 300 ms after the last keystroke on a local network.
- **Server cost**: the web never polls except the admin strip (30 s, visible tab only) and the player's progress (10 s). Home rows page on demand. Artwork responses are immutable (F005-R19), so revisits hit the browser cache.
- **Bundle**: hls.js and the player are a lazy chunk; the home's initial JS (gzip) stays under 150 kB beyond F007's shared code.
- **Accessibility**: WCAG 2.2 AA. Cards are links with the title as name, rows are lists with a heading, the grid exposes its size (`aria-rowcount`) and position, every control in the player is a labeled button reachable by keyboard, the timeline is a slider with the time in words (`aria-valuetext`), subtitles are on when the profile chose them, focus is visible on every card and control, `prefers-reduced-motion` stops crossfades and autoplaying hero motion, touch targets ≥ 44 px.
- **Privacy**: no request to a third party before the user asks for it (the trailer is the only one, R20); images come from the server, never from TMDB.
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
- [x] **Random hero and rows on a Pi** — ✅ As written in R31.
- [x] **Hidden items and `total`** — ✅ Counts are always after the age predicate, everywhere.
- [x] **No PWA** — ✅ Settled in the intent.

## Acceptance criteria
**Shell and profiles**
- [ ] Given an account with three profiles and a new browser tab, when I open `/`, then the picker titled "Who's under the plaid?" shows; when I pick a profile with a PIN and type it, then `POST /session/profile` is sent and the home loads for that profile.
- [ ] Given an account with one profile, when I open `/`, then the home shows without the picker.
- [ ] Given a profile with `locale: "fr"` on an English account, when I pick it, then the UI switches to French without a reload.
- [ ] Given an admin, then the header shows the server strip with the server name and the active streams count, and it opens `/admin`; given a member, then it isn't there and `GET /playback` is never called.
- [ ] Given a 375 px wide screen, then the libraries are in the bottom tab bar and nothing scrolls horizontally except the rows.

**Home**
- [ ] Given a movie stopped at 1:02:13 with a backdrop, when I open the home, then the hero shows it with "Resume" and the time left, and Continue watching lists it first with a progress bar.
- [ ] Given nothing in progress, then the Continue watching row isn't shown and the hero is a recently added item with a backdrop and a logo, the same one all day for this profile.
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
- [ ] Given `GET /home` for a profile with nothing in progress, then there's no `continue_watching` row, and the hero is the same on two calls the same day.

**Cross-cutting**
- [ ] Given every browsing screen at 375 px in French with German-length strings, then nothing overflows and there's no horizontal page scroll.
- [ ] Given the light theme, then the home, grid and detail pages keep AA contrast; the player stays dark.
- [ ] Given `make check-i18n`, then no key is missing in `fr` and no screen has a hardcoded string.
- [ ] Given keyboard only, then I can pick a profile, search, open a movie, start it, change subtitles and leave the player.

## Revisions
- 2026-10-29 (planning): the package is `internal/catalog` (`library/browse.go` already means the folder picker); `item_genres`, `newest_child_added_at` and the search index are kept in sync by SQLite triggers, and a `search_docs` table gives each FTS row a stable rowid (plan step 3).
- 2026-10-29: approved. Genres are grouped and filtered by TMDB genre id (R26, R27, R31, data model). Backlog #123 (stall prompt) and #124 (CSP `frame-src`).
