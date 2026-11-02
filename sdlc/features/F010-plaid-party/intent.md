---
id: F010
title: Plaid Party — watch together
owner: gh-prim
reviewer: TBD
status: accepted
phase: 3
apps: [server, web, apple]
created: 2026-10-26
---

# F010 — Plaid Party: watch together

## Problem
Watching a movie remotely with friends on Plex means counting "3, 2, 1, play" over the phone and drifting out of sync after ten minutes. This is **the** feature that must set PLAID apart.

## Expected outcome
I start a Plaid Party on a movie and send the link. My friends join from any device, each with a stream suited to their connection. We stay perfectly in sync, we react, we chat, and anyone can ask for a pause.

## Users and systems affected
- Host, participants (members, F009 friends, guests via link).
- Server (rooms, clock, WebSocket, scheduling, votes, reminders).
- Web player, iPhone/iPad app (F014) and Apple TV app (F011), all in v1. Android follows later.

## Screens
Claude Design canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz), page V2:
- **Step 23, Plaid Party**, boards Party01–Party27: chat on the phone or iPad while the TV plays (01–03), start, schedule, invite, join by code, scheduled party and countdown (04–09), watching on iPhone, iPad and web (10–15, with sync states), end, host managing people, host leaving, transcode limit, invitation received, home row (16–21), Apple TV next episode, start from the TV, remote reactions (22–24), full party (25), admin settings (26–27).
- **Upcoming tab** (boards PartiesWeb, PartiesPh): episode releases of followed shows and Plaid Parties, as a list or a calendar, with invitations.
- **Movie vote** (boards VotePh, VoteBig): everyone proposes, ranks and has one veto.

## Success criteria
- [ ] Create a room on a media item and join by link or by code.
- [ ] Play / pause / seek sync, < 500 ms gap between participants under normal conditions.
- [ ] Gentle drift correction (adjusted speed), auto-resync after buffering.
- [ ] Control modes: "host only" or "everyone".
- [ ] Emoji reactions, chat, pause request ("bathroom break").
- [ ] Presence list: who's here, who's loading.
- [ ] Each participant has their own playback strategy (F006).
- [ ] Watch on a TV with the chat on a phone or iPad, or on iPhone, iPad or web with the chat beside the video; reactions from the Apple TV remote.
- [ ] Schedule a party with invitations (yes / maybe / no), a 15-minute reminder and a waiting room with a countdown.
- [ ] Movie vote: the host opens it, everyone proposes (2 each), ranks a top 3 (3, 2, 1 points, anonymous) and has one veto; a tie is drawn by lot.
- [ ] Upcoming tab (web, iPhone, iPad): episode releases of followed shows and Plaid Parties, as a list or a calendar, with "notify me" and "plan a party" on a release.
- [ ] Series: move to the next episode together. Room limit set by the admin.

## Constraints
- Depends on F003, F006 and F009.
- The protocol must be documented (WebSocket events in `api/`), so it can be ported identically to Apple and Android.

## Out of scope
Built-in video / voice (v2?), SharePlay (phase 4 bonus), Android (follows the documented protocol later).

## Decisions (from the V2 design, 2026-11-01)
- **Anonymous guests: yes.** A guest joins from the link with a first name only. They see this one movie and nothing else on the server.
- **Host leaves:** the host chooses, either hand the host role to another participant or end the party for everyone.
- **Transcode budget:** when the server can't transcode for everyone, the host is asked to choose: one shared quality for everyone who needs a transcode (e.g. 720p), or an order (one participant first, the others wait).
- **Where people watch:** on a TV with the chat on the phone or tablet, or directly on the iPhone, iPad or web player with the chat beside the video.
- **Scheduling:** a party can be scheduled for a date and time, with invitations (yes / maybe / no) and a reminder 15 minutes before; a waiting room shows a countdown and who is ready.
- **Series:** at the end of an episode, the party moves to the next one together once participants are ready, with a short countdown.
- **Room size:** 8 participants by default, host included. An admin can change the limit in Settings (from 2 to 20). The limit exists because every remote participant is a stream leaving the home connection; when the room is full, the join screen says so and the host can raise the limit if they are an admin.

## Open questions
None for now.
