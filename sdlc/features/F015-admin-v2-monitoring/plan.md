---
id: F015
title: Admin V2 and monitoring
owner: gh-prim
reviewer: TBD
status: in-progress      # written after the code (team's request); approved in PR 77's review
spec: ./spec.md
branch: feat/F009-library-sharing   # shipped in PR 77 with F009, at the team's request
---

# F015 — Plan: Admin V2 and monitoring

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.
> Written after the code at the team's request (2026-11-02): it records what was built, in the order it was built.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` | modify | Stats history, activity, monitoring, library report, party settings, invitation fields |
| `server/internal/db/migrations/0013_activity.sql`, `0014_invitation_links.sql`, `0015_monitoring.sql` | create | Play log, invitation links, monitoring columns (R4, R6, R10) |
| `server/internal/playback/activity.go` | create | Server samples, play log, Activity with totals and filter (R4, R5) |
| `server/internal/playback/monitoring.go` | create | Bytes per day and person, places, device health (R7, R8) |
| `server/internal/playback/service.go` | modify | Log at the end of a play, `Geo` locator option, bitrate per stream |
| `server/internal/geoip/geoip.go` | create | DB-IP Lite download, refresh, lookup, private-address filter (R6) |
| `server/internal/library/report.go` | create | Per-title aggregate for the report (R9) |
| `server/internal/library/store.go`, `sysstats/sysstats.go` | modify | Sizes per library, recently added, memory |
| `server/internal/accounts/{invitations,party,sharing}.go` | create/modify | Invitation history and tokens, party settings, last device (R10, R11) |
| `server/internal/handlers/{server_stats,monitoring,sharing,libraries,playback}.go` | create/modify | Endpoints |
| `server/cmd/plaid/main.go` | modify | Start the locator, pass it to playback |
| `web/src/admin/AdminLayout.tsx`, `summary.ts`, `routes.tsx` | modify | Shell, sidebar, ⌘K, routes (R1) |
| `web/src/admin/v2.tsx` | modify | Section, Group, Row as cards in the shell (R3) |
| `web/src/admin/{Dashboard,Libraries,People,Playing,Devices,Settings,AdminSettings,RemoteAccess,Review,Sharing}.tsx` | modify | Pages per the boards (R2, R3, R12) |
| `web/src/admin/{Lists,Party,Monitoring,WorldMap,Report}.tsx` | create | Groups, Invitations, Activity, Unmatched, Plaid Party, Monitoring, map, report |
| `web/src/{hooks/usePolling.ts,lib/format.ts,ui/Icon.tsx}` | modify | Reload on a filter change, percentages, icons |
| `web/package.json` | modify | d3-geo, topojson-client, world-atlas (ISC) |
| locales | modify | en and fr |

## Steps, in order
1. [x] Play log, server samples, `/server/stats/history`, `/activity`, sizes, bitrate, memory, last device. (R4, R5)
2. [x] Sidebar shell, dashboard, Libraries and People tables, Activity, Unmatched, Groups, Invitations pages. (R1, R2)
3. [x] Diff with boards AdmV2 a–c: the V2 kit renders cards, so the remaining section pages move to the shell; per-page work for Now playing, a library, a person, a group, Devices, Transcoding, Metadata, Remote access, Problems, Settings. (R3)
4. [x] Invitation history and copyable links (migration 0014); Plaid Party settings and page; Activity over a year with a person filter. (R5, R10, R11)
5. [x] Unmatched rows clickable; season and episode when matching a series file. (R12)
6. [x] `geoip` package and monthly download; play log gets bitrate, reasons and place (migration 0015). (R6)
7. [x] `/monitoring` and its page: map, bytes per day and per person, device health. (R7, R8)
8. [x] `/library-report` and its page: counters, filters, poster wall. (R9)

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| The geolocation file can't be downloaded (offline server, DB-IP down) | medium | The map says so; plays still log; retried daily; last month's file tried when this month's isn't out |
| A bad download replaces a good database | low | Opened as a database before the swap; written to a temp file, renamed |
| Report or monitoring slow on a large library on a Pi | medium | One aggregate query, then in-memory filtering; to measure with the F008 perf data |
| Keeping invitation tokens in clear | low | Admin-only endpoint; cleared on use or cancel; single use and expiry still apply |
| Restyling the shared V2 kit breaks a page | medium | Every page checked in the browser; admin tests updated |

**Riskiest step:** step 6, a network download into the data folder on every install.

## Rejected options
- **Map tiles (OpenStreetMap, MapLibre)**: a tile server or a large tile set; country outlines drawn locally are enough for dots by city.
- **MaxMind GeoLite2**: needs an account and a licence key per install; DB-IP Lite is free without one (attribution).
- **Country-level database (4 MB)**: smaller, but a dot per country says little; the city database (60 MB compressed) was chosen.
- **Keeping client addresses in the log**: not needed for the map; a city is enough and safer.
- **Alerts, automations, newsletter (as in Tracearr)**: out of scope by decision.

## Expected evidence
- [x] Tests added: `TestActivity` (filter, year, totals), `TestMonitoring`, `TestLibraryReport`, `TestPartySettings`, `TestInvitation` (history, token), `geoip` (download, fallback month, garbage refused, private addresses), web tests for Invitations, Plaid Party, Monitoring, the report, Unmatched rows, the episode step and the path guess.
- [ ] Web test for the Activity page (period tabs, person filter).
- [x] `make test` green (`✅ all checks passed`, 246 web tests).
- [ ] Screenshots attached to the PR.
- [ ] Monitoring and the report measured on a Pi-class container.
- [x] Manual check: every page in the browser against the boards; deployed to srv-plex-prod (`0.1.19+local.11022002`), DB-IP database downloaded on start.

## Log
- 2026-11-02: coded before the docs at the team's request; commits are tagged `F007` (the admin they extend) because this feature had no number yet.
- 2026-11-02: step 21 placed in PR 77 with F009 at the team's request.
- 2026-11-02: the V2 kit (`Section`, `Group`, `Row`) was restyled instead of rewriting each page; per-page work only where the boards differ.
- 2026-11-02: compared with Tracearr; the team kept monitoring and the library report, without alerts, automations or newsletter.
- 2026-11-02: production showed Unmatched rows that looked clickable but weren't, and series files that couldn't be matched (the server needs the season and episode); both fixed.
