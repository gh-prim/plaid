---
id: F005
title: Metadata and artwork
owner: gh-prim
reviewer: TBD
status: accepted
phase: 1
apps: [server]
created: 2026-10-26
---

# F005 — Metadata and artwork

## Problem
A file name makes nobody want to watch anything. What creates the "Netflix" experience is the posters, backdrops, synopses, cast and trailers.

## Expected outcome
Every movie, show, season and episode that F004 recognizes is enriched automatically (title, synopsis, year, genres, cast, ratings, certification, poster, backdrop, logo, trailer), in the language the admin picks for the library. Wrong matches are easy to fix, and the library stays browsable when the internet is down.

## Users and systems affected
- Admin (matching review and corrections), all users (display, parental control through F003 profiles).
- Server; TMDB API (main provider); fanart.tv API (extra artwork); local image cache in the data directory.
- Builds on F004: `items` and `media_files`, parsed titles, years, episode numbers, `{tmdb-…}` / `{imdb-…}` hints, unrecognized files and scan events.

## Success criteria
- [ ] Automatic matching via TMDB (title + year, or an ID hint), with a confidence score. Above the threshold the match is applied; below it the item waits for review with its best candidates.
- [ ] Manual "Fix match": search TMDB, pick a candidate, and the match is locked against rescans. Single fields can be edited and locked. The same flow identifies F004's unrecognized files.
- [ ] fanart.tv as a second artwork source (logos, clearart, extra backdrops), behind a provider interface so more providers can come later.
- [ ] Images cached in the data directory (never in library folders), resized on demand to a fixed set of widths for each client.
- [ ] Metadata language and region configurable per library, falling back to English field by field.
- [ ] Local `.nfo` files and images next to the media take priority when present.
- [ ] Certifications per region, so F003 profiles' `maxAge` / `allowUnrated` can work.
- [ ] Trailers (TMDB video keys), TMDB collections, and ambient colors taken from posters and backdrops.
- [ ] Offline: items stay browsable with their parsed titles and local data, matching is queued and retried when the network comes back, and cached data never expires because of an outage.

## Constraints
- API keys: PLAID ships a project TMDB key and a project fanart.tv key, and the admin can override both with their own. Use stays non-commercial, with the attribution TMDB's terms require.
- Respect the providers' rate limits. Startup must not wait on the network. A Raspberry Pi 4 must cope with a 10,000-item library.
- PLAID never writes inside library folders (F004 R23).

## Out of scope
OpenSubtitles subtitles (separate feature), intros and end credits, TVDB and anime-specific ordering, writing `.nfo` files, per-user metadata languages, the UI (F007 for the admin, F008 for browsing).

## Open questions
- [x] TVDB in addition to TMDB for shows and anime? → No for v1: its licence costs money. TVDB goes to the backlog behind the provider interface.
- [x] Generate ambient colors from the poster, for the UI? → Yes.
- [x] Which providers? → TMDB plus fanart.tv.
- [x] API key? → Project keys built in, admin can override.
- [x] Metadata language? → Per library, English fallback.
- [x] Image cache? → Data directory, resized on demand.
- [x] Corrections? → Search and pick, with locked matches and fields.
- [x] Offline? → Local first, retry later.
