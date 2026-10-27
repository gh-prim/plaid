---
id: F010
title: Plaid Party — watch together
owner: TBD
reviewer: TBD
status: draft
phase: 3
apps: [server, web]
created: 2026-10-26
---

# F010 — Plaid Party: watch together

## Problem
Watching a movie remotely with friends on Plex means counting "3, 2, 1, play" over the phone and drifting out of sync after ten minutes. This is **the** feature that must set PLAID apart.

## Expected outcome
I start a Plaid Party on a movie and send the link. My friends join from any device, each with a stream suited to their connection. We stay perfectly in sync, we react, we chat, and anyone can ask for a pause.

## Users and systems affected
- Host, participants (members, F009 friends, guests via link).
- Server (rooms, clock, WebSocket); web player in v1, then all apps.

## Success criteria
- [ ] Create a room on a media item and join by link or by code.
- [ ] Play / pause / seek sync, < 500 ms gap between participants under normal conditions.
- [ ] Gentle drift correction (adjusted speed), auto-resync after buffering.
- [ ] Control modes: "host only" or "everyone".
- [ ] Emoji reactions, chat, pause request ("bathroom break").
- [ ] Presence list: who's here, who's loading.
- [ ] Each participant has their own playback strategy (F006).

## Constraints
- Depends on F003, F006 and F009.
- The protocol must be documented (WebSocket events in `api/`), so it can be ported identically to Apple and Android.

## Out of scope
Built-in video / voice (v2?), SharePlay (phase 4 bonus).

## Open questions
- [ ] Anonymous guest without an account, with access limited to this one movie: yes or no?
- [ ] Max room size?
- [ ] If the host leaves: host transfer or end of the room?
- [ ] Transcode budget: F006 limits simultaneous transcodes per server (F006-R26, 1 by default on a Pi). Four participants needing a transcode can't all start. Share one transcode between participants who need the same quality, or give each room its own budget?
