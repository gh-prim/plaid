---
id: F015
title: Admin V2 and monitoring
owner: gh-prim
reviewer: TBD
status: draft            # written after the code (team's request, 2026-11-02); approved in PR 77's review
intent: ./intent.md
decisions: [0002]
---

# F015 — Spec: Admin V2 and monitoring

## Summary
The admin moves to a sidebar shell (canvas page V2, step 21, boards AdmFR/EN 01–03, then AdmV2EN/FR a–c): the server and its state on top, sections grouped as Overview (Dashboard, Now playing, Activity, Monitoring), Content (Libraries, Library report, Unmatched, Metadata), Access (People, Groups, Invitations, Devices) and Server (Transcoding, Remote access, Plaid Party, Problems, Settings), each with its count; ⌘K jumps to a person, a library or a section. Every page has a top bar and cards. The server keeps a log of finished plays (who, what, device, method, reasons, bitrate and, for remote plays, a city) and a day of server samples, which feed the dashboard, Activity and Monitoring. The library report lists every movie and show with its size, best picture, files and plays.

## User journeys
1. The admin opens the dashboard: four numbers with their trends, who is watching (Stop on each), server health, what needs them, hours watched over 7 days, recently added. "Scan all" checks every library.
2. Activity: the week's hours, people, most watched title and Direct Play share; the last plays over 7 days, 30 days or a year, for one person or everyone.
3. Monitoring: where remote plays came from on a world map, the cities; data sent per day and who uses the most; devices worst first with their health score.
4. Library report: four counters (titles, never watched, not watched for a year, several files) that filter a poster wall, with library, quality, genre and sort.
5. Invitations: copy a pending link again, see who accepted, resend an expired one.
6. Plaid Party: turn it on or off, people per party, guests without an account.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F015-R1 | Sidebar shell with grouped sections and counts (streams, pending invitations, open problems); one top bar per page; ⌘K jump; on a phone the sidebar folds into a menu. | must |
| F015-R2 | Dashboard (AdmEN01): streams, active people, storage, upload with trends; Right now with Stop; CPU, memory, disk, transcodes; needs attention; hours per day over 7 days; recently added; Scan all. | must |
| F015-R3 | Every section page (boards AdmV2 a–c) in the shell: Libraries, a library, Unrecognized, Review, Now playing (2-minute bitrate curve per stream), Activity, Unmatched, Groups, a group, Invitations, a person, Devices (pair a TV inline), Transcoding, Metadata (Refresh all), Remote access, Plaid Party, Problems, Settings. | must |
| F015-R4 | The server logs every finished play: account and names (kept if the account goes), item, device, method, reasons, average bitrate, watched time, start and end. | must |
| F015-R5 | Activity over 1–366 days, filtered by account, with totals (plays, people, Direct Play, transcoded, most played title). | must |
| F015-R6 | A remote play is placed with the DB-IP Lite city database, downloaded into the data folder and refreshed monthly. Private, loopback, link-local and CGNAT addresses are never placed. Only country, city and coordinates are kept, never the address. | must |
| F015-R7 | Monitoring over 1–366 days: bytes per day, per person (most first), places (plays, people, last), local/remote/unplaced counts, whether the database is ready. | must |
| F015-R8 | Device health per account and device: plays, direct play, direct stream, transcodes, plays converted for a codec or format, and a score = 50 × direct play rate + 30 × (1 − transcode rate) + 20 × (1 − codec problem rate). Worst first. | must |
| F015-R9 | Library report: every movie and show not hidden, with bytes and files (a show's episodes summed), best picture height, duplicate (a movie or an episode with 2+ files), plays and last play (watch state), added date, genres. Filters: never watched, stale (added 6+ months ago, not played for a year), duplicates, quality (4K ≥ 1600 lines, 1080p ≥ 900, 720p ≥ 600, SD), genre; sort by size, added, plays, title; pages of 60. Totals cover the whole scope whatever the filters. | must |
| F015-R10 | Invitations list pending first, then accepted or expired ones for 30 days; a pending invitation returns its token so its link can be copied again; cancelled ones disappear. Resend creates a new link for the same name and groups. | must |
| F015-R11 | Plaid Party settings: on (default), people per party 2–16 (8), guests without an account (off). Who can start or join stays each group's right (F009). | should |
| F015-R12 | Unmatched rows open the library's unrecognized files, else its titles to review, else the library. Matching a file in a shows library asks for the season and episode(s), guessed from the path. | must |

## Design

### Data model
- `0013_activity.sql`: `play_log` (id, user_id NULL on delete, display_name, profile_name, item_id, title, device_name, method, remote, started_at, ended_at, watched_ms), index on started_at.
- `0014_invitation_links.sql`: `invitations.token`, cleared when used or cancelled.
- `0015_monitoring.sql`: `play_log.bitrate_bps`, `reasons`, `country`, `city`, `lat`, `lon`.
- Settings keys `party.enabled`, `party.max_people`, `party.guests`.
- In memory: a day of one-minute server samples (streams, transcodes, upload).
- File: `<data>/geoip/dbip-city-lite.mmdb`.

### API
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/server/stats/history` | A day of samples |
| GET | `/api/v1/activity` | Hours per day, last 50 plays, totals, recently added; `days` 1–366, `userId`, `tzOffsetMinutes` |
| GET | `/api/v1/monitoring` | Bytes per day and per person, places, device health; `days`, `tzOffsetMinutes` |
| GET | `/api/v1/library-report` | `libraryId`, `show`, `quality`, `genreId`, `sort`, `offset`, `limit` |
| GET, PATCH | `/api/v1/settings/party` | Plaid Party settings |
| GET | `/api/v1/invitations` | Now with `status`, `token` (pending), `acceptedAt`, `userId` |

Also: `ServerStats` memory, `LibraryCounts.sizeBytes`, `ActivePlayback.bitrateBps`, `AdminUser.lastDevice`.

### Behavior per app
- **server**: `playback` logs plays and places them through `geoip.Locator`; `geoip` downloads the month's file (last month's when not out yet), checks it opens, swaps it in, and looks addresses up; `library.Store.Report` aggregates per title in one query; handlers filter, sort and page.
- **web**: `admin/AdminLayout.tsx` shell; the V2 kit (`v2.tsx`: Section, Group, Row) renders cards; new pages Activity, Monitoring (`WorldMap.tsx` with d3-geo and world-atlas), Library report, Groups, Invitations, Unmatched, Plaid Party.
- **apple / android**: out of scope.

### UI / Art direction
Canvas page V2: step 21 (AdmFR/EN 01–03) and boards AdmV2ENa/b/c (FR a/b/c). Monitoring and the report follow the same cards and tables. The map draws Natural Earth country outlines (world-atlas, ISC), dots sized by plays. Every page shows a spinner while loading; empty states have a line each.

### i18n
Namespaces `admin.nav.*`, `admin.dash.*`, `admin.activity.*`, `admin.monitoring.*`, `admin.report.*`, `admin.invitations.*`, `admin.groups.*`, `admin.transcoding.*`, `admin.remote.*`, `admin.problems.*`, `party.*`, `matchFile.episode.*`, in en and fr, plurals with `_one/_other` (`_many` in French). Sizes, rates, durations and percentages through `lib/format` (Intl).

## Non-functional requirements
- Monitoring and Activity read `play_log` by its `started_at` index; the report runs one aggregate query then filters in memory. Not measured on a Pi yet.
- The map has no network dependency in the browser; the geolocation file is downloaded by the server only, monthly, about 60 MB compressed.
- Privacy: no address stored; local plays never located; the admin only.
- Pages poll only while visible (`usePolling`).

## ⚠️ Watch-outs
- [x] Invitation tokens kept in clear for copy (migration 0014): admins only, cleared on use or cancel; an expired unused one keeps a token that no longer opens anything.
- [x] Geolocation download: integrity checked by opening the file before swapping; capped at 1 GB; failures logged, retried daily.
- [x] DB-IP's licence (CC BY 4.0) needs attribution: shown under the map.
- [ ] Plays logged before migration 0015 have no bitrate or place: Monitoring fills in with time.
- [ ] Commits were tagged F007 (the admin it extends) before this feature had a number.

## Acceptance criteria
- [x] Given a finished play, then `play_log` holds it with its bitrate and reasons; given a remote play from a placed address, then its city and coordinates, not the address.
- [x] Given plays, when the admin asks `/activity?days=366&userId=…`, then only that account's plays and totals come back.
- [x] Given two remote plays from Lyon and one unplaced, then Monitoring shows Lyon with 2 plays, 1 person, and 1 unplaced.
- [x] Given a TV with 1 direct play in 3 and 2 codec problems, then its score is 33 and it comes first.
- [x] Given a library, then the report counts titles, never watched and duplicates, and pages by 60 with the chosen filter and sort.
- [x] Given a database download that isn't one, then it's refused and the old file stays.
- [x] Given a pending invitation, then its link can be copied; once accepted it shows Accepted without a token; once cancelled it's gone.
- [x] Given the Unmatched row of a library with unrecognized files, when clicked, then its files open; given a shows library, when a show is picked for a file, then the season and episode are asked, pre-filled from the path.

## Revisions
- 2026-11-02: written after the code at the team's request; the scope (no alerts, automations or newsletter) was set when comparing with Tracearr.
