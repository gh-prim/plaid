---
id: 0010
title: Remote push notifications through getplaid.direct
status: proposed
date: 2026-11-02
deciders: [gh-prim]
supersedes: null
---

# 0010 — Remote push notifications through getplaid.direct

## Context
F010 needs to reach people whose app is closed: a Plaid Party invitation, a party starting, a vote closing, "your episode is on the server". On iOS that means APNs. APNs only accepts requests signed with the app developer's key, which can't ship inside a self-hosted server. Web push (VAPID) doesn't have that problem but is out of scope for now.

## Options
### A. No remote push
In-app notifications over the WebSocket while the app is open, plus local notifications the iOS app schedules for reminders it already knows.
- ✅ Nothing to run, nothing leaves the server.
- ❌ An invitation is only seen when the person opens PLAID.

### B. A push relay on getplaid.direct
The server sends `{deviceToken, payload}` to getplaid.direct, authenticated with its server token (ADR 0009); the relay signs with PLAID's APNs key. Payload is either empty ("Something new on <server>", the app fetches the rest) or encrypted with a key the app shares with its server, so the relay never reads names or titles.
- ✅ Works like any app; opt-in, same trust model as remote access.
- ❌ One more endpoint to run and rate-limit; device tokens transit through our service.

### C. Each admin brings their own push service (ntfy, Pushover…)
- ✅ No PLAID infrastructure.
- ❌ Doesn't reach the PLAID iOS app; backlog #94 covers this for admins.

## Decision
Proposed: **B**, with encrypted payloads, off unless remote access is on. F010 ships with A and adds B once accepted.

## Consequences
- services/direct gains `POST /push` (per-server rate limit, no storage beyond counters).
- The iOS app registers for notifications and sends its device token to its own server only.
- Revisit for Android (FCM has the same constraint) and web push.
