---
id: F005
title: Metadata and artwork
owner: gh-prim
reviewer: TBD
status: approved         # draft | approved
intent: ./intent.md
decisions: [0002, 0004, 0006, 0007]
---

# F005 — Spec: Metadata and artwork

## Summary
F005 turns F004's parsed items into rich ones. In the background, every movie, show, season and episode is matched to TMDB, from an ID hint or `.nfo` when there is one, else from its title and year with a confidence score. Matched items get a title, synopsis, genres, cast and crew, rating, a certification for the library's region, a trailer key and their TMDB collection, all in the library's language with English as the fallback. Posters, backdrops, logos, clearart, stills and portraits come from TMDB, fanart.tv and the files next to the media, local first. They are stored in the data directory, resized on demand for each client, and come with ambient colors for the UI. Uncertain matches wait for the admin's review. "Fix match" searches TMDB, applies the pick and locks it, and the same flow identifies F004's unrecognized files. F005 also enforces F003's profile age limits on the item read API. When the internet is down, everything already fetched stays available, and the work waits and retries.

## User journeys

**Admin, first library**
1. Creates a Movies library (F004). The new library's metadata language and region default to the admin's locale (`fr` → French, France). Both can be changed.
2. While F004 is still analyzing files, posters start showing up: matching runs on items as soon as they exist and doesn't wait for `ffprobe`.
3. A few minutes later most movies have a poster, synopsis and cast. The library page reads "1,212 matched · 23 need a second look · 5 not found".
4. The admin opens "Needs review". *Dune (2021)* shows its parsed name and three candidates with posters, years and scores. One click on the right one and it's matched and locked.

**Admin, fixing a wrong match**
1. *The Office* matched the UK show, but the folder holds the US show. On the show: Fix match → search "The Office 2005" → pick → done.
2. The show, its seasons and its episodes are refetched. The pick is locked, so no rescan or refresh undoes it.
3. The admin doesn't like the poster. They open "Change poster", pick another one from TMDB or fanart.tv, and that choice is locked too.

**Admin, unrecognized file**
1. `VTS_01_1.mkv` couldn't be parsed (F004 R11). From the "Not recognized" list: Identify → search "Amélie" → pick. The file becomes the movie *Amélie (2001)*, and it stays that way on every rescan.

**Admin, local metadata**
1. Their folders already hold Kodi-style `movie.nfo`, `poster.jpg` and `fanart.jpg`. PLAID uses them first and fills only the missing fields from TMDB.

**Admin, offline**
1. The internet goes down during a scan. New movies show up with their parsed titles. Items that were already matched keep their posters. The server status shows "Can't reach TMDB right now. We'll try again in a bit."
2. Once the connection is back, the queued items are matched with no action needed.

**Member with a kids profile (age limit 10)**
1. Browses the library: PG-13, R, TV-14 and unrated titles are absent from lists, and opening one of them directly returns `404`.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F005-R1 | **Trigger.** Every visible F004 item (`movie`, `show`, and through their show, `season` and `episode`) gets a metadata row in state `pending` when it's created. F004's reconciler notifies F005 (in-process callback) of created and relinked items. Matching never waits for `ffprobe`, and startup never waits on the network. | must |
| F005-R2 | **Persistent queue.** Pending work lives in the database (`item_metadata.state`, `next_attempt_at`), so a restart resumes it. A server-wide pool of 4 workers handles movies and shows by priority: new items first, then retries, then scheduled refreshes. | must |
| F005-R3 | **Sure matches.** A locked manual match wins. Otherwise: a TMDB or IMDb id from an `.nfo` `uniqueid`, then F004's `{tmdb-…}` / `{imdb-…}` hint. An IMDb id is resolved with TMDB `/find`. A sure match has confidence `1.0` and source `nfo` or `hint`. | must |
| F005-R4 | **Search matching.** Movies use `/search/movie?query=&year=`. Shows use `/search/tv?query=&first_air_date_year=`. With no result, the search runs again without the year. For each of the top 10 results, `confidence = 0.7 × titleSimilarity + 0.3 × yearScore`. `titleSimilarity` is the best normalized Levenshtein similarity against the result's title and original title, after normalizing (lowercase, accents removed, `&` → `and`, punctuation removed, leading articles in the supported languages removed). `yearScore` is `1` for the same year, `0.7` for ±1, `0.5` when the parsed year is unknown, else `0`. | must |
| F005-R5 | **Decision.** The best candidate is applied (state `matched`, source `auto`) when its confidence is ≥ 0.85 and it leads the second best by more than 0.05. Otherwise the top 5 candidates are stored and the state is `needs_review`. With no result, the state is `unmatched`. In both cases the item keeps showing its parsed title. | must |
| F005-R6 | **Seasons and episodes.** Once a show is matched, each season present on disk is fetched (`/tv/{id}/season/{n}`) and its episodes are matched by number, season 0 being specials. A date-based episode (F004 `air_date`) matches the episode with that air date, looked up in the seasons that aired that year first, then in all of them. A multi-episode file shows the metadata of each episode it holds. An episode TMDB doesn't know stays `unmatched` with its parsed data and is tried again at the show's next refresh. | must |
| F005-R7 | **Fields stored.** All kinds: title, original title, sort title (leading article moved), overview, provider ids (TMDB, IMDb, TVDB via `external_ids`). Movies: tagline, release date, runtime, genres, studios, rating (TMDB `vote_average`, `vote_count`), certification, collection, trailer. Shows: first and last air dates, status (`returning`, `ended`, `canceled`, `in_production`), networks, genres, rating, certification, trailer. Seasons: name, overview, air date, poster. Episodes: name, overview, air date, runtime, rating, still image. | must |
| F005-R8 | **Cast and crew.** Movies: the first 20 cast members by billing order, plus directors and writers. Shows: the first 20 from `aggregate_credits`, plus creators. Each person has a name, character or job, order, and a portrait. People are shared across items and libraries, keyed by TMDB person id. Episode guest stars aren't stored. | must |
| F005-R9 | **One call per movie.** A movie is fetched with `/movie/{id}?language=&append_to_response=credits,videos,images,release_dates,external_ids,translations&include_image_language={lang},en,null`. A show is one call with the same idea (`aggregate_credits`, `content_ratings`), plus one call per season on disk. | should |
| F005-R10 | **Language.** Each library has a `metadata_language` (ISO 639-1) and a `metadata_region` (ISO 3166-1 alpha-2), sent to TMDB as `language={lang}-{REGION}`. Both default, when the library is created, to the creating admin's locale (`en` → `en`/`US`, `fr` → `fr`/`FR`). They can be changed with `PATCH /libraries/{id}`, which schedules a low-priority background refresh of the library's text and re-picks its unlocked artwork. Locked choices and local images stay. | must |
| F005-R11 | **English fallback, field by field.** An empty title, overview or tagline in the library language is taken from the English translation (`translations`, so no extra call for movies and shows; one extra `en-US` season call only when episode overviews are missing). The API reports the language each text field is in. | must |
| F005-R12 | **Local `.nfo` (Kodi format).** Read when present: `movie.nfo` or `<video name>.nfo` for movies, `tvshow.nfo` in the show folder, `<episode file name>.nfo`. Fields: `title`, `originaltitle`, `sorttitle`, `plot`, `tagline`, `year`, `premiered`, `runtime`, `mpaa`, `genre`, `studio`, `set`, `uniqueid` (`tmdb`, `imdb`, `tvdb`), `actor` (`name`, `role`, `order`). A local field beats the provider field (source `local`). Fields the `.nfo` doesn't have come from TMDB. Files are capped at 1 MB and parsed with `encoding/xml`. A broken `.nfo` is logged and skipped. | must |
| F005-R13 | **Local images.** Recognized next to the media, case-insensitive, `.jpg`, `.jpeg`, `.png` or `.webp`: posters `poster`, `folder`, `cover`, `<video name>-poster`; backdrops `fanart`, `backdrop`, `background`, `<video name>-fanart`; logos `logo`, `clearlogo`; `clearart`; `banner`; `landscape`, `thumb`; season posters `season01-poster`, `season-specials-poster` in the show folder or `poster` in the season folder; episode stills `<episode file name>-thumb`. A local image is the default choice for its kind. It's read in place and never copied or modified. | must |
| F005-R14 | **Sidecar change detection.** F004's walker reports, per folder it visits, the `.nfo` and image files with their size and mtime, without probing them. F005 rereads a sidecar when it's new or changed, and applies the change without a provider call. | must |
| F005-R15 | **TMDB artwork.** Chosen per kind and language: posters, logos and season posters prefer the library language, then English, then no language. Backdrops prefer images with no text (`null` language), then the library language. Within a tier the highest `vote_average` wins, then the largest image. | must |
| F005-R16 | **fanart.tv artwork.** Movies (by TMDB id): `hdmovielogo`, `hdmovieclearart`, `moviebackground`, `movieposter`, `moviethumb`. Shows (by TVDB id from `external_ids`): `hdtvlogo`, `hdclearart`, `showbackground`, `tvposter`, `tvthumb`, `seasonposter`. Logos and clearart come from fanart.tv first and TMDB second. Posters and backdrops come from TMDB first and fanart.tv second. Without a TVDB id, a show gets no fanart.tv images. | must |
| F005-R17 | **What's downloaded, and when.** These are downloaded right after the match, in order: poster (TMDB `w780`), backdrop (`w1280`), logo (`original`, PNG), clearart, season posters. Episode stills (`w780`), people portraits (`h632`) and every non-selected candidate are fetched lazily, the first time they're asked for, then kept. | must |
| F005-R18 | **Artwork storage.** Downloaded images are stored once by content (SHA-256) under `$PLAID_DATA_DIR/artwork/originals/<2 hex>/<sha>.<ext>`, which isn't a cache: it's kept offline. Resized variants go to `$PLAID_DATA_DIR/cache/artwork/<sha>/<width>.<ext>` and can be regenerated. Nothing is written inside library folders (F004 R23). Originals no longer referenced by any `artwork` row are deleted by a daily cleanup. | must |
| F005-R19 | **On-demand resizing.** `GET /images/{imageId}?w=` serves a variant at the smallest allowed width ≥ `w`, among 92, 185, 342, 500, 780, 1280, 1920 and 3840, never upscaled past the original. Photos come as JPEG (quality 82); logos and clearart as PNG with transparency. A variant is made on first request with at most 2 resizes at once, then cached. `ETag` is the content hash plus the width. `Cache-Control: private, max-age=31536000, immutable`, since the `imageId` changes when the content changes. | must |
| F005-R20 | **Variant cache cap.** `PLAID_ARTWORK_CACHE_MAX_MB` (default 2048) caps the variant cache. Past the cap, the least recently served variants are evicted. Originals are never evicted. | should |
| F005-R21 | **Ambient colors.** When an image is stored (poster, backdrop, season poster, still), F005 computes its `dominant`, `vibrant`, `muted` and `dark` colors (hex) from a 64 px thumbnail (median cut), plus `onDominant` (`light` or `dark`) for legible text, and stores them with the image. | should |
| F005-R22 | **Trailers.** From TMDB `videos`: YouTube only, type `Trailer` first, then `Teaser`, official first, library language then English, newest first. Up to 3 are stored, as `{site, key, name, language, official}`. Clients build the YouTube URL. The server never fetches YouTube. | should |
| F005-R23 | **Collections.** A movie's `belongs_to_collection` creates or reuses a collection in the same library (name, overview, poster, backdrop from `/collection/{id}`, in the library language). Collections list their movies present in the library, by release date. | should |
| F005-R24 | **Certifications.** Movies: `release_dates` for the library region (theatrical first, then any type with a certification), else `US`. Shows: `content_ratings` for the region, else `US`. A built-in table maps each certification to F003's age values (`0`, `7`, `10`, `12`, `13`, `16`, `18`) for at least US (film and TV), GB, FR, DE, ES, IT, NL, CA, AU, BR, JP. A certification missing from the table counts as unrated. Seasons and episodes take their show's age. An `.nfo` `mpaa` value goes through the same table. | must |
| F005-R25 | **Enforcing age limits.** One shared predicate in `internal/access`, which F006 (playback) and F008 (search, filters) reuse. On every item read (`GET /libraries/{id}/items`, `GET /items/{id}`, collections, people filmographies, `GET /images/{id}` of an item's artwork), an item is left out (and returns `404` when asked for directly) when the active profile has `maxAge` and the item's age is above it, or when the item is unrated and `allowUnrated` is false. This applies to admins' profiles too. Admin-only matching routes aren't filtered. | must |
| F005-R26 | **Fix match.** `POST /items/{id}/match` (admin) with a TMDB id applies that match (source `manual`, confidence `1.0`, `match_locked`), refetches the item and, for a show, its seasons and episodes, and replaces the unlocked fields and artwork. `DELETE /items/{id}/match` clears the match and its lock and puts the item back to `pending`. | must |
| F005-R27 | **Metadata search.** `GET /metadata/search` (admin) searches TMDB movies or shows by free text (with an optional year) or looks up an id (`tmdb:603`, `imdb:tt0133093`). It returns up to 20 candidates with title, original title, year, overview, a poster and the R4 score against the item when `itemId` is given. Candidate posters are served by the server (`GET /metadata/candidate-images/{path}`, TMDB paths only), so clients never call TMDB directly. | must |
| F005-R28 | **Identifying unrecognized files.** `POST /media-files/{fileId}/identify` (admin): for a Movies library `{tmdbId}`, for a Shows library `{showTmdbId, seasonNumber, episodeNumbers[]}`. It creates or reuses the items, links the file, and records a lasting identification (`file_identifications`) that F004's reconciler honors on later scans instead of parsing the name. A move (F004 R16) keeps it. | must |
| F005-R29 | **Field edits and locks.** `PATCH /items/{id}/metadata` (admin) sets `title`, `sortTitle`, `originalTitle`, `overview`, `tagline`, `releaseDate`, `genres`, `certification`. Each edited field is locked. `unlock: [field]` releases a lock, and the field takes the provider value again on the next refresh. Refreshes and rematches never overwrite locked fields. | must |
| F005-R30 | **Artwork choice.** `GET /items/{id}/artwork?kind=` (admin) lists the candidates from local files, TMDB and fanart.tv, with their source, language, size and votes. `PUT /items/{id}/artwork/{kind}` `{candidateId}` selects one and locks the kind. `DELETE` on the same route unlocks it, back to the automatic choice. | must |
| F005-R31 | **Refresh.** Matched items are refreshed on a schedule: movies released less than 90 days ago every 7 days, other movies every 90 days; shows that are `returning` or `in_production` every 3 days, others every 90 days. A new episode file that its season doesn't have triggers a season refetch if the cached season is older than 24 h. `POST /items/{id}/refresh` and `POST /libraries/{id}/metadata/refresh` (admin) force a refresh (`{images: true}` also re-picks unlocked artwork). | must |
| F005-R32 | **Rate limits.** Token bucket per provider: TMDB 20 requests/s, fanart.tv 5 requests/s. Image downloads have their own limit of 8 at a time. A `429` honors `Retry-After`. Responses are never cached across items beyond what's stored. | must |
| F005-R33 | **Offline and errors.** A network error, timeout (10 s per request) or `5xx` never changes a match or deletes stored data. The item stays in its current state, with `next_attempt_at` backed off exponentially (1 min → 6 h, with jitter). After 5 consecutive failures a provider is marked `unreachable`, and only one probe request per minute goes out until one succeeds. A `401` marks the provider `invalid_key` and pauses its queue until the key changes. | must |
| F005-R34 | **Keys.** As in ADR 0007: the admin-saved key, then `PLAID_TMDB_API_KEY` / `PLAID_FANART_API_KEY`, then the key built in at release time. With no TMDB key, F005 is disabled (`no_api_key` status). Items keep their parsed titles and local data, and `.nfo` and local images still apply. An optional fanart.tv personal key (`fanartPersonalKey`) is sent as `client_key`. Keys are write-only in the API, never logged, and stripped from any logged URL. Admin-saved keys are stored in plain text in `settings`, and the server warns at startup if `plaid.db` isn't mode `0600`. | must |
| F005-R35 | **Status.** `GET /metadata/status` (admin) returns each provider's state (`ok`, `unreachable`, `invalid_key`, `no_api_key`, `disabled`), last error and time, which key source is in use (`custom`, `env`, `builtin`), and per-library counters (`pending`, `matched`, `needsReview`, `unmatched`). The F004 SSE stream `/libraries/{id}/scan/events` gets a `metadata` event with these counters, at most every 2 s while work is going on. | must |
| F005-R36 | **Read API.** `ItemSummary` gains `displayTitle` (the matched title, else the parsed one), `matchStatus`, `poster` and `backdrop` (`{imageId, width, height, colors}`), `rating`, `certification`, `ageRating` and `genres`. `ItemDetail` gains every R7–R8 field, `logo`, `clearart`, trailers, collection, cast and crew, field languages and locks, and its `providerIds`. Sorting, filtering and search over these stay with F008. | must |
| F005-R37 | **Attribution.** `GET /server` exposes `attributions: [{provider, name, url, notice}]` for the providers in use, so F007/F008 can show the notices TMDB and fanart.tv ask for. | must |
| F005-R38 | **Duplicates.** When two items of one library match the same TMDB id, both stay, the newer one gets `duplicateOf` pointing to the older, and it's listed in `GET /libraries/{id}/review` with the reason `duplicate`. Merging them into one item with several versions is backlog #19. | should |
| F005-R39 | **Episode-level fix.** `POST /items/{id}/match` on an episode with `{seasonNumber, episodeNumber}` relinks its files to another episode of the same show (for example a misnumbered special). | could |

## Design

### Data model
Migration `0004_metadata.sql`. Ids are UUIDv7 `TEXT`, timestamps RFC 3339 UTC, as in F003 and F004. F004's `items` table is left as F004 parses it. Matched data lives next to it, so a rescan never fights a match.

```sql
ALTER TABLE libraries ADD COLUMN metadata_language TEXT NOT NULL DEFAULT 'en';  -- ISO 639-1
ALTER TABLE libraries ADD COLUMN metadata_region   TEXT NOT NULL DEFAULT 'US';  -- ISO 3166-1 alpha-2

CREATE TABLE item_metadata (
  item_id          TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
  state            TEXT NOT NULL DEFAULT 'pending'
                   CHECK (state IN ('pending','matched','needs_review','unmatched')),
  match_source     TEXT CHECK (match_source IN ('nfo','hint','auto','manual','parent')),
  confidence       REAL,
  match_locked     INTEGER NOT NULL DEFAULT 0,
  tmdb_id          INTEGER,                  -- episodes/seasons: the show's id + numbers
  imdb_id          TEXT,
  tvdb_id          INTEGER,
  title            TEXT,
  original_title   TEXT,
  sort_title       TEXT,
  overview         TEXT,
  tagline          TEXT,
  release_date     TEXT,                     -- movie release / show first air / episode air date
  end_date         TEXT,                     -- show last air date
  show_status      TEXT,
  runtime_minutes  INTEGER,
  rating           REAL,
  rating_count     INTEGER,
  certification    TEXT,
  certification_region TEXT,
  age_rating       INTEGER,                  -- R24: 0, 7, 10, 12, 13, 16, 18 or NULL (unrated)
  genres           TEXT,                     -- JSON [{"id": 878, "name": "Science-Fiction"}]
  studios          TEXT,                     -- JSON [string] (studios or networks)
  trailers         TEXT,                     -- JSON (R22)
  collection_id    TEXT REFERENCES collections(id) ON DELETE SET NULL,
  duplicate_of     TEXT REFERENCES items(id) ON DELETE SET NULL,
  field_languages  TEXT,                     -- JSON {"overview": "en"} when fallen back
  locked_fields    TEXT,                     -- JSON ["title","poster"]
  local_fields     TEXT,                     -- JSON: fields taken from .nfo
  attempts         INTEGER NOT NULL DEFAULT 0,
  next_attempt_at  TEXT,                     -- retries (R33)
  refresh_at       TEXT,                     -- scheduled refresh (R31)
  last_error       TEXT,
  fetched_at       TEXT,
  updated_at       TEXT NOT NULL
);
CREATE INDEX item_metadata_queue ON item_metadata(state, next_attempt_at);
CREATE INDEX item_metadata_refresh ON item_metadata(refresh_at);
CREATE INDEX item_metadata_tmdb ON item_metadata(tmdb_id);

CREATE TABLE match_candidates (            -- R5, kept for needs_review items
  item_id      TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  rank         INTEGER NOT NULL,
  tmdb_id      INTEGER NOT NULL,
  title        TEXT NOT NULL,
  original_title TEXT,
  year         INTEGER,
  overview     TEXT,
  poster_path  TEXT,                         -- TMDB path, served through R27
  confidence   REAL NOT NULL,
  PRIMARY KEY (item_id, rank)
);

CREATE TABLE people (
  id            TEXT PRIMARY KEY,
  tmdb_id       INTEGER UNIQUE,
  name          TEXT NOT NULL,
  portrait_id   TEXT REFERENCES artwork(id) ON DELETE SET NULL,
  updated_at    TEXT NOT NULL
);

CREATE TABLE item_credits (
  item_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  person_id  TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
  role       TEXT NOT NULL CHECK (role IN ('cast','director','writer','creator')),
  character  TEXT,
  ord        INTEGER NOT NULL,
  PRIMARY KEY (item_id, person_id, role)
);
CREATE INDEX item_credits_person ON item_credits(person_id);

CREATE TABLE collections (
  id          TEXT PRIMARY KEY,
  library_id  TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  tmdb_id     INTEGER NOT NULL,
  name        TEXT NOT NULL,
  overview    TEXT,
  updated_at  TEXT NOT NULL,
  UNIQUE (library_id, tmdb_id)
);

CREATE TABLE artwork (
  id           TEXT PRIMARY KEY,
  owner_type   TEXT NOT NULL CHECK (owner_type IN ('item','person','collection')),
  owner_id     TEXT NOT NULL,              -- cleaned up with its owner by the store
  kind         TEXT NOT NULL CHECK (kind IN ('poster','backdrop','logo','clearart','banner','thumb','still','portrait')),
  source       TEXT NOT NULL CHECK (source IN ('local','tmdb','fanart')),
  remote_url   TEXT,                       -- allow-listed hosts only (NFR security)
  local_path   TEXT,                       -- R13, read in place
  language     TEXT,
  width        INTEGER,
  height       INTEGER,
  votes        REAL,
  sha256       TEXT,                       -- NULL until downloaded (lazy, R17)
  ext          TEXT,
  colors       TEXT,                       -- JSON (R21)
  selected     INTEGER NOT NULL DEFAULT 0, -- one selected per owner and kind (backdrops: up to 3)
  locked       INTEGER NOT NULL DEFAULT 0, -- R30
  local_mtime  TEXT,
  fetched_at   TEXT,
  created_at   TEXT NOT NULL
);
CREATE INDEX artwork_owner ON artwork(owner_type, owner_id, kind);
CREATE INDEX artwork_sha ON artwork(sha256);

CREATE TABLE file_identifications (        -- R28, honored by F004's reconciler
  file_id         TEXT PRIMARY KEY REFERENCES media_files(id) ON DELETE CASCADE,
  kind            TEXT NOT NULL CHECK (kind IN ('movie','episode')),
  tmdb_id         INTEGER NOT NULL,        -- movie id, or show id for episodes
  season_number   INTEGER,
  episode_numbers TEXT,                    -- JSON [int]
  created_by      TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at      TEXT NOT NULL
);

CREATE TABLE sidecar_files (               -- R14, reported by F004's walker
  path        TEXT PRIMARY KEY,
  library_id  TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  size        INTEGER NOT NULL,
  mtime       TEXT NOT NULL,
  seen_at     TEXT NOT NULL
);

CREATE TABLE settings (                    -- server-wide key/value, first user: F005 keys
  key         TEXT PRIMARY KEY,
  value       TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
```

Seasons and episodes inherit from their show: `match_source = 'parent'`, and `tmdb_id` holds the show's id. The episode's identity is the show id plus its numbers.

### Architecture
A new `internal/metadata` package:
- **Providers**: `tmdb` and `fanart` clients behind a small `Provider` interface (search, details, season, images), so TVDB can be added later. Base URLs can be injected, so tests replay recorded JSON from `testdata/` through `httptest`, and `make test` never calls the network.
- **Matcher**: pure functions for normalizing, similarity, scoring and the decision (R4, R5), table-tested with no I/O.
- **NFO**: Kodi `.nfo` parser (R12). **Sidecars**: local image naming rules (R13).
- **Queue**: picks due rows from `item_metadata`, a pool of 4 workers, per-provider token buckets and circuit state (R2, R32, R33).
- **Artwork**: downloader (host allowlist, size and pixel caps), content-addressed store, resizer (`golang.org/x/image/draw`, CatmullRom; decoding with `image/jpeg`, `image/png`, `golang.org/x/image/webp`), color extraction, variant cache with LRU (R17–R21). Pure Go, so it builds with `CGO_ENABLED=0`.
- **Certifications**: embedded table `certifications.json` and the age lookup (R24).
- **Hooks into F004**: `library.Engine` gets an `ItemsChanged(libraryID, itemIDs)` callback and a `SidecarsSeen(libraryID, []Sidecar)` callback. The reconciler checks `file_identifications` before it parses a name (R28). These are the only changes to F004's package.
- **Age filter**: one SQL predicate used by every item read query (R25), built from the session's active profile (`auth.Principal`).

### API
Every route is added to `api/openapi.yaml` first, under `/api/v1`, with the `metadata` tag. Item routes keep F004's `libraries` tag.

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/libraries/{id}/items` | session | F004, plus R36 fields and the R25 filter |
| GET | `/items/{id}` | session | F004, plus R36 detail and the R25 filter |
| PATCH | `/libraries/{id}` | admin | F004, plus `metadataLanguage`, `metadataRegion` (R10) |
| GET | `/libraries/{id}/review` | admin | Paginated `needs_review`, `unmatched` and duplicate items, each with a `reason` and its candidates (R5, R38) |
| POST | `/libraries/{id}/metadata/refresh` | admin | `{images?: bool}` → `202` (R31) |
| GET | `/metadata/search` | admin | `?kind=movie\|show&query=&year=&itemId=` → `{candidates: [...]}` (R27) |
| GET | `/metadata/candidate-images/{path}` | admin | TMDB `w185` proxy; `path` must match `^[A-Za-z0-9]+\.(jpg\|png)$` (R27) |
| POST | `/items/{id}/match` | admin | `{tmdbId}`, or for an episode `{seasonNumber, episodeNumber}` (R26, R39) → `200` ItemDetail |
| DELETE | `/items/{id}/match` | admin | Unmatch, back to `pending` → `204` |
| PATCH | `/items/{id}/metadata` | admin | `{title?, sortTitle?, …, unlock?: [field]}` → `200` (R29) |
| POST | `/items/{id}/refresh` | admin | `{images?: bool}` → `202` |
| GET | `/items/{id}/artwork` | admin | `?kind=` → candidates (R30) |
| PUT | `/items/{id}/artwork/{kind}` | admin | `{candidateId}` → `200` |
| DELETE | `/items/{id}/artwork/{kind}` | admin | Unlock, back to automatic → `204` |
| POST | `/media-files/{fileId}/identify` | admin | R28 → `200` ItemDetail |
| GET | `/images/{imageId}` | session | `?w=` → image bytes (R19); `404` if filtered (R25) |
| GET | `/collections` | session | `?libraryId=` (R23) |
| GET | `/collections/{id}` | session | Collection with its movies |
| GET | `/people/{id}` | session | Name, portrait, and the items they appear in that the profile may see |
| GET | `/metadata/status` | admin | R35 |
| GET | `/metadata/settings` | admin | `{tmdb: {keySource}, fanart: {keySource, personalKeySet}}`, never the keys |
| PATCH | `/metadata/settings` | admin | `{tmdbApiKey?: string\|null, fanartApiKey?: string\|null, fanartPersonalKey?: string\|null}`. The key is checked against the provider first (TMDB `/configuration`); `null` goes back to env/built-in |

SSE: `/libraries/{id}/scan/events` gains `event: metadata` with `{pending, matched, needsReview, unmatched}` (R35).

New error codes: `invalid_api_key`, `provider_unavailable` (`503`, search or match while unreachable), `metadata_disabled`, `match_not_found` (unknown TMDB id), `item_kind_mismatch` (e.g. a movie id on a show), `field_not_editable`, `invalid_language`, `invalid_region`, `already_identified`.

Image response: `Content-Type: image/jpeg` or `image/png`, `ETag`, `Cache-Control: private, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`. Images need a session, like everything else (F003 deny-by-default). `<img>` tags work through the web cookie, and native apps send the bearer token.

### Behavior per app
- **server**: as above. The metadata queue starts with the server, after the library engine, and doesn't block startup.
- **web**: no new screens in F005. F007 builds the admin views (review, fix match, artwork choice, language, keys, status, attribution on the About page). F008 builds browsing (posters, detail pages, colors). Until then, the existing F004 pages may show `displayTitle`.
- **apple / android**: out of scope. The API is designed for them: width-based image variants and colors.

### UI / Art direction
No screens in F005. Reference copy for F007, following the design system's warm tone:
- Matching in progress: "Fetching posters and synopses… grab a blanket."
- Review needed: "23 titles need a second look."
- Not found: "We couldn't find this one. Try another title or a TMDB link."
- Offline: "Can't reach TMDB right now. Your library is still here, and we'll try again in a bit."
- Invalid key: "TMDB didn't accept this key. Check it and try again."
- Empty synopsis: "No synopsis yet."

### i18n
The server returns codes, states and data, never UI sentences. Keys proposed for F007 and F008 to adopt (`en` / `fr`):

| Key | en | fr |
|-----|----|----|
| `metadata.state.pending` | Fetching posters and synopses… grab a blanket. | On récupère affiches et résumés… attrape un plaid. |
| `metadata.review.count` (plural) | {count, plural, one {# title needs a second look.} other {# titles need a second look.}} | {count, plural, one {# titre mérite un second coup d'œil.} other {# titres méritent un second coup d'œil.}} |
| `metadata.unmatched.hint` | We couldn't find this one. Try another title or a TMDB link. | Impossible de trouver celui-ci. Essaie un autre titre ou un lien TMDB. |
| `metadata.provider.unreachable` | Can't reach TMDB right now. Your library is still here, and we'll try again in a bit. | Impossible de joindre TMDB pour l'instant. Ta bibliothèque est toujours là, on réessaie dans un moment. |
| `errors.invalid_api_key` | TMDB didn't accept this key. Check it and try again. | TMDB n'a pas accepté cette clé. Vérifie-la et réessaie. |
| `metadata.overview.empty` | No synopsis yet. | Pas encore de résumé. |
| `metadata.fixMatch` | Fix match | Corriger la correspondance |
| `about.attribution.tmdb` | This product uses the TMDB API but is not endorsed or certified by TMDB. | Ce produit utilise l'API TMDB mais n'est ni approuvé ni certifié par TMDB. |

Dates (release, air) are stored as ISO dates and formatted by clients (`Intl.DateTimeFormat`). Runtimes are formatted as durations by clients, and ratings with `Intl.NumberFormat`. Metadata text is in the library's language (R10), see the watch-outs.

## Non-functional requirements
- **Performance (Raspberry Pi 4, 50 Mbit/s, 1,000 movies + 100 shows / 9,000 episodes)**: every movie poster is visible within 10 min of the items existing; full enrichment (eager artwork) in under 45 min; at most 20 TMDB requests/s; peak memory added by F005 < 150 MB; idle CPU ~0 when the queue is empty.
- **Images**: making a `w342` poster variant from a `w780` original takes < 300 ms p95 on a Pi 4; a cached variant is served in < 20 ms; at most 2 resizes at once, so playback isn't starved.
- **Disk**: about 2 GB of originals for the reference library, plus the variant cap (R20). Documented in `docs/install.md` as part of the data directory, which should be backed up.
- **Resilience**: losing the network or the provider never deletes or downgrades data (R33). A crash mid-fetch leaves rows consistent (each item is written in one transaction), and the work resumes from the queue.
- **Security**: outgoing HTTP only to `api.themoviedb.org`, `image.tmdb.org`, `webservice.fanart.tv`, `assets.fanart.tv` (and their redirects to the same hosts), over HTTPS. Downloads are capped at 20 MB and 8,000 × 8,000 px (`DecodeConfig` before decoding). The candidate-image proxy only takes TMDB path names. `.nfo` files are capped at 1 MB. Keys are write-only and never logged. Every admin route goes through `RequireAdmin`.
- **Privacy**: PLAID sends providers the titles, years and ids of the library's items and the server's IP. No account, profile or watch data. This is documented in `docs/install.md`. Clients never call providers directly.
- **Tests**: no network in `make test`; TMDB and fanart.tv are replayed from recorded responses.

## ⚠️ Watch-outs
Settled by gh-prim on 2026-10-27. ✅ = resolved, ⏸ = tracked elsewhere.

- [x] **API keys** — ✅ [ADR 0007](../../decisions/0007-metadata-provider-keys.md) accepted (option B): keys injected at release build time from CI secrets, admin override, env var for dev builds.
- [x] **Contradiction with CLAUDE.md** — ✅ CLAUDE.md updated: metadata follows the library's language. Per-profile metadata languages are backlog #109.
- [x] **F004 changes** — ⏸ The three hooks (items-changed callback, sidecar reporting, `file_identifications` in the reconciler) land in F005's first PR, with F004's tests extended, before any metadata code.
- [x] **Age enforcement** — ✅ F005 owns the shared predicate (`internal/access`, R25). F006's and F008's intents now say they must apply it. Parental controls aren't advertised until F006 enforces it too.
- [x] **WebP output** — ✅ JPEG and PNG for v1 (pure Go). WebP/AVIF variants are backlog #110.
- [x] **4K TV backdrops** — ⏸ `w1280` for now. A high-resolution setting comes with the TV apps (backlog #111).
- [x] **Threshold and weights** — ⏸ The plan builds a table of 50+ real names (remakes, foreign titles, numbers in titles, *The Office* US/UK) and tunes 0.85 / 0.05 / 0.7 / 0.3 against it before freezing them.
- [x] **Duplicates** — ✅ Both kept, flagged in the review list with reason `duplicate` (R38). Merging is backlog #19 and #71.
- [x] **Anime and alternative orderings** — ⏸ Backlog #112, together with TVDB.
- [x] **Library language change** — ✅ Refetches text and re-picks unlocked artwork in a low-priority background refresh (R10).
- [x] **Provider terms** — ⏸ The owner creates the TMDB and fanart.tv apps and checks the terms on caching, attribution and revocation, noted in ADR 0007. Blocks the release of F005, not the plan.
- [x] **Images and auth** — ✅ Images need a session for now. Signed short-lived URLs come with F006's signed playback links (open question in F006's intent).
- [x] **Settings table** — ✅ Plain text in `settings`, startup warning if `plaid.db` isn't `0600` (R34). Keys stay write-only and are never logged.

## Acceptance criteria
- [ ] Given `Alien (1979) {tmdb-348}.mkv`, when it's scanned, then the item is `matched` with source `hint`, confidence 1, TMDB id 348, without any search call.
- [ ] Given a `movie.nfo` with `<uniqueid type="imdb">tt0133093</uniqueid>` and `<plot>Local plot</plot>`, then the item is matched to TMDB 603 through `/find`, and its overview is "Local plot" (source `local`).
- [ ] Given the matcher test table, then every case gets the expected decision, including at least: *The Matrix (1999)* → 603 `matched`; *Dune (2021)* → 438631 `matched`, and *Dune* without a year → `needs_review` (1984 vs 2021); *Amelie (2001)* without the accent → 194 `matched`; *The Office* (shows, no year) → `needs_review`; *Asdfgh (2020)* → `unmatched`.
- [ ] Given a matched show with `Season 01` and `Specials` on disk, then exactly one season call is made for season 1 and one for season 0, and each episode file is linked to its TMDB episode name and still.
- [ ] Given a date-based episode `The Daily Show/2024-10-27.mkv`, then it matches the episode that aired that day.
- [ ] Given a French library and a movie whose French overview is empty, then the overview is the English one and `fieldLanguages.overview` is `en`.
- [ ] Given a movie folder with `poster.jpg`, then the selected poster is that local file, and nothing in the folder is created or modified (tree hash checked as in F004).
- [ ] Given `fanart.jpg` is added to a movie folder after the match, then after the next scan the selected backdrop is the local file, with no TMDB call.
- [ ] Given a poster original of 780 px, when `GET /images/{id}?w=300` is called, then a 342 px wide JPEG comes back. A second call is served from the variant cache with the same `ETag`, and a request with `If-None-Match` returns `304`.
- [ ] Given `?w=5000`, then the original width is served, never upscaled.
- [ ] Given a matched movie, then its poster has `dominant`, `vibrant`, `muted` and `dark` colors as `#rrggbb` and an `onDominant` value.
- [ ] Given a movie certified `PG-13` in the US and a library region `US`, then `ageRating` is 13. Given a profile with `maxAge = 12`, when it lists the library or opens that item or its poster, then the movie is left out and the direct calls return `404`.
- [ ] Given a profile with `maxAge = 16` and `allowUnrated = false`, then items without a certification are left out.
- [ ] Given a show matched to the wrong TMDB id, when the admin calls `POST /items/{id}/match` with the right id, then the show and all its episodes carry the new metadata, `matchLocked` is true, and a full rescan plus `POST /items/{id}/refresh` don't change the match.
- [ ] Given an edited and locked title, when the item is refreshed, then the title stays. After `unlock: ["title"]` and a refresh, it's the provider's title again.
- [ ] Given an unrecognized file identified as TMDB movie 194, then it's linked to a movie item *Amélie*, and after a full rescan and a move of the file inside the library, it's still the same item.
- [ ] Given the TMDB stub returns network errors, when 10 new items are scanned, then they're listed with their parsed titles in `pending`, already matched items keep their metadata and posters, `/metadata/status` reports `unreachable`, and once the stub recovers they all end up `matched` without any manual action.
- [ ] Given the TMDB stub returns `429` with `Retry-After: 2`, then no request goes out to it for 2 s.
- [ ] Given the TMDB stub returns `401`, then the status is `invalid_key` and the queue pauses. `PATCH /metadata/settings` with a key the stub rejects returns `422 invalid_api_key` and stores nothing.
- [ ] Given no TMDB key at all, then the server starts, `/metadata/status` reports `no_api_key`, and `.nfo` data and local posters still apply.
- [ ] Given any request log, metadata status or error, then no API key value appears in it.
- [ ] Given `plaid.db` with mode `0644`, when the server starts, then it logs a warning about the file's permissions.
- [ ] Given a member session, when it calls any admin metadata route, then `403 forbidden`.
- [ ] Given `GET /metadata/candidate-images/../../etc/passwd` or a non-TMDB path, then `400`, with no outgoing request.
- [ ] Given a movie belonging to a TMDB collection with two of its movies in the library, then `GET /collections?libraryId=` lists the collection with both movies in release order.
- [ ] Given a movie with YouTube trailers in French and English and a French library, then the first stored trailer is the official French one.
- [ ] Given two movie folders both matched to TMDB 603, then both items stay, the newer one has `duplicateOf` set to the older one, and it appears in the review list with reason `duplicate`.
- [ ] Given the recorded-response perf profile (1,000 movies, 100 shows, 9,000 episodes, TMDB stub with 50 ms latency), then enrichment finishes within the NFR budget and never goes above 20 requests/s to the stub.

## Revisions
- **2026-10-27, PR B (matcher tuning, plan step 10).** The weights (0.7 / 0.3), the threshold (0.85) and the margin (0.05) are kept. Two rules are added to R5, tuned on 86 real names (`server/internal/metadata/match/testdata/cases.json`):
  - **Popularity breaks an exact tie.** Within the margin, the best candidate still wins when it's at least **20×** more popular than every candidate in the tie. That way *Alien* (1979) beats an obscure *The Alien* (1979) at 83×, and *The Daily Show* beats its namesakes at 110×. *The Office* (US/UK, 12.5×), *Dune*, *Oldboy* and the other remakes without a year still go to review. So do *Arrival* (2016, 12.5×) and *It* (2017, 15.6×): popularity can't tell them apart from *The Office*.
  - **Retry without a leading article.** When no candidate reaches the threshold and the title starts with an article, R4 searches once more without it. TMDB's English search doesn't find "Les Intouchables", but finds "Intouchables".
  - Searches always use English (`en-US`). Release names usually carry English titles, and the original title catches the rest (*Der Untergang*, *La Casa de Papel*, *Le Dîner de cons*).
- **2026-10-27, PR B (apply).**
  - A refresh never searches again: a matched item keeps its TMDB id, so a change in popularity can't send it back to review. The order of R3 is: locked match → `.nfo` id → current match → file name hint → search.
  - `unmatched` items are tried again after 7 days (TMDB may add the title).
  - A collection that can't be fetched doesn't fail the movie (R23 is a should): it's retried at the next refresh.
  - Invalid `metadataLanguage` / `metadataRegion` values return `422 validation_failed` with the field, like the rest of `PATCH /libraries/{id}`, instead of the separate `invalid_language` / `invalid_region` codes.
- **2026-10-27, PR C (artwork).**
  - **R14:** a changed `.nfo` is applied through an immediate refresh of its movie or show, which makes one TMDB call. The plan simplified sidecar handling to "detect, then refresh", so artwork and `.nfo` fields are chosen in one place.
  - **R15:** posters and logos whose language is neither the library's, English nor "no text" are left out of the candidates, rather than ranked last.
- **2026-10-27, PR D (admin API).**
  - `item_kind_mismatch` covers matches that can't fit the item (a TMDB id on a season or episode, episode numbers on a movie). A movie id sent for a show can't be told apart, because TMDB ids are per type: it gives `match_not_found` when TMDB has no show with that id.
  - `metadata_disabled` is `409`.
  - Collections with no movie the active profile may see are hidden (R23, R25).
