---
id: F004
title: Libraries and file scanning
owner: gh-prim
reviewer: TBD
status: draft            # draft | approved | in-progress | done
spec: ./spec.md
branch: feat/F004-library-scanning
---

# F004 — Plan: Libraries and file scanning

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

**Starting point (checked 2026-10-27):** same server as F003's plan describes (strict oapi-codegen handlers, `withRecover(withAccessLog(mux))`, SQLite with embedded migrations). `github.com/fsnotify/fsnotify` v1.9.0 and `golang.org/x/sys` are already in `go.sum` (indirect, via `air`). Neither the Docker image (`debian:trixie-slim` + `ca-certificates tzdata`) nor the `.deb` (`dependencies: [adduser, ca-certificates]`) ships `ffprobe` yet; ADR 0006 (option C) says both must depend on the distro `ffmpeg`. CI's `server.yml` doesn't install FFmpeg. The systemd unit is hardened (`ProtectSystem=strict`, `ProtectHome=read-only`, `MemoryDenyWriteExecute`, `SystemCallFilter=~@privileged @resources`), which matters for running `ffprobe` as a child process.

**Depends on F003 PR 1** (typed `apierr`, `auth.Principal`, `auth.RequireAdmin`, deny-by-default middleware, `handlers.Deps`, `statusRecorder.Unwrap`). Steps 1–7 below touch only `internal/library` and can start before it is merged; step 8 onward needs it on `main`.

**Delivery:** three PRs:
- **PR A — engine** (steps 1–7): parsing, walking, probing, reconciling. No API, no wiring. Mostly pure Go packages.
- **PR B — API and live updates** (steps 8–12).
- **PR C — packaging and performance** (steps 13–14).

## Files touched
| File | Action | Why |
|------|--------|-----|
| `server/go.mod`, `go.sum` | modify | `fsnotify`, `golang.org/x/sys` become direct; add `github.com/sabhiram/go-gitignore` (R6) |
| `server/internal/db/migrations/0003_libraries.sql` | create | Tables from the spec's data model |
| `server/internal/library/store.go` + `_test.go` | create | SQL for libraries, folders, files, items, links, streams; batched writes ≤ 500 rows per transaction (R14, R24) |
| `server/internal/library/extensions.go` | create | Video and subtitle extensions (R4, R13) |
| `server/internal/library/ignore.go` + `_test.go` | create | Built-in ignore rules + `.plaidignore` stack (R5, R6) |
| `server/internal/library/walker.go` + `_test.go` | create | `WalkDir` with ignore rules, no symlink escape, stops a folder on I/O errors (R2, R17, NFR safety) |
| `server/internal/library/parse_movie.go` + `_test.go` | create | `ParseMovie(relPath)` (R7, R8) |
| `server/internal/library/parse_episode.go` + `_test.go` | create | `ParseEpisode(relPath)` (R9, R10) |
| `server/internal/library/parse_common.go` | create | Release-tag stripping, separators, localized season words, language names |
| `server/internal/library/testdata/names_movies.txt`, `names_episodes.txt` | create | Real-world name table (input → expected), read by the parser tests |
| `server/internal/library/subtitles.go` + `_test.go` | create | External subtitle matching, language/forced/SDH flags (R13) |
| `server/internal/library/probe.go` + `_test.go` | create | `ffprobe` discovery, run with timeout, JSON → `ProbeResult`, HDR detection (R12, R25) |
| `server/internal/library/testdata/probe/*.json` | create | Recorded `ffprobe` outputs: SDR h264 2.0, HEVC HDR10 multi-audio, Dolby Vision P8.1, HDR10+, HLG, PGS subtitles, broken file |
| `server/internal/library/fingerprint.go` + `_test.go` | create | SHA-256 of size + first and last MiB (R16) |
| `server/internal/library/reconcile.go` + `_test.go` | create | Walk vs DB: new, changed, unchanged, moved, missing, unavailable; item creation and linking; hiding and 30-day purge (R8, R10, R11, R14–R17) |
| `server/internal/library/fstype_linux.go`, `fstype_other.go` | create | `statfs` magic → `nfs`, `cifs`, `smb2`, `fuse`, `9p`, … ; inotify limit from `/proc/sys/fs/inotify/max_user_watches` (R18) |
| `server/internal/library/watcher.go` + `_test.go` | create | Recursive fsnotify, 10 s size-stable debounce, watch-limit fallback (R18) |
| `server/internal/library/scheduler.go` + `_test.go` | create | Per-library queue (one running + one queued), periodic timers, probe pool `min(4, NumCPU)`, progress state and pub/sub (R19–R21, R24) |
| `server/internal/library/engine.go` | create | Owns store, scheduler, watchers; `Start(ctx)` loads libraries in the background; `Close()` |
| `server/internal/library/browse.go` + `_test.go` | create | `/fs/browse` listing and folder validation helpers (R2, R3) |
| `server/internal/library/scan_e2e_test.go` | create | Acceptance criteria on a temp tree with a fake prober |
| `server/internal/library/readonly_test.go` | create | Tree hash before/after a full scan (R23, acceptance) |
| `server/internal/library/perf_test.go` | create | 10,000 generated files, `//go:build perf` (NFR performance) |
| `api/openapi.yaml` | modify | Tag `libraries`: every route of the spec's API table, schemas, new error codes |
| `server/internal/api/api.gen.go`, `web/src/api/schema.d.ts` | regenerate | From `openapi.yaml` |
| `server/internal/handlers/handlers.go` | modify | `Deps.Library *library.Engine` |
| `server/internal/handlers/libraries.go`, `items.go`, `fs.go`, `capabilities.go` + `_test.go` each | create | Strict handlers; `auth.RequireAdmin` where the spec says admin (R1) |
| `server/internal/handlers/scan_events.go` + `_test.go` | create | SSE response type implementing the generated `Visit…Response`: flush, 15 s heartbeat, `X-Accel-Buffering: no` (R21, watch-out SSE) |
| `server/internal/apierr/apierr.go` | modify | `folder_not_found`, `folder_unreadable`, `folder_overlap`, `library_type_immutable`, `ffprobe_missing`, `not_found` |
| `server/internal/config/config.go` + `_test.go` | modify | `PLAID_FFPROBE_PATH`, `PLAID_BROWSE_ROOTS` (colon-separated absolute paths) |
| `server/cmd/plaid/main.go` | modify | Build and start the engine; cancel SSE streams on shutdown (`srv.RegisterOnShutdown`) |
| `Dockerfile` | modify | `apt-get install ffmpeg` in the runtime stage (ADR 0006) |
| `.goreleaser.yaml` | modify | `.deb` `dependencies: [adduser, ca-certificates, ffmpeg]` |
| `scripts/test-deb.sh` | modify | Checks: `ffprobe` installed; a library on a generated sample is scanned and probed **by the hardened service** |
| `.github/workflows/server.yml` | modify | `apt-get install ffmpeg` so the real-ffprobe test runs in CI |
| `packaging/plaid.env` | modify | Commented `PLAID_FFPROBE_PATH`, `PLAID_BROWSE_ROOTS` |
| `docs/install.md` | modify | Folder permissions per install method, Docker `:ro` mounts, raising `fs.inotify.max_user_watches`, NAS shares and periodic scans |
| `CLAUDE.md` | modify | `make test-deb` check count; `ffmpeg` in the toolchain line; how to run the perf test |
| `sdlc/roadmap.md` | modify | Plan status |

## Decisions taken in this plan
- **inotify limit (spec watch-out):** the `.deb` does **not** ship a `sysctl.d` drop-in. Changing a kernel-wide setting from a media-server package is surprising, it can't apply in Docker anyway, and kernels ≥ 5.11 already scale `max_user_watches` with RAM (≈ 8k on 512 MB up to 1M). The fallback (R18) plus the `docs/install.md` section and the capability endpoint cover it.
- **Probe priority:** the hardened unit forbids `setpriority` (`~@resources`), so "nice-level priority" is best effort: after `cmd.Start`, `unix.Setpriority(PRIO_PROCESS, pid, 10)`; `EPERM` is logged once at debug and ignored. The real protection is the worker cap (`min(4, NumCPU)`). We don't loosen the unit for this.
- **HDR10+ detection:** HDR10+ lives in per-frame side data, not stream metadata. Only when the stream's `color_transfer` is `smpte2084`, a second, cheap call reads one frame (`-select_streams v:0 -read_intervals %+#1 -show_frames`) to look for `SMPTE2094-40`. Dolby Vision from the stream's `DOVI configuration record` (profile + compatibility id), HDR10 from `smpte2084` + BT.2020, HLG from `arib-std-b67`.
- **"Unavailable" heuristic (R17):** a folder is `unavailable` when its root is missing or unreadable, when it's empty while the DB has non-missing files for it (the classic empty mount point), or when the walk hits an I/O error (`EIO`, `ENOTCONN`, `ESTALE`). In all these cases no file of that folder is touched.
- **Library type after creation:** the PATCH schema accepts an optional `type` only so that a different value returns `422 library_type_immutable` (the same value is a no-op). `encoding/json` silently ignores unknown fields, so leaving `type` out of the schema would make the error code unreachable.

## Steps, in order
Each step ends with a commit `F004: …`. Tests come with the code.

### PR A — engine (no dependency on F003)
1. [ ] **Migration `0003_libraries.sql` + store** (R14, R24). Tables exactly as the spec. `Store` methods used by the reconciler, all writes through a `Batch` that commits every 500 rows. Tests: migration on a fresh DB and after `0002`; `ON DELETE CASCADE` from a library removes folders, files, items, links, streams; `UNIQUE(path)` on folders and files.
2. [ ] **Movie parser** (R7, R8). Pure function returning `{Title, Year, Edition, Hints, Version}`. Order: braces tags (`{edition-…}`, `{tmdb-…}`, `{imdb-…}`) → year in `()`/`[]`/dotted → cut at the first release tag (`2160p 1080p 720p 480p BluRay WEB-DL WEBRip HDTV REMUX x264 x265 HEVC H.264 AAC DTS …`) → separators `.`/`_` to spaces → edition words. Year rule: the **last** plausible year (1900–current+1) is the year, so `2001 A Space Odyssey (1968)` keeps `2001` in the title. Falls back to the parent folder name when the file name yields no title. Tests: every acceptance case + ≥ 40 real names in `testdata/names_movies.txt`.
3. [ ] **Episode parser** (R9, R10). Show = first path segment under the library folder; season folder regex with the localized words from the spec (case-insensitive, `Specials` = 0); episode patterns `S01E02`, `s1e2`, `1x02`, `S01E01-E03`, `S01E01E02`, dates `YYYY-MM-DD` / `YYYY.MM.DD`; file-name season wins over folder. Multi-episode returns a list. Tests: every acceptance case + ≥ 40 names in `testdata/names_episodes.txt`, including `Season 1` vs `S01` folder, a file directly in the show folder, `Сезон 2`.
4. [ ] **Ignore rules + walker** (R4, R5, R6, R2). Built-in rules; `sample` word + size < 300 MB; `.plaidignore` parsed per directory and stacked for its subtree. Walker uses `WalkDir`, never follows symlinked directories, follows a symlinked file only if its resolved path stays inside the library folder. Tests on a temp tree: each ignored name, a `.plaidignore` with `*.mkv` + `!keep.mkv`, a symlink loop, a symlink to `/etc/passwd`.
5. [ ] **External subtitles** (R13). Siblings sharing the video's base name; suffix tokens parsed right to left: `forced`, `sdh`/`cc`/`hi`, language as ISO 639-1, 639-2/T, 639-2/B or English name (`french`) → stored as 639-2/B; `.sub` only counts with a matching `.idx`. Stored as streams with `idx ≥ 1000`. Tests: `.fr.forced.srt`, `.en.sdh.srt`, `.eng.ass`, `Movie.srt` (`und`), orphan `.sub`.
6. [ ] **Prober** (R12, R25). Discovery: `PLAID_FFPROBE_PATH`, else `exec.LookPath`; version from `ffprobe -version`. Run `ffprobe -v error -print_format json -show_format -show_streams` with a 30 s context; kill on timeout. Mapping covers every field of R12; raw JSON kept. HDR as in "Decisions". Tests: each `testdata/probe/*.json` fixture → expected `ProbeResult` (golden); timeout with a fake `ffprobe` script that sleeps; missing binary → `ErrFFprobeMissing`; plus `TestProbeRealFile` that generates a 1 s file with `ffmpeg -f lavfi` and probes it, skipped when `ffmpeg` isn't on `PATH`.
7. [ ] **Reconciler + fingerprint** (R8, R10, R11, R14–R17). For one folder: walk → compare by path (size + mtime unchanged → skip, no probe) → new files fingerprinted → a new file matching a file that went missing **in this scan** in the same library takes over its row (path updated, items kept) → remaining unseen files get `missing_since` (kept if already set) → items with no non-missing file get `hidden = 1` → files missing > 30 days deleted, then orphan items. Items: movies grouped by `(library, normalized title, year)`, each file keeps its `edition`; shows/seasons created on demand; a multi-episode file linked to each episode. Unavailable guard runs **before** any write. Tests (fake prober, fake clock): rescan with no change → 0 new, 0 changed, 0 probes; folder moved inside the library → same item id; mount point present but empty → folder `unavailable`, no `missing_since`; deleted file → hidden, then removed after 30 days; file returning on day 29 → visible again, same item; two versions → one movie, two files; unrecognized names stored with `parse_status = unrecognized`; a crash simulated between batches leaves a state the next scan completes. → `make test`, **open PR A.**

### PR B — API and live updates (needs F003 PR 1 on `main`)
8. [ ] **Scheduler + engine** (R19, R20, R21, R24, NFR startup). One goroutine per library for walking; probes go through a shared pool of `min(4, NumCPU)` workers; per-library state machine `idle → queued → scanning → analyzing → idle`; a request during a scan sets `queued` once (more requests are no-ops → `202`). Timers: 15 min unwatched / 6 h watched, or `scan_interval_s` (300–86,400). `Engine.Start` returns immediately and loads libraries in a goroutine; startup never runs a full scan (R15). Progress counters and pub/sub (buffered channel per subscriber, slow subscribers drop to the latest snapshot). Tests with a fake clock: queue-once, interval defaults, `Close()` stops everything (`goleak`-style check via `runtime.NumGoroutine` before/after).
9. [ ] **Watcher** (R18). Filesystem type from `statfs` at folder save and at startup; `nfs`, `cifs`, `smb2`, `smbfs`, `fuse*`, `9p` aren't watched. Recursive: add a watch per directory at start and on `Create` of a directory. Debounce: on any event for a path, poll its size every 2 s; enqueue once unchanged for 10 s. `ENOSPC` from `Add` → close the folder's watcher, mark it unwatched, log a warning, expose it in capabilities. Tests: a file written in chunks over 15 s isn't processed before it stops growing (fake clock + real fsnotify on a temp dir); `ENOSPC` path via an injected `Add` func; fs type mapping table.
10. [ ] **Libraries API + folder validation + browse** (R1, R2, R3, R23, R25). OpenAPI first, `make generate`, then handlers. Validation: absolute, `filepath.Clean`, `EvalSymlinks`, `Stat` is dir, `os.Open` + `ReadDir(1)` for readable/traversable; overlap check against every other library's folders (prefix with a trailing separator, both directions) → `409 folder_overlap`; errors `422 folder_not_found` / `folder_unreadable`, nothing created. `POST` → `201` and a full scan queued; `PATCH` adding a folder scans it; `DELETE` → `204`, rows only. `/fs/browse`: rejects relative paths and any `..` segment, lists directories only (no files), `readable` per entry, restricted to `PLAID_BROWSE_ROOTS` when set (`parent` stops at a root). `/server/capabilities`. Tests: every acceptance criterion on validation and roles (member → `403` on `POST /libraries` and `/fs/browse`, `GET /libraries` works), a library on a `chmod 000` folder.
11. [ ] **Items and unrecognized API** (R11, R22). `GET /libraries/{id}/items?kind=&parentId=&cursor=&limit=` ordered by `(sort_title, id)`, cursor = opaque base64 of the last pair, `limit` ≤ 200; hidden items excluded. Technical summary: max resolution, HDR format, distinct audio languages. `GET /items/{id}` with files and streams. `GET /libraries/{id}/unrecognized` paginated the same way. Tests: pagination over 450 items has no duplicates or gaps; show → seasons → episodes navigation.
12. [ ] **Scan progress + SSE** (R21, watch-out SSE). `GET /libraries/{id}/scan` returns the snapshot. `/scan/events`: OpenAPI declares `text/event-stream`; the handler returns a custom type implementing the generated response interface that writes `event: progress|completed|failed`, `data: <json>`, flushes via `http.NewResponseController` (works thanks to F003's `Unwrap`), sends a progress event at least every 2 s while scanning and `: ping` every 15 s, sets `Cache-Control: no-cache` and `X-Accel-Buffering: no`, and ends on client disconnect or server shutdown (`RegisterOnShutdown` cancels a base context so `Shutdown` doesn't wait 10 s). Tests: `httptest.Server` client reads ≥ 2 progress events then `completed`; shutdown with an open stream returns within 1 s. → `make test`, **open PR B.**

### PR C — packaging and performance (the `test-deb` check needs F003 PR 2 for `/setup/admin`)
13. [ ] **FFmpeg distribution** (R25, ADR 0006). Dockerfile runtime stage installs `ffmpeg` (`--no-install-recommends`); `.deb` depends on `ffmpeg`; `server.yml` installs it; `test-deb.sh` adds: "ffprobe installed", "sample library scanned by the service" (generate a 2 s file with `ffmpeg` into `/srv/media`, `chmod o+rx`, create admin via `/setup/admin` from localhost, create the library, poll `/scan` until `idle`, check `probe_status = ok` via `/items`). This last check proves `ffprobe` works under `MemoryDenyWriteExecute` and the syscall filter. `docs/install.md` sections; `CLAUDE.md` check count. Record image size before/after in the PR.
14. [ ] **Performance and read-only proofs** (NFR, R23). `readonly_test.go`: SHA-256 over (path, size, mtime, mode, content of small files) of the tree before/after a full scan + rescan + move + delete. `perf_test.go` (`go test -tags perf ./internal/library/`): 10,000 sparse files in a realistic layout, fake prober; asserts walk+parse < 2 min and unchanged rescan < 60 s, reports peak heap. Run it in the throttled container (`PLAID_TEST_CPUS=0.5 PLAID_TEST_MEMORY=1g`) and paste the numbers. → `make test`, **open PR C.**

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| A drive unmounts mid-scan and files get `missing_since`, then purged | medium | Unavailable guard before any write, I/O errors abort the folder, 30-day grace, dedicated tests (step 7) |
| Move detection relinks the wrong file (two identical-size files) | low | Fingerprint includes first/last MiB; only matches files that went missing **in the same scan and library**; ambiguous matches (2+ candidates) fall back to "new + missing" |
| Parser regressions as names get added | high | Table files in `testdata/`; every bug becomes a line |
| `ffprobe` blocked by the hardened systemd unit | medium | Step 13 test runs a probe through the real service; if it fails, adjust the unit in the same PR and note it in the Log |
| `ffmpeg` dependency bloats the image/`.deb` install | certain | Accepted by ADR 0006; size recorded in PR C |
| inotify exhaustion on big libraries | medium | Fallback to periodic scan + warning + docs |
| fsnotify on macOS (kqueue, one fd per file) exhausts fds in `make dev` | medium | Dev-only, same code path as Linux; if it shows up, document `ulimit -n` in `CLAUDE.md` and note it in the Log |
| SSE buffered by proxies | medium | Headers + heartbeat + polling endpoint |
| Conflicts with F003 on shared files | high | Start after F003 PR 1; own OpenAPI tag; regenerate instead of hand-merging generated files |
| SQLite write contention between the scanner and auth (`last_seen_at`) | low | WAL + `busy_timeout(5000)` from F001; batches ≤ 500 rows keep transactions short |

**Riskiest step:** step 7, the reconciler. It's the only code that deletes rows, and a wrong decision there loses watch history (F006) for real users. It gets the most tests, a fake clock for the 30-day rule, and a dedicated review pass on the unavailable guard.

## Rejected options
- **Watching NFS/SMB with polling watchers** (stat every file every N s): heavy on a NAS for little gain over the 15-min rescan + "Scan now".
- **Probing during the walk:** a slow `ffprobe` would hold the walker; the spec wants files listed fast and analyzed in the background.
- **Content hash of whole files for move detection:** reads terabytes; size + first/last MiB is enough to tell real files apart.
- **Recursive inotify via fanotify:** needs `CAP_SYS_ADMIN`, incompatible with the hardened unit.
- **Writing our own gitignore matcher:** negations, `**` and anchoring are easy to get wrong; a small, tested library is safer.
- **Running the SSE stream outside the generated router:** it would skip the OpenAPI contract and F003's auth middleware ordering; a custom response type keeps both.
- **A `sysctl.d` drop-in in the `.deb`**: see "Decisions taken in this plan".

## Expected evidence
- [ ] Tests added: parser tables, ignore/walker, subtitles, probe fixtures (+ real ffprobe test in CI), reconciler scenarios, scheduler, watcher, handlers, SSE, read-only tree hash
- [ ] `make test` green, output pasted in each PR
- [ ] `make test-deb` green with the two new checks; image size before/after
- [ ] Perf numbers on the throttled profile (walk+parse, unchanged rescan, peak memory)
- [ ] Manual check: `make dev`, create a Movies and a Shows library with `curl` on a real folder, copy a file in, watch it appear through `/scan/events`
- [ ] Manual check: unplug (or unmount) a USB drive during a library's life → folder `unavailable`, items still there after re-plug
- [ ] No screenshots: F004 has no UI (F007)

## Log
_Nothing yet._
