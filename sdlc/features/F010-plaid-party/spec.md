---
id: F010
title: Plaid Party — watch together
owner: gh-prim
reviewer: gh-prim
status: approved
intent: ./intent.md
decisions: [0009, 0010]
---

# F010 — Spec: Plaid Party, watch together

## Summary
A Plaid Party is a room on one movie or episode: a host, up to 8 people by default (2–20, admin setting), each watching on their own device with their own stream (F006), kept in sync by the server over one WebSocket. People join by a 6-character code or a link; with the admin's consent a guest joins from the link with a first name only and sees that one title. Inside, people chat, send reactions and ask for a pause; the host decides who controls playback, can hand the host role over, remove someone or end the party. Parties start now or are **scheduled** with invitations (yes / maybe / no), a reminder 15 minutes before and a waiting room with a countdown; the group can **vote** on the movie first (2 proposals each, an anonymous top 3, one veto). Shows move to the next episode together. A new **Upcoming** tab lists the releases of the shows you follow and your parties, as a list or a calendar. Everything runs on web, iPhone, iPad and Apple TV; on a TV, the chat and controls live on the phone or iPad of the same account. Screens come from the Claude Design canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz), page V2, step 23 (boards Party01–Party27), the Upcoming boards (PartiesWeb, PartiesPh) and the vote boards (VotePh, VoteBig).

## User journeys

**Start now, watch on the TV (Party04, 06, 01–03).**
1. Alex opens Nosferatu on the iPhone → "Plaid Party". The sheet: Now / Schedule, where you watch (Apple TV · Salon, which is online; or this iPhone), who controls (only you / everyone) → "Start the party".
2. The Apple TV opens the player on its own and joins as Alex's screen; the iPhone becomes the companion: code `K7P-4QX`, "Copy", "Share the link", "Go to the party".
3. On the iPhone: the TV's progress, play/pause and ±10 s, who's here ("4 in the party · Sam is loading"), the chat, reactions, the message box. On the TV: avatars top right (a loading one dimmed), reactions floating up from the left, the last message bottom left, a pause request at the top ("Mia asks for a pause · answer on your iPhone").

**Join (Party20, 21, 07, 14, 25).**
1. Mia was invited: the invitation, the home row ("Alex and 2 others · join"), the Upcoming tab and the notification all have "Join". One tap and she's in, with her own stream (iPad · 1080p). No code to type.
2. Hugo wasn't invited, Alex sends him the code at the last minute: he types it in the app ("Join a party"). The code and the link are for people outside the invitations.
3. Camille, no account, opens the link in a browser: "Alex invites you to a Plaid Party · Nosferatu · Started 38 min ago · 3 people watching", a first name, "Join", "Your first name is enough, you only see this movie." Or "Have an account? Sign in".
4. When the party is full: "The party is full. 8 people are already watching, the most on this server. Alex knows you're waiting." She's queued and gets in as soon as a seat frees up.

**Sync (Party15).** Sam's stream stalls: everyone pauses, "Waiting for Sam. His movie is loading. Everyone resumes together, at 38:12", and the host can "Resume without Sam". Léo is 1.2 s behind: "You were 1.2 s behind. We caught you up gently." Mia pauses: "Mia paused · 2 s ago". Léo goes back to 37:40: "Everyone followed."

**Pause request.** In "host only" mode, Mia taps "Pause?": the host sees "Mia asks for a pause · You're the host, it's your call" → "Pause now" / "In 5 min".

**Host (Party17, 18, 19).** The host sees everyone (place, device, quality, loading), switches control, gives one person control, hands the host role over or removes someone. Leaving, the host picks who takes over or "End for everyone". When the server can't convert for everyone: "The server is at its limit. Sam and Léo need a converted version, but your server can only prepare one at a time." → "The same 720p version for both" / "Sam first, Léo waits" / "Cancel".

**Schedule, invite, wait (Party05, 08, 09, 20, 21).** Alex schedules Saturday 21:00, invites Mia, Sam, Léo, and a link; "Remind everyone 15 min before". Invitees get the invitation (app notification, Upcoming tab, home row), answer "I'll be there" / "Maybe" / "No". The scheduled party lists answers, "Start now". From 15 minutes before, a waiting room counts down with who is ready; at zero the movie starts for those ready ("Everyone's here · Enjoy the show"), "Start without waiting" for the host.

**Vote (VotePh, VoteBig).** On a scheduled party with no movie yet, the host opens a vote: until Thursday 18:00 everyone proposes up to 2 films (search, plus suggestions "Nobody has seen it"), until Friday 18:00 everyone ranks a top 3 (3, 2, 1 points, anonymous) and may veto one film. Results show the points, the vetoed films and who voted (never for what). A tie is drawn by lot, live for everyone. The winner becomes the party's movie.

**Next episode (Party22).** At an episode's end credits: "Plaid Party · next episode together · S1 E3 · Llamigos · 3 of 4 ready · Starting in 8 s" / "Let's stop here".

**End (Party16).** "End of the party · Nosferatu, tonight with Alex, Mia, Sam, Léo · 1 h 34 together · 46 reactions · 1 pause asked · Most reactions at 1:12:40 😱" → "Schedule the next one" / "Watch the end again" / "Leave".

**Start from the TV (Party23, 24).** On the Apple TV, a title → "Plaid Party": "Your party is open. The chat is on your iPhone: open PLAID, it offers to join. Your friends enter this code." Long press on play opens the reactions; swipe to choose, click to send.

**Upcoming (PartiesWeb, PartiesPh).** A tab with All / Episodes / Parties and List / Calendar: invitations to answer on top, then day by day: released episodes ("Released · file pending" / "Notify me" / "Plan a party"), upcoming ones, live parties ("Join"), scheduled ones ("You're hosting").

**Admin (Party26, 27).** Settings › Plaid Party: on/off, people per party (2–20, 8 by default, with what it means for the upload), guests without an account, the upload speed.

## Functional requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| **Rooms** | | |
| F010-R1 | A party has an id, a code, a host (account + profile, or none while scheduled by an organizer who isn't connected), an item (movie or episode; empty while a vote is open), a control mode (`host`, `everyone`), a status (`scheduled`, `voting`, `lobby`, `live`, `ended`, `cancelled`), an optional scheduled time and note, and is created by someone whose F009 `party` right is `start` (admins always). Others get `403 party_not_allowed`. | must |
| F010-R2 | Code: 6 characters from F003-R20's alphabet (no `0 O 1 I L`), shown `XXX-XXX`, unique among parties not ended, valid until the party ends. Link: `<public base>/p/<code>` where the base is the getplaid.direct address when remote access is on (ADR 0009), else the address the host is using. | must |
| F010-R3 | Joining needs the `join` or `start` right (members), or a guest join (R5). **Invitees join by the party's id, never by code**: from the invitation, the notification, the home row, Upcoming or a party page, one action. The host can invite more people during a live party ("Invite" picks among the server's people and adds them as invitees, who get a notification with "Join"). The code and the link are for last-minute joiners outside the invitations. Joining by code is rate-limited per IP and per account (10 wrong codes in 10 min → 15 min backoff, like F009-R13). | must |
| F010-R4 | People per party, host included: admin setting `maxPeople` 2–20, 8 by default. A participant is a person (account + profile, or guest), whatever their number of devices. Joining a full party puts the person in a queue (FIFO); they get in when a seat frees up, and the host is told someone is waiting. A host who is admin can raise the limit from that prompt. | must |
| F010-R5 | Guests: when the admin allows it (`guests`, off by default), someone without an account joins from the link with a first name (1–32 chars). They get a guest token scoped to that party only: its item's playback, its room, its artwork. Every other route answers `401`. No watch state is saved. The token dies with the party or when the guest is removed. | must |
| F010-R6 | Access to the title: being in a party gives access to that party's item, even outside the person's libraries (F009-R7), for the party's duration only. The profile's age limit (F006-R9) still applies: a profile that can't see the item can't join (`403 party_age_limit`, shown as "This one isn't for this profile"). | must |
| F010-R7 | A party ends when the host ends it, when the item ends without a next episode (R30) and everyone has left the end screen, or when nobody has been connected for 10 minutes. A scheduled party nobody started is cancelled 3 hours after its time. | must |
| **Sync** | | |
| F010-R8 | The server holds the reference playback state: `{state: playing|paused|waiting, positionMs, atServerMs, rate: 1}`. Every change is broadcast; clients compute the target position as `positionMs + (serverNow − atServerMs)` when playing. | must |
| F010-R9 | Clock: each client estimates its offset from the server with `sync.ping` / `sync.pong` (5 pings on join, then one every 30 s, keeping the round trip with the lowest latency). | must |
| F010-R10 | Drift correction: below 150 ms nothing; from 150 ms to 3 s the client plays at 0.95 or 1.05 until it's back within 50 ms ("You were 1.2 s behind. We caught you up gently." past 1 s); beyond 3 s it seeks. | must |
| F010-R11 | Waiting: a participant whose screen buffers for more than 1.5 s puts the room in `waiting` at the current position ("Waiting for Sam"); it resumes for everyone when every screen reports ready. After 20 s the host (or anyone in `everyone` mode) can "Resume without Sam"; Sam catches up by seeking when ready. | must |
| F010-R12 | Each participant's screen has its own F006 playback session (F006-R38): its own profile, capability profile and decision, started at the room's position. Quality changes stay personal. | must |
| F010-R13 | Seek, play, pause from someone who has control are applied to the room and shown to others ("Mia paused", "Léo went back to 37:40 · Everyone followed"). | must |
| **Control** | | |
| F010-R14 | Control modes: `host` (only the host and the people the host gives control to) or `everyone`. Set at creation, changed by the host at any time. | must |
| F010-R15 | Pause request: anyone without control asks for a pause (at most once a minute per person); the host gets "Pause now" / "In 5 min"; "In 5 min" shows a countdown to everyone and pauses then. With no host connected, a request pauses at once. | must |
| F010-R16 | The host can hand the host role to a participant with an account, give or take control from one person, and remove someone (they can't rejoin that party; a removed guest's token dies). | must |
| F010-R17 | Host leaving: the app asks "Hand over to <first>" / "Pick someone else" / "End for everyone". A host who disconnects without choosing keeps the role for 2 minutes, then it goes to the participant with an account who joined first; with nobody left, the party waits (R7). | must |
| **Social** | | |
| F010-R18 | Chat: text messages (1–500 chars, 1 per second, burst of 5), with the position they were sent at ("Mia · 38:02"). System lines for joins, leaves and quality ("Léo joined from the iPhone · 720p"). Messages live in memory only; someone joining gets the last 200; nothing is stored after the party. | must |
| F010-R19 | Reactions: a fixed set of six (😂 😱 ❤️ 👏 🦇 🍿), at most 3 per second per person, broadcast with who sent them and counted per minute of the item for the end summary. | must |
| F010-R20 | Presence: per participant, name, avatar (F009), role, where they watch (device type and quality; the place line of the boards is a watch-out), state (`here`, `loading`, `waiting`, `away`), and whether they are a guest. | must |
| F010-R21 | End summary: who was there, time together, number of reactions, pause requests, the minute with the most reactions and its top emoji. "Schedule the next one" prefills a new party with the same people. Kept with the party for 30 days. | should |
| **Devices** | | |
| F010-R22 | Where you watch: a person picks their screen when starting or joining: this device, or one of their account's Apple TVs that is online (its app open and connected). The server sends that TV `tv.cast`; it opens the player and joins as the person's screen; the starting device becomes a companion (chat, reactions, controls, presence). | must |
| F010-R23 | A party started on the Apple TV offers the same account's iPhone or iPad, when its app is open, to join as companion (`party.companion`). | must |
| F010-R24 | The Apple TV shows the room over the picture (avatars, reactions, last message, pause request, waiting, next episode) and sends reactions with the remote (long press on play opens the six; swipe to choose, click to send). Chat is read-only on the TV. | must |
| **Transcode budget** | | |
| F010-R25 | Shared encode: several F006 sessions of the same file, audio track, burned-in subtitle and quality share one FFmpeg run and its segments, counting once toward `maxTranscodes` (F006-R26). Each keeps its own token. Available to party sessions only. | must |
| F010-R26 | When a participant's screen needs a transcode and none is free (`503 transcode_limit_reached`), the room tells the host who is waiting and offers: one shared quality for everyone who needs a transcode (the highest quality all of them can take, e.g. 720p), or an order (one first, the others wait for a free transcode), or cancel (they stay without picture). The others keep watching. | must |
| F010-R27 | Upload budget: the admin can give the upload speed (Mbps). Remote screens are then capped so their sum fits 80% of it; when they can't all have their default quality, the later ones get a lower one and the host sees why. | should |
| **Scheduling** | | |
| F010-R28 | Scheduling: a date and time (15 min to 60 days ahead), a note (≤ 140 chars), invitees among the server's people who have the `join` or `start` right, plus the link. Each invitee answers `yes`, `maybe`, `no` and can change until the start. The organizer can move, cancel or start early. | must |
| F010-R29 | Reminders: 15 minutes before, every invitee who answered yes or maybe gets a reminder (R41) and the waiting room opens: countdown, who is ready (their screen loaded the start), "Invite more", "Start without waiting" for the host. At the time, the movie starts for everyone ready; the others join in sync when they arrive. | must |
| **Shows** | | |
| F010-R30 | Next episode together: on an episode's end credits (ADR 0008, or its last 30 s), the room shows the next episode, how many are ready (their next session started), and starts it 10 s later; the host can "Let's stop here". Each screen opens a new F006 session on the next item. | must |
| **Vote** | | |
| F010-R31 | A scheduled party can open a vote instead of a movie: a proposal deadline and a ranking deadline (both before the party's time). Invitees with an account propose up to 2 items each among the titles every invitee with an account can see (R6 does not apply to proposals); one proposal per item. | must |
| F010-R32 | Ranking: each voter ranks a top 3 (3, 2, 1 points) and may veto one proposal. A vetoed item is out. Ballots are anonymous: the API shows who voted, never what anyone ranked or vetoed (admins included). A voter can change their ballot until the deadline. | must |
| F010-R33 | Result at the ranking deadline: most points wins; a tie is drawn by lot from a seed stored with the vote and shown to everyone, so the draw can be checked again. If every proposal is vetoed or nobody voted, the host picks. The winner becomes the party's item and invitees are told. | must |
| F010-R34 | Suggestions while proposing: titles none of the invitees has watched (from their watch state, shown only as "Nobody has seen it"). | could |
| **Upcoming** | | |
| F010-R35 | Followed shows: a show is followed by a profile when it has started it and not finished it, or when it marked it "Follow" from the show page. | must |
| F010-R36 | Release data: for followed shows the server fetches TMDB's next and recent episodes (`/tv/{id}` `next_episode_to_air`, `last_episode_to_air`, and the current season) once a day, keeping episodes from 7 days back to 60 days ahead with their air date. | must |
| F010-R37 | `GET /upcoming` lists, for the active profile and a date range: invitations to answer, episodes of followed shows (`released_on_server`, `released_pending_file`, `upcoming`), and parties (live, scheduled, hosting, invited, with the answer). Filter by kind. | must |
| F010-R38 | "Notify me" on an episode: the profile is told when the file lands on the server (F004 scan) or, if it is already released, nothing changes. "Plan a party" on an episode schedules a party on it, even before its file exists; if the file still isn't there 15 minutes before, the host is told. | should |
| F010-R39 | Web, iPhone and iPad show the Upcoming tab as a list and a calendar (month), with the invitations on top and a badge with the number of invitations to answer. | must |
| F010-R40 | Home: a "Plaid Parties" row with live parties the person can join and their next scheduled ones, only when there's at least one. | must |
| **Notifications** | | |
| F010-R41 | In-app notifications over the WebSocket for an invitation, a reminder, a party starting, a vote opening or closing, and an episode now on the server. iOS also schedules a local notification for each reminder it knows about. Remote push while the app is closed follows ADR 0010. | must (in-app), should (push) |
| **Admin and protocol** | | |
| F010-R42 | Settings › Plaid Party: `enabled`, `maxPeople` (2–20), `guests`, `uploadMbps` (optional). Turning parties off ends live ones with a message. Who can start or join stays each group's `party` right (F009). | must |
| F010-R43 | Admin: `GET /parties` (admin) lists live parties with their people and streams; the admin can end one. Each party's streams show in `GET /playback` with their party id. | should |
| F010-R44 | The WebSocket protocol is documented in `api/events.md` (connection, auth, every message, errors, versions) and REST routes in `openapi.yaml`, so Android can follow without reading the server. | must |
| F010-R45 | Every party event (created, started, joined, left, removed, host changed, ended, vote closed) is logged with `slog`, never with chat text, codes or tokens. | must |

## Design

### Data model
New migration `00NN_plaid_party.sql`:

- `parties` (id TEXT PK, code TEXT, status TEXT, item_id TEXT NULL → items, organizer_user_id, organizer_profile_id, host_user_id NULL, host_profile_id NULL, control_mode TEXT, scheduled_at INTEGER NULL, note TEXT NULL, link_enabled INTEGER, created_at, started_at NULL, ended_at NULL, summary_json TEXT NULL). Unique index on `code` where status not in (`ended`, `cancelled`).
- `party_invites` (party_id, user_id, answer TEXT `pending|yes|maybe|no`, answered_at, reminded_at). PK (party_id, user_id).
- `party_removed` (party_id, user_id NULL, guest_id NULL): who can't come back.
- `party_guests` (id, party_id, first_name, avatar_color, token_hash, created_at, removed_at NULL).
- `party_votes` (party_id PK, proposals_until, ranking_until, seed TEXT, closed_at NULL, winner_item_id NULL), `party_proposals` (party_id, item_id, user_id, created_at; unique (party_id, item_id)), `party_ballots` (party_id, user_id, first_item, second_item, third_item, veto_item NULL, updated_at). Ballot rows are read only by the tally and by their own voter.
- `show_follows` (profile_id, show_id, created_at) for the explicit "Follow".
- `upcoming_episodes` (show_id, season, episode, tmdb_id, title, air_date TEXT, fetched_at). `release_alerts` (profile_id, show_id, season, episode, created_at, notified_at NULL).
- `settings`: `party.maxPeople` (now 2–20), `party.uploadMbps`.

The room itself (participants, devices, playback state, chat, reactions, queue, pending pause requests) lives in memory in `internal/party`; a restart ends live parties (clients see "The server restarted, the party has ended"; scheduled ones are untouched).

### API
REST, carried into `api/openapi.yaml`:

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/parties` | `{itemId?, controlMode, scheduledAt?, note?, invite: [userId], vote?: {proposalsUntil, rankingUntil}, screen?: {sessionId}}` → `201 Party` (with `code`, `link`) |
| GET | `/parties` | Mine: live I can join, scheduled I host or am invited to. Admin: `?all=1` |
| GET | `/parties/{id}` | Detail: item, host, invites and answers, status, vote state |
| PATCH | `/parties/{id}` | Organizer/host: `scheduledAt`, `note`, `itemId`, `controlMode` |
| DELETE | `/parties/{id}` | Organizer: cancel; admin: end |
| POST | `/parties/{id}/invites` | `{userIds}` |
| PUT | `/parties/{id}/answer` | `{answer: yes|maybe|no}` |
| GET | `/parties/people` | People I can invite: id, display name, avatar. No usernames |
| GET | `/party-codes/{code}` | Public, rate-limited: host first name, item title and poster, started ago, people watching, full or not, guests allowed. `404` otherwise |
| POST | `/party-codes/{code}/guest` | Public, rate-limited: `{firstName}` → guest token (web: `HttpOnly` cookie `plaid_guest`) |
| GET | `/parties/{id}/vote` | Proposals, who voted, my ballot, results once closed |
| POST / DELETE | `/parties/{id}/vote/proposals/{itemId}` | Propose / withdraw |
| PUT | `/parties/{id}/vote/ballot` | `{ranking: [id, id, id], veto?: id}` |
| GET | `/parties/{id}/vote/suggestions` | R34 |
| GET | `/upcoming` | `?from=&to=&kind=all|episodes|parties` |
| PUT / DELETE | `/shows/{id}/follow` | Follow a show |
| PUT / DELETE | `/upcoming/alerts/{episodeTmdbId}` | Notify me |
| GET / PATCH | `/settings/party` | Adds `uploadMbps`; `maxPeople` max 20 |
| POST | `/playback` | Gains `partyId`: the server sets the start position from the room |

WebSocket, documented in `api/events.md`: one connection per client, `GET /api/v1/ws` (cookie with same-origin `Origin` check, bearer, or guest token). Messages follow its conventions (`{type, data, ts}`; client messages carry an `id` the server echoes in `ack` / `error`).

- Clock: `sync.ping {t0}` → `sync.pong {t0, ts}`.
- Client → server: `party.join {partyId|code, screen: "this"|tvSessionId|"companion"}`, `party.leave`, `party.report {state: playing|paused|buffering|ready|ended, positionMs, playbackSessionId}`, `party.control {action: play|pause|seek, positionMs?}`, `party.pause.request`, `party.pause.answer {requestId, answer: now|in5}`, `party.react {emoji}`, `party.chat {text}`, `party.host.set {to}`, `party.control.grant {to, on}`, `party.mode.set {mode}`, `party.remove {who}`, `party.end`, `party.resume.without {who}`, `party.transcode.choose {choice: shared|order, first?}`, `party.next.stop`, `party.queue.leave`.
- Server → client: `party.snapshot` (everything on join), `party.playback`, `party.presence`, `party.waiting {for, positionMs}`, `party.pause.requested`, `party.pause.scheduled {atServerMs}`, `party.reaction`, `party.chat`, `party.host.changed`, `party.transcode.limit {waiting, options}`, `party.next {item, startsAt, ready, of}`, `party.queued {position}`, `party.removed`, `party.ended {summary}`, `tv.cast {partyId}`, `party.companion {partyId}`, `notify {kind, …}`.
- Errors: `not_allowed`, `not_in_party`, `no_control`, `rate_limited`, `party_full`, `party_ended`, `bad_message`.

### Behavior per app
- **server**: `internal/party` (rooms, clock, sync rules, queue, budget), `internal/realtime` (WebSocket hub, auth, per-connection rate limits), F006 changes (party sessions, shared encodes R25, guest-owned sessions), daily TMDB release fetch in `internal/metadata`, scheduler for reminders, auto-start and vote deadlines (one goroutine, minute ticks, catches up after a restart).
- **web**: launch sheet from title pages, the party player (video, side panel with people and chat, reactions, sync toasts), guest join page `/p/:code`, full-party queue, Upcoming tab (list, calendar, invitation), home row, vote screens, admin settings (Party26–27).
- **apple (iOS/iPadOS)**: launch, schedule, invite (share sheet), join by code and by universal link, companion screen (TV playing), watching on iPhone (portrait, landscape with chat) and iPad (split), host sheets, invitation, home row, Upcoming, vote, local reminders.
- **apple (tvOS)**: start a party from a title (code, waiting for companions), the overlay over the player, remote reactions, next episode, vote winner reveal, `tv.cast`.
- **android**: out of scope (follows the documented protocol later).
- **getplaid.tv** (board TW03, the website's Plaid Party page): out of scope, it ships with the site.

### UI / Art direction
The V2 language: liquid glass panels over the blurred backdrop, cream for the single primary action, Bricolage for titles, Instrument Sans for text, the people's `party-*` avatar colors. Board per screen:

| Board | Screen |
|-------|--------|
| Party01, 02 | Companion on iPhone / iPad while the TV plays |
| Party03, 24 | Apple TV overlay, remote reactions |
| Party04, 05 | Launch sheet (Now / Schedule) |
| Party06, 07 | Invite (code, link), join with a code |
| Party08, 09 | Scheduled party, waiting room countdown |
| Party10, 11, 12 | Watching on iPhone (portrait, landscape), iPad |
| Party13, 14, 15, 25 | Web party, guest join, sync states, full party |
| Party16–19 | End, host manages people, host leaves, transcode limit |
| Party20, 21 | Invitation received, home row |
| Party22, 23 | Apple TV next episode, start from the TV |
| Party26, 27 | Admin settings |
| PartiesWeb, PartiesPh | Upcoming tab |
| VotePh, VoteBig | Vote |

States: loading (shimmer on panels and avatars, never a frozen blank), connection lost ("Reconnecting…", then the snapshot catches up), party ended, removed, code unknown, guests not allowed, parties off. Motion: one per surface (countdown, vote reveal), off with reduced motion.

### i18n
New namespaces `party.*`, `upcoming.*`, `vote.*` on web (JSON catalogs), iOS/tvOS (String Catalogs), server (`notify` titles in the person's locale). Main keys (en / fr):

| Key | en | fr |
|-----|----|----|
| `party.title` | Plaid Party | Plaid Party |
| `party.start.now` / `.schedule` | Now / Schedule | Maintenant / Programmer |
| `party.start.whereTitle` | Where you watch | Où tu regardes |
| `party.start.tvHint` | The movie on the TV, the chat here | Le film sur la TV, la discussion ici |
| `party.start.thisDevice` | The movie and the chat here | Le film et la discussion ici |
| `party.start.controlTitle` | Who controls playback | Qui contrôle la lecture |
| `party.control.host` / `.everyone` | Only you / Everyone | Toi seul / Tout le monde |
| `party.start.controlHint` | The others can always ask for a pause. | Les autres peuvent toujours demander une pause. |
| `party.start.cta` | Start the party | Lancer la party |
| `party.invite.ready` | Your party is ready. | Ta party est prête. |
| `party.invite.hint` | Send the code or the link. Your friends join from the app or a browser. | Envoie le code ou le lien. Tes amis rejoignent depuis l’app ou un navigateur. |
| `party.invite.copy` / `.share` / `.valid` | Copy / Share the link / Valid for the whole party | Copier / Partager le lien / Valable pendant toute la party |
| `party.join.title` / `.hint` | Join a party / The code your friend sent you. | Rejoindre une party / Le code que ton ami t’a envoyé. |
| `party.count` (plural) | {count} in the party | {count} dans la party |
| `party.presence.loading` | {name} is loading | {name} charge |
| `party.pause.ask` | Pause? | Pause ? |
| `party.pause.requested` | {name} asks for a pause | {name} demande une pause |
| `party.pause.hostHint` | You're the host, it's your call. | Tu es l’hôte, c’est toi qui décides. |
| `party.pause.now` / `.in5` | Pause now / In 5 min | Mettre en pause / Dans 5 min |
| `party.pause.answerOnPhone` | answer on your iPhone | réponds sur ton iPhone |
| `party.sync.waiting` | Waiting for {name} | On attend {name} |
| `party.sync.waitingHint` | Their movie is loading. Everyone resumes together, at {position}. | Son film charge. Tout le monde reprend ensemble, à {position}. |
| `party.sync.resumeWithout` | Resume without {name} | Reprendre sans {name} |
| `party.sync.caughtUp` | You were {seconds} s behind. We caught you up gently. | Tu étais {seconds} s en retard. On t’a recalé en douceur. |
| `party.sync.paused` | {name} paused | {name} a mis en pause |
| `party.sync.seeked` | {name} went back to {position} · Everyone followed | {name} est revenu à {position} · Tout le monde a suivi |
| `party.chat.placeholder` | Message | Message |
| `party.chat.joined` | {name} joined from {device} · {quality} | {name} a rejoint depuis {device} · {quality} |
| `party.guest.invite` | {host} invites you to a Plaid Party | {host} t’invite à une Plaid Party |
| `party.guest.started` | Started {ago} · {count} watching | Ça a commencé il y a {ago} · {count} personnes regardent |
| `party.guest.hint` | Your first name is enough, you only see this movie. | Ton prénom suffit, tu ne vois que ce film. |
| `party.guest.signIn` | Have an account? Sign in | Tu as un compte ? Se connecter |
| `party.full.title` | The party is full. | La party est complète. |
| `party.full.hint` | {count} people are already watching, the most on this server. {host} knows you're waiting. | {count} personnes regardent déjà, c’est le maximum sur ce serveur. {host} a été prévenu que tu attends. |
| `party.full.queue` / `.leave` | You get in as soon as a seat frees up / Leave the queue | Tu entres dès qu’une place se libère / Quitter la file |
| `party.host.leaveTitle` | Leaving the party? | Tu quittes la party ? |
| `party.host.leaveHint` | You're the host. Pick who takes over, or end the party for everyone. | Tu es l’hôte. Choisis qui prend la suite, ou arrête la party pour tout le monde. |
| `party.host.handTo` / `.pickOther` / `.endAll` | Hand over to {name} / Pick someone else / End for everyone | Passer l’hôte à {name} / Choisir quelqu’un d’autre / Terminer pour tout le monde |
| `party.host.giveControl` / `.remove` | Give them control / Remove from the party | Lui donner le contrôle / Retirer de la party |
| `party.limit.title` | The server is at its limit | Le serveur est au maximum |
| `party.limit.hint` | {names} need a converted version, but your server can only prepare {count} at a time. | {names} ont besoin d’une version convertie, mais ton serveur ne peut en préparer qu’une à la fois. |
| `party.limit.shared` / `.order` | The same {quality} version for both / {name} first, {other} waits | Une même version {quality} pour les deux / {name} d’abord, {other} attend |
| `party.schedule.cta` | Schedule for {date} at {time} | Programmer pour {date} à {time} |
| `party.schedule.reminder` | Remind everyone 15 min before | Rappel à tout le monde 15 min avant |
| `party.answer.yes` / `.maybe` / `.no` | I'll be there / Maybe / No | Je serai là / Peut-être / Non |
| `party.lobby.before` | before it starts, at {time} | avant le début, à {time} |
| `party.lobby.startNow` / `.allHere` / `.enjoy` | Start without waiting / Everyone's here / Enjoy the show | Commencer sans attendre / Tout le monde est là / Bonne séance |
| `party.next.title` | Plaid Party · next episode together | Plaid Party · épisode suivant ensemble |
| `party.next.ready` | {ready} of {count} are ready | {ready} sur {count} sont prêts |
| `party.next.startsIn` / `.stop` | Starting in {seconds} s / Let's stop here | Lancer dans {seconds} s / On s’arrête là |
| `party.end.title` | End of the party | Fin de la party |
| `party.end.peak` | Most reactions at {position} | Le plus de réactions à {position} |
| `party.end.next` / `.rewatch` / `.leave` | Schedule the next one / Watch the end again / Leave | Programmer la prochaine / Revoir la fin / Quitter |
| `party.tv.open` | Your party is open. | Ta party est ouverte. |
| `party.tv.openHint` | The chat is on your iPhone: open PLAID, it offers to join. Your friends enter this code. | La discussion se passe sur ton iPhone : ouvre PLAID, il te propose de rejoindre. Tes amis entrent ce code. |
| `party.tv.reactHint` | Swipe to choose, click to send · hold ▶︎ to open | Glisse pour choisir, appuie pour envoyer · reste appuyé sur ▶︎ pour ouvrir |
| `vote.open.title` / `.hint` | The group chooses, not you / Everyone proposes, everyone ranks. The most liked movie wins. | Le groupe choisit, pas toi / Tout le monde propose, tout le monde classe. Le film le plus apprécié gagne. |
| `vote.rank.hint` | Drag to rank. 1st = 3 points, 2nd = 2, 3rd = 1. Your vote is anonymous. | Glisse pour classer. 1er = 3 points, 2e = 2, 3e = 1. Ton vote est anonyme. |
| `vote.veto` / `.vetoed` | Your veto / Out by a veto | Ton veto / Éliminés par un veto |
| `vote.anonymous` | Anonymous votes: you see who voted, never for what. | Votes anonymes : on voit qui a voté, jamais pour quoi. |
| `vote.tie` | Tie at {points} points. Nobody decides. Chance does, live for everyone. | Égalité à {points} points. Personne ne tranche. Le hasard décide, en direct pour tout le monde. |
| `upcoming.title` / `.hint` | Upcoming / Your shows' releases and your Plaid Parties. | À venir / Les sorties de tes séries et tes Plaid Parties. |
| `upcoming.filter.all` / `.episodes` / `.parties` | All / Episodes / Parties | Tout / Épisodes / Parties |
| `upcoming.view.list` / `.calendar` | List / Calendar | Liste / Calendrier |
| `upcoming.episode.pending` | Released · file pending | Sorti · fichier en attente |
| `upcoming.notifyMe` / `.planParty` | Notify me / Plan a party | Me prévenir / Planifier une party |
| `upcoming.followHint` | Only the shows you follow appear: started, or followed from their page. | Seules les séries que tu suis apparaissent : commencées, ou suivies depuis leur page. |
| `settings.party.maxPeopleHint` | With {count} people, up to {remote} can be away from home. At {mbps} Mbps, only {fit} of them get 8 Mbps; the others share a lower quality. | (fr in catalog) |

Plurals (`party.count`, invitations badge, "{n} films proposés"), dates and times (scheduled time, calendar, "in 6 days"), durations (time together, countdowns) and positions (`38:12`) use the platform formatters. German check on the launch sheet, the host sheets and the TV overlay pills.

## Non-functional requirements
- **Sync**: p95 gap between two screens < 500 ms on a LAN and on 4G with < 100 ms RTT, measured by a test harness of simulated clients; a seek or pause reaches every screen in < 300 ms + RTT.
- **Server cost**: a live party with 20 people adds < 2% CPU on a Raspberry Pi 4 and < 10 MB of memory (excluding transcodes); messages are fanned out without blocking (per-connection send buffer, a slow client is dropped after 5 s of backlog and reconnects).
- **SQLite**: the room never writes during playback; writes happen at create, join (invites, guests), answer, vote and end; each in a short transaction (memory "Stay on SQLite").
- **Security**: codes and guest tokens are bearer secrets (rate limits R3; guest tokens 32 random bytes, only their SHA-256 stored); the WebSocket checks `Origin` for cookies; chat is plain text everywhere (no HTML, links not auto-opened on TV).
- **Privacy**: chat isn't stored; ballots aren't exposed; `/parties/people` shows display names and avatars only; a guest sees no other title, person or library.
- **Accessibility**: reactions and toasts are announced politely (`aria-live`), every control reachable by keyboard and by tvOS focus, countdowns not the only signal (text says what happens), reduced motion respected.
- **Reconnection**: a client that drops reconnects with backoff (1, 2, 4… 30 s) and resumes with a fresh snapshot without leaving the party for 2 minutes.

## ⚠️ Watch-outs

- [x] ✅ (as proposed, 2026-11-02) **ADR 0010, remote push (proposed).** Invitations, "your episode is here" and reminders on a closed iOS app need APNs, and a self-hosted server can't hold PLAID's APNs key. Proposed: relay through getplaid.direct (payload-free "something new, open the app" or end-to-end encrypted), opt-in like remote access. Until decided, v1 does in-app + local reminders only (R41).
- [x] ✅ (as proposed, 2026-11-02) **Shared encodes change F006** (R25): one FFmpeg run read by several sessions. Seeks must stay together, which a party guarantees, but "Resume without Sam" means Sam's later seek must not move the shared run under the others. Proposed: a session that drifts > 30 s from the shared run's position leaves it and gets its own (subject to the limit).
- [x] ✅ (kept as decided in the intent, 2026-11-02) **"Order" choice (R26)** means the second person has no picture until a transcode frees, possibly the whole movie. Keep it as decided, or replace it with "lower quality only for {name}"?
- [x] ✅ (as proposed, 2026-11-02) **Guest access (R5, R6)** is the first route into the server without an account: a public join page, a public lookup by code, playback for a non-account. Needs the deny-by-default middleware (F003-R12) to learn a third identity kind, and F006-R7 (sessions belong to an F003 session) to accept a guest owner.
- [x] ✅ (as proposed, 2026-11-02) **Party grants access to its item (R6)** beyond F009's libraries: a member without the "Kids" library can watch a Kids movie if invited. Fine for a party, but say so in the admin's sharing screen?
- [x] ✅ (as proposed, 2026-11-02) **Existing API vs intent**: `PartySettings.maxPeople` is 2–16 in `openapi.yaml` (step 21); intent and Party27 say 2–20 → changed to 20. Party26 shows "Who can start a party: Everyone", which isn't a setting today (F009 rights decide) → drop the row, or make it a shortcut that edits the groups?
- [x] ✅ (as proposed, 2026-11-02) **Upload speed (R27)**: admin-entered, or measured (F015 samples the upload)? Spec says entered, optional.
- [x] ✅ (as proposed, 2026-11-02) **Presence place line** ("Nantes", "Lyon", "Dans le train" on Party02, 12, 17): no source for it. Proposed: drop it, show device and quality only; or an optional status the person types.
- [x] ✅ (as proposed, 2026-11-02) **"Follow" and lists**: the design says "your list" (Upcoming hint) and "In 3 lists" (vote suggestions); there's no watchlist (backlog #5). Spec replaces them with an explicit "Follow" on show pages and drops "In 3 lists".
- [x] ✅ (as proposed, 2026-11-02) **Release times**: TMDB gives air dates, not times; the boards show "Releases at 09:00". Spec shows the date only.
- [x] ✅ (as proposed, 2026-11-02) **Vote suggestions (R34)** read other invitees' watch state, even aggregated ("Nobody has seen it"). Acceptable? It's `could`.
- [x] ✅ (as proposed, 2026-11-02) **Party on a future episode (R38)**: the item doesn't exist on the server until it's scanned; the party points at the show + season + episode and resolves the item when the file lands.
- [x] ✅ (as proposed, 2026-11-02) **One PR for the whole feature** (CLAUDE.md) across server, web, iOS and tvOS will be very large; the plan splits it into commits.
- [x] ✅ (as proposed, 2026-11-02) **WebSocket library**: none in `server/go.mod` yet (scan events use SSE). Plan picks one (e.g. `github.com/coder/websocket`), checked behind getplaid.direct and reverse proxies.
- [x] ✅ (as proposed, 2026-11-02) **Restart ends live parties** (rooms in memory, like F006 sessions). Acceptable for v1?
- [x] ✅ (as proposed, 2026-11-02) Website page TW03 is out of scope here.

## Acceptance criteria
- [ ] Given a member with `party: start`, when they `POST /parties` on a movie, then they get a code `XXX-XXX` and a link, and a member with `party: none` gets `403 party_not_allowed`.
- [ ] Given Mia is invited to a live party, when she taps "Join" on the invitation or the home row, then her app sends `party.join {partyId}` without any code and she's in.
- [ ] Given Hugo isn't invited, when he sends `party.join {partyId}`, then he gets `not_allowed`; with the code, he's in.
- [ ] Given a live party, when the host invites Léo, then Léo gets a `notify` with a "Join" action.
- [ ] Given a live party, when a member with `join` sends `party.join` with the code, then everyone gets a `party.presence` with them `loading`, then `here` once their screen reports `ready`.
- [ ] Given guests are off, when someone opens `/p/<code>`, then the page says the party needs an account and offers to sign in.
- [ ] Given guests are on, when Camille joins with a first name, then her token plays the party's item and gets `401` on `/items`, `/libraries` and another item's playback.
- [ ] Given a party of `maxPeople`, when one more person joins, then they get `party.queued {position: 1}` and the host is told; when someone leaves, the first in the queue gets in.
- [ ] Given two simulated clients with 80 ms RTT and a 40 ms clock skew, when the host plays, pauses and seeks 20 times, then the p95 gap between them stays under 500 ms.
- [ ] Given a client 1.2 s behind, when it applies R10, then it plays at 1.05 until within 50 ms and doesn't seek; at 4 s behind, it seeks.
- [ ] Given a screen reports `buffering` for 1.6 s, when the room is playing, then everyone gets `party.waiting {for: [Sam]}` and play resumes for all when Sam reports `ready`; after 20 s the host can resume without him.
- [ ] Given `host` mode, when a participant without control sends `party.control pause`, then they get `no_control`; `party.pause.request` reaches the host, and "In 5 min" pauses everyone 5 min later.
- [ ] Given the host sends `party.host.set {to: Mia}`, then everyone gets `party.host.changed` and Mia's controls appear; given the host disconnects for 2 min, the role goes to the first account participant.
- [ ] Given the host removes Léo, when Léo rejoins with the code, then he gets `not_allowed`.
- [ ] Given `maxTranscodes = 1` and two participants needing 720p of the same file and tracks, when the host picks "shared", then one FFmpeg run serves both and `GET /playback` counts one transcode.
- [ ] Given a scheduled party with three invitees, when the time is 15 min away, then those who answered yes or maybe get a reminder and the lobby opens; at the time, the item starts for the ready screens.
- [ ] Given an episode party, when the end credits start, then everyone gets `party.next` with the next episode and it starts 10 s later unless the host stops.
- [ ] Given a vote with proposals A, B, C, ballots giving A 7, B 7, C 4 and no veto, when the ranking deadline passes, then the tie is drawn from the stored seed, everyone sees the draw, and recomputing it from the seed gives the same winner.
- [ ] Given someone ranked, when anyone (admin included) reads the vote, then they see who voted and the totals, never one ballot.
- [ ] Given a profile following a show with an episode airing in 3 days, when it opens Upcoming, then the episode is listed on that day as upcoming, with "Notify me" and "Plan a party".
- [ ] Given "Notify me" on an aired episode with no file, when a scan adds it, then the profile gets a `notify` event.
- [ ] Given parties exist, when the home loads, then a "Plaid Parties" row shows live and next parties; with none, no row.
- [ ] Given an Apple TV of Alex's account is open, when Alex starts a party from the iPhone choosing that TV, then the TV gets `tv.cast`, plays, and the iPhone shows the companion.
- [ ] Given a party ends, when everyone leaves, then the summary is kept and no chat message is stored anywhere.
- [ ] Every message in `api/events.md` has an example, and the protocol tests replay those examples against the server.

## Revisions
_None yet._
