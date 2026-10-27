---
id: F004
title: Libraries and file scanning
owner: gh-prim
reviewer: TBD
status: draft            # draft | approved
intent: ./intent.md
decisions: [0001, 0002, 0005, 0006]
---

# F004 — Spec: Libraries and file scanning

## Summary
F004 turns folders on disk into a structured library. The admin creates a library of type Movies or Shows and points it at one or more folders. The scanner walks them, recognizes movies (title, year, edition) and episodes (show, season, episode numbers) from file and folder names, analyzes every video with `ffprobe` (container, codecs, resolution, HDR, audio and subtitle tracks, external subtitle files), and stores it all in SQLite. After the first scan it keeps the library up to date by watching the file system, with a periodic rescan for network mounts and as a safety net. Files are never modified. Matching items to TMDB and fetching artwork is F005; F004 only knows what the names and the files themselves say.

## User journeys

**Admin, first library**
1. Settings → Libraries → Add library. Picks a type (Movies or Shows), a name ("Movies"), and one or more folders with a folder picker that browses the server's disk.
2. The server checks each folder exists and is readable by the PLAID process. If not: "PLAID can't read this folder." with a link to the permissions help (`docs/install.md`).
3. Saves. A full scan starts. The library page shows progress: files found, files analyzed, current folder, and an estimate.
4. When it's done: "Scan complete. 1,240 movies found." Files whose names couldn't be understood are listed under "Not recognized", with their paths.

**Admin, daily life**
1. Copies `Dune Part Two (2024)/Dune Part Two (2024).mkv` into the Movies folder. A few seconds after the copy finishes, the movie appears in the library, already analyzed. Nothing to click.
2. Renames or moves a folder inside the library: the item stays the same (same watch history, F006), only its path changes.
3. Unplugs the USB drive by mistake: the library shows "Folder unavailable"; nothing is deleted. Plugs it back: everything is there.
4. On a NAS share (SMB/NFS) where changes aren't reported, new files show up after the next periodic scan (15 min by default), or right away with "Scan now".

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F004-R1 | Libraries have a name (1–64 chars), a type (`movies` or `shows`, fixed after creation), and 1–16 folders. Only admins create, edit, delete or scan libraries (F003 `RequireAdmin`). Every signed-in account can read them (sharing rules come with F009). | must |
| F004-R2 | Folder validation on save: absolute path, exists, is a directory, readable and traversable by the PLAID process. Paths are cleaned and symlinks resolved; a folder can't be inside or contain another library's folder. | must |
| F004-R3 | Folder picker API: lists sub-directories of a given absolute path (names only, no files), admin only, with each entry's readability. Starts from `/` or a configured list of roots (`PLAID_BROWSE_ROOTS`). | must |
| F004-R4 | Video files recognized by extension: `mkv mp4 m4v avi mov wmv ts m2ts webm mpg mpeg flv ogv 3gp`. Case-insensitive. | must |
| F004-R5 | Always ignored: hidden files and folders (leading `.`), `@eaDir`, `#recycle`, `$RECYCLE.BIN`, `System Volume Information`, `lost+found`, partial downloads (`.part`, `.!qB`, `.crdownload`, `.tmp`), samples (`sample` as a word in the name and under 300 MB), and folders named `Extras`, `Featurettes`, `Behind The Scenes`, `Deleted Scenes`, `Trailers`, `Interviews`, `Shorts` (extras come later, backlog #23). | must |
| F004-R6 | `.plaidignore` files (gitignore syntax) in any folder exclude matching paths below it. | should |
| F004-R7 | Movie parsing, from the file name, else its folder name: title and year (`Movie (2019)`, `Movie.2019.1080p.BluRay.x264-GRP`, `Movie [2019]`), edition (`{edition-Director's Cut}` and the words `Director's Cut`, `Extended`, `Unrated`, `Theatrical`, `Remastered`, `IMAX`), and hints (`{tmdb-12345}`, `{imdb-tt0133093}`) kept for F005. Release tags (resolution, source, codec, group) are stripped from the title. | must |
| F004-R8 | Several files of the same movie (`Movie (2019) - 1080p.mkv` and `Movie (2019) - 2160p.mkv` in one folder, or with different editions) become one movie item with several files, each file keeping its edition label. | must |
| F004-R9 | Show parsing: the show is the top folder under the library folder; seasons from folders `Season 01`, `Season 1`, `S01`, `Specials` (season 0) and their localized forms (`Saison`, `Staffel`, `Temporada`, `Stagione`, `Seizoen`, `Sezon`, `Säsong`, `Sæson`, `Kausi`, `Сезон`); episodes from the file name: `S01E02`, `s1e2`, `1x02`, multi-episode `S01E01-E03` / `S01E01E02`, and date-based `2024-10-27` / `2024.10.27`. The season in the file name wins over the folder. | must |
| F004-R10 | A multi-episode file is linked to every episode it contains. | must |
| F004-R11 | Files that can't be parsed (no title, or no episode number in a Shows library) are stored with status `unrecognized` and listed per library; they don't block the scan. Manually identifying them is F005. | must |
| F004-R12 | `ffprobe` analysis per file: container, duration, overall bitrate; per video stream: codec, profile, width, height, frame rate, bit depth, HDR format (HDR10, HDR10+, Dolby Vision profile, HLG); per audio stream: codec, channels, layout, language, title, default, forced; per subtitle stream: codec, language, title, default, forced, image-based or text. Raw probe JSON kept for later features. Timeout 30 s per file; a failed probe marks the file `probe_failed` and retries on the next scan. | must |
| F004-R13 | External subtitles next to a video (`Movie (2019).srt`, `.en.srt`, `.fr.forced.srt`, `.en.sdh.srt`, `.ass`, `.ssa`, `.vtt`, `.sub` + `.idx`) are linked to it, with language (ISO 639-1/2 codes and English language names), `forced` and `sdh` flags. | must |
| F004-R14 | Items and files are separate: items (`movie`, `show`, `season`, `episode`) are what people browse and watch; files are what's on disk. A file links to one or more items, an item to one or more files. Watch state (F006) attaches to items, never to files. | must |
| F004-R15 | Incremental scan: a file whose path, size and modification time are unchanged is skipped without probing. Startup never triggers a full rescan. | must |
| F004-R16 | Move detection: a new file whose size and quick fingerprint (SHA-256 of size + first and last 1 MiB) match a file that just went missing in the same library takes over its row, so its items and watch history stay. | must |
| F004-R17 | Missing files: a file not found during a scan gets `missing_since`; items with no remaining file are hidden, not deleted. After 30 days missing they're removed. If a library folder itself is unreachable or empty when it had files, the scan stops for that folder and marks it `unavailable` without touching its files. | must |
| F004-R18 | File watching (fsnotify / inotify, recursive) on local file systems: changes are debounced (a file is processed once its size has been stable for 10 s) and scanned incrementally. Network and FUSE file systems (`nfs`, `cifs`/`smb`, `fuse.*`, `9p`) aren't watched. If the inotify watch limit is reached, the folder falls back to periodic scans and the admin sees a warning. | must |
| F004-R19 | Periodic scan of every library, default every 15 minutes for unwatched folders and every 6 hours for watched ones (safety net). Interval configurable per library (5 min to 24 h). | must |
| F004-R20 | Manual scan: "Scan now" per library (incremental) and "Full rescan" (re-probes every file). One scan per library at a time; a request during a running scan is queued once. | must |
| F004-R21 | Scan progress: state (`idle`, `queued`, `scanning`, `analyzing`), counters (found, new, changed, missing, analyzed, failed, unrecognized), current folder, start time, readable through the API while it runs, plus a server-sent events stream for live updates. | must |
| F004-R22 | Read API for items: list a library's items by kind and parent (shows → seasons → episodes) with parsed title, year, numbers, file count and technical summary (resolution, HDR, audio languages). Enough for F005 to match and for F007/F008 to show something; sorting, filters and search belong to F008. | should |
| F004-R23 | Deleting a library removes its rows (folders, files, items no longer linked) and never touches files on disk. PLAID never writes inside library folders. | must |
| F004-R24 | Scanning runs in the background with a bounded worker pool: file walking on one goroutine per library, probing on `min(4, NumCPU)` workers server-wide, database writes batched (≤ 500 rows per transaction). | must |
| F004-R25 | `ffprobe` is found at startup (`PLAID_FFPROBE_PATH`, else `PATH`); if it's missing, libraries can still be created and files listed, analysis is skipped with a clear warning in the logs and the API (`ffprobe_missing`). Distribution is decided by ADR 0006. | must |

## Design

### Data model
Migration `0003_libraries.sql`. Ids are UUIDv7 `TEXT` (F003 R24), timestamps RFC 3339 UTC.

```sql
CREATE TABLE libraries (
  id               TEXT PRIMARY KEY,
  name             TEXT NOT NULL,
  type             TEXT NOT NULL CHECK (type IN ('movies','shows')),
  scan_interval_s  INTEGER,                 -- NULL = defaults (R19)
  last_scan_at     TEXT,
  created_at       TEXT NOT NULL,
  updated_at       TEXT NOT NULL
);

CREATE TABLE library_folders (
  id           TEXT PRIMARY KEY,
  library_id   TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  path         TEXT NOT NULL UNIQUE,        -- cleaned, symlinks resolved
  fs_type      TEXT,                        -- from statfs, e.g. ext4, nfs, cifs
  watched      INTEGER NOT NULL DEFAULT 0,
  status       TEXT NOT NULL DEFAULT 'ok' CHECK (status IN ('ok','unavailable','unreadable')),
  created_at   TEXT NOT NULL
);

CREATE TABLE items (
  id              TEXT PRIMARY KEY,
  library_id      TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  kind            TEXT NOT NULL CHECK (kind IN ('movie','show','season','episode')),
  parent_id       TEXT REFERENCES items(id) ON DELETE CASCADE,  -- season → show, episode → season
  title           TEXT,                     -- parsed; F005 adds matched metadata
  sort_title      TEXT,
  year            INTEGER,
  season_number   INTEGER,
  episode_number  INTEGER,
  air_date        TEXT,                     -- date-based episodes
  hints           TEXT,                     -- JSON: {"tmdb": "12345", "imdb": "tt…"}
  hidden          INTEGER NOT NULL DEFAULT 0, -- no file left (R17)
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
CREATE INDEX items_library_kind ON items(library_id, kind);
CREATE INDEX items_parent ON items(parent_id);

CREATE TABLE media_files (
  id              TEXT PRIMARY KEY,
  library_id      TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  folder_id       TEXT NOT NULL REFERENCES library_folders(id) ON DELETE CASCADE,
  path            TEXT NOT NULL UNIQUE,
  size            INTEGER NOT NULL,
  mtime           TEXT NOT NULL,
  fingerprint     BLOB,                     -- R16
  edition         TEXT,
  parse_status    TEXT NOT NULL CHECK (parse_status IN ('ok','unrecognized')),
  probe_status    TEXT NOT NULL DEFAULT 'pending' CHECK (probe_status IN ('pending','ok','failed','skipped')),
  container       TEXT,
  duration_ms     INTEGER,
  bitrate         INTEGER,
  width           INTEGER,
  height          INTEGER,
  video_codec     TEXT,
  hdr_format      TEXT,
  probe_json      TEXT,
  probed_at       TEXT,
  missing_since   TEXT,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
CREATE INDEX media_files_library ON media_files(library_id);
CREATE INDEX media_files_missing ON media_files(library_id, missing_since);

CREATE TABLE item_files (
  item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  file_id  TEXT NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
  PRIMARY KEY (item_id, file_id)
);

CREATE TABLE media_streams (
  file_id     TEXT NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
  idx         INTEGER NOT NULL,             -- ffprobe stream index; external subs use 1000+
  type        TEXT NOT NULL CHECK (type IN ('video','audio','subtitle')),
  codec       TEXT,
  language    TEXT,                         -- ISO 639-2/B, 'und' if unknown
  title       TEXT,
  channels    INTEGER,
  is_default  INTEGER NOT NULL DEFAULT 0,
  is_forced   INTEGER NOT NULL DEFAULT 0,
  is_sdh      INTEGER NOT NULL DEFAULT 0,
  is_image    INTEGER NOT NULL DEFAULT 0,   -- PGS, VobSub
  external_path TEXT,                       -- external subtitle file
  PRIMARY KEY (file_id, idx)
);
```

### Scanner architecture
`internal/library` package:
- **Walker**: walks a folder (`filepath.WalkDir`), applies ignore rules (R5, R6), and emits candidate files. It never follows symlinks out of the library folder (no loops, no escaping).
- **Parser**: pure functions `ParseMovie(relPath)` and `ParseEpisode(relPath)`, table-tested with real-world names (see Acceptance criteria). No I/O.
- **Reconciler**: compares the walk with the database (R15–R17), creates and relinks items, writes in batches.
- **Prober**: worker pool calling `ffprobe -v error -print_format json -show_format -show_streams`, with a timeout and `nice`-level priority so playback isn't affected.
- **Watcher**: one fsnotify watcher per watched folder, debounced per path (R18).
- **Scheduler**: one queue per library, periodic timers (R19), manual triggers (R20), progress state (R21) published to SSE subscribers.

### API
All admin-only except the reads. Added to `api/openapi.yaml` first.

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/libraries` | session | List with folder status, item counts, scan state |
| POST | `/libraries` | admin | `{name, type, folders: [path], scanIntervalSeconds?}` → `201`, starts a full scan |
| GET | `/libraries/{id}` | session | Detail |
| PATCH | `/libraries/{id}` | admin | `{name?, folders?, scanIntervalSeconds?}`; adding a folder scans it |
| DELETE | `/libraries/{id}` | admin | `204` (R23) |
| POST | `/libraries/{id}/scan` | admin | `{full: false}` → `202` |
| GET | `/libraries/{id}/scan` | admin | Current progress (R21) |
| GET | `/libraries/{id}/scan/events` | admin | `text/event-stream`: `progress`, `completed`, `failed` events |
| GET | `/libraries/{id}/unrecognized` | admin | Paginated list of unrecognized files |
| GET | `/libraries/{id}/items` | session | `?kind=&parentId=&cursor=&limit=` (R22) |
| GET | `/items/{id}` | session | Item with files and streams |
| GET | `/fs/browse` | admin | `?path=` → `{path, parent, entries: [{name, path, readable}]}` (R3) |
| GET | `/server/capabilities` | admin | `{ffprobe: {found, version}, inotifyLimit, inotifyUsed}` |

Pagination: cursor-based (`nextCursor`), `limit` ≤ 200. New error codes: `folder_not_found`, `folder_unreadable`, `folder_overlap`, `library_type_immutable`, `scan_in_progress` (queued instead, returns `202`), `ffprobe_missing`, `not_found`.

### Behavior per app
- **server**: as above. The scan engine starts with the server, loads libraries, arms watchers and timers.
- **web**: none required by F004. The library screens (create, folder picker, progress, unrecognized list) are F007's admin area, built on this API. A developer can drive F004 entirely through the API.
- **apple / android**: out of scope.

### UI / Art direction
No screens in F004. The API returns machine-readable states and counters; F007 turns them into copy following the design system content rules (for example "Scan complete. 12 new movies are ready.", the reference line in the design system README).

### i18n
No user-facing strings in F004 itself: error `code`s and scan states are translated by clients (`errors.folder_unreadable`, `scan.state.analyzing`, …), defined in F007. Language codes from files and probes are stored as ISO 639-2/B; display names come from `Intl.DisplayNames` on clients, never from the server.

## Non-functional requirements
- **Performance (10,000 files, Raspberry Pi 4, USB 3 disk)**: first walk and parse < 2 min; first full analysis < 45 min in the background (ffprobe dominates); an unchanged rescan < 60 s with zero probes; idle CPU ~0 between events; memory < 150 MB during a scan.
- **Startup**: the server is serving requests in < 1 s even with 10 libraries; watchers are armed in the background.
- **Safety**: read-only on media (R23), never follows symlinks outside library folders, never deletes rows because a mount disappeared (R17).
- **Resilience**: a crash mid-scan leaves a consistent database (batches are transactions); the next scan resumes from what's stored.
- **Security**: folder browse and library management are admin-only; `/fs/browse` lists directory names only and rejects relative paths and `..`.

## ⚠️ Watch-outs
- [ ] **ffprobe isn't installed today** (neither in the Docker image nor as a `.deb` dependency). F004 needs it; F006 needs FFmpeg. Proposed [ADR 0006](../../decisions/0006-ffmpeg-distribution.md) must be decided before F004's plan.
- [ ] **Unrecognized files** (open question in the intent): spec proposes storing them with `parse_status = unrecognized` and listing them per library (R11); fixing them by hand ("this is *Alien* (1979)") is F005, since it means searching TMDB. Reviewer to confirm.
- [ ] **Periodic scan interval for SMB/NFS** (open question in the intent): spec proposes **15 minutes** for unwatched folders and 6 hours for watched ones, configurable per library (R19). An unchanged 10,000-file share costs one directory walk (no probes), acceptable every 15 min.
- [ ] **Unplugged drives** (R17) are the classic way media servers wipe libraries. The "folder unreachable or suddenly empty" guard and the 30-day grace period need dedicated tests, including a mount point that exists but is empty after an unmount.
- [ ] **inotify limits**: big libraries can exceed `fs.inotify.max_user_watches` on older systems. R18 falls back to periodic scans and warns; `docs/install.md` should document raising the limit. The `.deb` could ship a `sysctl.d` drop-in: decide in the plan.
- [ ] **Permissions**: the most common support issue (`plaid` user can't read `/home/…`). R2 checks readability at save time with a clear error; R3 shows readable/unreadable in the picker.
- [ ] **Parallel work with F003**: admin-only routes depend on F003's `auth.RequireAdmin` and `Principal`. Agree on that interface first (F003 plan step 1).
- [ ] **Scope against F005**: F004 stores parsed titles and hints only. Show folder names like `The Office (US)` or ambiguous movie titles are resolved by F005's matching, not by more parsing heuristics here.
- [ ] **SSE through reverse proxies**: some proxies buffer `text/event-stream`. Send `X-Accel-Buffering: no` and a heartbeat every 15 s; the polling endpoint stays as a fallback.
- [ ] **Anime absolute numbering and multi-part movies** (`Movie CD1/CD2`) aren't covered; they'll need their own items in the backlog if people ask.

## Acceptance criteria
- [ ] Given a folder the server can't read, when I create a library with it, then `422 folder_unreadable` and nothing is created.
- [ ] Given a library folder `/media/movies`, when I create another library with `/media/movies/kids`, then `409 folder_overlap`.
- [ ] Given the parser test table, then every case passes, including at least: `The Matrix (1999)/The Matrix (1999).mkv` → *The Matrix*, 1999; `The.Matrix.1999.1080p.BluRay.x264-GRP.mkv` → *The Matrix*, 1999; `Blade Runner (1982) {edition-Final Cut}.mkv` → edition *Final Cut*; `Alien (1979) {tmdb-348}.mkv` → hint tmdb 348; `2001 A Space Odyssey (1968).mkv` → title *2001 A Space Odyssey*, year 1968; `Breaking Bad/Season 01/Breaking.Bad.S01E02.720p.mkv` → S1E2; `Dark/Saison 2/Dark - 2x05.mkv` → S2E5; `Lost/Staffel 1/Lost S01E01-E02.mkv` → S1E1 and S1E2; `The Daily Show/2024-10-27.mkv` → air date 2024-10-27; `Show/Specials/Show S00E01.mkv` → season 0.
- [ ] Given `Movie (2019) - 1080p.mkv` and `Movie (2019) - 2160p.mkv` in the same folder, then one movie item with two files.
- [ ] Given a scanned library, when I rescan without changes, then no file is probed again and the scan reports 0 new, 0 changed.
- [ ] Given a scanned movie, when I move its folder elsewhere inside the library, then after the next scan it's the same item id with the new path.
- [ ] Given a library on an unmounted drive (mount point present but empty), when a scan runs, then the folder is marked `unavailable` and no file gets `missing_since`.
- [ ] Given a file deleted from disk, then its item is hidden at the next scan and removed 30 days later.
- [ ] Given a watched local folder, when a 4 GB file is copied in, then it's not probed before the copy finishes, and it appears analyzed within 30 s after the copy ends.
- [ ] Given a CIFS or NFS folder, then it isn't watched and is rescanned every 15 minutes by default.
- [ ] Given a video with English and French audio and an external `.fr.forced.srt`, then its streams list both audio languages and a forced French text subtitle.
- [ ] Given a 4K HDR10 file and a Dolby Vision file, then their `hdr_format` is `hdr10` and `dolby_vision` (with profile).
- [ ] Given `ffprobe` isn't installed, then the server starts, the library is created, files are listed with `probe_status = skipped`, and `GET /server/capabilities` reports it missing.
- [ ] Given a member session, when I `POST /libraries` or `GET /fs/browse`, then `403 forbidden`; `GET /libraries` works.
- [ ] Given a scan in progress, when I subscribe to `/scan/events`, then I receive progress events at least every 2 s until `completed`.
- [ ] Given 10,000 generated files on the throttled test profile, then the unchanged rescan finishes in under 60 s.
- [ ] Given any scan, then no file inside a library folder is created, modified or deleted (checked by comparing a tree hash before and after in tests).

## Revisions
_None yet._
