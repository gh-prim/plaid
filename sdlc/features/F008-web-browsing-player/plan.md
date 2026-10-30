---
id: F008
title: Web — browsing and player
owner: gh-prim
reviewer: TBD
status: in-progress      # draft | approved | in-progress | done
spec: ./spec.md
branch: feat/F008-browsing-player
---

# F008 — Plan: Web — browsing and player

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

One PR for the whole feature (CLAUDE.md), from `feat/F008-browsing-player`, built in four parts that land as commits, each green with `make test`:
- **Part A — Server** (R26–R32): API, migration, search index, the catalog queries, home and up-next. No UI; it doesn't need the canvas.
- **Part B — Web foundations**: dependencies, the shell, routing, the profile picker, cards and rows, the design system's media and player components ported.
- **Part C — Browsing** (R5–R25): home, grids, detail pages, search.
- **Part D — Player** (R33–R46).

The screens come from the Claude Design canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz), pages Browse and Player, made with [claude-design-prompt.md](claude-design-prompt.md). Parts B–D take it as input; the PR attaches PNG exports of the boards it builds next to screenshots. Its wrap-up boards ("Rejected directions, proposed components, keys added" and "Player: motion, rejected directions, proposed components") list the components to port; they're built on the design system's existing ones (`pl-player`, `pl-tl`, `pl-next`, `pl-resume`, `pl-ppanel`, `pl-pmsg`, `pl-vol`, `pl-still`, `pl-avatar`, `pl-pin`, `pl-person`, `pl-menu`, `pl-select`, `pl-slider`, `pl-toggle`, all already in `web/src/design/components.css`).

## Files touched

### API
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` | modify | `listLibraryItems` gains `sort`, `order`, `seed`, `watch`, `genre[]`, `yearFrom`, `yearTo`, `resolution[]`, `hdr`, `offset`, `total` (R26); `ItemPage.total`; `ItemSummary.addedAt` and `ItemSummary.show` (`ShowRef: {id, displayTitle, logo?}`) (R26, R30). New routes `GET /libraries/{id}/facets` → `LibraryFacets` (with `total`), `GET /search` → `SearchResults` (`PersonRef`), `GET /home` → `Home` (`ambient`, `HomeRow`, `HomeRowType`, `GenreRef`, `SeeAll`), `GET /home/rows/{rowId}` → `ItemPage`, `GET /items/{id}/up-next` → `UpNext` or `204` (R27, R28, R31, R32). New tag `catalog`. |
| `server/internal/api/api.gen.go`, `web/src/api/schema.d.ts` | regenerate | `make generate`, never by hand. |

### Server
| File | Action | Why |
|------|--------|-----|
| `server/internal/db/migrations/0007_catalog.sql` | create | `item_genres`, `items.newest_child_added_at`, `items_library_added` index, `search_docs` + `search_index` (FTS5), the triggers that keep them in sync, and the backfill of existing rows (spec data model). |
| `server/internal/db/db_test.go` | modify | FTS5 is compiled in (`pragma_compile_options` has `ENABLE_FTS5`), `remove_diacritics 2` and prefix queries work, triggers fill and empty the index and `item_genres`, backfill on a seeded DB. |
| `server/internal/catalog/` (`catalog.go`, `query.go`, `facets.go`, `search.go`, `home.go`, `upnext.go` + `_test.go` each) | create | The new package (named `catalog`, not `browse`, which `library/browse.go` already uses for the folder picker). `catalog.New(Options{DB, Now})`; it reads `items`, `item_metadata`, `item_genres`, `watch_state`, the search index; every query takes an `access.Limit` and applies `Limit.SQL("m")` before counting. |
| `server/internal/catalog/perf_test.go` | create | `//go:build perf`: 10,000 items, 30,000 people, 2,000 movies + 200 shows for home; prints `GET /search` p95, `/home` and a filtered `total` page against the NFRs. |
| `server/internal/library/items.go` | modify | `Item` gains `CreatedAt` and `NewestChildAddedAt` (for `addedAt`); `ItemQuery` stays for the parent/child listing; top-level listing with sort/filters delegates to `catalog`. |
| `server/internal/metadata/read.go` | modify | `Summary` gains `Logo` (only shows need it, for `ItemSummary.show`), read with the existing `selectedArt`. |
| `server/internal/handlers/items.go` | modify | `ListLibraryItems` routes to `catalog.List` when sort/filter/offset/total are present on a top-level listing, validates every new parameter through `problems.add`; `toItemSummary` sets `addedAt`; a `withShows` pass sets `show` on seasons and episodes in one query per page. |
| `server/internal/handlers/catalog.go` | create | `GetLibraryFacets`, `Search`, `GetHome`, `GetHomeRow`, `GetUpNext`: principal → `AccessLimit` → `catalog` → `summaries` → `withUserStates` → `withShows`. `403 profile_required` without an active profile. |
| `server/internal/handlers/handlers.go` | modify | `Deps.Catalog *catalog.Service` (nil-able, like the other services). |
| `server/cmd/plaid/main.go` | modify | Build `catalog.New` and pass it in `Deps`. |
| `server/internal/handlers/catalog_e2e_test.go` | create | The spec's server acceptance criteria through HTTP on `newLibEnv`, with age limits (reusing `rate`). |
| `server/internal/handlers/libraries_e2e_test.go` | modify | `newLibEnv` builds the catalog service like `main.go`. |

### Web
| File | Action | Why |
|------|--------|-----|
| `web/package.json`, `pnpm-lock.yaml` | modify | `hls.js` (player chunk only), `@tanstack/react-virtual` (grid). |
| `web/src/App.tsx` | modify | Routes `/`, `/library/:id`, `/library/:id/collections`, `/collection/:id`, `/item/:id`, `/person/:id`, `/search`, `/watch/:itemId`; the player lazy-loaded. |
| `web/src/pages/Home.tsx` | replace | The F003 placeholder becomes the R5 home; the `adminStep` redirect stays. `welcome.*` keys removed. |
| `web/src/pages/ProfilePicker.tsx` + test | modify | R2: restyled with `Avatar`/`pl-pin`, `profiles.title`, the tab-session flag. |
| `web/src/auth/RequireAuth.tsx` + test | modify | Sends to `/profiles` once per tab session when the account has 2+ profiles (R2). |
| `web/src/browse/Shell.tsx`, `Sidebar.tsx`, `TopBar.tsx`, `TabBar.tsx`, `ProfileMenu.tsx`, `ServerCard.tsx` | create | R1, R4, R50: sidebar ≥ 768 px with library counts from facets `total`, top bar with search, tab bar under 768 px; `ServerCard` reuses F007's polling hook at 30 s. |
| `web/src/browse/theme.ts` | create | R4: `System`/`Dark`/`Light` in `localStorage`, `data-theme` on `<html>`. |
| `web/src/browse/cards/PosterCard.tsx`, `LandscapeCard.tsx`, `CardMenu.tsx` | create | R7, R8: `srcset` over F005 widths, `loading="lazy"`, missing-artwork state, progress thread, watched mark, unwatched count. |
| `web/src/browse/Row.tsx`, `AmbientTop.tsx` | create | R5, R6: shelf paging on scroll end, the ambient backdrop behind the first shelves. |
| `web/src/browse/HomePage.tsx` | create | R5–R9 from `GET /home`. |
| `web/src/browse/LibraryGrid.tsx`, `FilterBar.tsx`, `LetterIndex.tsx`, `gridQuery.ts` + tests | create | R10–R15: virtualized grid, URL ↔ query mapping (`gridQuery.ts`, pure and tested). |
| `web/src/browse/Collections.tsx`, `CollectionPage.tsx` | create | R16. |
| `web/src/browse/ItemPage.tsx`, `MovieHeader.tsx`, `ShowSeasons.tsx`, `CastRow.tsx`, `TrailerDialog.tsx`, `VersionSelect.tsx`, `EditMenu.tsx`, `ArtworkPicker.tsx` | create | R17–R21, R23, R24. |
| `web/src/browse/PersonPage.tsx` | create | R22. |
| `web/src/browse/Search.tsx`, `SearchDropdown.tsx`, `useSearch.ts` + test | create | R25: debounce 150 ms, `AbortController`, keyboard navigation. |
| `web/src/browse/*.test.tsx` | create | The browsing acceptance criteria with `fakeApi`. |
| `web/src/player/Player.tsx`, `Controls.tsx`, `Timeline.tsx`, `TrackMenus.tsx`, `InfoPanel.tsx`, `NextEpisode.tsx`, `StillWatching.tsx`, `ResumePrompt.tsx`, `PlayerError.tsx`, `ShortcutsDialog.tsx` | create | R33–R45 on the design system's `pl-player` family. |
| `web/src/player/capabilities.ts` + test | create | R34: the profile from `isTypeSupported`/`canPlayType`, cached in `sessionStorage`; tested with stubbed answers for Chrome, Firefox and Safari. |
| `web/src/player/engine.ts` + test | create | R35: `direct_play` → `<video src>`, native HLS, or hls.js (dynamic import); `segment_not_ready` retry; one place that attaches and detaches. |
| `web/src/player/session.ts` + test | create | R37, R39, R40: start, `PATCH`, progress every 10 s and on events, `DELETE` with `keepalive` on `pagehide`, the `404` "stopped" path; fake timers. |
| `web/src/player/trackMemory.ts`, `shortcuts.ts`, `upNext.ts` + tests | create | R38, R42, R43, R44: pure logic, tested without a `<video>`. |
| `web/src/player/mediaSession.ts` | create | R46 (could). |
| `web/src/lib/format.ts` + test | modify | Durations (`Intl.DurationFormat` with a fallback), time left, `1:02:13` clock, bitrates. |
| `web/src/locales/en/common.json`, `web/src/locales/fr/common.json` | modify | The spec's i18n tables. |
| `web/src/browse/browse.css`, `web/src/player/player.css` | create | Layout on the tokens; component styles stay in `components.css`. |
| `web/src/test/fakeVideo.ts` | create | A minimal `HTMLMediaElement` stub (play, pause, `currentTime`, events) so the player logic is testable in jsdom. |

### Docs
| File | Action | Why |
|------|--------|-----|
| `sdlc/features/F008-web-browsing-player/claude-design-prompt.md` | modify | The canvas link once made. |
| `sdlc/roadmap.md` | modify | Plan status. |
| `CLAUDE.md` | modify | The catalog perf command, next to the other perf commands. |

## Steps, in order

### Part A — Server
1. [x] **Prove FTS5 first**: a test in `db_test.go` that opens a DB, checks `ENABLE_FTS5` in `pragma_compile_options`, creates a `unicode61 remove_diacritics 2` table with `prefix='1 2 3'`, and finds "Amélie" with `amel*` and "The Lord of the Rings" with `rings*`. Run it on amd64 and in an arm64 container (`docker run --platform linux/arm64 golang:1.26 go test ./internal/db/`). If it fails, stop and switch to the rejected `LIKE` option with a Log entry. (R28)
2. [x] `api/openapi.yaml`: every change in the API table, `make generate`, stub handlers returning `501` so the build stays green. (R26–R32)
3. [x] Migration `0007_catalog.sql`, tests first in `db_test.go`: triggers on `item_metadata` insert/update fill `item_genres` from `json_each(genres)` (skipping `id = 0`, which admin edits produce) and upsert `search_docs`/`search_index` with the display title (metadata title, else the parsed one) and original title; triggers on `items` insert/update/delete keep episode and parsed-title rows and set the show's `newest_child_added_at` when an episode is inserted; triggers on `people` insert/update/delete keep person rows. `search_docs(rowid INTEGER PRIMARY KEY, kind, ref_id UNIQUE, library_id)` gives each document a stable rowid so a trigger deletes by rowid, never by scanning. The migration backfills from existing rows. (R26, R28, R29, data model)
4. [x] `catalog.List`: tests first on a seeded store for each sort (title, added, released, rating, played with the profile's `watch_state`, random), both orders, each filter alone and combined, `offset`, `total` after the age predicate, shows' `added` by newest episode. `random` takes the filtered ids and shuffles them in Go with `seed` (a library is at most a few thousand ids). Handler: parameter validation (`400 validation_failed` with field details), `addedAt`, e2e tests. (R26)
5. [x] `ItemSummary.show`: `metadata.Summary` gains `Logo`; `withShows` fills `show` for seasons and episodes from one query per page (episode → season → show). Test on continue watching and an episode list. (R30)
6. [x] `catalog.Facets` + handler: `total` (top-level items, for the sidebar counts), genres by count with ids, decades, resolutions, HDR, letters with `#`, all after the age predicate; tests then e2e. (R27)
7. [x] `catalog.Search` + handler: `MATCH` on the normalized query with each word as a prefix (`"ali"*`), FTS5 special characters escaped, joined to `items`/`item_metadata` for `hidden = 0` and the age predicate, people only when they have a visible credit; ranking exact → prefix → `bm25` → `rating_count`; groups capped by `limit`. Tests: accents, case, word prefix, special characters (`"`, `*`, `:`), a hidden item, a child profile, a new item found right after its metadata is written. (R28, R29)
8. [x] `catalog.UpNext` + handler: show, season, episode cases, specials skipped, across seasons, end of show → `204`, age predicate. Reuses the ordering `WatchStore.ContinueWatching` uses (`season 0` last). (R32)
9. [x] `catalog.Home` + handlers: `ambient` (latest continue-watching item with a backdrop, else the most recently added matched item with a backdrop), `continue_watching` from `WatchStore.ContinueWatching`, `recently_added` per visible library, up to 4 genre rows grouped by `genre_id` (≥ 10 items, the profile's most watched genres first, then size; seeded daily order; name from a library in the UI's language), row ids encoding type and params, `GET /home/rows/{rowId}` paging. Tests with a fixed clock for "stable for the day". (R31)
10. [x] `perf_test.go` in `catalog`, run unthrottled and in the `--cpus 0.5 --memory 1g` container; numbers pasted in the PR against the NFRs (search p95 < 100 ms, home < 150 ms). Add the command to `CLAUDE.md`. `make test`. (NFR)

### Part B — Web foundations
11. [x] Dependencies: `hls.js` and `@tanstack/react-virtual`, pinned. `format.ts` durations with tests (en, fr, the fallback without `Intl.DurationFormat`). (R47)
12. [x] Theme (`theme.ts`) and the shell: sidebar with library counts (facets `total`), top bar with search, profile menu with Appearance, bottom tab bar under 768 px, server card for admins (who watches what, polls `GET /playback` every 30 s while visible, never for members: test), About link with attributions. (R1, R4, R50)
13. [x] Profile picker: restyle, `profiles.title`, PIN on `pl-pin`, tab-session flag in `RequireAuth`, profile locale applied on switch (tests: 3 profiles new tab → picker; 1 profile → home; `fr` profile switches language). (R2, R3)
14. [x] Cards and rows: `PosterCard`, `LandscapeCard`, `CardMenu` (mark watched), `Row` with paging, `AmbientTop`; tests for every state (progress, watched, unwatched count, missing artwork, `S1 E4` line, time left). Port any card component the canvas adds to `src/ui/` like F007 did. (R6–R8)
15. [x] Routes in `App.tsx`, player lazy chunk; `make test`; screenshots of the shell and cards in dark and light.

### Part C — Browsing
16. [x] Home page from `GET /home` with all empty states (member with no library, first scan, nothing in progress). (R5, R9)
17. [x] `gridQuery.ts` (URL ↔ API query, tested both ways), then `LibraryGrid` with `@tanstack/react-virtual` over `total`, pages of 200 fetched by visible range (a jump fetches its page first), skeletons, scroll restore on Back; `FilterBar` from facets with chips and Clear all; `LetterIndex` (could). Test: 5,000 fake items never render more than 300 cards. (R10–R15)
18. [x] Collections and collection page. (R16)
19. [x] Item page: movie header (logo, scrim from `colors`, `onDominant`, badges, `lang="en"` fallbacks), actions (Play, Resume + Start over, watched, Version), cast row, trailer dialog (no YouTube request before the click: test asserts no iframe in the DOM), files-missing and unavailable states. (R17–R20, R24)
20. [x] Show page: seasons tabs in `?season=`, episodes, main action from `up-next`, mark season watched. (R21)
21. [ ] Person page. (R22)
22. [ ] Admin Edit menu: artwork picker (`GET /items/{id}/artwork`, `PUT`/`DELETE …/{kind}`), Fix match link to `/admin/libraries/:id/review`, refresh. Member sees no menu (test). (R23)
23. [ ] Search: `useSearch` (debounce, abort, tests with fake timers), dropdown and page, keyboard navigation, `/` shortcut. (R25)
24. [ ] Locale pass (fr, +35 % pseudo-locale) and light theme at 375, 1440 and 2560 px: manual screenshots. `make test`. (R47–R49)

### Part D — Player
25. [ ] `capabilities.ts` with stubbed browsers (Chrome: h264/vp9/av1/aac/opus, no mkv; Firefox: no hevc; Safari: hevc, native HLS, E-AC-3; HDR only with `dynamic-range: high`). (R34)
26. [ ] `engine.ts`: direct play, native HLS, hls.js via `import('hls.js')`, `segment_not_ready` retry after `Retry-After`, text tracks; tested with `fakeVideo` and an hls.js mock. (R35)
27. [ ] `session.ts`: start with file, position and remembered tracks; progress every 10 s and on pause/seek/buffering; `DELETE` on leave and `pagehide` with `keepalive`; `404` → stopped state; `PATCH` for audio/quality/burn-in with reload at position, `503` kept stream. Fake timers. (R33, R37, R39)
28. [ ] Player UI: controls and auto-hide, timeline slider with `aria-valuetext`, menus, info panel, resume prompt, errors with the `forceMethod: transcode` retry and "Copy link for VLC". (R36, R40, R41, R45)
29. [ ] `trackMemory.ts`, `shortcuts.ts` and the shortcuts dialog. (R38, R42)
30. [ ] `upNext.ts`: the card at 30 s, the 10 s countdown, the new session in the same player with the old one stopped, "Still watching?" after 3 untouched autoplays, end of show, the per-profile autoplay setting. (R43, R44)
31. [ ] Media Session (could). (R46)
32. [ ] Browser matrix, by hand, on F006's sample files and the perf clips: Chrome, Firefox, Safari macOS, Safari iOS 17.1+, for H.264 MP4 (direct play), HEVC MKV (transcode), a file with forced and SDH subtitles, a PGS subtitle (burn-in), an audio switch; 10 minutes of hls.js playback without a stall (the F006 criterion moved here). Results as a table in the PR.
33. [ ] `make test`; real run against `make dev` with a TMDB-matched library (movies and a show with two seasons), a child profile, an admin stopping a stream; PNG exports next to screenshots; timing of 3 people from home to playing. Plan status `done`, roadmap updated.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| FTS5 missing or different on arm64 builds of `modernc.org/sqlite` | low | Step 1 proves it on both before anything depends on it; fallback documented under Rejected options. |
| Triggers slow down a 10,000-file first scan or F005's metadata pass on a Pi | medium | One FTS row per item and per person, writes by rowid; F004's and F005's perf tests rerun in the throttled container after step 3, with their numbers in the PR. If the scan slows by more than 10 %, the index is filled after each batch instead (Log entry). |
| Sort + filters + `total` + resolution filter over a recursive CTE get slow on a Pi with 5,000 items | medium | Resolution and HDR filters use an `EXISTS` on the item's files (episodes for shows) with the existing indexes; step 10 measures a filtered `total` page; an index is added if needed. |
| Search results leak hidden items through people or episodes | medium | Every search branch joins `items.hidden = 0` and the age predicate; people only with a visible credit; e2e tests with a child profile on each group. |
| The virtualized grid stutters or loses its place on Back | medium | `@tanstack/react-virtual` with fixed row heights from the column count, scroll offset in history state, a 5,000-item manual test on a mid-range laptop and a phone (step 24). |
| Browser codec detection says yes and the file still fails (MKV in Chromium, HEVC without hardware) | high | R45's one automatic transcode retry; the step 32 matrix; "Copy link for VLC" as the last resort. |
| Progress lost on tab close | medium | `fetch(…, {keepalive: true})` on `pagehide`; the server also updates from requested segments for HLS (F006-R34); tested with fake timers and checked by hand in step 33. |
| The canvas arrives late or changes the components | medium | Part A doesn't depend on it; Part B ports the existing `pl-player` family first; the Log records every change once the canvas lands. |
| Files the other dev may be editing: `handlers/items.go`, `metadata/read.go`, `library/items.go`, `App.tsx`, the catalogs | medium | Rebase on `main` before each part; the server changes add functions next to existing ones rather than reshaping them. |

**Riskiest step:** step 3, the migration and its triggers. It touches the tables every F004 scan and F005 match writes, runs inside their transactions on every write, and a wrong trigger would corrupt the index or slow down scans silently. It gets its tests first, a backfill test on a seeded DB, and the F004/F005 perf runs before Part A continues.

## Rejected options
- **Composing the home, search and up-next in the browser** from `GET /libraries/{id}/items`: downloads whole libraries to the browser on a Pi, and every native app would rebuild the same logic (spec watch-out).
- **Hooks in Go at each write site** (`writeRow`, `writeCredits`, `PatchMetadata`, `Identify`, the scanner's insert, `relinkEpisode`) instead of triggers: six call sites in two packages, and any future write path would forget the index. Triggers keep it in one migration.
- **A normalized `LIKE 'x%'` search** on a title column: no word-prefix match ("rings" wouldn't find *The Lord of the Rings*) without scanning; kept ready as the fallback if step 1 fails.
- **Keyset cursors for the grid**: a scrollbar sized by `total` needs random access; `offset` on a page of 200 is cheap at library sizes and keeps the existing cursor for every other caller.
- **Writing our own virtualizer**: grid math, resize handling and scroll restoration are where the bugs are; `@tanstack/react-virtual` is headless (no styles, no components), so it doesn't bring a look, which is what F007 rejected frameworks for.
- **hls.js everywhere, Safari included**: iOS Safari's MSE support is partial; native HLS is the reliable path there and avoids loading hls.js at all.
- **Storing track memory and autoplay on the server**: a new settings API for per-profile preferences; `localStorage` per profile is enough for one browser in v1 (the native apps will want a server setting later).

## Expected evidence
- [ ] Tests added: FTS5 and trigger tests in `db`, `catalog` unit tests (list, facets, search, home, up-next), `catalog_e2e_test.go` with age limits; web tests for every browsing and player acceptance criterion, `gridQuery`, `useSearch`, `capabilities`, `engine`, `session`, `trackMemory`, `shortcuts`, `upNext`, `format`.
- [ ] `make test` green, output pasted in the PR (last line `✅ all checks passed`).
- [ ] Perf: catalog numbers (search p95, home, filtered total) unthrottled and in the Pi-like container; F004 scan and F005 metadata perf before/after the migration.
- [ ] Screenshots next to the canvas exports: profile picker, home (states), grid (filtered, empty, skeletons), movie (bright and dark backdrop, no artwork, versions, admin Edit), show, person, search, unavailable, player (controls on white and black frames, menus, info, next episode, still watching, errors); French and +35 % at 375 px; light theme on home and a movie page.
- [ ] Browser matrix table (step 32), including 10 minutes of hls.js playback without a stall.
- [ ] Manual check: 5,000-item grid on a mid-range laptop and a phone, smooth, Back restores the place.
- [ ] Manual check: a child profile never sees a title above its limit on home, grid counts, search, a person page or up-next.
- [ ] Manual check: close the tab during playback, reopen: Continue watching resumes within 10 s of where it stopped.
- [ ] Manual check: keyboard only and VoiceOver: pick a profile, search, open a movie, play, change subtitles, leave; `prefers-reduced-motion` on.
- [ ] Manual check: 3 people timed from home open to playing.

## Log
Deviations from the plan found during the build (date + what + why).
- 2026-10-29: plan approved by gh-prim; build starts on `feat/F008-browsing-player`.
- 2026-10-30 (steps 19–20): the pages needed three API additions, openapi first: `ItemSummary.libraryId` (seasons and episodes are listed with `GET /libraries/{id}/items?parentId=`, genre links need the library), `ItemMetadata.genres` with TMDB ids (genre links filter by id), and `ItemSummary.runtimeMinutes` plus `overview` on seasons and episodes only (episode rows; grids stay light). `GET /items/{id}` now fills `show`, so an episode or season id opens its show at `?season=`. Following the canvas, the trailer plays in place (a poster with "Play trailer" and the notice, then the youtube-nocookie embed) rather than in a dialog, and seasons use a picker (a list, a number grid above 12 seasons, Specials last, type a number to jump) rather than tabs; the season stays in `?season=`. The version select offers "Best for this device" (the server's choice) first. The year falls back to TMDB's date when the name has none, and a show shows its first–last air years. Keys that held both a string and children were split (`item.trailer.title`, `item.collection.partOf`); `item.rating` and `item.seasonPicker.title` are plurals since they take a count; the rating shows one decimal.
- 2026-10-30 (step 18): the grid shows a "Collections" link when `GET /collections?libraryId=` has any; the collections view is a poster grid with each collection's visible movie count; a collection page reuses the ambient top (now `AmbientTop art=`, for the home too) over its name, overview and movies.
- 2026-10-30 (step 17): the spec's keys `grid.sort`, `grid.filters`, `grid.filter.watch` and `grid.filter.resolution` are both a string and the parent of other keys, which i18next can't hold; their own text moved to `.label`. Dynamic keys (`grid.sort.${s}`) would be pruned by the extractor, so every label is spelled out. The URL keeps defaults out (`sort=title`, the sort's default order); a decade is one chip (`decade=1990` → `yearFrom`/`yearTo`); "720p and lower" is `resolution=sd` in the URL and `720p`+`sd` to the API; Random gets a seed when picked, kept in the URL for the visit. The grid uses `useWindowVirtualizer` with two screens of overscan, rows of `columnsFor(width)` posters (120–200 px), pages of 200 fetched by visible range; its scroll margin is measured by a ResizeObserver on the grid and the header (chips move it). Scroll position is kept per history entry in sessionStorage. Selected chips are cream, not amber (canvas: selection and filters stay neutral). An unknown library shows the unavailable screen. The header "Filters (n)" counts active filters (`grid.filters.count`).
- 2026-10-30 (step 16): the home is `browse/HomePage.tsx`; `pages/Home.tsx` (the F003 placeholder) is removed, its admin redirect to the onboarding kept. A member can't learn the admin's name (`/server` doesn't give it, and adding it would publish it before sign-in), so the member's empty home says "Ask your server's admin to add a library." without `{{adminName}}`. The first-scan state counts the movies and shows found so far from `GET /libraries`, under an indeterminate thread. Seen on real TMDB data, the Continue watching card had no `S1 E2`: episodes carried no `seasonNumber` because the scanner stores it on the season. `library.Item` now takes the season's number for episodes (one subquery in `itemColumns`); the up-next e2e test reads episode numbers straight from the list again. Three F003/F007 tests that looked for the old "PLAID" heading now look for the shell's profile menu.
- 2026-10-30 (step 15): the browsing routes share one layout route (`RequireStep` + `Shell` + `<Outlet>`); each page's route lands with its step (16 home, 17–18 grid and collections, 19–22 detail and person, 23 search, 25 `/watch` as the lazy player chunk) rather than as empty stubs now. There is no `/dev/ui` gallery in the code (F007's Log mentions one), so the cards are seen on the real home in step 16; screenshots of the shell were taken in dark and light on a local dev server with a generated test admin. `make test` green. Part B is done.
- 2026-10-30 (step 14): the time left needs a duration, and summaries carried none. `UserState` gains `durationMs` (the length of the file last played, already in `watch_state.duration_ms`; openapi first, `make generate`, e2e check). A next episode not started yet has no time left, only its `S1 E4 · Title` line. Cards live in `browse/cards/` with a shared `Thread` (the design system's `pl-progress`) and `media.ts` (srcset over F005's widths: posters 185/342/500, landscape 342/500/780; progress; unwatched episodes). The card's line joins its parts with `card.separator` (" · "); "More info" on a movie card is `card.moreInfo` now that `home.hero.moreInfo` is gone. The card menu marks watched or unwatched and links to the show (episodes) or the item. The row pages with an IntersectionObserver on its last item, 600 px ahead.
- 2026-10-30 (step 13): the PIN is four boxes drawn over one numeric field (screen readers hear one field, paste works) and sends itself at the fourth digit, so the "Open profile" button is gone. `errors.invalid_pin` already says "Wrong PIN. Try again.", so no `profiles.pin.wrong`; `errors.profile_locked` takes the canvas's wording. A PIN tile is named "Ada, PIN" (`profiles.withPin`). The canvas's "Manage profiles" link has nowhere to go yet (no profile management screen on the web), so it isn't there. The tab flag is `plaid.profilePicked` in sessionStorage; `RequireStep` sends an app route to `/profiles` with the route it was headed to, and the picker goes back there. The account's locale isn't applied to the UI anywhere today, so R3 only switches to the active profile's `locale` when it has one, and back to the account's when the next profile has none. Tests clear sessionStorage between tests; the Pair tests mark their tab as already picked.
- 2026-10-30 (step 12): the shell wraps the F003 home placeholder until step 16; its account bar is gone (the shell has the same links). `nav.appearance` can't be both a string and the parent of `.system`/`.dark`/`.light`, so the label is `nav.appearance.label`. Pair a TV and Sign out reuse `pairing.title` and `auth.signOut` (same words) instead of new `nav.*` keys; the server card says `nav.serverCard.watching` / `nav.serverCard.idle` and lists `profileName` and `itemTitle` as two lines, so `nav.serverCard.stream` isn't needed; the scanning line waits for a scan percentage the client can compute (Part C). The wordmark is the serif "plaid" without the Foxy dot, which waits for F002. Icons are the design system's (`tv`, `folder`, `search`, `more`, `users`), not the canvas's text glyphs. Sign out moved into the profile menu, so four F003 tests now find the active profile by the profile menu button.
- 2026-10-30 (step 10): `catalog/perf_test.go`, generated data (6,000 movies + 400 shows + 4,000 episodes + 30,000 people for search; 2,000 movies + 200 shows of 10 episodes for the home), median / p95. Unthrottled (18 CPUs): search 2.5 / 20 ms, home 12.5 / 12.7 ms, filtered grid page of 200 with total 10.7 / 11.3 ms, facets 10.7 / 10.9 ms. In the `--cpus 0.5 --memory 1g` container (GOMAXPROCS 2): search 4.8 / 75 ms (target p95 < 100 ms), home 17.7 / 68 ms (target < 150 ms), grid 13.6 / 69 ms, facets 13.9 / 66 ms. The throttled p95s sit near the CFS quota period, so they're the container's slices more than the queries. The home figure covers the catalog and loading every row's items and summaries, not the HTTP layer. Part A is done.
- 2026-10-30 (step 9): row ids are `continue_watching`, `recently_added.<libraryId>` and `genre.<tmdbId>`; cursors are base64 offsets like continue watching's. The home reads up to 200 continue-watching ids from F006 and pages them itself. An episode in progress lends its show's backdrop to `ambient`. A genre row mixes every visible movie and show of the genre across libraries, shuffled with a seed from the profile, the local date and the genre; its "See all" opens the library holding most of it, sorted by title. "Most watched" counts movies and shows with any watched or started item. The genre name comes from a library whose `metadata_language` is the profile's (else the account's) locale language, else the first library by name. Every row's items load in one batch. `GET /home` without an active profile is `403 profile_required`.
- 2026-10-30 (step 8): the scanner numbers seasons on the season item and leaves `season_number` empty on episodes, so up next reads the season's number (the e2e test caught it; the unit seed now mirrors the scanner). On a show or season: the latest-played episode in progress, else the first unwatched after the latest watched, else the first unwatched; all watched gives `204`. From a special, the next special. A movie gives `204`; an item hidden, unknown or above the age limit gives `404`.
- 2026-10-30 (step 7): the query keeps only letters and digits as words (anything else separates them), each a quoted prefix, all required, so no FTS5 syntax gets through. Each group reads its 200 best BM25 rows (title weighted 10, original title 5) after the hidden and age filters, then ranks them in Go: exact title or original title, then titles starting with the query, then BM25 (shorter titles win), then vote count. A person is found only through a credit on a visible item. `q` is counted in runes after trimming; out of range is `validation_failed` (422). `metadata.Store.Portraits` gives people their portrait.
- 2026-10-30 (step 6): letters fold accents ("Élise" is E) and put digits, symbols and other scripts (Japanese, Korean, "ø") under "#", listed first then A–Z; genres tie on count by name; resolutions list only the buckets with items, in the order 2160p, 1080p, 720p, sd; items with no file present count in no bucket. Facets and the grid share `techCTE` and `yearExpr`.
- 2026-10-30 (step 5): `withShows` runs wherever summaries are built (the F004 listing, the catalog page, `summariesOf` behind continue watching, collections and people), with `library.Store.Shows` mapping seasons and episodes to their show in one query, then the shows' items and summaries: three queries per page whatever its size.
- 2026-10-30 (step 4): `listLibraryItems` takes the catalog path only when one of F008's parameters is set (`sort`, `order`, `seed`, a filter, `offset` or `total`); without them it keeps F004's keyset listing, so the admin and onboarding screens are untouched. On the catalog path `cursor` and `parentId` are `validation_failed`, and paging is by `offset` (no `nextCursor`; a short page or `total` ends it). Validation answers 422 like every `validation_failed` in the API, not the 400 the spec wrote. `library.Store.Items` is exported so the handler loads the catalog's ids; `library.Item` gains `CreatedAt` and `NewestChildAddedAt`. Resolution buckets go by the largest file's width first (≥ 3000 → 2160p, ≥ 1800 → 1080p, ≥ 1200 → 720p, else sd), so scope films (1920×800) are 1080p. Default order: ascending for `title`, descending for the other sorts.
- 2026-10-30 (step 3): `0007_catalog.sql` with its triggers. F004 and F005 write `item_metadata` and `people` with `INSERT … ON CONFLICT DO UPDATE`, which fires the UPDATE triggers; nothing uses `INSERT OR REPLACE`. The older upgrade tests roll 0007 back too, since its triggers read `item_metadata`. Perf, before → after the migration, same machine: F004 walk and parse of 10,000 files 0.61 s → 1.36 s unthrottled (3 runs each), 1.59 s → 3.11 s in the `--cpus 0.5 --memory 1g` container (target < 2 min), first scan with fake analysis 1 m 59.6 s → 2 m 2.2 s throttled, unchanged rescan unchanged (0.4 s); F005 metadata for 11,000 items 2 m 36 s → 2 m 37 s (bound by the TMDB rate limit). The triggers cost about 70 µs per inserted item.
- 2026-10-30: spec aligned on the canvas (sidebar shell, no home hero, `ambient` in `GET /home`, `LibraryFacets.total`, copy). Steps 6, 9, 12 and 14 and the files table follow; `Hero.tsx`/`ServerStrip.tsx`/`Header.tsx` become `AmbientTop.tsx`/`ServerCard.tsx`/`Sidebar.tsx` + `TopBar.tsx`.
- 2026-10-29 (step 1): FTS5 proven (`TestFTS5Search`): `ENABLE_FTS5` compiled in, `remove_diacritics 2` and prefix queries pass on darwin/arm64, linux/arm64 and linux/amd64 (`golang:1.26` containers). The `LIKE` fallback is not needed.
