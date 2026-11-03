---
id: F010
title: Plaid Party — watch together
owner: gh-prim
reviewer: gh-prim
status: approved
spec: ./spec.md
branch: feat/F010-plaid-party
---

# F010 — Plan: Plaid Party, watch together

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

One PR from `feat/F010-plaid-party`, cut from `main`. Every step below is a commit (`F010: …`), in this order; server first so the web and Apple clients build on a protocol that is already tested. Boards are named from the canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz), page V2, step 23.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `server/internal/db/migrations/0017_plaid_party.sql` | create | `parties`, `party_invites`, `party_removed`, `party_guests`, `party_votes`, `party_proposals`, `party_ballots`, `show_follows`, `upcoming_episodes`, `release_alerts` (spec data model) |
| `server/internal/party/store.go` (+ `_test`) | create | SQL for parties, invites, guests, votes, follows, alerts; short transactions |
| `server/internal/party/room.go` (+ `_test`) | create | The in-memory room: participants, devices, queue, control, playback state, waiting, pause requests, host rules, chat and reaction limits, summary. Pure state machine with an injected clock |
| `server/internal/party/sync.go` (+ `_test`) | create | Reference state math, drift rules (R8–R10), shared with the simulator |
| `server/internal/party/sync_test.go` | create | Simulated clients (RTT, skew, jitter, seek cost) driven by the room; the p95 test |
| `server/internal/party/service.go` (+ `_test`) | create | Create, join (by id for invitees, by code otherwise), invite, answer, removal, end; owns rooms; wires playback and realtime |
| `server/internal/party/scheduler.go` (+ `_test`) | create | Minute ticker: reminders, lobby, auto start, 3 h cancel, vote deadlines, release alerts; catches up after a restart |
| `server/internal/party/vote.go` (+ `_test`) | create | Proposals, ballots, tally (3/2/1, veto), seeded tie draw |
| `server/internal/party/upcoming.go` (+ `_test`) | create | Followed shows, `GET /upcoming` assembly |
| `server/internal/realtime/hub.go`, `conn.go` (+ `_test`) | create | WebSocket endpoint `/api/v1/ws`: auth, `Origin` check, ping, per-connection send queue, rate limits, routing to party, `notify`, `tv.cast`, `party.companion` |
| `server/internal/auth/principal.go`, `middleware.go`, `cookies.go`, `public_routes_test.go` | modify | Third identity: party guest (cookie `plaid_guest` or bearer guest token), allowed only on the party's routes; two new public routes |
| `server/internal/accounts/party.go` | modify | `maxPeople` 2–20, `uploadMbps` |
| `server/internal/playback/manager.go`, `service.go`, `hls.go`, `runner.go`, `decide.go` (+ tests) | modify | Party sessions (`PartyID`, guest owner without watch state, start at room position), access override R6 with age check kept, shared encodes R25, upload cap R27 |
| `server/internal/access/access.go` | modify | "In this party" grants the party's item (R6) |
| `server/internal/metadata/tmdb/client.go`, `server/internal/metadata/service.go` | modify | `/tv/{id}` next/last episode and current season for followed shows, daily (R36) |
| `server/internal/library/` (scan hook) | modify | Tell party when an episode file appears (alerts R38, future-episode parties) |
| `server/internal/handlers/parties.go`, `upcoming.go` (+ `_e2e_test`) | create | REST routes of the spec |
| `server/internal/handlers/handlers.go`, `cmd/plaid/main.go` | modify | Wire party service, hub, scheduler |
| `server/go.mod` | modify | `github.com/coder/websocket` |
| `api/openapi.yaml`, `server/internal/api/api.gen.go`, `web/src/api/schema.d.ts` | modify / regenerate | Party, vote, upcoming, follow, guest routes; `PartySettings` |
| `api/events.md` | modify | The whole protocol with one example per message (R44) |
| `web/src/party/socket.ts`, `clock.ts`, `sync.ts` (+ tests) | create | WebSocket client with reconnect, clock offset, drift correction on the player engine |
| `web/src/party/PartyPlayer.tsx`, `PartyPanel.tsx`, `Chat.tsx`, `Reactions.tsx`, `People.tsx`, `SyncToasts.tsx`, `HostSheets.tsx`, `EndSummary.tsx`, `party.css` (+ tests) | create | Party13, 15–19 |
| `web/src/party/LaunchSheet.tsx`, `InviteSheet.tsx`, `JoinCode.tsx`, `GuestJoin.tsx`, `Queue.tsx`, `ScheduledParty.tsx`, `Lobby.tsx` (+ tests) | create | Party04–09, 14, 25 |
| `web/src/party/vote/*` (+ tests) | create | VoteBig (web), proposals, ranking with veto, results, tie draw |
| `web/src/upcoming/*` (+ tests) | create | PartiesWeb: list, calendar, invitation |
| `web/src/player/Player.tsx`, `engine.ts` | modify | A `party` mode: controls go through the room, `playbackRate` for drift, report buffering and ready |
| `web/src/browse/ItemPage.tsx`, `MovieHeader.tsx`, `ShowSeasons.tsx`, `HomePage.tsx`, `TabBar.tsx`, `TopBar.tsx` | modify | "Plaid Party" action, "Follow", home row (Party21), Upcoming tab with badge |
| `web/src/App.tsx` | modify | Routes `/party/:id`, `/p/:code`, `/upcoming`, `/party/:id/vote` |
| `web/src/admin/Party.tsx` (+ `admin.test.tsx`) | modify | Party26–27: limit up to 20, upload speed, the "Who can start" row removed |
| `web/src/locales/{en,fr}/*.json` | modify | `party.*`, `upcoming.*`, `vote.*` |
| `apple/Common/API/PartySocket.swift`, `PartyModels.swift`, `PartySync.swift` | create | `URLSessionWebSocketTask` client, clock offset, drift via `AVPlayer.rate` |
| `apple/Common/API/APIClient.swift`, `Models.swift` | modify | REST routes, models |
| `apple/iOS/Party/*` | create | Launch, schedule, invite (share sheet), join code, companion (Party01, 02), watching (10–12), host sheets (17–19), end (16), invitation (20), Upcoming (PartiesPh), vote (VotePh) |
| `apple/iOS/Views/HomeView.swift`, `ItemView.swift`, `MainTabs.swift`, `iOS/App/*`, `iOS/Player/PlayerScreen.swift` | modify | Home row, launch action, Upcoming tab, universal link `/p/:code`, local reminders, party mode in the player |
| `apple/PLAID/Party/*` | create | tvOS: start from the TV (23), overlay (03), remote reactions (24), next episode (22), winner reveal (VoteBig), `tv.cast` |
| `apple/PLAID/Player/PlayerView.swift`, `Views/ItemView.swift`, `App/*` | modify | Party mode in the player, launch action, keep the socket while the app is open |
| `apple/**/Localizable.xcstrings` | modify | en, fr |
| `apple/project.yml` (and the iOS entitlements it generates) | modify | Associated domain for `/p/` links, as F009 did for invitation links |
| `sdlc/roadmap.md`, this plan | modify | Status |

## Steps, in order

**Server**
1. [x] Migration `0017_plaid_party.sql` and `party/store.go`, with store tests (insert, unique code among live parties, invite answers, removed people, guest token hash). R1, R2, R5, R16, R28, R31–R33, R35–R36.
2. [x] Settings: `maxPeople` 2–20, `uploadMbps`; `openapi.yaml` `PartySettings`; `make generate`; tests on bounds. R42.
3. [x] Guest identity in `auth`: `Principal.GuestID` and `PartyID`, guest tokens (32 bytes, SHA-256 kept), cookie `plaid_guest`, deny-by-default extended (a guest reaches only `/ws`, its party, its item's playback, stream and artwork). Tests: a guest gets `401` everywhere else (table of every route from `openapi.yaml`). R5.
4. [x] `realtime` hub: `/api/v1/ws` with `coder/websocket`, auth (cookie + `Origin`, bearer, guest), `{type, data, ts, id}` envelope, `ack`/`error`, ping every 25 s, send queue of 256 with drop after 5 s of backlog, token-bucket limits per message type, `sync.ping`/`pong`. Check the HTTP server's write timeout doesn't cut long-lived connections (same path as scan events). Tests with `httptest`. R9, R44, NFR.
5. [x] `party/room.go`: the state machine with a fake clock. Table tests for join and queue (R4), control modes and grants (R13, R14), pause request and "in 5 min" (R15), host hand-over, 2 min disconnect rule, removal (R16, R17), waiting at 1.5 s and resume without after 20 s (R11), chat and reaction limits, 200-message history (R18, R19), presence (R20), end rules (R7), summary (R21).
6. [x] `party/sync.go` and its simulator (`sync_test.go`): R8–R10 math shared by the room and simulated clients; the acceptance test with 80 ms RTT, 40 ms skew, 20 actions, p95 < 500 ms; drift at 1.2 s → rate 1.05, at 4 s → seek.
7. [x] Playback for parties: `POST /playback` with `partyId` starts at the room's position; guest-owned sessions (no watch state, no profile); R6 access override in `access` while the person is in the party, age limit still checked (`403 party_age_limit`). Tests. R5, R6, R12.
8. [x] Shared encodes in `playback`: runs keyed by (file, audio track, burned subtitle, quality), reference-counted, one slot of `maxTranscodes`; each session keeps its token and resolves segments to the shared run; a session more than 30 s away from the run's position leaves it (its own run, subject to the limit). Then the R26 flow: a `503` for a party screen raises `party.transcode.limit` to the host; "shared" moves the waiting screens to the common quality, "order" queues them for the next free slot. Upload cap R27. Tests with the fake FFmpeg (`testdata/fakeffmpeg.sh`). R25–R27.
9. [x] `party/service.go` + REST `handlers/parties.go`: create (right `start`), `GET /parties`, detail, patch, cancel, invites (also during a live party, with `notify`), answer, `/parties/people`, public lookup and guest join by code with the per-IP backoff, admin list and end. `party.join` by id only for invitees, by code for others. E2E tests from the acceptance criteria. R1–R5, R16, R28, R43.
10. [x] Device routing in the hub: online TVs of an account (F003 session device type `tv` with a live socket), `tv.cast`, `party.companion` to the same account's iPhone/iPad. Tests. R22, R23.
11. [x] Next episode: on the item's end-credits segment (`internal/segments`, ADR 0008) or its last 30 s, `party.next` with ready count and a 10 s start; host stop. Tests. R30.
12. [x] `party/vote.go` + routes: proposals (2 each, items every account invitee can see), ballots (top 3 + veto, editable until the deadline), anonymity (handlers never serialize ballots; test that no response holds another voter's ranking), tally, seed from `crypto/rand` stored before the draw, deterministic draw from it; host picks when nothing is left; suggestions R34. R31–R34.
13. [x] Scheduler: reminders at −15 min (`notify` to yes/maybe), lobby, start at the time for ready screens, cancel at +3 h, vote deadlines, missing-file warning for future-episode parties. Restart test: ticks missed while down run once. R29, R33, R38, R41.
14. [x] Upcoming: `show_follows` routes, daily TMDB fetch for followed shows (recorded fixtures, `scripts/record-metadata-fixtures.sh` extended), `GET /upcoming`, alerts fired from the scan hook. Tests. R35–R38.
15. [x] `api/events.md`: every message with an example; a test parses the file and replays each client example against the hub, checking the server examples' shape. `slog` events without chat, codes or tokens (R45). R44.

**Web**
16. [x] `party/socket.ts`, `clock.ts`, `sync.ts`: reconnect with backoff (1 → 30 s), snapshot on reconnect, offset estimate, R10 on the engine (`playbackRate` 0.95/1.05, seek past 3 s). Fake-timer tests; check Safari's native HLS honours `playbackRate`.
17. [x] Player party mode + `PartyPlayer` (Party13): side panel with people, chat, reactions, "Ask for a pause", invite with code; `SyncToasts` (Party15); host sheets (17–19); end summary (16); loading shimmers. Tests per component.
18. [x] Launch sheet from the item page (Party04, 05, the web version of the iPhone sheet), invite (06), join with a code (07), scheduled party (08), lobby with countdown (09).
19. [x] Guest page `/p/:code` (Party14) with the guests-off and unknown-code states, full party queue (25).
20. [x] Upcoming tab (PartiesWeb: list, calendar, invitation card) with the invitations badge, "Follow" on show pages, home row (Party21), vote screens (VoteBig web: propose, rank with right-click veto, results, tie draw animation).
21. [x] Admin › Settings › Plaid Party (Party26, 27).
22. [x] Catalogs `en` and `fr`, `make check-i18n`; German-length check on the launch sheet and host sheets.

**Apple**
23. [x] Common: `PartySocket` (URLSessionWebSocketTask, same reconnect and clock rules), models, `PartySync` driving `AVPlayer.rate` and seeks; the REST calls.
24. [x] iOS: launch and schedule (Party04, 05), invite with the share sheet (06), join with a code (07), companion (01, 02), watching on iPhone and iPad with the party panel (10–12), host sheets (17–19), end (16), invitation (20), home row (21), Upcoming (PartiesPh), vote (VotePh), "Follow". Universal link `/p/:code`. Local reminder notifications (`UNUserNotificationCenter`) for parties answered yes or maybe.
25. [x] tvOS: start from the TV with the code (23), overlay on the player (03), reactions with the remote (24), next episode (22), winner reveal (VoteBig), `tv.cast` opens the player.
26. [x] String Catalogs `en` and `fr`.

**Wrap-up**
27. [ ] Roadmap and plan status, `make test`, evidence below.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Shared encodes break F006 (a session's seek moves the run under others, segment cleanup deletes what another reads) | high | Refcount, cleanup by the slowest reader, the 30 s leave rule, tests with the fake FFmpeg; feature-flagged to party sessions only |
| WebSockets cut by the HTTP server's timeouts, getplaid.direct or a user's reverse proxy | medium | Server pings every 25 s; check on the deploy box behind getplaid.direct; document proxy headers in `api/events.md` |
| Drift correction feels bad (rate changes audible, HLS seek lands on a keyframe a second away) | medium | 0.95/1.05 only, seek past 3 s, then correct the rest with rate; simulator plus a real 3-device check |
| Safari / AVPlayer ignore small rate changes on HLS | medium | Fallback: seek past 1 s instead of rate when the engine reports it can't change rate |
| Guest identity opens a hole in deny-by-default auth | medium | Route table test from `openapi.yaml` for guests, like `public_routes_test.go` |
| SQLite contention from the scheduler and answers | low | Writes only on actions, short transactions, scheduler batches per tick |
| PR size, conflicts with F008, F011, F014 work in the same player and views files | high | Commits per step above; rebase often; touch player files in their own commits |
| Restart ends live parties | accepted | Snapshot-free by design (spec); clients say so |

**Riskiest step:** 8, shared encodes and the transcode-limit flow, because it changes F006's one-run-per-session model under live playback.

## Rejected options
- **SSE + REST for party messages** (like scan events): two channels per client, no low-latency client → server path for reports and pings. The spec asks for WebSocket.
- **Room state in SQLite**: survives restarts but writes on every play, pause and report, against "stay on SQLite, short writes". Memory like F006 sessions.
- **Peer-to-peer sync between clients**: no single clock, breaks with guests behind NAT.
- **One transcode per party for everyone** (Plex-style group stream): kills each person's own decision (F006), Direct Play on the TV included.
- **Splitting server, web and Apple into several PRs**: CLAUDE.md forbids it; commits instead.

## Expected evidence
- [ ] Tests added: store, room table tests, sync simulator (p95 printed), hub, guest route table, shared encodes, vote tally and anonymity, scheduler catch-up, upcoming, `events.md` replay, web component and sync tests.
- [ ] `make test` green, output pasted.
- [ ] Throttled Pi container: a 20-person simulated party, CPU and memory numbers against the NFR.
- [ ] Screenshots next to each board: web Party13–15, 25, 26, 27, PartiesWeb, VoteBig; iPhone Party01, 04–08, 10, 11, 16–21, PartiesPh, VotePh; iPad Party02, 09, 12; Apple TV Party03, 22–24 (simulator only).
- [ ] Manual check: Apple TV simulator + iPhone simulator + two browsers in one party, play / pause / seek / buffer / host leaves / guest join / next episode.

## Log
- 2026-11-03, proposals review: the web proposals step follows the VotePropose board (header with host and deadline, two slots, live library search with filters, Plaid's suggestions as posters, the group's proposals with their proposer, then "everyone ranks"). `PartyItem` carries a `posterId`, and proposals are a `PartyProposal` with the proposer as a `PartyPerson`. The board's list and rating reasons ("In 3 lists", "Rated 4.5 by Sam") and the "In your lists" filter are left out: PLAID has no lists or user ratings yet.
- 2026-11-03, calendar review: the Upcoming page is lit by a backdrop like the PartiesWeb board (the next party's title, else the next release's show), so `PartyItem` and `UpcomingEpisode` carry a `backdropId`. The toolbar is the board's glass pills with List/Calendar icons; "Join a party" left the toolbar (the board has none; codes still open from `/p/{code}` links). The side panel shows the title's picture and "Plan a party for the release", which schedules a party on a future episode (`showId` + numbers, R38) from the web too.
- 2026-11-03, after review: the web follows two boards added to the canvas. VoteModal: "New party" in Upcoming opens the vote modal (6 days from the day after tomorrow, a time, the deadlines on a timeline with a pencil each, invitees as avatars, remote, reminder switch; alone on the server, an account invitation link for admins). The reminder switch needs `reminder` on `PartyCreate`/`Party` and the `no_reminder` column (migration 0018); with it off, the lobby still opens 15 min before but nobody is notified. PartiesWeb "Vue calendrier": the month title with arrows, a legend, six weeks with episode and party chips (hosting, going, invitation, live) and the selected day on the side; it shows even when empty. Party buttons use the V2 cream primary.
- 2026-11-03, steps 23–26: iOS has its own `PartyPlayerScreen` and tvOS a custom `AVPlayerLayer` player (to catch the long press on play). "Start now" on a scheduled party moves `scheduledAt` to a minute ahead (no REST route to start early). Vote deadlines are computed (ranking closes 3 h before, proposals halfway). Ranking is tap-in-order with a veto per row. iOS gets its first entitlements file (associated domains for `/p/` links); the `apple-app-site-association` file on getplaid.direct isn't part of this.
- 2026-11-03, steps 16–22: a party lets its people read its title (`GET /items/{itemId}`, guests included): the player needs the files and tracks. The admin's existing `party.*` i18n keys moved under `party.settings.*` (they clashed with the room's keys). Dynamic keys from server values are kept by `preservePatterns` in `i18next.config.ts`.
- 2026-11-03, step 9: the public code routes are `/party-codes/{code}` and `/party-codes/{code}/guest`, not `/parties/code/…`: Go's mux rejects `/parties/code/{code}` next to `/parties/{partyId}/vote`.
- 2026-11-03, step 14: release dates come from the existing `Show` and `Season` provider calls (latest two seasons) instead of new TMDB endpoints, so no new recordings; tests use a fake provider. Alerts are keyed by show, season and episode (`/shows/{showId}/alerts/{season}/{episode}`), matching how a future-episode party is stored.
- 2026-11-03, step 8: R27 caps each new remote party screen to what's left of 80% of the upload (floor 1.5 Mb/s) when it starts; the host isn't told why yet.
- 2026-11-02, step 6: the simulator is a deterministic discrete-time model in `party/sync_test.go` driving the room directly, not simulated clients over the real hub: the hub has its own tests (step 4), and a deterministic model gives stable p95 numbers in `make test`.
