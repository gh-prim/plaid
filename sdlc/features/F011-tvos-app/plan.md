---
id: F011
title: Apple TV app
owner: gh-prim
reviewer: gh-prim
status: in-progress
spec: ./spec.md
branch: feat/F011-tvos-app
---

# F011 — Plan: Apple TV app

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` | modify | `/items/{itemId}/segments`, `ItemSegments`, `Segment` (R15) |
| `server/internal/api/api.gen.go`, `web/src/api/schema.d.ts` | regenerate | `make generate` |
| `server/internal/discovery/discovery.go` | create | Bonjour announcement (R1) |
| `server/internal/segments/segments.go`, `segments_test.go` | create | TheIntroDB lookup, cache, merge with detected credits (R15) |
| `server/internal/segments/detect.go`, `detect_test.go` | create | FFmpeg `blackframe` end-credits detection (R15, ADR 0008) |
| `server/internal/db/migrations/0010_segments.sql`, `0011_detected_credits.sql` | create | `item_segments` |
| `server/internal/db/db_test.go` | modify | rollback drops `item_segments` |
| `server/internal/handlers/segments.go`, `handlers.go` | create / modify | `GetItemSegments`, Deps |
| `server/cmd/plaid/main.go` | modify | wire segments (FFmpeg path) and Bonjour |
| `server/go.mod`, `go.sum` | modify | `github.com/libp2p/zeroconf/v2` |
| `apple/project.yml`, `apple/.gitignore` | create | xcodegen project; generated `PLAID.xcodeproj` not committed |
| `apple/PLAID/App/*` | create | app state, Keychain, language override (R3–R5), Top Shelf writer |
| `apple/PLAID/API/*` | create | API client, models, Bonjour browser (R2) |
| `apple/PLAID/Views/*` | create | side menu, home, cards, libraries, search, item, person, collection, settings, sign-in (R6–R12, R18, R20) |
| `apple/PLAID/Player/*` | create | player, next-episode card, hero preview, trailer button (R13–R17) |
| `apple/PLAID/Resources/*` | create | String Catalog en/fr, fonts, icons |
| `apple/Shared/ShelfStore.swift`, `apple/TopShelf/*` | create | Top Shelf extension (R19) |
| `sdlc/decisions/0008-intro-credits-segments.md` | create | ADR |
| `sdlc/roadmap.md` | modify | F011 row, ADR 0008 |

## Steps, in order
Each step is one commit or a few, all in one PR from `feat/F011-tvos-app`.
1. [x] Bonjour announcement in the server (R1).
2. [x] OpenAPI `/segments`, `make generate`; migrations 0010/0011; `segments` package with tests against a TheIntroDB stub; handler (R15).
3. [x] Credits detection with tests on a generated clip (skips without FFmpeg) (R15).
4. [x] tvOS app skeleton: discovery, pairing, profiles, Keychain, language (R2–R5).
5. [x] Home, side menu, cards, item pages (R6, R7, R10, R12).
6. [x] Player: HLS, resume, progress, tracks memory, quality, errors (R13, R14).
7. [x] Skip intro and next-episode card from segments (R16, R17).
8. [x] Libraries, search, person and collection pages (R8, R9, R11).
9. [x] Settings, About, Top Shelf (R18, R19).
10. [x] Error state for links; menu kept folded when opened by a link (R7, R20).
11. [ ] `make test` green; build the app for the simulator and the device; screenshots; open the PR.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| FFmpeg detection loads a Pi during playback | medium | one run at a time, `nice 19`, last 10 min at 2 fps, once per file |
| TheIntroDB down or rate-limiting | medium | cached 30 days (3 when empty), 2 req/s, failures give an empty list |
| tvOS focus engine traps focus (menu opens, can't go back) | high | 64 pt menu focus frames, `pageFocusTick` to hand focus to the page, rail disabled until the page is ready |
| Signing on another dev's machine | medium | team via `PLAID_TEAM`, App Group documented |
| No CI for the app | high | built by hand in the simulator; tracked as a watch-out |

**Riskiest step:** 5, the focus behavior of the side menu; every regression so far came from it.

## Rejected options
- Audio-fingerprint intro detection across a season: too heavy for a Pi (ADR 0008).
- In-app YouTube playback (YouTubeKit, yt-dlp): blocked by YouTube's bot checks, not something to bypass.
- Committing `PLAID.xcodeproj`: generated, and it holds the signing team and per-user state.

## Expected evidence
- [ ] Tests added: `segments_test.go`, `detect_test.go`, migration rollback.
- [ ] `make test` green
- [ ] Screenshots from the simulator: home, menu open, library with filters, title page, person page, next-episode card, settings.
- [ ] Manual check: pairing from a fresh install; next-episode card on For All Mankind S1E5 (credits detected at 57:16).

## Log
- 2026-10-31: the code was written before this plan (prototype first, by the owner's choice); the intent, spec and plan document what was built.
- 2026-10-31: trailers open the YouTube app instead of playing inside the app.
