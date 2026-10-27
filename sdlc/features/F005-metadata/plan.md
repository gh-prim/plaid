---
id: F005
title: Metadata and artwork
owner: gh-prim
reviewer: TBD
status: in-progress      # draft | approved | in-progress | done
spec: ./spec.md
branch: feat/F005-metadata
---

# F005 — Plan: Metadata and artwork

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

**Starting point (checked 2026-10-27, `main` at `0a24632`):**
- **Server:** strict oapi-codegen handlers (`handlers.Deps`, one field per feature), `auth.Principal{UserID, SessionID, ProfileID, Role}`, `auth.RequireAdmin`, typed `apierr`. SQLite through `modernc.org/sqlite`, embedded migrations `0001`–`0003`.
- **Builds:** `CGO_ENABLED=0` everywhere (Makefile, Dockerfile, GoReleaser).
- **F004's `library` package:**
  - `Scanner.link()` (`reconcile.go`) parses and links a file. It's called for new files, for changed files, and on every scan for `unrecognized` files.
  - `Walk()` (`walker.go`) only keeps videos and subtitles.
  - `Engine` exposes `Store()`, `Subscribe()` and the scheduler's `Event{Type, Progress, Error}`.
  - `Store.ListItems` sorts on `itemSortKey` over `items.sort_title`.
- **F003:** profiles carry `max_age` / `allow_unrated` (`accounts/profiles.go`), but no route enforces them yet.
- **Release:** GoReleaser runs only on tags (`release.yml`), with `ldflags` for `buildinfo`. `docker.yml` builds the image without publishing it (`push: false`).
- **Dependencies:** `golang.org/x/text` is already indirect. There's no `golang.org/x/image` and no `golang.org/x/time` yet.

**Other dev:** F006 (playback) has no plan yet. F005 touches no file F006 is expected to own. The shared points are `internal/access` (F006 must call it) and `handlers.Deps` (one field each, which merges cleanly).

**Delivery: four PRs**, each one releasable on its own, since every merge to `main` ships:
- **PR A — foundations** (steps 1–6): F004 hooks, migration, settings, the age-limit predicate on F004's read routes, the certification table. Merging it releases a server that enforces age limits. Until PR B, nothing is rated yet, so profiles with `allowUnrated = false` see nothing, which is correct.
- **PR B — matching and text metadata** (steps 7–15): TMDB client, matcher, `.nfo`, queue, apply, refresh, read API fields.
- **PR C — artwork** (steps 16–22): local images, TMDB and fanart.tv artwork, the store, resizing, colors, `/images`.
- **PR D — admin API, keys and proofs** (steps 23–30): search, fix match, identify, edits, artwork choice, settings, status, SSE, attribution, release key injection, docs, perf.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `server/go.mod`, `go.sum` | modify | Add `golang.org/x/image` (draw, webp decode) and `golang.org/x/time` (rate). `golang.org/x/text` becomes direct (accent folding) |
| `server/internal/db/migrations/0004_metadata.sql` | create | Every table and column in the spec's data model |
| `server/internal/db/db.go` + `db_test.go` | modify | `CheckPermissions(dataDir)` warns if `plaid.db` is readable by group or others (R34); upgrade test from `0003` |
| `server/internal/settings/settings.go` + `_test.go` | create | Typed get/set/delete over the `settings` table |
| `server/internal/access/access.go` + `_test.go` | create | `Limit{MaxAge *int, AllowUnrated bool}`, `ForPrincipal(ctx, accounts)`, and `SQL(alias)` giving one fixed predicate plus its args (R25). No dependency on `library` or `metadata` |
| `server/internal/library/hooks.go` | create | `Hooks{ItemsChanged func(libraryID string, ids []string); Sidecars func(libraryID string, s []Sidecar); Identify func(ctx, fileID string) (*Identification, error)}` in `EngineOptions` |
| `server/internal/library/walker.go` + `_test.go` | modify | Also collect `.nfo`, `.jpg`, `.jpeg`, `.png`, `.webp` (size, mtime) per directory into `WalkResult.Sidecars`, with no extra reads (R14) |
| `server/internal/library/reconcile.go` + `_test.go` | modify | `link()` asks `Hooks.Identify` before parsing (R28). Collect created and relinked item ids per batch and call `ItemsChanged` after commit (R1). Pass sidecars on (R14) |
| `server/internal/library/items.go` + `_test.go` | modify | `ItemQuery.Access access.Limit`, joined with `item_metadata` for the predicate (R25). `itemSortKey` prefers `item_metadata.sort_title` |
| `server/internal/library/engine.go` | modify | `Publish(libraryID, Event)` so F005 can send `metadata` events on the scan stream (R35) |
| `server/internal/metadata/certs/certifications.json`, `certs.go` + `_test.go` | create | Certification → age table for US film/TV, GB, FR, DE, ES, IT, NL, CA, AU, BR, JP (R24) |
| `server/internal/metadata/provider.go` | create | `Provider` interface, shared types (`Candidate`, `MovieDetails`, `ShowDetails`, `Season`, `ImageRef`) |
| `server/internal/metadata/tmdb/client.go` + `_test.go` | create | Search, find, movie, show, season, collection, configuration (R3, R4, R6, R9, R11); bearer or `api_key` auth; key redaction |
| `server/internal/metadata/fanart/client.go` + `_test.go` | create | Movie and TV images, `client_key` (R16) |
| `server/internal/metadata/httpx/httpx.go` + `_test.go` | create | Shared client: host allowlist, redirect check, 10 s timeout, size cap, per-host `rate.Limiter`, `Retry-After`, circuit state (R32, R33, NFR security) |
| `server/internal/metadata/testdata/tmdb/*.json`, `fanart/*.json` | create | Recorded responses (see step 7) |
| `scripts/record-metadata-fixtures.sh` | create | Records the fixtures with the developer's own key, `jq`-trimmed. Never run in CI |
| `server/internal/metadata/match/normalize.go`, `score.go` + `_test.go` | create | Normalization, Levenshtein, confidence, decision (R4, R5) |
| `server/internal/metadata/match/testdata/cases.json` | create | 50+ real names, each with its recorded search results and the expected decision |
| `server/internal/metadata/nfo/nfo.go` + `_test.go`, `testdata/*.nfo` | create | Kodi `.nfo` parser (R12) |
| `server/internal/metadata/sidecar/sidecar.go` + `_test.go` | create | Local image names → kind, season, episode (R13) |
| `server/internal/metadata/store.go` + `_test.go` | create | SQL for `item_metadata`, candidates, people, credits, collections, artwork, identifications, sidecars |
| `server/internal/metadata/queue.go` + `_test.go` | create | Due-row picker, 4 workers, priorities, backoff with jitter (R2, R31, R33) |
| `server/internal/metadata/apply.go` + `_test.go` | create | Merges provider, `.nfo` and locks into one item write. Show cascade to seasons and episodes, collections, trailers, certifications, duplicates (R6–R11, R22–R24, R29, R38) |
| `server/internal/metadata/service.go` | create | Owns everything. `Start(ctx)`, `Close()`, the methods the handlers use, hook implementations, key resolution (R34) |
| `server/internal/metadata/keys.go` | create | `builtinTMDBKey`, `builtinFanartKey` (set by `-ldflags -X`), resolution order from ADR 0007 |
| `server/internal/metadata/artwork/select.go` + `_test.go` | create | Choice per kind and language (R15, R16) |
| `server/internal/metadata/artwork/store.go` + `_test.go` | create | Content-addressed originals, download with caps, daily cleanup (R17, R18) |
| `server/internal/metadata/artwork/resize.go` + `_test.go` | create | Width ladder, decode caps, CatmullRom, JPEG 82 or PNG, a 2-slot semaphore (R19) |
| `server/internal/metadata/artwork/variants.go` + `_test.go` | create | Variant cache, LRU index rebuilt from the directory at startup, mtime touched at most once a day (R20) |
| `server/internal/metadata/artwork/colors.go` + `_test.go` | create | Median cut on a 64 px thumbnail, `onDominant` by relative luminance (R21) |
| `server/internal/metadata/artwork/testdata/*` | create | Small generated JPEG, PNG and WebP images, plus an oversized-header image |
| `server/internal/metadata/e2e_test.go` | create | Spec acceptance criteria against stub providers |
| `server/internal/metadata/perf_test.go` | create | `//go:build perf`: 1,000 movies, 100 shows, 9,000 episodes, 50 ms stub latency |
| `api/openapi.yaml` | modify | `metadata` tag, every route in the spec's API table, `ItemSummary` and `ItemDetail` fields, `Server.attributions`, new error codes |
| `server/internal/api/api.gen.go`, `web/src/api/schema.d.ts` | regenerate | From `openapi.yaml` |
| `server/internal/apierr/apierr.go` | modify | `invalid_api_key`, `provider_unavailable`, `metadata_disabled`, `match_not_found`, `item_kind_mismatch`, `field_not_editable`, `invalid_language`, `invalid_region`, `already_identified` |
| `server/internal/handlers/handlers.go` | modify | `Deps.Metadata *metadata.Service` |
| `server/internal/handlers/items.go`, `libraries.go` + tests | modify | Pass `access.Limit`, map the new fields, `metadataLanguage` / `metadataRegion` |
| `server/internal/handlers/metadata.go`, `images.go`, `collections.go`, `people.go` + `_test.go` each | create | New routes. `images.go` writes bytes and conditional `304` |
| `server/internal/handlers/capabilities.go` | modify | `GetServer` adds `attributions` (R37) |
| `server/internal/config/config.go` + `_test.go` | modify | `PLAID_TMDB_API_KEY`, `PLAID_FANART_API_KEY`, `PLAID_ARTWORK_CACHE_MAX_MB` |
| `server/cmd/plaid/main.go` | modify | Permission check; build the metadata service, wire it into the library hooks, `Start`/`Close` order |
| `.goreleaser.yaml` | modify | `-X …/internal/metadata.builtinTMDBKey={{ .Env.TMDB_API_KEY }}` and the fanart.tv one |
| `.github/workflows/release.yml` | modify | Pass `secrets.TMDB_API_KEY` / `FANART_API_KEY` to the GoReleaser step only |
| `docs/install.md` | modify | Keys, what's sent to providers (privacy), disk use and backups, attribution |
| `sdlc/backlog.md` | modify | Only if new follow-ups come up during the build |

## Steps, in order
Each step brings its tests in the same commit.

### PR A — foundations
1. [x] **Migration `0004_metadata.sql`.** Tables in dependency order (`collections` and `artwork` before `item_metadata` and `people`). An upgrade test from a `0003` database with items runs, then checks the defaults (`metadata_language = 'en'`, `metadata_region = 'US'`). Library creation fills language and region from the creating admin's locale (`library.NewLibrary` gets `MetadataLanguage` / `MetadataRegion`; the `en` → `US`, `fr` → `FR` mapping lives in the handler). (R10, data model)
2. [x] **`settings` package and DB permission check.** Get, set and delete with `updated_at`. `db.CheckPermissions` logs `warn` when `mode & 0o077 != 0`, and `main.go` calls it after `db.Open`. Tests cover `0600` (silent) and `0644` (warning), with a captured `slog` handler. (R34)
3. [x] **F004 hooks.**
   - `EngineOptions.Hooks`, with no-op defaults, so F004's tests pass unchanged.
   - The reconciler collects the ids of items it inserts in `link()` and of items whose `item_files` gained a file. After each batch commits, it calls `ItemsChanged`.
   - `link()` calls `Hooks.Identify(fileID)` first. When it returns an identification, the parser is skipped and the file is linked to the identified item. A movie with a `tmdb_id` is looked up through `item_metadata` (`Identification.ItemID`); an episode through the show's `item_metadata.tmdb_id` plus its numbers, and the season and episode items are created if missing.
   - New F004 tests: the hook is called once per new item; an identified file survives a rename that would otherwise make it `unrecognized`, a full rescan, and a move (R16). (R1, R28)
4. [x] **Sidecar reporting.** The walker records sidecar files by extension in the same `ReadDir` pass (no `stat` beyond the `DirEntry.Info()` it already does). `WalkResult.Sidecars` is passed to `Hooks.Sidecars` once per scanned folder. Walker test: a tree with `movie.nfo`, `poster.jpg` and a `Season 01/season01-poster.jpg`. The F004 read-only tree-hash test still passes. (R14)
5. [x] **Certification table.** `certs.json` maps `{country: {system: {cert: age}}}`, with ages limited to F003's values. `certs.Age(country, cert)` returns `(age, ok)`. Table test, plus a test that every age is in `{0, 7, 10, 12, 13, 16, 18}`. `.nfo` `mpaa` values (`Rated PG-13`, `FR:12`, `DE:FSK 16`) are normalized here. (R24)
6. [x] **`internal/access` and enforcement on F004 routes.**
   - `access.ForPrincipal` loads the active profile. With no profile picked, it uses the account's default profile, as F003 does.
   - `Limit.SQL("m")` returns `(m.age_rating IS NOT NULL AND m.age_rating <= ?) OR (m.age_rating IS NULL AND ?)`. Seasons and episodes carry their show's `age_rating`, copied by apply (step 13) in the same transaction, so the predicate never needs to walk `parent_id`.
   - `ListItems` uses `LEFT JOIN item_metadata` and adds the predicate. `GetItem`, from the handler, returns `404 not_found` when it's filtered out.
   - Tests in `access` and `handlers/items_test.go`: maxAge 12 vs a PG-13 movie, unrated with `allowUnrated` false, an episode of a TV-MA show, an admin with a kids profile, and no limit. (R25)

### PR B — matching and text metadata
7. [x] **Fixtures and the recording script.** `scripts/record-metadata-fixtures.sh` (shellcheck-clean) reads `TMDB_API_KEY` / `FANART_API_KEY` from the environment. It fetches the search, details, season, collection, find and images responses the tests need, trims them with `jq` (images limited to the top 10 per kind), and writes them to `testdata/`. Fixtures are committed. The script is documented at the top of the file and in `CLAUDE.md`'s commands table.
8. [x] **`httpx` client.** Host allowlist (`api.themoviedb.org`, `image.tmdb.org`, `webservice.fanart.tv`, `assets.fanart.tv`, HTTPS only), redirects checked against the same list, `io.LimitReader` at 20 MB, 10 s timeout. A `rate.Limiter` per provider (TMDB 20/s, fanart.tv 5/s) and a separate 8-slot download semaphore. A `429` sets a `notBefore` from `Retry-After`. Circuit: 5 consecutive failures → `unreachable`, one probe a minute. A `401` → `invalid_key`. URLs are logged with `api_key` / `client_key` replaced by `REDACTED`. Tests use `httptest.NewTLSServer`, and the allowlist can be overridden only in tests (unexported option). (R32, R33, NFR security)
9. [x] **TMDB client.** Methods return the `provider.go` types. Text fallback (R11) is built from `translations` in the same response. Episode overview fallback makes one extra `en-US` season call only when the season has episodes with an empty overview. Trailers are sorted per R22, and certifications extracted per R24 (theatrical `type = 3` first). Tests decode every fixture and check the fields used by R7–R9. (R3, R6, R7–R9, R11, R22–R24)
10. [x] **Matcher.**
    - `Normalize`: NFKD, accents removed, lowercase, `&` → `and`, punctuation → space, leading articles removed (`the a an le la les l' un une der die das ein eine el los las il lo gli de het een`), spaces collapsed.
    - `Similarity` uses a rune Levenshtein. `Score` and `Decide` follow the spec.
    - `cases.json` holds 50+ names: remakes (*Dune*, *Total Recall*, *RoboCop*), foreign titles (*Amélie*, *Les Intouchables*, *Der Untergang*), numbers (*2001*, *1917*, *Se7en*), sequels, *The Office* US/UK, *House of Cards* 1990/2013.
    - Each case says whether its expected decision is `confirmed` or `tuned`. Tuning runs here: if the constants change, the spec's Revisions section records the new values in the same commit. (R4, R5, watch-out "threshold")
11. [x] **`.nfo` and sidecar parsers.** The `nfo` parser takes a 1 MB `io.LimitReader`, uses `encoding/xml`, and tolerates the URL-only `.nfo` Kodi allows (a TMDB or IMDb URL on one line → `uniqueid`). The `sidecar` classifier covers every R13 name, case-insensitive. Table tests with real Kodi, Radarr and Sonarr samples. (R12, R13)
12. [x] **Store and queue.**
    - `ItemsChanged` inserts `item_metadata` rows as `pending`, with `next_attempt_at = now`, for movies and shows only. Seasons and episodes are created as `pending` and handled through their show.
    - Workers pick rows ordered by state priority: new, then retry, then refresh. Each row is claimed with `UPDATE … SET next_attempt_at = now + lease WHERE item_id = ? AND next_attempt_at = ?`, so two workers never take the same item.
    - Backoff is `min(6h, 1m × 2^attempts) ± 20%`. With no TMDB key, nothing is claimed except `.nfo`/local-only work.
    - Tests use a fake clock and a fake provider. (R1, R2, R33, R34)
13. [x] **Apply.** One transaction per movie, and per show with its seasons and episodes.
    - Order of precedence for each field: locked value → `.nfo` value → provider value in the library language → English fallback. `field_languages` and `local_fields` are kept up to date.
    - Credits: upsert `people` by `tmdb_id`, then replace `item_credits` for the item. Collections: upsert per library. Duplicates (R38): an older item with the same `tmdb_id` in the library → `duplicate_of`.
    - The show's `age_rating` is copied to its seasons and episodes (R24, used by step 6).
    - Errors never touch stored fields (R33).
    - Tests: a sure match through a hint and through an `.nfo` id (`/find` for IMDb), `needs_review` storing 5 candidates, `unmatched`, a show with season 0 and a date-based episode, multi-episode files, a locked title kept, an `.nfo` plot winning, a French library with an English fallback, and a network error mid-show leaving the previous data intact. (R3, R5–R8, R11, R12, R22–R24, R29, R38)
14. [x] **Refresh scheduling.** `refresh_at` is set per R31 at every apply. A missing-episode refetch runs when a newly linked episode has no TMDB match and its season was fetched more than 24 h ago. `PATCH /libraries/{id}` with a new language or region sets `refresh_at = now` (low priority) for the whole library. (R10, R31)
15. [x] **Read API, text fields.** In `openapi.yaml`: the `ItemSummary` and `ItemDetail` fields of R36, except artwork (PR C). Then `make generate`, handler mapping, and `displayTitle` (matched title, else the parsed one). Handler tests check the JSON shape of a matched movie, a show, an episode and a still-pending item. (R36)

### PR C — artwork
16. [x] **Local images.** `Hooks.Sidecars` diffs against `sidecar_files` by `(size, mtime)`. New or changed images create or update `artwork` rows with source `local` and `local_path`. Removed ones drop their rows. A changed `.nfo` re-applies its item with no provider call. Tests: add `fanart.jpg` after the match, change a `.nfo` plot, delete `poster.jpg` (falls back to TMDB). (R12–R14)
17. [x] **Artwork selection.** Candidates per kind come from TMDB `images` and fanart.tv, ranked per R15 and R16, with local images always first and locked kinds untouched. Backdrops: up to 3 selected. Tests are tables over the recorded image lists. (R15, R16, R30)
18. [x] **fanart.tv client.** Movies by TMDB id; shows by the TVDB id from `external_ids` (no TVDB id, no call). `client_key` is sent when a personal key is set. Tests over the fixtures. (R16, R34)
19. [x] **Originals store and downloads.**
    - Eager downloads after apply, in the R17 order: poster `w780`, backdrop `w1280`, logo `original`, clearart, season posters. Each goes through `httpx` and is written to a temp file then renamed into `artwork/originals/<2>/<sha>.<ext>`.
    - `DecodeConfig` rejects images over 8,000 × 8,000 px before anything is stored. Dimensions and `sha256` are recorded.
    - Lazy kinds (stills, portraits, non-selected candidates) keep `sha256 = NULL` until first asked for.
    - A daily cleanup removes originals no row references (`NOT EXISTS (SELECT 1 FROM artwork WHERE sha256 = ?)`).
    - Tests cover dedup of the same bytes, rejection of the oversized header, and a cleanup that keeps referenced files. (R17, R18, NFR security)
20. [x] **Resizer and variants.**
    - Width ladder `92 185 342 500 780 1280 1920 3840`, no upscaling (the original width is served instead).
    - JPEG quality 82 for photos; PNG for logos and clearart. Local WebP originals are decoded with `x/image/webp`.
    - A 2-slot semaphore and `singleflight` per `(sha, width)`, so a burst of first requests resizes once.
    - The variant cache index is rebuilt by walking `cache/artwork` at startup in the background. Eviction happens past `PLAID_ARTWORK_CACHE_MAX_MB`.
    - Tests: 780 → `w=300` gives 342; `w=5000` gives the original width; the PNG keeps its alpha; concurrent identical requests resize once (a counter); eviction under a tiny cap; a benchmark on a 780 px poster. (R19, R20)
21. [x] **Colors.** Median cut into 5 boxes on a 64 px CatmullRom thumbnail. `dominant` is the most populated box, `vibrant` has the highest saturation × value, `muted` has the lowest saturation among the rest, and `dark` is the darkest. `onDominant` is `light` when the relative luminance is < 0.5. Computed when an original is stored, for posters, backdrops, season posters and stills. Golden tests on generated images (a solid red → `#ff0000` dominant). (R21)
22. [x] **`/images/{imageId}`.**
    - Session required. The owner item is checked with `access` (R25), so a filtered item gives `404`.
    - A lazy row is fetched synchronously (10 s) on first request; a provider error gives `503 provider_unavailable`.
    - Headers: `ETag: "<sha>-<w>"`, `If-None-Match` → `304`, `Cache-Control: private, max-age=31536000, immutable`, `nosniff`.
    - `ItemSummary` and `ItemDetail` gain `poster`, `backdrop`, `logo`, `clearart`, stills and portraits as `{imageId, width, height, colors}`.
    - Handler tests cover each case. (R19, R25, R36)

### PR D — admin API, keys and proofs
23. [x] **Search and candidate images.** `GET /metadata/search` parses `tmdb:` / `imdb:` ids or free text, scores against `itemId` when given, and returns `503 provider_unavailable` while TMDB is unreachable. `GET /metadata/candidate-images/{path}` checks the path against `^[A-Za-z0-9]+\.(jpg|png)$` (otherwise `400`, with no request), then proxies TMDB `w185` with a 1-hour in-memory cache. Tests include the traversal attempt from the acceptance criteria. (R27)
24. [x] **Fix match, unmatch, refresh.**
    - `POST /items/{id}/match` validates the kind (a movie id on a show → `422 item_kind_mismatch`; unknown → `404 match_not_found`), fetches, applies with `match_locked`, and cascades for shows.
    - An episode with `{seasonNumber, episodeNumber}` relinks its files (R39).
    - `DELETE /items/{id}/match` resets to `pending` and unlocks.
    - `POST /items/{id}/refresh` and `/libraries/{id}/metadata/refresh` set `refresh_at = now`, with `images: true` re-picking unlocked kinds.
    - Tests: *The Office* UK → US, with the lock surviving a rescan and a refresh. (R26, R31, R39)
25. [x] **Identify unrecognized files.** `POST /media-files/{fileId}/identify` checks that the file is `unrecognized` or not identified yet (`409 already_identified` otherwise), writes `file_identifications`, links the file through the same code path as the F004 hook (step 3), sets `parse_status = ok`, then applies. Test: *Amélie*, then a full rescan and a move. (R28)
26. [x] **Field edits and artwork choice.** `PATCH /items/{id}/metadata` only accepts the R29 fields (otherwise `422 field_not_editable`), locks them, and handles `unlock`. `GET`, `PUT` and `DELETE /items/{id}/artwork/{kind}` cover listing candidates, selecting one and locking it, and unlocking it. Tests cover locks surviving refresh. (R29, R30)
27. [x] **Settings, status, SSE, attribution.**
    - `GET` / `PATCH /metadata/settings`: the key is validated with TMDB `/configuration` (`422 invalid_api_key`, nothing stored), never echoed back, and `null` falls back to env, then the built-in key.
    - `GET /metadata/status` per R35. The library SSE stream gets a `metadata` event, sent through `Engine.Publish` and throttled to one every 2 s per library.
    - `GET /server` gets `attributions`.
    - Tests: grep every captured log line and response body for the test key value (it must not appear), the SSE event shape, and a member getting `403` on every admin route (a table over the generated operation list). (R34, R35, R37)
28. [x] **Collections and people routes.** `GET /collections?libraryId=`, `GET /collections/{id}` (movies by release date, filtered by `access`), and `GET /people/{id}` (filmography filtered by `access`). (R8, R23, R25)
29. [x] **Release keys and docs.**
    - `.goreleaser.yaml` `ldflags` read `{{ .Env.TMDB_API_KEY }}` / `{{ .Env.FANART_API_KEY }}` (empty is allowed: the binary then reports `no_api_key`).
    - `release.yml` sets them from secrets on the GoReleaser step only. Pull-request workflows never see them; GoReleaser runs only on tags.
    - The Dockerfile doesn't take keys while the image isn't published. A comment points to ADR 0007 (build secrets when publishing starts).
    - `docs/install.md` gets keys, privacy, disk and backup, and attribution. ADR 0007 gets the result of the terms check (watch-out "provider terms").
30. [x] **End-to-end and performance.**
    - `e2e_test.go` runs every acceptance criterion in the spec against stub providers, including offline and recovery, `429`, `401`, no key at all, and the read-only tree hash.
    - `perf_test.go` (`-tags perf`) generates the reference library and checks the request rate (≤ 20/s), the time to all posters, and peak memory with `runtime.ReadMemStats`. Run it in the throttled container from CLAUDE.md and paste the numbers in the PR.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| The R25 predicate leaks restricted items through a route we forgot (people, collections, images, SSE counters) | medium | One `access.Limit` argument, required in every store read method. A test table over the generated route list: each session route is called with a kids profile against a fixture library holding one restricted item |
| F004 regression from the hooks (move detection, unrecognized re-parse, read-only guarantee) | medium | Hooks are no-ops by default; every F004 test runs unchanged. New tests for identified files across rename, move and full rescan. The tree-hash test runs with metadata enabled |
| Wrong automatic matches erode trust (remakes, foreign titles) | medium | Threshold and margin tuned on 50+ real cases (step 10). The ambiguity margin sends close calls to review instead of guessing |
| TMDB rate limits or bans, or a revoked built-in key | low | 20/s token bucket, `Retry-After`, circuit breaker, admin override key (ADR 0007) |
| SQLite write contention between F004 scan batches and F005 applies on a Pi | medium | One transaction per item, short, outside the scan's batch transactions. WAL is already on. The perf test runs a scan and enrichment at the same time |
| Image decoding uses too much memory (big local images, decompression bombs) | low | `DecodeConfig` cap before decoding, 2 resizes at once, 20 MB download cap |
| Disk growth from originals and variants | low | Variant cap and LRU, daily cleanup of orphaned originals, sizes documented |
| Fixture drift from TMDB's real API | low | Fixtures are recorded from the real API by the script. Field mapping is tested against them; a schema change shows up the next time the script is run |
| A merge to `main` ships each PR as a release | certain | Each PR is consistent on its own (see Delivery). PR A's age enforcement is correct even with no metadata |

**Riskiest step:** step 13 (apply). It merges four sources (locks, `.nfo`, provider language, English fallback) into one write, cascades a show to its seasons and episodes, and must never lose data on a partial failure. It gets the most tests in the plan, and it's reviewed with its test table before the queue is wired to it.

## Rejected options
- **Matched columns added to `items`:** F004 writes `items` on every rescan. Separate rows keep parsing and matching independent and make "never overwrite a match" easy to prove.
- **Images as BLOBs in SQLite:** the database would grow by gigabytes, backups and `VACUUM` would get slow, and WAL churn would increase. Files on disk, addressed by hash, are simpler.
- **Pre-generating every width at download time:** settled in the intent (resize on demand). It costs disk and slows the first scan on a Pi.
- **libvips / `bimg` or WebP encoding through CGO:** breaks `CGO_ENABLED=0` cross-compiling. WebP is backlog #110.
- **Resizing through `ffmpeg`:** a process per variant is too much overhead for thumbnails. Kept as the future path for WebP only.
- **`disintegration/imaging`:** unmaintained. `golang.org/x/image/draw` covers what's needed.
- **A generic HTTP response cache for providers:** what matters is stored in tables already. Only the candidate-image proxy keeps a short in-memory cache.
- **Fetching people portraits and episode stills eagerly:** roughly 15,000 more downloads on the reference library, for images most people never open. Lazy fetching keeps the first enrichment within the budget.
- **Recording fixtures in CI:** CI would need provider keys on PR workflows (ADR 0007 forbids it), and tests would depend on the network.

## Expected evidence
What proves it's done, to paste into each PR:
- [ ] Tests added: migration upgrade, settings, permission check, F004 hooks (identify, sidecars, items changed), `access` table, certs table, `httpx` (allowlist, `429`, circuit, redaction), TMDB and fanart.tv mapping, matcher cases (50+), `.nfo`, sidecars, queue, apply, refresh, artwork selection, store, resizer, colors, variants, every handler, the e2e acceptance criteria
- [ ] `make test` green, output pasted in each PR
- [ ] Perf numbers on the throttled profile (time to all posters, full enrichment, request rate, peak memory) and the resize benchmark on a Pi-like container
- [ ] Manual check: `make dev` with a real key on a real folder (a few movies, one show with specials, a `.nfo`, a `poster.jpg`); `curl` the items, a poster at `w=342`, a fix match, an identify; then cut the network (`PLAID_TMDB_API_KEY` stays, firewall the host) and watch the items stay browsable and recover
- [ ] Manual check: a release build from a tag reports `keySource: builtin` (PR D)
- [ ] ADR 0007 updated with the provider terms check before the F005 release
- [ ] No screenshots: F005 has no UI (F007, F008)

## Log
Deviations from the plan found during the build (date + what + why).
- **2026-10-27, PR A (steps 1–6)**, by gh-prim with Claude. Divergences from the steps above:
  - **No `Identify` hook.** The reconciler only re-parses files that are still `unrecognized`, and Identify (step 25) sets `parse_status = ok` in the same transaction as the `file_identifications` row. So an identified file is never re-parsed, and a move keeps its row and links. Instead of a hook, `TestIdentifiedFileSurvivesRescansAndMoves` proves it: incremental and full rescans, a content change and a move all keep the item. Step 25 must keep writing both in one transaction.
  - **`ItemsCreated`, not "created and relinked".** The hook reports the items a scan inserted. A relinked item (a file that came back) already has its metadata row. It's called once after the scan's last commit, so a scan that fails half-way reports nothing: **step 12 adds a startup and periodic sweep** that inserts `pending` rows for items with no `item_metadata`. The hook is only the fast path.
  - **Sidecars** are reported per folder that was available in the scan, after commit (`Hooks.Sidecars(libraryID, folderPath, files)`). The walker reports `.nfo`, `.jpg`, `.jpeg`, `.png` and `.webp` files, with only the hidden-file and `.plaidignore` rules (the `sample` and partial-download rules don't apply to them). Folders like `Extras` stay skipped.
  - **No `access.ForPrincipal`.** `access` stays dependency-free (`Limit`, `Allows`, `SQL`). `accounts.Service.AccessLimit(ctx, principal)` returns the active profile's `access.Limit`. With no profile picked, it uses the account's **default** profile (F003 treats that session as unrestricted for managing profiles, which is a different rule).
  - **`items.go` qualifies its columns** (`items.`) since it now joins `item_metadata m`, and `Item.AgeRating` carries the rating so `GetItem` can return `404`.
  - **`plaid.db` is created `0600`** when it doesn't exist yet, so new installs never see the R34 warning. Existing installs get the warning until the admin runs `chmod 600`.
  - **Library metadata locale:** `metadataLocale` in `handlers/libraries.go` maps the creating admin's locale to a language and region (`en` → US, `pt` → BR, `ja` → JP, …; anything unusable → `en`/`US`). `Store.CreateLibrary`'s optional argument became `CreateOptions`.
- **2026-10-27, PR B (steps 7–15)**, by gh-prim with Claude. Divergences from the steps above:
  - **Provider types live in `internal/metadata/provider`**, not `metadata/provider.go`, so `tmdb` and the `metadata` service don't import each other. A `metadata/tmdbtest` package replays the recordings for every test that needs TMDB (a request with no recording fails the test).
  - **Recording script:** `perl` builds the file names (`iconv` fails on accents under macOS). `include_video_language` was added so French libraries get French trailers (R22). There are 124 files (1.1 MB).
  - **Matcher tuning:** two rules added to R5 (a popularity tie-break at 20×, a retry without the leading article) and English-only searches. They're recorded in the spec's Revisions with the cases they come from.
  - **Refreshes keep the match** (no new search), unmatched items retry after 7 days, and a collection that can't be fetched doesn't fail the movie. These are in the spec's Revisions too.
  - **The queue records an outcome even during shutdown** (a detached context for the bookkeeping writes): found by `TestQueuePausedAndFailedJobs`.
  - **`.nfo` files are read from disk at apply time** (the item's folders, `movie.nfo` / `<video>.nfo` / `tvshow.nfo`). `sidecar_files` and the sidecar hook are used by PR C (images, change detection). `.nfo` actors without a TMDB id aren't stored yet.
  - **New seasons and episodes of a matched show** are handled by an immediate refresh of the show (`ItemsCreated` → `refresh_at = now`), which refetches the seasons on disk. The plan's "older than 24 h" condition is dropped: a show refresh is a handful of calls.
  - **Read API:** `ItemSummary` gains `displayTitle`, `matchStatus`, `releaseDate`, `rating`, `certification`, `ageRating` and `genres`. The rest of R36 is nested under `ItemDetail.metadata` (`ItemMetadata`) instead of being flattened, so lists stay light. `Library` exposes `metadataLanguage` / `metadataRegion`. Invalid values give `422 validation_failed` (see the spec's Revisions).
  - **Keys:** `PLAID_TMDB_API_KEY` (config) and the settings table are read now. The release-time built-in key (`metadata.builtinTMDBKey`) is declared but only set by PR D. Without any key, `.nfo` data still applies and items stay `pending`.
- **2026-10-27, PR C (steps 16–22)**, by gh-prim with Claude. Divergences from the steps above:
  - **Local images are read from disk at apply time**, like the `.nfo` (the item's folders; for shows the show folder, plus `seasonNN-poster` there and `poster` in season folders). `Hooks.Sidecars` only updates `sidecar_files` and refreshes right away the movies and shows whose folder gained, changed or lost a file. There's a single place where artwork is chosen. A changed `.nfo` is applied through that refresh, so it costs one TMDB call (see the spec's Revisions, R14).
  - **Selection (R15/R16):** posters and logos in a language other than the library's or English (or no text) are left out: a Korean poster in a French library is worse than none (spec Revisions). Local images always come first, and a locked kind is never touched.
  - **Artwork runs after the metadata transaction**, never inside it: an image that can't be fetched doesn't undo a match. The failure is logged, and the image is fetched on its first request instead.
  - **Kinds:** poster, backdrop (up to 3), logo and clearart are downloaded right after the match. Movie thumbs (fanart.tv), episode stills and portraits (`people.portrait_id`) are recorded and fetched on first request. Banners only come from local files.
  - **fanart.tv:** recordings in `testdata/fanart` (`ONLY=fanart scripts/record-metadata-fixtures.sh`). `metadata/tmdbtest` also replays fanart.tv and serves generated images (JPEG or PNG at the requested TMDB size, with a color from the name).
  - **`httpx.ErrRequestFailed`** wraps network errors, so `metadata.IsProviderError` can turn a failed lazy download into `503 provider_unavailable`.
  - **Cleanup** walks the originals through an `os.Root`, so a symlink in the data directory can't make it delete anything outside.
  - **API:** `ItemSummary.poster|backdrop|still`, `ItemMetadata.logo|clearart|backdrops` and `Credit.portrait`, as `Artwork {imageId, width, height, source, colors}`. `GET /images/{imageId}?w=` returns JPEG or PNG with `ETag` / `304`, `Cache-Control: private, max-age=31536000, immutable` and `nosniff`, and a 404 for an item the profile may not see.
  - **Config:** `PLAID_FANART_API_KEY`, and `PLAID_ARTWORK_CACHE_MAX_MB` (at least 64). The fanart.tv built-in key is set by PR D, like TMDB's.
  - **Speed:** a 780 px poster resized to 342 px takes about 18 ms on an M-series Mac (`BenchmarkPosterVariant`). The Pi number comes with PR D's perf run. Apply tests turn artwork on only where they test it.
- **2026-10-27, PR D (steps 23–30)**, by gh-prim with Claude. Divergences from the steps above:
  - **`item_kind_mismatch`** is for matches that don't fit the item: a TMDB id on a season or an episode, season/episode numbers on a movie or show, or identifying a file in a library of the other type. A movie id given for a show can't be detected: TMDB ids are per type, so it's looked up as a show and gives `match_not_found` if TMDB has no show with that id.
  - **`metadata_disabled` is a `409`**: search, fix match and identify need a TMDB key.
  - **Fix match runs synchronously** and answers with the item already matched. An episode fixed with `{seasonNumber, episodeNumber}` answers with the episode its files moved to (the old one is deleted when it has no file left).
  - **Identify** creates the movie, or the show/season/episode items, directly in the metadata store (no F004 hook, see PR A). It reuses an item already matched to the same TMDB id and accepts only `unrecognized` files (`409 already_identified` otherwise).
  - **`refresh` with `images: true`** forgets the unlocked artwork choices of the item and its children, so they're picked and downloaded anew.
  - **Live counters:** the scan SSE stream subscribes to the metadata service (`Service.Subscribe`), which publishes at most one `metadata` event every 2 s per library (trailing). F004's `library.Event` is unchanged. `ScanEvent.progress` is optional now that an event can be `metadata` only.
  - **Key checks** go through `Options.CheckKey` (tests replace it). The default asks TMDB `/configuration` and fanart.tv `/movies/603`.
  - **Collections** with no movie the profile may see are left out of the list, and `GET /collections/{id}` gives `404` for them. Collection posters and backdrops are fetched on first request.
  - **Release keys:** `.goreleaser.yaml` reads `{{ index .Env "TMDB_API_KEY" }}` (empty in local builds). `release.yml` passes the secrets to the GoReleaser step only. The Dockerfile has a comment and no keys (the image isn't published).
  - **Performance:** `perf_test.go` uses a dynamic TMDB stub with 50 ms of latency behind the real `httpx` client (so the 20 req/s limit is exercised), plus generated images. On an M-series Mac: all 1,000 posters in 1 min 41 s, all 11,000 items in 2 min 37 s, 3,100 requests with at most 20 in any second, peak heap 23 MB. The throttled-container (Pi-like) run is still to do.
  - **Rate limit burst is 1** (TMDB and fanart.tv). With a burst of 20, the perf test saw 37 requests in one second, above R32's "20 requests/s".
  - **Open point:** one early perf run stopped making progress after 554 movies, with the workers idle and 446 rows due. Its logs were discarded, and two other heavy processes were running at the same time. `Claim` worked on a copy of its database, and the clean reruns (including one alongside `make test -race`) finished. The cause isn't established. Watch for it in the throttled run; the perf test now keeps warnings on stderr and logs progress every 30 s.
  - **Acceptance criteria** are covered across `metadata/*_test.go`, `metadata/artwork`, `metadata/match`, `metadata/tmdb` and `handlers/*metadata*_test.go`, not in one `e2e_test.go`.
