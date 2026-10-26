---
id: F008
title: Web — browsing and player
owner: TBD
reviewer: TBD
status: draft
phase: 2
apps: [web]
created: 2026-10-26
---

# F008 — Web: browsing and player

## Problem
Once the media library is indexed, starting a movie should be tempting and take three clicks, like on Netflix, but at home.

## Expected outcome
A home screen with rows ("Continue watching", "Recently added", by genre), immersive detail pages, instant search, and a full-featured player that resumes where you left off.

## Users and systems affected
- All users.
- Web; F002 design system; F005 and F006 APIs.

## Success criteria
- [ ] Profile picker on launch ("Who's under the plaid?" ("Qui est sous le plaid ?")).
- [ ] Home in rows, library grids with filters and sorting.
- [ ] Movie page and series page (seasons, episodes, cast, trailer).
- [ ] Instant search.
- [ ] hls.js player: quality, audio tracks, subtitles, keyboard shortcuts, auto next episode.

## Constraints
Follows the F002 design system. Smooth with several thousand posters (virtualization).

## Out of scope
Plaid Party (F010), downloads.

## Open questions
- [ ] Installable PWA as early as v1?
