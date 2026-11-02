---
id: F015
title: Admin V2 and monitoring
owner: gh-prim
reviewer: TBD
status: draft            # written with the code (team's request, 2026-11-02); accepted in PR 77's review
phase: 3
apps: [server, web]
created: 2026-11-02
---

# F015 — Admin V2 and monitoring

## Problem
F007's admin grew one section at a time: big tiles and rows built for a TV-like look, a hub of links, and no way to see what happens over time. With sharing (F009), the admin now looks after other people: who watches what, from where, on which device, and what the library holds that nobody watches. Finding a group or an invitation was hard enough that the team asked where they were.

## Expected outcome
A dense, sober admin like a server tool: a sidebar with every section and its count, a top bar per page, cards and tables. On top of today's pages, the admin sees activity over time, where remote plays come from on a map, the data each person uses, which devices play badly, and a report of the library's titles by size, quality and plays.

## Users and systems affected
- The server's admins (web).
- Server: activity and monitoring data (a play log, server samples, geolocation), library report, invitation history, Plaid Party settings.

## Success criteria
- [ ] Every admin page follows the V2 boards (canvas steps 21 and boards AdmV2 a–c, 20 screens), reachable from the sidebar.
- [ ] The dashboard shows what plays now, server health, what needs the admin, hours watched and what was added.
- [ ] Activity covers a week, a month or a year, for everyone or one person.
- [ ] Remote plays appear on a map by city, without sending addresses anywhere and without keeping them.
- [ ] Data sent per day and per person; a health score per device.
- [ ] A library report: titles never watched, not watched for a year, with several files, filtered by quality and genre, with their size.

## Constraints
- Runs on a Raspberry Pi: aggregation over a year of plays and 10,000 titles stays in hundreds of milliseconds.
- No tile server or online map: the map is drawn locally.
- Geolocation from a free database the server downloads (DB-IP Lite, CC BY 4.0, attribution shown); home plays are never located.
- i18n en and fr for everything.

## Out of scope
- Alerts and notifications (Discord, ntfy, email…), automations, trust scores: decided against on 2026-11-02.
- Newsletters of recently added titles.
- Several servers in one admin, Tautulli or Jellystat imports.

## Open questions
None: decided with the team while coding (2026-11-02).
