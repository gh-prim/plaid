---
id: F005
title: Metadata and artwork
owner: TBD
reviewer: TBD
status: draft
phase: 1
apps: [server]
created: 2026-10-26
---

# F005 — Metadata and artwork

## Problem
A file name makes nobody want to watch anything. What creates the "Netflix" experience is the posters, backdrops, synopses, cast and trailers.

## Expected outcome
Every recognized movie, show, season and episode is enriched automatically (title, synopsis, year, genres, cast, ratings, poster, backdrop, logo), in the language chosen by the admin, with a way to fix a wrong match.

## Users and systems affected
- Admin (corrections), all users (display).
- Server; TMDB external API; local image cache.

## Success criteria
- [ ] Automatic matching via TMDB (title + year), with a confidence score.
- [ ] Manual "Fix match".
- [ ] Images cached and resized server-side (thumbnails suited to each client).
- [ ] Metadata language configurable per library, falling back to English.
- [ ] Local `.nfo` files and images take priority when present.

## Constraints
- TMDB API key: provided by the admin, or a project key? To check against the terms of use.
- Respect rate limits.

## Out of scope
OpenSubtitles subtitles (separate feature), intros and end credits.

## Open questions
- [ ] TVDB in addition to TMDB for shows/anime?
- [ ] Generate ambient colors from the poster, for the UI?
