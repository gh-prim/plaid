---
id: F008
title: Web — browsing and player
owner: gh-prim
reviewer: TBD
status: accepted
phase: 2
apps: [server, web]
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
A responsive web app (desktop, tablet, phone browsers), not a PWA. Follows the F002 design system. Smooth with several thousand posters (virtualization). Search and filters apply F005's age-limit predicate (F005-R25).

## Out of scope
Plaid Party (F010), downloads, PWA (install, offline, service worker): phones and TVs get the native apps (phases 4 and 5, ADR 0005).

## Open questions
- [x] **Installable PWA as early as v1?** No. A responsive web app only: the iOS, iPadOS, tvOS, Android and Android TV apps cover phones and TVs, so a PWA would add nothing.
