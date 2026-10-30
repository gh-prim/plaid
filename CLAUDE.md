# PLAID

A self-hosted alternative to Plex, with a "plaid & chill" vibe. Full vision: [sdlc/vision.md](sdlc/vision.md).

## Process (mandatory)
We follow the SDLC described in [sdlc/README.md](sdlc/README.md): `intent.md` → `spec.md` → `plan.md` → code.
- **Never write application code without a `plan.md` in `approved` status** for the feature in question. If asked for code without a plan, first suggest going through the `new-intent`, `intent-to-spec` then `spec-to-plan` skills.
- If the implementation drifts from the plan, update `plan.md` in the same commit.
- Commits and branches cite the feature ID (`F006: …`, `feat/F006-…`).
- We are two devs: a PR's author never approves it themselves.
- **One PR per feature.** All of a feature's code ships in a single PR from a single `feat/FNNN-…` branch. Never split a feature into several PRs, stacked or not (no "PR A/B/C"); split the work into commits instead. The intent, spec and plan keep their own doc PRs.

## Repo
| Folder | Contents |
|--------|----------|
| `sdlc/` | vision, roadmap, ADRs, features (intent/spec/plan), templates |
| `api/` | `openapi.yaml`, the API source of truth, from which the clients are generated |
| `server/` | Go (coming) |
| `web/` | React + Vite (coming) |
| `apple/` | SwiftUI iOS/iPadOS/tvOS (coming) |
| `android/` | Kotlin + Compose (coming) |

## Commands
Run from the repo root. Toolchain: Go 1.26, Node 24+, pnpm 10, golangci-lint v2, shellcheck, Docker (for `make docker`), FFmpeg (`ffprobe`; tests that need it skip without it).

| Command | What it does | Success looks like |
|---------|--------------|--------------------|
| `make setup` | Installs web deps and Go modules | exits 0 |
| `make dev` | Go server on :32500 (auto-restart via air) + Vite on :5173 (proxies `/api`) | open http://localhost:5173 |
| `make generate` | Regenerates `server/internal/api/api.gen.go` and `web/src/api/schema.d.ts` from `api/openapi.yaml` | no output; commit the generated files |
| `make test` | lint + Go tests (`-race`) + web tests + i18n check + codegen drift check | last line `✅ all checks passed` |
| `make lint` | golangci-lint, tsc, ESLint, Prettier check, shellcheck | exits 0 |
| `make build` | Web build embedded into `server/bin/plaid` | binary in `server/bin/` |
| `make docker` | Builds image `plaid:dev` | then `docker compose up -d`, health at :32500 |
| `make deb` | Snapshot binaries + `.deb` (amd64, arm64) + checksums in `dist/` | `release succeeded` |
| `make test-deb` | Installs the `.deb` in a container with real systemd and runs 27 checks (install, upgrade, script, a library scanned with ffprobe and played (remux, software transcode) by the hardened service, no ffmpeg left after stop, restart, remove, purge). `IMAGE=debian:trixie ARCH=arm64` to change target | `✅ … all checks passed` |

Single checks: `make test-server`, `make test-web`, `make check-i18n`, `make check-codegen`.

TMDB and fanart.tv test recordings (F005, `server/internal/metadata/testdata/{tmdb,fanart}`): refresh them with `TMDB_API_KEY=… FANART_API_KEY=… scripts/record-metadata-fixtures.sh` (needs curl, jq, perl; `ONLY=tmdb` or `ONLY=fanart` for one of them). Images aren't recorded: the test server generates them. Tests never call TMDB; a request with no recording fails the test. Matcher decisions are frozen in `server/internal/metadata/match/testdata/cases.json`; re-recording can shift popularity, so rerun `make test-server` and check the cases.

Metadata performance (F005, 1,000 movies + 9,000 episodes against a 50 ms TMDB stub, not part of `make test`): `go -C server test -tags perf -run Perf -v -timeout 30m ./internal/metadata/`, or in the throttled container below with `./internal/metadata/`.

Playback performance (F006, start, seek and transcode speed per method on generated 1080p H.264 and 4K HEVC clips, not part of `make test`): `go -C server test -tags perf -run Perf -v -timeout 30m ./internal/playback/`. It prints a table against the spec's NFRs; with a GPU it also measures the hardware transcode. For Pi-like numbers, build the clips once unthrottled, then run throttled with the same `TMPDIR` (the clips are cached there):
```bash
docker volume create plaid-perf
docker run --rm -v "$PWD/server":/src:ro -v plaid-perf:/work -e GOCACHE=/work/gocache -e GOPATH=/work/gopath -e TMPDIR=/work/tmp -e GOFLAGS=-buildvcs=false -w /src golang:1.26 bash -c 'mkdir -p /work/tmp && apt-get update -qq && apt-get install -y -qq ffmpeg && go test -tags perf -run Perf -timeout 30m ./internal/playback/'
docker run --rm --cpus 0.5 --memory 1g -v "$PWD/server":/src:ro -v plaid-perf:/work -e GOCACHE=/work/gocache -e GOPATH=/work/gopath -e TMPDIR=/work/tmp -e GOFLAGS=-buildvcs=false -w /src golang:1.26 bash -c 'apt-get update -qq && apt-get install -y -qq ffmpeg && go test -count=1 -tags perf -run Perf -v -timeout 30m ./internal/playback/'
```

Catalog performance (F008, search over 10,000 items and 30,000 people, the home over 2,000 movies and 200 shows, a filtered grid page and the facets, against the spec's NFRs; not part of `make test`): `go -C server test -tags perf -run Perf -v ./internal/catalog/`. For Pi-like numbers, run it in the throttled container below with `./internal/catalog/`.

Library scan performance (F004, 10,000 sparse files, not part of `make test`): `go -C server test -tags perf -run Perf -v ./internal/library/`. For Pi-like numbers, run it in a throttled container: `docker run --rm --cpus 0.5 --memory 1g -v "$PWD/server":/src:ro -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath -w /src golang:1.26 go test -tags perf -run Perf -v ./internal/library/`.
A single `make test` target must pass before any PR.

On Apple Silicon, `make test-deb ARCH=amd64` only works on distros with systemd < 255 (e.g. `debian:bookworm`): newer systemd can't run under x86 emulation. Real amd64 coverage is the `package-test` CI job.

**CI has one entry point**, `.github/workflows/ci.yml`: it detects which areas a PR touches, calls the matching reusable workflows (`server`, `web`, `codegen`, `package-test`, `docker`) and ends with **CI OK**, the only check `main` requires. A new workflow must be called from `ci.yml` and listed in `ci-ok`'s `needs`, or it won't gate merges.

**Releases are automatic.** Every push to `main` tags the next version and publishes a GitHub release (binaries, `.deb`, checksums) from that tag. Patch by default; write `#minor` or `#major` in the PR title or description to bump more, `[skip release]` to skip. Version logic: `scripts/next-version.sh`.

Generated files (`api.gen.go`, `schema.d.ts`) are never edited by hand. Web uses TypeScript 6: TypeScript 7 (native) breaks `openapi-typescript` and `typescript-eslint`.

## Conventions
- **Everything in the repo is in English**: code, identifiers, comments, commit messages, branch names, PR descriptions, API and logs, as well as docs, specs, ADRs and skills. The team may chat in French.
- **No user-facing string is hardcoded.** Every UI text goes through i18n:
  - English (`en`) is the source locale; French (`fr`) is the first translation.
  - Web: i18n keys + JSON catalogs. Apple: String Catalogs (`.xcstrings`). Android: `strings.xml`. Server: localized error messages follow the user's locale; metadata (synopses, titles) comes in each library's metadata language (F005), with English as the fallback.
  - Keys are semantic (`home.continueWatching.title`), never the English sentence itself.
  - Plurals, dates, durations and numbers use the platform's locale formatters, never manual concatenation.
- Every API change goes through `api/openapi.yaml` first.

## Definition of done
- `make test` green, output pasted into the PR.
- For UI: screenshots attached.
- The "Expected evidence" boxes in `plan.md` are checked.

## What Claude gets wrong
_To fill in: as soon as Claude makes the same mistake twice, we add the correction here._
