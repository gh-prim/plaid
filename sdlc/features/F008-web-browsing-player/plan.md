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
2. [ ] `api/openapi.yaml`: every change in the API table, `make generate`, stub handlers returning `501` so the build stays green. (R26–R32)
3. [ ] Migration `0007_catalog.sql`, tests first in `db_test.go`: triggers on `item_metadata` insert/update fill `item_genres` from `json_each(genres)` (skipping `id = 0`, which admin edits produce) and upsert `search_docs`/`search_index` with the display title (metadata title, else the parsed one) and original title; triggers on `items` insert/update/delete keep episode and parsed-title rows and set the show's `newest_child_added_at` when an episode is inserted; triggers on `people` insert/update/delete keep person rows. `search_docs(rowid INTEGER PRIMARY KEY, kind, ref_id UNIQUE, library_id)` gives each document a stable rowid so a trigger deletes by rowid, never by scanning. The migration backfills from existing rows. (R26, R28, R29, data model)
4. [ ] `catalog.List`: tests first on a seeded store for each sort (title, added, released, rating, played with the profile's `watch_state`, random), both orders, each filter alone and combined, `offset`, `total` after the age predicate, shows' `added` by newest episode. `random` takes the filtered ids and shuffles them in Go with `seed` (a library is at most a few thousand ids). Handler: parameter validation (`400 validation_failed` with field details), `addedAt`, e2e tests. (R26)
5. [ ] `ItemSummary.show`: `metadata.Summary` gains `Logo`; `withShows` fills `show` for seasons and episodes from one query per page (episode → season → show). Test on continue watching and an episode list. (R30)
6. [ ] `catalog.Facets` + handler: `total` (top-level items, for the sidebar counts), genres by count with ids, decades, resolutions, HDR, letters with `#`, all after the age predicate; tests then e2e. (R27)
7. [ ] `catalog.Search` + handler: `MATCH` on the normalized query with each word as a prefix (`"ali"*`), FTS5 special characters escaped, joined to `items`/`item_metadata` for `hidden = 0` and the age predicate, people only when they have a visible credit; ranking exact → prefix → `bm25` → `rating_count`; groups capped by `limit`. Tests: accents, case, word prefix, special characters (`"`, `*`, `:`), a hidden item, a child profile, a new item found right after its metadata is written. (R28, R29)
8. [ ] `catalog.UpNext` + handler: show, season, episode cases, specials skipped, across seasons, end of show → `204`, age predicate. Reuses the ordering `WatchStore.ContinueWatching` uses (`season 0` last). (R32)
9. [ ] `catalog.Home` + handlers: `ambient` (latest continue-watching item with a backdrop, else the most recently added matched item with a backdrop), `continue_watching` from `WatchStore.ContinueWatching`, `recently_added` per visible library, up to 4 genre rows grouped by `genre_id` (≥ 10 items, the profile's most watched genres first, then size; seeded daily order; name from a library in the UI's language), row ids encoding type and params, `GET /home/rows/{rowId}` paging. Tests with a fixed clock for "stable for the day". (R31)
10. [ ] `perf_test.go` in `catalog`, run unthrottled and in the `--cpus 0.5 --memory 1g` container; numbers pasted in the PR against the NFRs (search p95 < 100 ms, home < 150 ms). Add the command to `CLAUDE.md`. `make test`. (NFR)

### Part B — Web foundations
11. [ ] Dependencies: `hls.js` and `@tanstack/react-virtual`, pinned. `format.ts` durations with tests (en, fr, the fallback without `Intl.DurationFormat`). (R47)
12. [ ] Theme (`theme.ts`) and the shell: sidebar with library counts (facets `total`), top bar with search, profile menu with Appearance, bottom tab bar under 768 px, server card for admins (who watches what, polls `GET /playback` every 30 s while visible, never for members: test), About link with attributions. (R1, R4, R50)
13. [ ] Profile picker: restyle, `profiles.title`, PIN on `pl-pin`, tab-session flag in `RequireAuth`, profile locale applied on switch (tests: 3 profiles new tab → picker; 1 profile → home; `fr` profile switches language). (R2, R3)
14. [ ] Cards and rows: `PosterCard`, `LandscapeCard`, `CardMenu` (mark watched), `Row` with paging, `AmbientTop`; tests for every state (progress, watched, unwatched count, missing artwork, `S1 E4` line, time left). Port any card component the canvas adds to `src/ui/` like F007 did. (R6–R8)
15. [ ] Routes in `App.tsx`, player lazy chunk; `make test`; screenshots of the shell and cards in dark and light.

### Part C — Browsing
16. [ ] Home page from `GET /home` with all empty states (member with no library, first scan, nothing in progress). (R5, R9)
17. [ ] `gridQuery.ts` (URL ↔ API query, tested both ways), then `LibraryGrid` with `@tanstack/react-virtual` over `total`, pages of 200 fetched by visible range (a jump fetches its page first), skeletons, scroll restore on Back; `FilterBar` from facets with chips and Clear all; `LetterIndex` (could). Test: 5,000 fake items never render more than 300 cards. (R10–R15)
18. [ ] Collections and collection page. (R16)
19. [ ] Item page: movie header (logo, scrim from `colors`, `onDominant`, badges, `lang="en"` fallbacks), actions (Play, Resume + Start over, watched, Version), cast row, trailer dialog (no YouTube request before the click: test asserts no iframe in the DOM), files-missing and unavailable states. (R17–R20, R24)
20. [ ] Show page: seasons tabs in `?season=`, episodes, main action from `up-next`, mark season watched. (R21)
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
- 2026-10-30: spec aligned on the canvas (sidebar shell, no home hero, `ambient` in `GET /home`, `LibraryFacets.total`, copy). Steps 6, 9, 12 and 14 and the files table follow; `Hero.tsx`/`ServerStrip.tsx`/`Header.tsx` become `AmbientTop.tsx`/`ServerCard.tsx`/`Sidebar.tsx` + `TopBar.tsx`.
- 2026-10-29 (step 1): FTS5 proven (`TestFTS5Search`): `ENABLE_FTS5` compiled in, `remove_diacritics 2` and prefix queries pass on darwin/arm64, linux/arm64 and linux/amd64 (`golang:1.26` containers). The `LIKE` fallback is not needed.
