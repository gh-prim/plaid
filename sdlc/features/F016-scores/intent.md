---
id: F016
title: Scores on movie and show pages
owner: gh-prim
reviewer: TBD
status: draft            # written after the code (owner's request, 2026-11-02); accepted in PR 80's review
phase: 2
apps: [server, web, apple]
created: 2026-11-02
---

# F016 — Scores on movie and show pages

## Problem
A movie or show page only says "TMDB 8.1 · 6,000 votes", in small text. To decide what to watch tonight, people look up what critics and the audience thought on Rotten Tomatoes, outside PLAID. Plex and the other players show those scores with their logos; PLAID doesn't.

## Expected outcome
Each movie and show page shows its scores at a glance, each with its source's mark: TMDB users, Rotten Tomatoes' Tomatometer (critics) and Popcornmeter (audience), on the web, the iPhone and iPad app and the Apple TV app.

## Users and systems affected
- Everyone who browses (members, friends, guests): web, iOS/iPadOS, tvOS item pages.
- The admin: a new provider key (MDBList) in Admin ▸ Metadata.
- Server: the metadata refresh (F005) fetches the scores; the item API returns them.

## Success criteria
- [ ] A movie or show with Rotten Tomatoes scores shows TMDB, Tomatometer and Popcornmeter with their marks on all three apps.
- [ ] A score nobody has is left out; with none, no empty row.
- [ ] The marks change with the score like Rotten Tomatoes' own: fresh or rotten for critics, full or tipped popcorn for the audience, at 60 %.
- [ ] Without an MDBList key, everything works as before, with TMDB only.
- [ ] The scores come back on their own as titles are refreshed; "Refresh all" brings them now.

## Constraints
- Rotten Tomatoes has no public API: the scores come from MDBList, which gathers them by TMDB id (free key, 1,000 requests a day).
- A failing or slow MDBList never holds up or undoes a match (F005).
- The key follows ADR 0007's order (admin's key, environment, built-in); release binaries never ship a built-in MDBList key, because of the daily quota.
- The marks are drawn by PLAID, not Rotten Tomatoes' own artwork.
- i18n: labels for screen readers in en and fr; percentages through the locale's formatter.

## Out of scope
- Other sources MDBList returns (IMDb, Metacritic, Letterboxd, Trakt…): one line each later if wanted.
- Sorting or filtering the grid by Rotten Tomatoes scores.
- Episode scores; "Certified Fresh" and "Verified Hot" badges.

## Open questions
None: decided with the owner while coding (2026-11-02).
