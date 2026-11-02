# PLAID WebSocket events

Real-time events exchanged between the server and clients over WebSocket.
REST endpoints live in [`openapi.yaml`](openapi.yaml); this file documents what OpenAPI can't.
It is the contract for every client (web, Apple, Android): `server/internal/handlers/events_doc_test.go` parses the
examples below and checks the server knows each message.

Today the socket carries Plaid Party (F010) and notifications.

## Conventions
- Messages are JSON objects: `{ "type": "namespace.event", "data": { … }, "ts": <server unix ms>, "id": "…" }`.
- `type` is a stable machine key, like `error.code` in the REST API.
- Server time (`ts`) is the reference clock; clients never trust their own clock for sync.
- A client message may carry an `id`; the server then answers `ack` (same `id`) when it's done, or `error`.
- Times are Unix milliseconds of the server clock; positions are milliseconds into the title.
- Unknown fields are ignored both ways, so either side can add some.

## Connection
`GET /api/v1/ws`, upgraded to WebSocket. Authentication is the same as the REST API:
- native apps: `Authorization: Bearer <session token>`;
- the web app: the `plaid_session` cookie; the server checks `Origin` (the server itself, or the app hosted on getplaid.direct);
- a party guest: `Authorization: Bearer g_…` or the `plaid_guest` cookie (F010-R5). A guest can only join their own party.

The server pings every 25 s. A client that can't keep up with its messages is disconnected; it reconnects with backoff
(1, 2, 4… up to 30 s) and joins again: `party.join` returns a fresh `party.snapshot`, and the person keeps their seat for
2 minutes. A client message is at most 8 KiB; 20 messages a second (burst 40) per connection.

Behind a reverse proxy, forward `Upgrade` and `Connection` headers and allow long-lived connections (e.g. nginx
`proxy_read_timeout 120s`).

## Replies

```json server
{ "type": "ack", "id": "c42", "ts": 1793660000000 }
```

```json server
{ "type": "error", "id": "c43", "ts": 1793660000000, "data": { "code": "no_control" } }
```

Codes: `bad_message`, `rate_limited`, `unknown_type`, `not_allowed`, `not_in_party`, `party_ended`, `party_not_started`,
`party_not_found`, `party_age_limit`, `profile_required`, `screen_offline`, `no_control`, `not_host`, `too_soon`,
`bad_reaction`, `bad_text`, `no_such_person`, `internal`.

## Clock (F010-R9)
Send 5 pings on connect, then one every 30 s. Keep the sample with the shortest round trip among the last 8:
`offset = ts − (sent + received) / 2`; server time ≈ local time + offset.

```json client
{ "type": "sync.ping", "id": "p1", "data": { "t0": 1793659999950 } }
```

```json server
{ "type": "sync.pong", "id": "p1", "ts": 1793660000000, "data": { "t0": 1793659999950 } }
```

## Plaid Party

### Joining
Invitees join by id, others by code (R3). A guest sends neither: they join their own party. `screen` is `"this"` (the
default: this device plays), `"companion"` (chat and controls only), or the F003 session id of one of the account's online
Apple TVs (R22): the server sends that TV `tv.cast` and this device becomes the companion.

```json client
{ "type": "party.join", "id": "j1", "data": { "partyId": "0193…", "screen": "this" } }
```

```json client
{ "type": "party.join", "id": "j2", "data": { "code": "K7P-4QX" } }
```

```json client
{ "type": "party.screens", "id": "s1" }
```

```json server
{ "type": "party.screens", "ts": 1793660000000, "data": { "screens": [ { "sessionId": "0192…", "name": "Salon", "type": "tvos" } ] } }
```

```json server
{ "type": "tv.cast", "ts": 1793660000000, "data": { "partyId": "0193…" } }
```

```json server
{ "type": "party.companion", "ts": 1793660000000, "data": { "partyId": "0193…", "screen": "Salon" } }
```

The joining connection first gets the whole state:

```json server
{ "type": "party.snapshot", "ts": 1793660000000, "data": {
  "partyId": "0193…", "itemId": "0191…", "mode": "host", "host": "u:alex", "me": "u:mia",
  "playback": { "state": "playing", "positionMs": 2292000, "atServerMs": 1793659990000, "rate": 1 },
  "presence": { "people": [], "count": 4, "max": 8, "queue": 0 },
  "chat": [], "pauseRequests": [], "reactions": ["😂", "😱", "❤️", "👏", "🦇", "🍿"] } }
```

A full party (R4) queues the person; they get a snapshot when a seat frees up.

```json server
{ "type": "party.queued", "ts": 1793660000000, "data": { "position": 1 } }
```

```json server
{ "type": "party.queue", "ts": 1793660000000, "data": { "waiting": ["Hugo"] } }
```

```json client
{ "type": "party.queue.leave" }
```

```json client
{ "type": "party.leave", "id": "l1" }
```

### Presence (R20)
`id` is `u:<userId>` or `g:<guestId>`. `state` is `here`, `loading`, `waiting` or `away`; `screen` is the device type of
the person's screen.

```json server
{ "type": "party.presence", "ts": 1793660000000, "data": { "count": 2, "max": 8, "queue": 0, "people": [
  { "id": "u:alex", "name": "Alex", "avatarColor": 1, "host": true, "guest": false, "canControl": true, "state": "here", "screen": "tvos", "quality": "original", "companion": true },
  { "id": "g:0194…", "name": "Camille", "avatarColor": 3, "host": false, "guest": true, "canControl": false, "state": "loading", "screen": "browser", "companion": false } ] } }
```

### Playback sync (R8–R13)
The room holds the reference state. While `playing`, the target position at server time `t` is
`positionMs + (t − atServerMs) × rate`. Clients check their drift every 250 ms: under 150 ms, nothing; from 150 ms to
3 s, play at 1.05 (behind) or 0.95 (ahead) until within 50 ms; beyond 3 s, seek. A player that can't change its rate seeks
past 1 s instead.

Screens report their state on every change and every 10 s while playing. `buffering` for more than 1.5 s puts the room in
`waiting` until every waited screen is `ready` again. `blocked` means the screen got `503 transcode_limit_reached` (R26).
`credits` means the end credits started (R30).

```json client
{ "type": "party.report", "data": { "state": "playing", "positionMs": 2292400, "playbackSessionId": "0195…", "quality": "1080p" } }
```

States: `loading`, `ready`, `playing`, `paused`, `buffering`, `ended`, `blocked`, `credits`.

Someone with control (host mode: the host and people given control; everyone mode: anyone):

```json client
{ "type": "party.control", "id": "c1", "data": { "action": "seek", "positionMs": 2260000 } }
```

`action` is `play`, `pause` or `seek`; `positionMs` is optional for play and pause.

```json server
{ "type": "party.playback", "ts": 1793660000000, "data": { "state": "playing", "positionMs": 2260000, "atServerMs": 1793660000000, "rate": 1, "action": "seek", "by": "u:leo", "byName": "Léo" } }
```

`action` there can also be `wait`, `resume` and `item` (a new title, paused at 0).

```json server
{ "type": "party.waiting", "ts": 1793660000000, "data": { "for": ["u:sam"], "names": ["Sam"], "positionMs": 2292000, "resumeWithoutAtMs": 1793660020000 } }
```

```json client
{ "type": "party.resume.without", "id": "r1" }
```

### Pause requests (R15)

```json client
{ "type": "party.pause.request", "id": "q1" }
```

```json server
{ "type": "party.pause.requested", "ts": 1793660000000, "data": { "requestId": 3, "by": "u:mia", "name": "Mia" } }
```

The host answers `now`, `in5` or `no`:

```json client
{ "type": "party.pause.answer", "id": "q2", "data": { "requestId": 3, "answer": "in5" } }
```

```json server
{ "type": "party.pause.scheduled", "ts": 1793660000000, "data": { "atServerMs": 1793660300000 } }
```

```json server
{ "type": "party.pause.answered", "ts": 1793660000000, "data": { "requestId": 3, "answer": "in5" } }
```

### Chat and reactions (R18, R19)
Chat: 1 to 500 characters, 1 a second (burst 5). Reactions: one of the six, 3 a second. System lines carry `system`
(`joined`, `left`, `removed`, `host`) and `values` for the client to phrase in its language.

```json client
{ "type": "party.chat", "id": "m1", "data": { "text": "Attends la scène de l’escalier" } }
```

```json server
{ "type": "party.chat", "ts": 1793660000000, "data": { "id": 12, "from": "u:mia", "name": "Mia", "text": "Il arrive par la porte ou par le mur cette fois ?", "positionMs": 2282000, "atServerMs": 1793660000000 } }
```

```json server
{ "type": "party.chat", "ts": 1793660000000, "data": { "id": 13, "system": "joined", "values": { "name": "Léo", "device": "ios", "quality": "720p" }, "positionMs": 2283000, "atServerMs": 1793660000000 } }
```

```json client
{ "type": "party.react", "data": { "emoji": "😱" } }
```

```json server
{ "type": "party.reaction", "ts": 1793660000000, "data": { "from": "u:sam", "name": "Sam", "emoji": "😱", "positionMs": 4360000 } }
```

### Host (R14, R16, R17, R29)

```json client
{ "type": "party.mode.set", "id": "h1", "data": { "mode": "everyone" } }
```

```json server
{ "type": "party.mode", "ts": 1793660000000, "data": { "mode": "everyone" } }
```

```json client
{ "type": "party.control.grant", "id": "h2", "data": { "to": "u:leo", "on": true } }
```

```json client
{ "type": "party.host.set", "id": "h3", "data": { "to": "u:mia" } }
```

```json server
{ "type": "party.host.changed", "ts": 1793660000000, "data": { "host": "u:mia", "name": "Mia" } }
```

```json client
{ "type": "party.remove", "id": "h4", "data": { "who": "g:0194…" } }
```

```json server
{ "type": "party.removed", "ts": 1793660000000 }
```

```json client
{ "type": "party.start", "id": "h5" }
```

```json client
{ "type": "party.end", "id": "h6" }
```

```json server
{ "type": "party.ended", "ts": 1793660000000 }
```

The end summary is on the party (`GET /parties/{id}`, `summary`).

### Transcode limit (R25, R26)
When a screen can't get a transcode, it reports `blocked`; the host gets the choice.

```json server
{ "type": "party.transcode.limit", "ts": 1793660000000, "data": { "waiting": [ { "id": "u:sam", "name": "Sam", "quality": "1080p" }, { "id": "u:leo", "name": "Léo", "quality": "720p" } ], "sharedQuality": "720p", "options": ["shared", "order", "cancel"] } }
```

```json client
{ "type": "party.transcode.choose", "id": "t1", "data": { "choice": "shared", "quality": "720p" } }
```

With `shared`, every screen that converts moves to that quality (PATCH its playback) and the waiting ones start with it:
one encode serves them all.

```json server
{ "type": "party.transcode.apply", "ts": 1793660000000, "data": { "quality": "720p", "waiting": ["u:sam", "u:leo"] } }
```

With `order`, the first retries and the others wait:

```json server
{ "type": "party.transcode.wait", "ts": 1793660000000, "data": { "after": "u:sam" } }
```

```json server
{ "type": "party.transcode.cancelled", "ts": 1793660000000 }
```

### Next episode (R30)

```json server
{ "type": "party.next", "ts": 1793660000000, "data": { "itemId": "0196…", "title": "Llamigos", "season": 1, "episode": 3, "runtimeMs": 180000, "startsAtServerMs": 1793660010000, "ready": 3, "of": 4 } }
```

```json client
{ "type": "party.next.stop", "id": "n1" }
```

```json server
{ "type": "party.item", "ts": 1793660000000, "data": { "itemId": "0196…" } }
```

Each screen then starts a new playback session on the new item (with `partyId`) and reports `ready`; the room starts at 0
once every screen is ready, or after 20 s.

## Notifications (F010-R41)
Sent to every connection of an account. `kind`: `party.invite`, `party.answer`, `party.cancelled`, `party.reminder`,
`party.vote.closed`, `party.vote.pick`, `party.file.missing`, `episode.available`. Clients fetch the details over REST.

```json server
{ "type": "notify", "ts": 1793660000000, "data": { "kind": "party.invite", "partyId": "0193…", "status": "scheduled" } }
```

```json server
{ "type": "notify", "ts": 1793660000000, "data": { "kind": "episode.available", "profileId": "0190…" } }
```
