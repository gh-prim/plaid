# PLAID

A self-hosted alternative to Plex, with a "plaid & chill" vibe. Full vision: [sdlc/vision.md](sdlc/vision.md).

## Process (mandatory)
We follow the SDLC described in [sdlc/README.md](sdlc/README.md): `intent.md` → `spec.md` → `plan.md` → code.
- **Never write application code without a `plan.md` in `approved` status** for the feature in question. If asked for code without a plan, first suggest going through the `new-intent`, `intent-to-spec` then `spec-to-plan` skills.
- If the implementation drifts from the plan, update `plan.md` in the same commit.
- Commits and branches cite the feature ID (`F006: …`, `feat/F006-…`).
- We are two devs: a PR's author never approves it themselves.

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
Run from the repo root. Toolchain: Go 1.26, Node 24+, pnpm 10, golangci-lint v2, shellcheck, Docker (for `make docker`).

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

Single checks: `make test-server`, `make test-web`, `make check-i18n`, `make check-codegen`.
A single `make test` target must pass before any PR.

**CI has one entry point**, `.github/workflows/ci.yml`: it detects which areas a PR touches, calls the matching reusable workflows (`server`, `web`, `codegen`, `package-test`, `docker`) and ends with **CI OK**, the only check `main` requires. A new workflow must be called from `ci.yml` and listed in `ci-ok`'s `needs`, or it won't gate merges.

**Releases are automatic.** Every push to `main` tags the next version and publishes a GitHub release (binaries, `.deb`, checksums) from that tag. Patch by default; write `#minor` or `#major` in the PR title or description to bump more, `[skip release]` to skip. Version logic: `scripts/next-version.sh`.

Generated files (`api.gen.go`, `schema.d.ts`) are never edited by hand. Web uses TypeScript 6: TypeScript 7 (native) breaks `openapi-typescript` and `typescript-eslint`.

## Conventions
- **Everything in the repo is in English**: code, identifiers, comments, commit messages, branch names, PR descriptions, API and logs, as well as docs, specs, ADRs and skills. The team may chat in French.
- **No user-facing string is hardcoded.** Every UI text goes through i18n:
  - English (`en`) is the source locale; French (`fr`) is the first translation.
  - Web: i18n keys + JSON catalogs. Apple: String Catalogs (`.xcstrings`). Android: `strings.xml`. Server: localized error messages and metadata language follow the user's locale.
  - Keys are semantic (`home.continueWatching.title`), never the English sentence itself.
  - Plurals, dates, durations and numbers use the platform's locale formatters, never manual concatenation.
- Every API change goes through `api/openapi.yaml` first.

## Definition of done
- `make test` green, output pasted into the PR.
- For UI: screenshots attached.
- The "Expected evidence" boxes in `plan.md` are checked.

## What Claude gets wrong
_To fill in: as soon as Claude makes the same mistake twice, we add the correction here._
