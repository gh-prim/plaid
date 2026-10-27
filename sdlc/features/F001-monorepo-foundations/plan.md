---
id: F001
title: Monorepo foundations
owner: gh-prim
reviewer: TBD
status: in-progress      # draft | approved | in-progress | done
spec: ./spec.md
branch: feat/F001-monorepo-foundations
---

# F001 — Plan: Monorepo foundations

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

**Starting point (checked 2026-10-26):** the repo contains only `CLAUDE.md`, `REVIEW.md`, `.claude/skills/` and `sdlc/`. No code, no `.gitignore`, no README. Remote: `github.com/gh-prim/plaid`. Local toolchain: Go 1.26.2, Node 26, pnpm 10, Docker 29, golangci-lint, shellcheck, jq.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `.gitignore`, `.editorconfig`, `.nvmrc` (`24`) | create | Hygiene, pinned Node LTS (R1, NFR toolchain) |
| `README.md` | create | Project intro + links to install docs and `sdlc/` |
| `Makefile` | create | `setup dev generate build test lint docker deb` (R11) |
| `api/openapi.yaml` | create | Contract: `/api/v1/health`, shared `Error` schema (R2, R4) |
| `api/events.md` | create | Stub for WebSocket events, filled by F010 (R2) |
| `server/go.mod`, `server/go.sum` | create | Module `github.com/gh-prim/plaid/server`, Go 1.26, `tool` directives for `oapi-codegen` and `air` (R3) |
| `server/internal/api/oapi-codegen.yaml` | create | Codegen config: `std-http-server`, `strict-server`, `models` |
| `server/internal/api/api.gen.go` | create (generated, committed) | Server interfaces + models (R3, R12) |
| `server/internal/api/generate.go` | create | `//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../../../api/openapi.yaml` |
| `server/internal/buildinfo/buildinfo.go` | create | `Version`, `Commit` set by `-ldflags`, start time (R4) |
| `server/internal/config/config.go` + `_test.go` | create | Env parsing, defaults, data dir creation (R6) |
| `server/internal/db/db.go` + `_test.go` | create | Open SQLite (modernc), WAL, run embedded migrations (R16) |
| `server/internal/db/migrations/0001_init.sql` | create | Empty initial migration |
| `server/internal/handlers/health.go` + `_test.go` | create | Implements `GetHealth` on the strict interface (R4) |
| `server/internal/httpserver/server.go` | create | `http.ServeMux`, mounts API under `/api/`, JSON 404 for unknown `/api/*` (R5) |
| `server/internal/httpserver/middleware.go` | create | Access log (slog), panic recovery (R7) |
| `server/internal/httpserver/spa.go` + `_test.go` | create | Static files from embed, SPA fallback, "web not built" fallback page (R5) |
| `server/internal/webui/webui.go` | create | `//go:embed all:dist` |
| `server/internal/webui/dist/.keep` | create | Keeps `go build` working without a web build |
| `server/cmd/plaid/main.go` | create | Wiring, slog setup, signal handling, graceful shutdown 10 s (R6, R7) |
| `server/cmd/plaid/main_test.go` | create | Shutdown test: in-flight request completes on SIGTERM (R7) |
| `server/.golangci.yml` | create | Lint config (v2 format) |
| `server/.air.toml` | create | Dev auto-restart on Go changes |
| `web/package.json`, `pnpm-lock.yaml` | create | React, Vite, TS, React Router, i18next, openapi-fetch; dev: Vitest, Testing Library, ESLint, Prettier, openapi-typescript, i18next-cli (R8–R10) |
| `web/vite.config.ts` | create | `/api` proxy to `:32500`, build to `web/dist` |
| `web/tsconfig*.json`, `web/eslint.config.js`, `web/.prettierrc` | create | Strict TS; ESLint with `i18next/no-literal-string` as error (R10) |
| `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx` | create | Entry, router, dark background inline (UI) |
| `web/src/api/schema.d.ts` | create (generated, committed) | Types from `openapi.yaml` (R8, R12) |
| `web/src/api/client.ts` | create | `openapi-fetch` client, base URL `/` |
| `web/src/i18n.ts` | create | i18next init, browser detection, `localStorage` persistence, updates `<html lang>` (R9) |
| `web/src/locales/{en,fr}/common.json` | create | Keys from the spec's i18n table (R9) |
| `web/src/pages/Welcome.tsx` + `Welcome.test.tsx` | create | Status page: loading/ok/error, version, language switcher (R8) |
| `web/src/i18n.test.ts` | create | Language detection, switching, persistence, `lang` attribute (R9) |
| `web/scripts/check-i18n.mjs` | create | Fails if `fr` lacks a key present in `en` (R10) |
| `web/i18next.config.ts` | create | `i18next-cli` config; `extract --ci` fails on keys used in code but missing from `en` (R10) |
| `.github/workflows/server.yml` | create | Go job, path filter `server/**`, `api/**` (R13) |
| `.github/workflows/web.yml` | create | Web job, path filter `web/**`, `api/**` (R13) |
| `.github/workflows/codegen.yml` | create | Drift check on `api/**`, `server/**`, `web/**` (R12) |
| `.github/workflows/docker.yml` | create | Multi-arch build on `main`; push to ghcr.io **disabled** until license (R18) |
| `.github/workflows/release.yml` | create | On tag `v*`: GoReleaser → binaries + `.deb` + checksums on the GitHub release (R21) |
| `.github/workflows/package-test.yml` | create | Builds the `.deb` and installs it on a real `ubuntu-24.04` runner (systemd available) (R21–R23) |
| `.github/dependabot.yml` | create | gomod, npm, github-actions (R20) |
| `Dockerfile`, `.dockerignore` | create | `node:24-slim` → `golang:1.26` → `debian:trixie-slim`, non-root `plaid` user (R14) |
| `docker-compose.yml` | create | Port 32500, `./data:/data`, `/path/to/media:/media:ro` example (R14) |
| `.goreleaser.yaml` | create | Linux amd64/arm64 builds, nFPM `.deb` (R21, R22) |
| `packaging/systemd/plaid.service` | create | Hardened unit (R23) |
| `packaging/plaid.env` | create | Default config, conffile (R22) |
| `packaging/deb/postinstall.sh`, `preremove.sh`, `postremove.sh` | create | User, dirs, enable/start, stop, purge behavior (R22) |
| `packaging/install.sh` | create | Arch detection, download latest `.deb`, SHA-256 check, `apt install` (R24) |
| `docs/install.md` | create | Docker, native `.deb`, install script, manual download, **media folder permissions** (watch-out) |
| `design/tokens.json` | create | Empty W3C Design Tokens groups (R19) |
| `.claude/settings.json`, `.claude/hooks/format.sh` | create | PostToolUse hook: gofmt / Prettier on the edited file only (R17) |
| `CLAUDE.md` | modify | Real commands + success signals (R15) |
| `sdlc/roadmap.md` | modify | Plan status |

## Steps, in order
Each step ends with a commit `F001: …` on `feat/F001-monorepo-foundations`. Tests are written in the same step as the code they cover.

1. [x] **Spike: OpenAPI 3.1 vs codegen** (R2, R3). Write `api/openapi.yaml` in 3.1 with `/api/v1/health` + `Error`. Run `oapi-codegen` and `openapi-typescript` on it. If `oapi-codegen` rejects 3.1 or generates wrong code, switch the file to **3.0.3** (supported by every generator we need, including Swift and Kotlin later) and record it in the spec's *Revisions*. Timebox: 1 h. → commit `api/`.
2. [x] **Repo skeleton** (R1). `.gitignore`, `.editorconfig`, `.nvmrc`, `README.md`, `design/tokens.json` (R19), empty `Makefile` with target names. → commit.
3. [x] **Go module + codegen** (R3, R12). `go mod init`, `tool` directives, `oapi-codegen.yaml`, `generate.go`, generate `api.gen.go`. `make generate` runs Go and TS codegen (TS part wired in step 8). → commit.
4. [x] **Config + buildinfo** (R6, R4). Write `config_test.go` first (defaults, env overrides, invalid port, data dir creation in a temp dir), then `config.go`. `buildinfo.go`. → commit.
5. [x] **Database bootstrap** (R16). `db_test.go` first: opens in a temp dir, `PRAGMA journal_mode` returns `wal`, migrations are idempotent across two opens. Then `db.go` with an embedded `migrations/` runner (a few lines with `embed.FS` + a `schema_migrations` table; no extra dependency). → commit.
6. [x] **Health handler** (R4). `health_test.go` first: `200`, body fields present, `database: "ok"`, and `"degraded"`/`503` when the DB ping fails (via a small interface). Then `health.go`. → commit.
7. [x] **HTTP server, middleware, SPA, main** (R5, R7). Tests first:
   - `spa_test.go`: `/` and `/library/123` → `index.html`; `/assets/x.js` → file with correct content type; `/api/v1/nope` → JSON 404 `not_found`; empty `dist` → "web not built" page with `200`.
   - `main_test.go`: start the server on a random port, fire a slow request, send SIGTERM (or cancel the context), assert the request completes and run returns nil.
   Then `server.go`, `middleware.go`, `spa.go`, `webui.go`, `main.go`. `-ldflags` injection wired in `make build`. → commit.
8. [x] **Web skeleton + TS client** (R8). `pnpm create vite` (React + TS) then trim. Add `openapi-typescript` → `src/api/schema.d.ts`, `openapi-fetch` client. `Welcome.test.tsx` first (mocked fetch: loading, ok with version, error). Then `Welcome.tsx`, `App.tsx`, Vite proxy. → commit.
9. [x] **i18n** (R9, R10). `i18n.test.ts` first. Then `i18n.ts`, catalogs with the spec's keys, language switcher, `<html lang>` sync. ESLint `no-literal-string`, `i18next-cli` config, `check-i18n.mjs`. Prove the guards: temporarily add a literal string and a missing `fr` key, confirm both fail, revert. → commit.
10. [x] **Makefile complete** (R11). `setup` (pnpm install, `go mod download`), `dev` (`make -j2 dev-server dev-web`: `go tool air` + `vite`), `build` (web build → copy `web/dist/*` into `server/internal/webui/dist/` → `go build -ldflags …`), `test` (go test -race, golangci-lint, pnpm typecheck/lint/test, i18n checks, drift check), `lint`, `docker`, `deb` (`goreleaser release --snapshot --clean`). → commit.
11. [x] **Claude Code hook** (R17). `.claude/hooks/format.sh` reads the hook JSON on stdin (`jq -r .tool_input.file_path`), runs `gofmt -w` on `*.go` or `pnpm exec prettier --write` on `web/**`, does nothing otherwise, always exits 0 quickly. Register in `.claude/settings.json` (PostToolUse, matcher `Edit|Write`). → commit.
12. [x] **CI workflows** (R12, R13, R20). `server.yml`, `web.yml`, `codegen.yml` with path filters and caching (Go build cache, pnpm store). `dependabot.yml`. Push the branch, confirm each job runs only on its paths with a dummy change. → commit.
13. [x] **Docker** (R14, R18). `Dockerfile` multi-stage, `.dockerignore`, `docker-compose.yml`. `docker.yml` builds `linux/amd64,linux/arm64` with buildx on `main`, `push: false` until the license is decided. Verify locally: `docker compose up`, open `:32500`, `docker compose exec plaid id -u` ≠ 0. → commit.
14. [x] **Native packaging** (R21–R23). `.goreleaser.yaml` (before hook: web build; builds linux amd64/arm64 with ldflags; nFPM `.deb` with contents, `config|noreplace` for `plaid.env`, scripts). `plaid.service`, `plaid.env`, `postinstall.sh` (create system user + `/var/lib/plaid`, `systemctl daemon-reload`, enable + start or restart on upgrade), `preremove.sh` (stop + disable on remove, not on upgrade), `postremove.sh` (on purge: remove `/etc/plaid`, keep `/var/lib/plaid`). `package-test.yml` on `ubuntu-24.04`: build snapshot `.deb`, `sudo apt install`, `systemctl is-active plaid`, `curl :32500/api/v1/health`, `ps -o user= -C plaid` = `plaid`, reinstall over itself with an edited `plaid.env` and check the edit survives, `apt remove` and check `/var/lib/plaid` remains. → commit.
15. [x] **Install script** (R24). `install.sh`: `set -euo pipefail`, require root, map `uname -m` (`x86_64`→`amd64`, `aarch64`→`arm64`), fetch latest release metadata from the GitHub API, download `.deb` + `checksums.txt`, `sha256sum -c`, `apt install`. `PLAID_DEB_URL` override for testing. Under 80 lines, shellcheck-clean. Add a `package-test.yml` step running it against the snapshot via the override. → commit.
16. [x] **Docs + CLAUDE.md** (R15, watch-outs). `docs/install.md` (three install paths, upgrade/remove/purge behavior, **media permissions**: `sudo usermod -aG <media-group> plaid` or `setfacl -R -m u:plaid:rx /path/to/media`, then `systemctl restart plaid`). Fill `CLAUDE.md` *Commands* with each `make` target and its success signal. → commit.
17. [x] **Release dry run** (R21). `make deb` locally produces both `.deb` files + checksums. Tag-based release is **not** pushed (repo visibility / license pending); the workflow is exercised with `--snapshot` only. Update roadmap, open the PR.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `oapi-codegen` doesn't support OpenAPI 3.1 well | High | Step 1 spike, fallback to 3.0.3, recorded in spec *Revisions* |
| `go:embed` + generated web assets make the tree dirty or break `go build` on a fresh clone | Medium | Embed `all:dist` with a committed `.keep`; `dist/*` gitignored; runtime fallback page when `index.html` is missing |
| Drift check flaky (generator output differs between machines) | Medium | Pin generator versions (Go `tool` directive, lockfile); generate in CI with the same `make generate` |
| `.deb` maintainer scripts misbehave on upgrade (service stopped and not restarted, conffile overwritten) | Medium | Upgrade scenario explicitly tested on a real Ubuntu runner (step 14) |
| systemd hardening blocks something legit (writing outside `/var/lib/plaid`, reading media) | Medium | Health + DB write tested under the real unit; media access documented; F006 will extend `ReadWritePaths` for transcode temp |
| `ubuntu-24.04-arm` runners unavailable for this repo | Medium | arm64 `.deb` is built and checksummed in CI; its install is verified manually on a Pi / arm VM before closing F001 |
| `i18next-cli` behavior differs from expectations (it's newer than `i18next-parser`) | Low | Fall back to `i18next-parser --fail-on-update`; the en/fr parity script is ours either way |
| CI path filters skip a job that should run (e.g. Makefile change) | Low | Root files (`Makefile`, workflows) trigger all jobs |

**Riskiest step:** step 14 (native packaging). Maintainer scripts run as root on users' machines, and a bad upgrade path can lose config or leave PLAID down after an update. It's the only step whose mistakes land outside our repo, which is why it gets a dedicated test workflow on a real systemd host.

## Rejected options
- **Chi / Echo / Gin router**: Go 1.22+ `net/http` routing covers our needs, and `oapi-codegen` has a `std-http-server` target. One dependency less.
- **`golang-migrate` / `goose` for migrations**: overkill for embedded SQL files run in order; a ~40-line runner is easier to audit. Revisit if migrations need down-steps.
- **Distroless runtime image**: settled in the spec (F006 needs FFmpeg and GPU drivers).
- **Snap or Flatpak instead of `.deb`**: confinement fights with reading arbitrary media folders and with GPU access; `.deb` + systemd is what Plex and Jellyfin users expect on Ubuntu.
- **Hand-written `dpkg-deb` packaging**: GoReleaser + nFPM produces amd64/arm64 packages, checksums and the GitHub release in one config, and can add RPM later for free.
- **`msw` for web tests**: a mocked `fetch` is enough for one endpoint; add `msw` when F007/F008 need richer API mocking.
- **Root `package.json` / pnpm workspace**: one JS package for now; a workspace can come when a second one appears (e.g. a shared design-tokens package from F002).

## Expected evidence
To paste into the PR:
- [x] `make test` output, green (local; fresh-clone run in CI pending push).
- [x] CI runs: server, web, codegen, docker, package-test all green on PR #1. *Pending:* a web-only PR proving the server job is skipped.
- [x] Screenshots of the welcome page in **en** and **fr** (ok state) from the embedded build at `:32500`. Error state covered by unit tests, no screenshot.
- [x] Proof of the guards failing: output of `make lint` with a literal string, `make test` with a missing `fr` key, CI drift check with an unregenerated spec (then reverted).
- [x] `docker compose up` + `docker compose exec plaid id -u` output (uid 1000).
- [x] `package-test.yml` log showing install, active service, health 200, user `plaid`, upgrade keeps `plaid.env`, remove keeps `/var/lib/plaid`.
- [x] arm64 check: `make test-deb` on Ubuntu 24.04/26.04 and Debian 12/13 (install script + health included). Replaces the Pi run, see Log.
- [x] Reboot-like check: container restart with systemd as PID 1, PLAID comes back up (all distros above).
- [x] Idle RSS and cold start measured on arm64 (Apple Silicon container: 13 MB, 29 ms; throttled to 0.25 CPU / 1 GB: 13 MB, ~200 ms) (NFR: < 40 MB, < 1 s).

## Log
Deviations from the plan found during the build.

- **2026-10-26 — OpenAPI 3.1 kept.** Step 1 spike passed: `oapi-codegen` v2.8.1 generates and builds from 3.1. No spec revision needed.
- **2026-10-26 — TypeScript 6, not 7.** TypeScript 7 (native) no longer exposes the JS compiler API that `openapi-typescript` and `typescript-eslint` need. Pinned `typescript@^6` until they support it.
- **2026-10-26 — No `web/scripts/check-i18n.mjs`.** `i18next-cli extract --ci --dry-run` (keys used in code but missing from `en`, unused keys) plus `i18next-cli status` (keys missing in `fr`) cover both checks. Proven on all three failure cases.
- **2026-10-26 — i18n key names.** Spec listed `common.language` and `common.language.en`, which can't coexist (a key can't be both a string and a group). Used `language.label`, `language.en`, `language.fr` in the `common` namespace. Recorded in the spec's *Revisions*.
- **2026-10-26 — Codegen drift check via temp dir.** `git diff --exit-code` would fail locally on uncommitted regenerated files; `scripts/check-codegen.sh` regenerates into a temp dir and diffs instead. Same guarantee, works locally and in CI.
- **2026-10-26 — systemd unit path `/usr/lib/systemd/system/`** instead of `/lib/systemd/system/` (spec R22). Same location on merged-usr systems (all supported targets); avoids dpkg aliasing issues.
- **2026-10-26 — GPU groups set in postinstall, not `SupplementaryGroups=`** (spec R23). A missing group makes systemd refuse to start the unit; postinstall adds `plaid` to `video`/`render` only where they exist.
- **2026-10-26 — Extra systemd hardening.** Beyond R23's list: empty capability set, syscall filter, `ProtectProc`, `RestrictNamespaces`, etc. `systemd-analyze security` went from 7.5 (EXPOSED) to 1.7 (OK). Devices stay visible for F006 (`/dev/dri`). F006 may need to relax `SystemCallFilter=~@resources` if FFmpeg sets priorities.
- **2026-10-26 — `make deb` uses `go run goreleaser@v2.19.0`**: no global install needed, version pinned.
- **2026-10-26 — Docker Compose uses a named volume** (`plaid-data`) instead of `./data`: a bind mount created by Docker is owned by root and the non-root container user couldn't write to it.
- **2026-10-26 — Local verification of the `.deb`** done in an Ubuntu 24.04 arm64 container with systemd (Apple Silicon host): install, service active as `plaid`, upgrade keeps edited config, install script, tampered package refused, restart brings the service back, remove keeps data + config, purge removes config only. Cold start 29 ms, idle RSS 13 MB, binary 11 MB.
- **2026-10-26 — Real Raspberry Pi test replaced by a container matrix.** A Pi would only add slower-hardware numbers, a real reboot and Raspberry Pi OS (Debian-based). Added `scripts/test-deb.sh` / `make test-deb`: 24 checks in a container with real systemd as PID 1. Results:
  - arm64 (native on Apple Silicon): Ubuntu 24.04, Ubuntu 26.04, Debian 12, Debian 13 — all pass.
  - amd64: Debian 12 passes under emulation; Ubuntu 24.04/26.04 and Debian 13 can't be tested locally because systemd ≥ 255 (`systemd-executor`) doesn't run under x86 emulation on ARM (even journald fails). Covered instead by the `package-test` CI job on a real x86 Ubuntu 24.04 runner — green on PR #1.
  - Throttled to 0.25 CPU + 1 GB RAM (≈ one Pi 4 core): cold start ~200 ms, idle RSS 13 MB, all checks pass.
  - Not covered: SD-card I/O and Raspberry Pi OS specifically (Debian 12/13 are its base).
- **2026-10-27 — Automatic releases** (asked after the plan was approved). `release.yml` now runs on every push to `main`: a `tag` job computes the next version with `scripts/next-version.sh` (patch by default, `#minor` / `#major` in the commit message, first release `v0.1.0`) and pushes the tag; a `build` job checks out that tag and runs GoReleaser, publishing a non-draft release. The build only runs on tags. Tags pushed with `GITHUB_TOKEN` don't trigger workflows, so the build is chained in the same workflow. `[skip release]` opts out.
