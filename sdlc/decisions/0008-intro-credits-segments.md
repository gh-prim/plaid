---
id: 0008
title: Intro and credits timestamps from TheIntroDB, end credits detected locally
status: accepted
date: 2026-10-31
deciders: [gh-prim]
supersedes: null
---

# 0008 — Intro and credits timestamps: TheIntroDB, end credits detected locally

## Context
The TV app (F011) needs to know where an episode's intro is (Skip intro) and where its end credits start (offer the next episode before they run). The next-episode card must show up on every episode, not only on popular shows. PLAID runs on modest hardware (Pi, NAS) and must not depend on a service to work.

## Options
### A. Full local analysis (Jellyfin-style audio fingerprinting across a season)
- ✅ Works offline, finds intros too.
- ❌ Heavy: decodes the audio of every episode of a season; minutes of CPU per season on a Pi.
- ❌ Lots of code and tuning to own.

### B. TheIntroDB only
- ✅ Free community API, keyed by TMDB id + season + episode + file length; intros, recaps, credits, previews.
- ✅ No CPU on the server.
- ❌ Coverage is partial: many episodes have no credits, or untrusted ones. The next-episode card would be missing or late.

### C. TheIntroDB, plus a local end-credits pass (black frames)
- ✅ TheIntroDB gives intros and recaps where known.
- ✅ End credits are mostly text on black: FFmpeg `blackframe` over the last 10 minutes at 2 fps finds them in a few seconds, once per file, at `nice 19`.
- ✅ Local detection wins over TheIntroDB for credits, so every episode gets a credits start.
- ❌ Shows whose credits run over pictures may be detected late or not at all (then the card falls back to the last seconds).

## Decision
We choose **C**. TheIntroDB answers are cached per item in `item_segments` (asked again after 30 days, or 3 days when it knew nothing; 2 requests/s, within its keyless limits). End credits of episodes are detected once per file with FFmpeg at the lowest priority and stored next to them; the detected start wins.

## Consequences
- A new outbound host (`api.theintrodb.org`), allow-listed like the metadata providers. Without network, there are no intros, credits still work.
- One short FFmpeg run per episode, the first time it is played, at the lowest CPU priority.
- Intro detection without TheIntroDB (option A) stays possible later as a separate feature.
