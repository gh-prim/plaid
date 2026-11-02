---
id: F016
title: Scores on movie and show pages
owner: gh-prim
reviewer: TBD
status: draft            # written after the code (owner's request, 2026-11-02); approved in PR 80's review
intent: ./intent.md
decisions: [0007]
---

# F016 — Spec: Scores on movie and show pages

## Summary
When F005 refreshes a matched movie or show, the server also asks MDBList for its Rotten Tomatoes scores by TMDB id: the Tomatometer (critics, `tomatoes`) and the Popcornmeter (audience, `popcorn`), out of 100. They are stored with the item's metadata and returned on `ItemMetadata`. Item pages on the web, iOS and tvOS draw one row of scores: TMDB users (the existing rating, as a percentage, from 50 votes), the Tomatometer and the Popcornmeter, each with its mark. The MDBList key is set like the other provider keys.

## User journeys
1. A member opens Dune: Part Two: under the year and runtime, "TMDB 81 %", a red tomato "92 %", a popcorn bucket "95 %".
2. A movie critics disliked shows a green splat; one the audience disliked, a tipped bucket.
3. A title Rotten Tomatoes doesn't know shows TMDB alone; a title with no score at all shows no row.
4. The admin opens Admin ▸ Metadata, sees MDBList with its state, chooses "Use my own key", pastes a key from mdblist.com; a wrong key is refused. "Refresh all" fetches the scores for the library.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F016-R1 | With an MDBList key, each movie or show refresh fetches `GET https://api.mdblist.com/tmdb/{movie\|show}/{tmdbId}?apikey=…` and keeps the `tomatoes` and `popcorn` scores (`score`, else `value`), rounded, when between 1 and 100; 0, null or absent means unknown. | must |
| F016-R2 | The scores are fetched before the metadata transaction and written in it, so a busy database retries them with the rest; an MDBList failure is logged and keeps the stored scores; a 404 stores none. | must |
| F016-R3 | The MDBList key follows ADR 0007 (`metadata.mdblist.api_key`, then `PLAID_MDBLIST_API_KEY`, then a built-in key set only by `scripts/deploy.sh`); a new key is checked against The Matrix (TMDB 603) before it's saved. | must |
| F016-R4 | MDBList appears in the metadata status (state, key source) and, while in use, in the attributions ("Rotten Tomatoes scores are provided by MDBList."). | must |
| F016-R5 | `ItemMetadata.criticScore` and `audienceScore` (0–100), absent when unknown. | must |
| F016-R6 | Item pages draw the scores row (web `Ratings`, Apple `ScoreBadges`): TMDB (`rating × 10`, rounded, from 50 votes), Tomatometer, Popcornmeter, in that order, each a mark then the percentage; tomato below 60 becomes a splat, popcorn below 60 a tipped bucket. The row replaces the old "TMDB x · n votes" text and the tvOS "★ x". | must |
| F016-R7 | Each score has a full label for screen readers and as a tooltip on the web ("Rotten Tomatoes Tomatometer (critics): 92 %"); the marks are hidden from them. | must |
| F016-R8 | API keys passed as `apikey` are redacted in provider logs, like `api_key`. | must |

## Design

### Data model
- `0016_ratings.sql`: `item_metadata.critic_score INTEGER`, `item_metadata.audience_score INTEGER` (NULL = unknown). `writeRow`'s upsert doesn't touch them.
- Settings key `metadata.mdblist.api_key`.

### API
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/items/{id}` | `metadata.criticScore`, `metadata.audienceScore` |
| GET, PATCH | `/api/v1/metadata/settings` | `mdblist.keySource` (`custom`, `env`, `builtin`, `none`); patch `mdblistApiKey` (`null` clears it; `422 invalid_api_key` with field `mdblistApiKey`) |
| GET | `/api/v1/metadata/status` | `providers[]` gains `mdblist` |
| GET | `/api/v1/server` | `attributions[]` gains `mdblist` |

### Behavior per app
- **server**: `metadata/mdblist` client (host `api.mdblist.com` only, 2 requests/s, burst 1); `Service.fetchRatings` before the transaction in `processMovie` and `processShow`; `Store.writeRatings` inside it; `Detail` reads the two columns.
- **web**: `browse/Ratings.tsx` in `MovieHeader`; Admin ▸ Metadata lets the admin set the MDBList key like TMDB's, with its own help and "no key" texts.
- **apple**: `Common/ScoreBadges.swift`, used by both item views (tvOS under the meta line at 34 pt marks, iOS under the facts line at 20 pt).

### UI / Art direction
Marks drawn by PLAID (no Rotten Tomatoes artwork): TMDB as a gradient pill with its name, a red tomato with green leaves, a green twelve-point splat, a cream-and-red striped bucket with yellow kernels, tipped when below 60. Numbers bold, tabular. Since V2.1 (F008, canvas step 22) the row shares one line with the genres and tech tags on the web.

### i18n
| Key | en | fr |
|-----|----|----|
| `item.ratings.title` | Scores | Notes |
| `item.ratings.tmdb_one/_other` (`_many`) | TMDB users: {{score}} · {{count, number}} vote(s) | Utilisateurs TMDB : {{score}} · {{count, number}} vote(s) |
| `item.ratings.critics` | Rotten Tomatoes Tomatometer (critics): {{score}} | Tomatometer de Rotten Tomatoes (critiques) : {{score}} |
| `item.ratings.audience` | Rotten Tomatoes Popcornmeter (audience): {{score}} | Popcornmeter de Rotten Tomatoes (public) : {{score}} |
| `metadata.key.mdblistHelp` | Stored on the server… Get a free key at mdblist.com (Preferences ▸ API)… | Enregistrée sur le serveur… Clé gratuite sur mdblist.com… |
| `metadata.state.noScores` | No key: Rotten Tomatoes scores aren't fetched. | Pas de clé : les notes Rotten Tomatoes ne sont pas récupérées. |

Apple String Catalog: `item.ratings.tmdb %@`, `item.ratings.critics %@`, `item.ratings.audience %@`. The `item.rating_*` keys are removed. Percentages via `Intl.NumberFormat` / `FormatStyle.percent` ("92 %" in French).

## Non-functional requirements
- One extra request per title refresh, at 2 per second: a 10,000-title library takes about 1 h 25 to fetch once, within MDBList's free quota only for libraries under 1,000 titles a day (the rest comes as titles refresh on later days).
- The key never leaves the server and never shows in logs.

## ⚠️ Watch-outs
- [x] MDBList's free quota (1,000 a day): over it, requests fail and the stored scores stay; no built-in key in releases.
- [x] A busy database lost score writes when they had their own statement (found on srv-plex-prod): they now go in the metadata transaction.
- [ ] Changing a title's match keeps its old scores until the new fetch succeeds.
- [ ] Rotten Tomatoes' trademarks: PLAID draws its own marks and names the source; to check before an App Store listing shows them.

## Acceptance criteria
- [x] Given MDBList answers The Matrix with `tomatoes` 83 and `popcorn` `score: null, value: 85.4`, then the client returns 83 and 85.
- [x] Given a `tomatoes` score of 0, then the Tomatometer is unknown.
- [x] Given an unknown TMDB id (404), then no scores and no error.
- [x] Given an item with critic 92 and audience 45 and 6,000 TMDB votes at 8.1, then the web page lists "TMDB users: 81 % · 6,000 votes", "…(critics): 92 %", "…(audience): 45 %".
- [x] Given no MDBList key, then the status lists MDBList as `no_api_key` and pages show TMDB only.
- [x] Given a real key on a test server, then Dune: Part Two gets 92 and 95 (checked by hand, 2026-11-02).

## Revisions
- 2026-11-02: written after the code at the owner's request.
