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
_To be filled in once the scaffold exists (F001)._ A single `make test` target must pass before any PR.

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
