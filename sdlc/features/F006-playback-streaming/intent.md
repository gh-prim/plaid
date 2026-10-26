---
id: F006
title: Playback — direct play and transcoding
owner: TBD
reviewer: TBD
status: draft
phase: 1
apps: [server]
created: 2026-10-26
---

# F006 — Playback: direct play and transcoding

## Problem
A 4K HEVC MKV with ASS subtitles doesn't play everywhere. Any file has to start on any device, without melting a small server when it isn't necessary.

## Expected outcome
The client declares what it can play. The server picks the lightest strategy: Direct Play → Direct Stream (remux) → Transcode HLS. Playback starts fast, seeking is responsive, and you resume where you left off, on any device.

## Users and systems affected
- All users.
- Server (FFmpeg, hardware acceleration); all client players.

## Success criteria
- [ ] Client capability profiles (codecs, containers, max resolution, subtitles).
- [ ] Direct Play via HTTP range requests.
- [ ] HLS with on-the-fly transcoding, quality selection, seeking without waiting for the transcode to finish.
- [ ] Hardware acceleration: VAAPI, QSV, NVENC, VideoToolbox, with software fallback.
- [ ] Audio track selection; text subtitles sent separately, image subtitles burned in.
- [ ] Progress saved (resume, watched/unwatched).
- [ ] Admin limits: number of simultaneous transcodes, max remote bitrate.

## Constraints
- FFmpeg processes are always cleaned up (no zombies when a client disappears).
- Essential foundation for Plaid Party (F010): each participant must be able to get their own stream.

## Out of scope
Advanced HDR → SDR tone-mapping (v2), Chromecast / AirPlay.

## Open questions
- [ ] Where to store temporary segments: RAM (tmpfs) or disk?
- [ ] Signed playback link, for external players like VLC?
