---
id: F016
title: Scores on movie and show pages
owner: gh-prim
reviewer: TBD
status: in-progress      # written after the code (owner's request); approved in PR 80's review
spec: ./spec.md
branch: feat/ratings-detail-v21   # PR 80, opened before this feature had a number
---

# F016 — Plan: Scores on movie and show pages

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.
> Written after the code at the owner's request (2026-11-02): it records what was built, in the order it was built.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` | modify | `criticScore`, `audienceScore`; `mdblist` in settings, patch, status, attributions (R3–R5) |
| `server/internal/db/migrations/0016_ratings.sql` | create | The two score columns |
| `server/internal/metadata/mdblist/client.go`, `client_test.go` | create | MDBList client and its test (R1) |
| `server/internal/metadata/service.go`, `keys.go` | modify | Key resolution, client, `builtinMDBListKey` (R3) |
| `server/internal/metadata/apply.go` | modify | `fetchRatings` before, `writeRatings` in the metadata transaction (R2) |
| `server/internal/metadata/read.go` | modify | `Detail.CriticScore`, `AudienceScore` |
| `server/internal/metadata/admin.go`, `keys_test.go` | modify | Settings, key check, status, attribution (R3, R4) |
| `server/internal/metadata/httpx/httpx.go` | modify | Redact `apikey` (R8) |
| `server/internal/settings/settings.go`, `config/config.go`, `cmd/plaid/main.go` | modify | Key name, `PLAID_MDBLIST_API_KEY` |
| `server/internal/handlers/items.go`, `metadata.go`, `metadata_admin_e2e_test.go` | modify | API fields, patch, the three providers |
| `scripts/deploy.sh` | modify | `MDBLIST_API_KEY` built in (R3) |
| `web/src/browse/Ratings.tsx`, `MovieHeader.tsx`, `browse.css`, `ItemPage.test.tsx` | create/modify | The scores row (R6, R7) |
| `web/src/admin/Settings.tsx` | modify | MDBList key in Admin ▸ Metadata |
| `web/src/locales/{en,fr}/common.json` | modify | i18n keys |
| `apple/Common/ScoreBadges.swift`, `API/Models.swift`, `Resources/Localizable.xcstrings` | create/modify | Shared badges, model fields, strings (R6, R7) |
| `apple/PLAID/Views/ItemView.swift`, `apple/iOS/Views/ItemView.swift` | modify | The row on tvOS and iOS |

## Steps, in order
1. [x] `mdblist` client with its test; migration 0016; key, client and status in the metadata service; `apikey` redacted. (R1, R3, R4, R8)
2. [x] Contract first: `openapi.yaml`, then `make generate`; handlers. (R5)
3. [x] Web: `Ratings` with drawn marks in `MovieHeader`; Admin ▸ Metadata key; i18n; page test. (R6, R7)
4. [x] `scripts/deploy.sh` builds `MDBLIST_API_KEY` in; deployed to srv-plex-prod, scores checked by hand.
5. [x] Apple: `ScoreBadges` in `Common`, used by both item views; built for both simulators, rendered with SwiftUI; uploaded to TestFlight.
6. [x] Fix found in production: score writes moved into the metadata transaction. (R2)

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| MDBList quota or outage | Medium | Failures logged, stored scores kept, no built-in key in releases |
| MDBList changes its source names | Low | Unknown sources are ignored; the client test pins `tomatoes` and `popcorn` |
| A score write lost to a busy database | Found | Written in the metadata transaction, retried with it |

**Riskiest step:** step 6, the write path shared with every metadata refresh.

## Rejected options
- OMDb: has the Tomatometer but not the Popcornmeter.
- Scraping Rotten Tomatoes: against its terms and brittle.
- A separate statement for the scores after the transaction: lost writes under load (step 6).

## Expected evidence
- [x] Tests added: `mdblist/client_test.go`, the item page's scores, the three providers in the admin end-to-end test.
- [x] `make test` green (pasted in PR 80).
- [ ] Screenshots (UI) compared to the mockup: attach to PR 80.
- [x] Manual check: real scores on a local server and on srv-plex-prod; Apple badges rendered.

## Log
- 2026-11-02: built before this plan at the owner's request ("skip intent for the moment"), on `feat/ratings-detail-v21` with F008's V2.1 detail pages and the step 14 logo; documented afterwards.
- 2026-11-02 (step 6): on srv-plex-prod, 4 score writes failed with `SQLITE_BUSY` while "Refresh all" ran; they had their own statement after the transaction. The fetch now comes before the transaction and the write goes in it.
- 2026-11-02 (step 5): the iOS simulator wouldn't take typed text, so the badges were checked by rendering `ScoreBadges` with SwiftUI on macOS, not in the signed-in app.
