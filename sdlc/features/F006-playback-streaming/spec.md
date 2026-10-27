---
id: F006
title: Playback — direct play and transcoding
owner: gh-prim
reviewer: TBD
status: draft            # draft | approved
intent: ./intent.md
decisions: [0001, 0002, 0003, 0006]
---

# F006 — Spec: Playback, direct play and transcoding

## Summary
F006 makes any recognized movie or episode playable on any client. The client starts a **playback session**: it says what it wants to watch (item, file, audio and subtitle tracks, start position) and what it can play (a capability profile). The server picks the lightest method that works: **Direct Play** (the file as is, over HTTP range requests), **Direct Stream** (video copied, container and if needed audio converted, served as HLS) or **Transcode** (video re-encoded with FFmpeg, hardware-accelerated when available, served as HLS). It says why. Seeking jumps straight to the requested point without waiting for the transcode to catch up. Text subtitles go out separately as WebVTT; image subtitles are burned in. Playback progress is saved per profile, so you resume where you left off and items get marked as watched. Admins cap the number of simultaneous transcodes and the bitrate sent outside the local network. Every stream URL carries a short-lived signed token, so native HLS players and external players like VLC work without headers. FFmpeg processes are always cleaned up. No new UI: the web player is F008, the admin screens are F007. When F006 is done, phase 1 is done: you can play a movie from `curl` or VLC through the API.

## User journeys

**Member, web browser, an MKV that needs no re-encoding** (the client side is F008; here it's what the API does)
1. Opens *The Matrix (1999)* (H.264 + DTS 5.1 in MKV) and presses Play. The client calls `POST /items/{id}/playback` with its profile: MP4 and HLS fMP4, H.264/HEVC/AV1, AAC/Opus, stereo.
2. The server answers `direct_stream`, reasons `container_not_supported` and `audio_codec_not_supported`: video copied, DTS converted to AAC stereo. It returns an HLS URL.
3. Playback starts in under 3 s. The client reports progress every 10 s.
4. The member jumps to 1:42:00. The server restarts FFmpeg at the nearest keyframe. The picture is back in a few seconds; nobody waits for the first 1 h 42 min to be processed.
5. Closes the tab at 1:50:00. The next day "Resume from 1:50:00" works on their phone.

**Member, 4K HDR file, laptop on hotel Wi-Fi** (remote)
1. Plays a 4K HEVC 10-bit HDR10 movie at 60 Mbps. The admin set the remote limit to 8 Mbps.
2. The server answers `transcode`, reasons `bitrate_too_high` (`remote_limit`): 1080p H.264 SDR at 8 Mbps, with basic tone mapping, on the Intel GPU (QSV).
3. The member picks "720p" in the player. The client calls `PATCH /playback/{id}`; the server restarts the transcode at the current position with the new quality.

**Member, anime with ASS subtitles and PGS signs**
1. Picks French ASS subtitles: the video is direct-streamed and the subtitles arrive as WebVTT (styling simplified).
2. Switches to the PGS track: image subtitles can't be sent separately, so the server switches to `transcode` with the subtitles burned in (reason `subtitle_burn_in`).

**Power user, VLC on the couch**
1. Asks for a playback link with the `external` profile (F008 will show a "Copy link for VLC" button; for now it's `curl`).
2. Pastes `http://server:32500/api/v1/stream/<token>/file` into VLC: the original file plays, seeking works through range requests. The link stops working 24 h after it was last used, or as soon as the member signs out that device.

**Admin, small server**
1. On a Raspberry Pi 4, sets "Simultaneous transcodes" to 1.
2. A second member starts a movie that needs a transcode: they get `503 transcode_limit_reached`, which the player turns into a clear message. Direct play and direct stream sessions don't count toward the limit and keep working.
3. `GET /playback` (admin) lists who is watching what, with the method, the reasons, hardware or software, and the transcode speed. The admin can stop a session.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| **Sessions and decision** | | |
| F006-R1 | Playback starts with `POST /items/{itemId}/playback`. Only `movie` and `episode` items are playable (`422 not_playable` otherwise). The request carries: an optional `fileId` (default: see R3), optional `audioStreamIndex` and `subtitleStreamIndex` (`null` = no subtitles), `startPositionMs`, optional `maxBitrate`, optional `quality`, `burnSubtitles` (default false), `forceMethod` (`transcode` or `direct_stream`, for debugging), and a capability profile (R2). | must |
| F006-R2 | The capability profile declares: `protocols` (`http`, `hls`), `containers` (for direct play), `video` codecs (each with `profiles`, max `level`, `maxWidth`, `maxHeight`, `maxBitDepth`, `hdr`: list of `hdr10`, `hdr10plus`, `hlg`, `dolby_vision`), `audio` codecs (each with `maxChannels`), `subtitles` (text formats it renders: `vtt`, `srt`, `ass`; image formats: `pgs`, `vobsub`), `canSwitchAudioTrack` (can pick a non-default audio track inside a file), and `maxBitrate`. It is sent with each playback request; the server doesn't store profiles. The preset `{"preset": "external"}` means "plays everything over HTTP" (VLC, mpv). | must |
| F006-R3 | File choice when `fileId` is omitted: among the item's available files (not missing, probed OK), the best one the client can play without transcoding; if none, the highest resolution. A `fileId` that isn't linked to the item returns `422 validation_failed`; a missing file returns `410 file_unavailable`; an unprobed one returns `409 file_not_analyzed`. | must |
| F006-R4 | Decision, in order: **Direct Play** if the client supports `http`, the container, the video stream (codec, profile, level, resolution, bit depth, HDR), the selected audio stream (codec, channels, and `canSwitchAudioTrack` if it isn't the default track), the selected subtitles are none or deliverable separately (R16), and the file bitrate fits the effective max bitrate (R22). Otherwise **Direct Stream** if the client supports `hls` and the video stream is acceptable as is: the video is copied and the audio is copied or converted. Otherwise **Transcode**. | must |
| F006-R5 | The response gives `sessionId`, `method` (`direct_play`, `direct_stream`, `transcode`), machine-readable `reasons` (e.g. `container_not_supported`, `video_codec_not_supported`, `video_profile_not_supported`, `resolution_too_high`, `bit_depth_not_supported`, `hdr_not_supported`, `audio_codec_not_supported`, `audio_channels_too_many`, `audio_track_switch_unsupported`, `subtitle_burn_in`, `bitrate_too_high`, `forced_by_client`), the `url` to play (with its token, R25), `durationMs`, the effective `startPositionMs`, the list of audio tracks, the list of subtitles with their URLs and whether each is `external` or `burned_in`, the available `qualities`, and whether hardware acceleration is in use. | must |
| F006-R6 | `PATCH /playback/{sessionId}` changes the audio track, subtitles, quality or max bitrate in place. The server makes a new decision and returns the same shape as R5. When the URL changes, the client reloads from its current position (`positionMs` in the request). | must |
| F006-R7 | A playback session belongs to an F003 session and to its active profile. A profile must be active (`403 profile_required`). Revoking the F003 session, signing out or disabling the account ends its playback sessions at once. A user only sees and changes their own playback sessions; admins see all of them (R29). | must |
| F006-R8 | Playback sessions live in memory; a server restart ends them (clients start new ones). A session ends with `DELETE /playback/{id}`, or after 24 h without any activity (progress report or stream request). | must |
| F006-R9 | Age limit (F003-R18): an item is refused with `403 content_restricted` if it is rated above the active profile's limit, or unrated while `allowUnrated` is false. Ratings come with F005; until then every item is unrated. | must |
| **Direct Play** | | |
| F006-R10 | `GET` and `HEAD /stream/{token}/file` serve the original file with `Accept-Ranges: bytes`, single `Range` requests (`206`), `If-Range`, `ETag` (size + mtime), the right `Content-Type` per container, `Content-Disposition: inline` with the file name. No FFmpeg is involved. | must |
| **HLS (Direct Stream and Transcode)** | | |
| F006-R11 | HLS output: version 7, fragmented MP4 (CMAF) segments, a VOD media playlist listing every segment of the whole duration from the first request, and a master playlist with one video variant, the audio rendition, and the text subtitle renditions (R16). Switching quality goes through R6, not adaptive bitrate (one encode per session, see watch-outs). | must |
| F006-R12 | Transcode segments are 6 s long. Keyframes are forced on segment boundaries, so segment N always starts at N × 6 s. | must |
| F006-R13 | Direct Stream segments follow the source keyframes: segments group keyframes into about 6 s (never less than 2 s, at most 10 s). The keyframe list comes from the container index (Matroska Cues, MP4 `stss`), read without decoding and cached per file (`media_keyframes`). If the file has no usable index, the server builds it with a background packet scan and the session is transcoded in the meantime (reason `keyframe_index_pending`). | must |
| F006-R14 | Seeking: a request for a segment the current FFmpeg run will produce within 3 segments waits for it (up to 20 s, then `503` with `Retry-After: 1`). A request further ahead, or before the first segment still on disk, restarts FFmpeg at that segment (`-ss` at the segment start, `-start_number N`). Seeking never waits for earlier segments to be produced. | must |
| F006-R15 | Throttling: when FFmpeg is more than 2 minutes ahead of the last requested segment, it is paused (`SIGSTOP`); it resumes (`SIGCONT`) when the client gets within 1 minute of it. | should |
| **Tracks and subtitles** | | |
| F006-R16 | Text subtitles (`subrip`, `ass`, `ssa`, `webvtt`, `mov_text`, external `.srt`, `.ass`, `.ssa`, `.vtt`) are delivered separately as WebVTT: `GET /stream/{token}/subtitles/{index}.vtt`, converted once with FFmpeg and cached per file and stream. In HLS they're also listed as subtitle renditions (one VTT segment for the whole duration), so native players (Safari, AVPlayer) show them. ASS styling is lost in WebVTT; a client that lists `ass` gets the original via `/subtitles/{index}.ass`. | must |
| F006-R17 | Image subtitles (PGS, VobSub, DVB) are burned into the video, which forces a transcode, unless the client lists the image format and plays the file directly. `burnSubtitles: true` burns text subtitles too (ASS with full styling). | must |
| F006-R18 | Default tracks when the request doesn't choose: the file's default audio track (else the first one); no subtitles, except a forced subtitle track in the audio track's language, which is selected and marked `forced`. | should |
| F006-R19 | Audio, when it has to change: converted to AAC (stereo, or 5.1 if the client declares `aac` with `maxChannels` ≥ 6), or kept as AC-3/E-AC-3 when the client supports them. Downmixes from more channels than the client accepts. Audio-only conversion keeps the session in Direct Stream. | must |
| **Transcoding** | | |
| F006-R20 | Video transcode targets H.264 High (level ≤ 4.1 at 1080p, ≤ 5.1 at 4K), 8-bit, SDR, using the client's resolution and bitrate limits and the chosen quality. HEVC output when the client supports it and a hardware HEVC encoder is available. | must (H.264), could (HEVC) |
| F006-R21 | Quality ladder, never above the source resolution or bitrate: `original` (Direct Play/Stream only), `2160p` 20 Mbps, `1080p` 8 Mbps, `720p` 4 Mbps, `480p` 1.5 Mbps, `360p` 0.75 Mbps. Default: the best quality that fits the effective max bitrate. | must |
| F006-R22 | Effective max bitrate = the lowest of the request's `maxBitrate`, the profile's `maxBitrate`, and, for remote clients, the admin's `remoteMaxBitrate`. A client is remote when its IP (the real one per F003's `PLAID_TRUSTED_PROXIES` rules) isn't loopback, in RFC 1918 / RFC 4193 / link-local ranges, or in the admin's extra `localNetworks`. | must |
| F006-R23 | HDR to SDR: when an HDR10 or HLG source is transcoded for a client without HDR support, the server applies basic tone mapping (software `zscale` + `tonemap=hable`, or the hardware filter when the accelerator has one). Dolby Vision without an HDR10 base layer (profile 5) is transcoded without tone mapping and flagged in the response (`colors_may_be_wrong`). Advanced tone mapping is out of scope (v2). | should |
| F006-R24 | Hardware acceleration: `none`, `vaapi`, `qsv`, `nvenc`, `v4l2m2m` (Raspberry Pi 4) or `videotoolbox` (macOS, dev only), set by the admin; `auto` (the default) tries them in that order and keeps the first one that passes a test. At startup and whenever the setting changes, the server runs a 1-second test encode (and decode, per codec) and stores which encoders and decoders work. Decoding, scaling and encoding stay on the GPU when the whole chain supports it. If an FFmpeg run using the GPU fails during its first 10 seconds, the session restarts in software and the failure is logged; after 3 failures in 10 minutes, hardware is disabled until the next restart or settings change, with a warning in `GET /server/capabilities`. | must |
| **Security, limits, administration** | | |
| F006-R25 | Stream URLs (`/stream/{token}/…`) are authenticated by the token alone, carried in the path so relative URLs in HLS playlists inherit it. Token: 32 random bytes, base64url, only its SHA-256 kept, scoped to one playback session, valid until the session ends (R7, R8). The access log replaces the token with `…`. These routes accept only `GET` and `HEAD`, and send `Cache-Control: private, no-store` on playlists and `Referrer-Policy: no-referrer`. | must |
| F006-R26 | Simultaneous transcodes: admin setting `maxTranscodes` (default: 1 with software only on 4 cores or fewer, otherwise 2; 4 with hardware). Only sessions that re-encode video count. A new session that would exceed it gets `503 transcode_limit_reached`; `PATCH` to a transcoded quality is refused the same way and the session keeps its current stream. | must |
| F006-R27 | Temporary files go to `PLAID_TRANSCODE_DIR` (default `$PLAID_DATA_DIR/transcodes`), one folder per session, emptied when the session ends. Segments more than 5 minutes behind the last requested one are deleted. Total size cap `transcodeDirMaxBytes` (default 10 GiB): above it, the oldest segments of every session are deleted first, and if that's not enough, new transcodes are refused with `503 transcode_storage_full`. The folder is emptied at startup. The subtitle and keyframe caches live in `$PLAID_DATA_DIR/cache/` and are capped at 1 GiB (least recently used goes first). | must |
| F006-R28 | FFmpeg processes are always cleaned up: each run is its own process group and is killed (`SIGTERM`, then `SIGKILL` after 5 s) when its session ends or restarts. On Linux, `Pdeathsig` kills FFmpeg if PLAID dies. A run gets no segment request for 60 s → it's stopped (the session stays, the next request restarts it at the right segment). Server shutdown stops every run before exiting. stderr is kept (last 64 KiB) for error logs. | must |
| F006-R29 | Admin: `GET /playback` lists every active session (user, profile, device, item, file, method, reasons, quality, hardware or software, transcode speed, position, remote or local). `DELETE /playback/{id}` ends any session. | must |
| F006-R30 | Playback settings, admin-only, stored in the new `settings` table: `maxTranscodes`, `remoteMaxBitrate` (bps, 0 = no limit, default 0), `localNetworks` (CIDR list), `hwAccel` (R24), `hwDevice` (e.g. `/dev/dri/renderD128`, default: first found), `transcodeDirMaxBytes`, `encoderPreset` (`fast`, `balanced`, `quality`; maps to the encoder's speed presets, default `fast`). `PLAID_FFMPEG_PATH` (ADR 0006) and `PLAID_TRANSCODE_DIR` stay environment variables. | must |
| F006-R31 | `GET /server/capabilities` (F004) gains `ffmpeg: {found, version}`, `hwAccel: {configured, active, device, encoders: [...], decoders: [...], disabledReason?}`. If FFmpeg is missing, Direct Play still works and the other methods return `503 ffmpeg_missing`. | must |
| **Progress and watch state** | | |
| F006-R32 | `POST /playback/{id}/progress` with `{positionMs, state}` (`playing`, `paused`, `buffering`, `stopped`). Clients send it every 10 s while playing and on every pause, seek and stop. It also keeps the session alive. The server writes watch state at most once every 10 s per session, and always on `stopped` and on `DELETE`. | must |
| F006-R33 | Watch state is per **profile** and per **item** (never per file, F004-R14): `positionMs`, `watched`, `playCount`, `lastPlayedAt`, the last file used. A position under 60 s isn't saved as a resume point. Reaching 90% of the duration, or less than 2 minutes before the end, marks the item watched, increments `playCount` once per session and resets the position. | must |
| F006-R34 | Clients that never report progress (VLC on a direct link) leave watch state untouched. HLS sessions without progress reports update the position from the last requested segment. | should |
| F006-R35 | `PUT /items/{itemId}/watched` and `DELETE /items/{itemId}/watched` mark an item watched or unwatched for the active profile. On a show or season, it applies to every episode below it. | must |
| F006-R36 | Item payloads (`GET /items/{id}`, `GET /libraries/{id}/items`) gain `userState` for the active profile: `positionMs`, `watched`, `playCount`, `lastPlayedAt`; for shows and seasons, `watchedEpisodes` and `totalEpisodes`. | must |
| F006-R37 | `GET /me/continue-watching`: the active profile's in-progress items (position saved, not watched), most recent first, plus for each show the next unwatched episode after the last one watched. Paginated like F004. | should |
| **For other features** | | |
| F006-R38 | The decision engine and session manager are a Go package (`internal/playback`) with an API other features call directly: F010 creates one session per Party participant, each with its own profile and decision. Sessions can start at any position, with the exact start time returned so a synced client can line up. | must |
| F006-R39 | Every playback event (session started with its method and reasons, quality change, FFmpeg run started or restarted with its command line minus tokens, hardware fallback, session ended with its duration) is logged with `slog`. | must |

## Design

### Data model
Migration `0004_playback.sql`. Same conventions as F003/F004 (UUIDv7 `TEXT` ids, RFC 3339 UTC timestamps).

```sql
-- Server-wide settings, one JSON value per key (first user: F006-R30).
CREATE TABLE settings (
  key         TEXT PRIMARY KEY,
  value       TEXT NOT NULL,               -- JSON
  updated_at  TEXT NOT NULL
);

-- Watch state per profile and item (R33).
CREATE TABLE watch_state (
  profile_id      TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  item_id         TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  position_ms     INTEGER NOT NULL DEFAULT 0,
  duration_ms     INTEGER,
  watched         INTEGER NOT NULL DEFAULT 0,
  play_count      INTEGER NOT NULL DEFAULT 0,
  last_played_at  TEXT,
  last_file_id    TEXT REFERENCES media_files(id) ON DELETE SET NULL,
  updated_at      TEXT NOT NULL,
  PRIMARY KEY (profile_id, item_id)
);
CREATE INDEX watch_state_recent ON watch_state(profile_id, last_played_at);

-- Keyframe index per file for Direct Stream segmenting (R13).
CREATE TABLE media_keyframes (
  file_id     TEXT PRIMARY KEY REFERENCES media_files(id) ON DELETE CASCADE,
  source      TEXT NOT NULL CHECK (source IN ('container','scan')),
  file_mtime  TEXT NOT NULL,               -- invalidated when the file changes
  keyframes   BLOB NOT NULL,               -- delta-encoded uvarint, milliseconds
  created_at  TEXT NOT NULL
);
```

Playback sessions, tokens and FFmpeg runs are in memory only (R8). A session holds: id, token hash, F003 session id, user id, profile id, item id, file id, decision, client IP and whether it's remote, the current FFmpeg run (if any), the last requested segment, the last position, and the last activity.

### Architecture (`server/internal/playback`)
- **Profile**: types for R2, and the `external` preset.
- **Decider**: a pure function `Decide(file, streams, request, profile, limits) Decision`, table-tested with real probe data. No I/O.
- **Manager**: creates, finds (by id and by token hash), changes and ends sessions; enforces `maxTranscodes`; listens to F003 session revocations; a reaper goroutine enforces R8 and R28's timeouts.
- **Transcoder**: builds FFmpeg command lines from a decision (per accelerator), starts and stops runs (process group, `Pdeathsig`, stderr ring buffer), watches progress through `-progress pipe:`, throttles (R15), knows which segments are on disk.
- **Segmenter**: turns a duration or a keyframe list into a segment list (R12, R13), writes playlists, maps segment number → start time.
- **Keyframes**: reads Matroska Cues and MP4 `stss`/`stts` in Go (reading the index only, never the whole file), with the fallback scan.
- **HWProbe**: startup and settings-change test encodes (R24).
- **WatchState**: SQL for `watch_state`, threshold rules (R33), continue watching (R37).
- **Handlers**: the routes below, plus a `/stream` router outside the F003 session middleware (it authenticates by token).

An FFmpeg transcode command looks like this (VAAPI, 1080p, burned PGS subtitles):
```
ffmpeg -hide_banner -nostdin -progress pipe:1
  -hwaccel vaapi -hwaccel_device /dev/dri/renderD128 -hwaccel_output_format vaapi
  -ss <segment start> -i <file>
  -filter_complex "[0:v]scale_vaapi=w=1920:h=-2,hwdownload,format=nv12[v];[v][0:s:2]overlay[o];[o]format=nv12,hwupload"
  -map [o] -map 0:a:1 -c:v h264_vaapi -b:v 8M -maxrate 8M -bufsize 16M
  -force_key_frames "expr:gte(t,n_forced*6)" -c:a aac -ac 2 -b:a 192k
  -f hls -hls_time 6 -hls_segment_type fmp4 -hls_playlist_type vod -start_number <N>
  -hls_flags temp_file -hls_segment_filename <dir>/%d.m4s <dir>/ffmpeg.m3u8
```
The playlist PLAID serves is its own (R11, generated from the segment list); FFmpeg's playlist is only used to know which segments are finished.

### API
Added to `api/openapi.yaml` first. Routes under `/api/v1`. "session" = F003 auth (cookie or bearer, with CSRF rules); "token" = playback token in the path (R25), added to the public-route list of F003-R12 because it has its own auth.

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| POST | `/items/{itemId}/playback` | session | Start a session: `{fileId?, audioStreamIndex?, subtitleStreamIndex?, startPositionMs?, maxBitrate?, quality?, burnSubtitles?, forceMethod?, profile}` → `201 PlaybackSession` |
| GET | `/playback/{sessionId}` | session | Current state of own session |
| PATCH | `/playback/{sessionId}` | session | `{audioStreamIndex?, subtitleStreamIndex?, quality?, maxBitrate?, burnSubtitles?, positionMs}` → `200 PlaybackSession` (R6) |
| POST | `/playback/{sessionId}/progress` | session | `{positionMs, state}` → `204` (R32) |
| DELETE | `/playback/{sessionId}` | session (own) or admin | `{positionMs?}` optional body → `204` |
| GET | `/playback` | admin | Active sessions (R29) |
| GET | `/settings/playback` | admin | Settings (R30) |
| PATCH | `/settings/playback` | admin | Partial update; changing `hwAccel` or `hwDevice` reruns the hardware test, returns `200` with the new settings and test result |
| PUT | `/items/{itemId}/watched` | session | Mark watched → `204` (R35) |
| DELETE | `/items/{itemId}/watched` | session | Mark unwatched → `204` |
| GET | `/me/continue-watching` | session | `?cursor=&limit=` (R37) |
| GET, HEAD | `/stream/{token}/file` | token | Direct Play (R10) |
| GET | `/stream/{token}/master.m3u8` | token | HLS master playlist |
| GET | `/stream/{token}/video/index.m3u8` | token | Media playlist |
| GET | `/stream/{token}/video/init.mp4` | token | fMP4 init segment |
| GET | `/stream/{token}/video/{n}.m4s` | token | Segment `n` (R14) |
| GET | `/stream/{token}/subtitles/{index}.m3u8` | token | Subtitle rendition playlist (R16) |
| GET | `/stream/{token}/subtitles/{index}.vtt` | token | WebVTT subtitles |
| GET | `/stream/{token}/subtitles/{index}.ass` | token | Original ASS/SSA (R16) |

`PlaybackSession`:
```json
{
  "id": "0193…",
  "itemId": "0192…",
  "fileId": "0192…",
  "method": "direct_stream",
  "reasons": ["container_not_supported", "audio_codec_not_supported"],
  "url": "/api/v1/stream/Zk3…/master.m3u8",
  "expiresAfterIdleSeconds": 86400,
  "durationMs": 8160000,
  "startPositionMs": 6120000,
  "quality": "original",
  "qualities": ["original", "1080p", "720p", "480p", "360p"],
  "hardwareAcceleration": null,
  "warnings": [],
  "audioTracks": [
    {"index": 1, "codec": "dts", "channels": 6, "language": "eng", "title": "DTS-HD MA", "default": true, "selected": true, "delivered": "aac"}
  ],
  "subtitles": [
    {"index": 3, "codec": "ass", "language": "fre", "forced": false, "selected": true, "delivery": "external", "url": "/api/v1/stream/Zk3…/subtitles/3.vtt"},
    {"index": 4, "codec": "hdmv_pgs_subtitle", "language": "eng", "forced": true, "selected": false, "delivery": "burn_in"}
  ]
}
```
Client capability profile (example: a Chromium browser):
```json
{
  "protocols": ["http", "hls"],
  "containers": ["mp4", "webm"],
  "video": [
    {"codec": "h264", "profiles": ["baseline", "main", "high"], "maxLevel": 52, "maxWidth": 3840, "maxHeight": 2160, "maxBitDepth": 8, "hdr": []},
    {"codec": "vp9", "maxBitDepth": 10}, {"codec": "av1", "maxBitDepth": 10}
  ],
  "audio": [{"codec": "aac", "maxChannels": 6}, {"codec": "opus", "maxChannels": 8}, {"codec": "mp3", "maxChannels": 2}],
  "subtitles": {"text": ["vtt"], "image": []},
  "canSwitchAudioTrack": false,
  "maxBitrate": null
}
```
F008 (web) builds its profile from `MediaSource.isTypeSupported` / `canPlayType`; the Apple and Android apps build theirs the same way later. The server doesn't trust the profile for security, only for compatibility.

New error codes: `not_playable`, `file_unavailable`, `file_not_analyzed`, `profile_required`, `content_restricted`, `transcode_limit_reached` (with `Retry-After: 30`), `transcode_storage_full`, `ffmpeg_missing`, `playback_session_not_found`, `segment_not_ready` (with `Retry-After: 1`), `hw_accel_unavailable` (settings test failed). An unknown or expired token returns `404 playback_session_not_found` (never `401`, so players don't prompt for credentials).

### Behavior per app
- **server**: everything above. The playback manager starts with the server, empties the transcode folder, runs the hardware test in the background (Direct Play works before it finishes) and subscribes to F003 session revocations.
- **web**: nothing required in F006. F008 builds the player (hls.js, capability detection, progress reports, quality, tracks, resume); F007 builds the playback settings screen and the "Now playing" admin view. A developer can test everything with `curl`, VLC, or `ffplay`.
- **apple / android**: out of scope; AVPlayer and ExoPlayer play the same HLS and use the same profile shape.

### Packaging (ADR 0006)
- `.deb`: the systemd unit gets `SupplementaryGroups=render video` so the `plaid` user can open `/dev/dri/*`. `MemoryDenyWriteExecute=true` may break some GPU drivers (see watch-outs). A custom `PLAID_TRANSCODE_DIR` needs a `ReadWritePaths=` override, documented in `docs/install.md`.
- Docker: document `--device /dev/dri:/dev/dri` with `group_add` for the render group's GID, and the NVIDIA Container Toolkit (`--gpus all`) for NVENC; the compose file gets a commented example. The image keeps the distro `ffmpeg` plus the VAAPI drivers (`intel-media-va-driver` and `mesa-va-drivers` on amd64).
- Hardware support with the distro FFmpeg (to confirm in the plan with a test on each target):

| Target | VAAPI | QSV | NVENC | V4L2 M2M | Notes |
|--------|-------|-----|-------|----------|-------|
| Debian 13 trixie amd64 (FFmpeg 7.1) | ✅ | ✅ (libvpl) | ✅ (needs the NVIDIA driver) | — | Main target |
| Debian 12 bookworm amd64 (FFmpeg 5.1) | ✅ | ⚠️ libmfx, older Intel only | ✅ | — | |
| Ubuntu 24.04+ amd64 | ✅ | ✅ | ✅ | — | |
| arm64 (Raspberry Pi 4) | — | — | — | ⚠️ H.264 encode only, no HEVC decode | Software in practice |
| arm64 (Raspberry Pi 5) | — | — | — | ❌ no hardware encoder | Software only |
| Docker image (trixie) | ✅ | ✅ | ✅ | — | Same as trixie |
| macOS (dev, `make dev`) | — | — | — | — | VideoToolbox |

### UI / Art direction
No screens in F006. The API returns codes and numbers; F007 and F008 turn them into copy following the design system content rules: functional text is short and plain, one warm line where it helps (an empty "Continue watching", a waiting state). Suggested copy for the codes clients will show is in the i18n table below, so F008 starts with the right tone.

### i18n
F006 adds no server-side strings. Clients translate the error codes and decision reasons. Suggested keys, to be added to the catalogs by the first client that shows them (F007/F008):

| Key | en | fr |
|-----|----|----|
| `errors.transcode_limit_reached` | The server is busy converting other videos. Try again in a moment, or pick a lower quality. | Le serveur convertit déjà d'autres vidéos. Réessaie dans un instant, ou choisis une qualité plus basse. |
| `errors.transcode_storage_full` | The server is out of space for video conversion. Ask the admin to free some up. | Le serveur n'a plus de place pour convertir la vidéo. Demande à l'admin d'en libérer. |
| `errors.content_restricted` | This profile can't watch this title. | Ce profil ne peut pas regarder ce titre. |
| `errors.file_unavailable` | This file can't be found right now. Its drive may be unplugged. | Ce fichier est introuvable pour l'instant. Son disque est peut-être débranché. |
| `errors.ffmpeg_missing` | This video needs FFmpeg, which isn't installed on the server. | Cette vidéo a besoin de FFmpeg, qui n'est pas installé sur le serveur. |
| `errors.profile_required` | Pick a profile first. | Choisis d'abord un profil. |
| `playback.method.direct_play` | Direct play | Lecture directe |
| `playback.method.direct_stream` | Direct stream | Flux direct |
| `playback.method.transcode` | Converting | Conversion |
| `playback.reason.subtitle_burn_in` | These subtitles are images, so they're drawn into the video. | Ces sous-titres sont des images, ils sont donc incrustés dans la vidéo. |
| `playback.reason.bitrate_too_high` | Lowered to fit your connection. | Qualité réduite pour ta connexion. |
| `playback.resume` | Resume from {{position}} | Reprendre à {{position}} |
| `playback.startOver` | Start over | Reprendre du début |
| `home.continueWatching.empty` | Nothing in progress. Pick something and get cozy. | Rien en cours. Choisis un film et installe-toi. |

`{{position}}` is a duration formatted with the locale's formatter (`Intl.DurationFormat` or equivalent), never concatenated by hand.

## Non-functional requirements
- **Start time** (local network, from `POST /playback` to the first frame playable): Direct Play < 1 s; Direct Stream < 3 s; Transcode 1080p H.264 with hardware < 4 s, in software on a 4-core x86 < 6 s.
- **Seek time** (request for a segment far away → segment served): Direct Stream < 3 s; Transcode < 4 s with hardware, < 6 s in software.
- **Throughput**: a transcode runs at ≥ 1.2× real time or it's flagged as too slow in `GET /playback` (`speed`) and logged. Raspberry Pi 4: Direct Play and Direct Stream of any file, including 4K HDR, and one software transcode of 1080p H.264 → 720p at ≥ 1.0×. Real-time 4K HEVC transcoding on a Pi is not a goal; it's documented.
- **Direct Play cost**: serving a file uses no FFmpeg and under 5% of a Pi 4 core at 80 Mbps (`http.ServeContent` / `sendfile`).
- **Memory**: PLAID's own memory per session < 5 MB (FFmpeg is outside). Segments never sit in PLAID's memory; they're served from disk.
- **SQLite**: progress writes are throttled (R32); no write on segment requests.
- **Robustness**: no FFmpeg process survives its session, a crash of PLAID, or a restart (R28). A client that disappears without `DELETE` stops costing CPU within 60 s.
- **Security**: tokens from `crypto/rand`, compared by hash; tokens never logged (R25); paths are always resolved from the database (file ids), never from the URL, so `/stream` can't read arbitrary files; segment numbers and stream indexes are validated as integers in range; FFmpeg is run with an argument list, never through a shell; subtitle and file names never reach a command line unescaped (they're passed as arguments).
- **Accessibility**: subtitles are first-class (R16–R18); forced subtitles are on by default.

## ⚠️ Watch-outs
To settle before `approved`. ✅ = resolved, ⏸ = tracked elsewhere, ❓ = needs a decision.

- [ ] ❓ **Tokens in URLs vs F003.** F003's NFRs say "no secrets in logs or URLs". R25 makes an exception: a playback-only token, scoped to one session, in the path, redacted from logs. The intent's answer ("yes, for every player") points this way, and native HLS players leave no alternative. Confirm the exception, and decide whether the token's 24 h idle lifetime is right for "Copy link for VLC" (it can also leak through VLC's history or a shared screenshot).
- [ ] ❓ **Remote limit depends on ADR 0003** (still `proposed`). R22 decides "remote" by IP ranges plus `localNetworks`. Behind Tailscale (100.64.0.0/10) or a reverse proxy on the same box, clients may look local or remote by mistake. Should `100.64.0.0/10` count as local by default? Should ADR 0003 be accepted first?
- [ ] ❓ **ADR 0006: stay on the distro FFmpeg?** The table above says the distro builds cover VAAPI, NVENC and recent QSV on amd64 and nothing useful on Pi. Proposal: keep option C for F006 (no superseding ADR) and revisit only if the plan's hardware tests or tone mapping fail. Confirm, or ask for an ADR on `jellyfin-ffmpeg`-style builds now.
- [ ] ❓ **systemd hardening vs GPU drivers.** `MemoryDenyWriteExecute=true` is known to break NVIDIA's CUDA (NVENC) and can break some VAAPI drivers; the `plaid` user also needs the `render` and `video` groups. Options: drop `MemoryDenyWriteExecute`, or keep it and document an override for NVENC. To decide with a real test in `make test-deb` (VAAPI can't run in CI; NVENC even less).
- [ ] ❓ **One quality per session, no adaptive bitrate.** R11 gives one HLS variant per session, quality changes via `PATCH`. ABR (several variants encoded at once) would multiply the CPU cost on small servers. OK for v1? The client then has to pick a quality from its measured bandwidth (F008).
- [ ] ❓ **Direct Stream keyframe index (R13).** Reading Matroska Cues and MP4 `stss` in Go is new code; files without Cues (some old MKVs, `.ts`, `.avi`) fall back to a full packet scan, which reads the whole file (minutes for a 50 GB file on USB 2). Meanwhile those sessions are transcoded. Accept, or simplify: Direct Stream only for MKV/MP4 with an index, and transcode everything else?
- [ ] ❓ **Simultaneous transcode defaults (R26) and Plaid Party.** F010 gives each participant their own stream: four friends on mobile with a 4K source means four transcodes. With the default of 1 on a Pi, the Party can't start. Is the limit per server enough, or does F010 need a per-room budget (e.g. one shared transcode per quality)? Track in F010's intent.
- [ ] ❓ **Age limits block kids' profiles until F005.** R9 enforces F003-R18 now, but nothing has a rating before F005, and `allowUnrated` defaults to false when a limit is set. So a restricted profile can't play anything until F005 lands. Accept (safer), or skip enforcement until F005 and record it there?
- [ ] ❓ **Watched threshold and resume minimum** (R33: 90% or 2 min left; nothing saved under 60 s). Plex uses 90%, Jellyfin 90% with a 5% minimum. A long credits sequence (Marvel) marks movies watched early, which is usually what people want. Confirm the numbers.
- [ ] ❓ **Default storage cap** (R27: 10 GiB, segments kept 5 min behind the playhead). A 4K remux at 60 Mbps writes ~450 MB per minute: 5 minutes behind plus 2 ahead is about 3 GB per session. Fine on a NAS; tight on a Pi with an SD card. Lower the default, or derive it from the free space?
- [ ] ❓ **HDR tone mapping scope (R23).** The intent puts advanced tone mapping out of scope. R23 keeps a basic one as `should`, because a 4K HDR movie transcoded without it looks washed out, which is the most common remote case. Dolby Vision profile 5 stays wrong. Confirm the line.
- [ ] ⏸ **VideoToolbox** is in the intent, but releases are Linux only (`.goreleaser.yaml`). It's kept for `make dev` on macOS, without CI coverage. Shipping a macOS server would be a separate intent.
- [ ] ⏸ **Watch history is lost when F004 deletes an item** after 30 days missing (F004-R17, cascade on `watch_state`). Acceptable for v1; a "keep history by TMDB id" improvement belongs with F005.
- [ ] ⏸ **Track preferences** (preferred audio and subtitle language per profile, remembered choices per show) aren't in F003's profile model. R18 uses the file's defaults. Add to the backlog.
- [ ] ⏸ **Trickplay, intro and credits detection, Chromecast, AirPlay, downloads**: out of scope (intent, roadmap).

## Acceptance criteria
- [ ] Given the decider test table with real `ffprobe` outputs, then every case returns the expected method and reasons, including at least: H.264/AAC MP4 + browser profile → `direct_play`; H.264/DTS MKV + browser profile → `direct_stream` [`container_not_supported`, `audio_codec_not_supported`]; HEVC 10-bit HDR10 MKV + browser profile without HEVC → `transcode` [`video_codec_not_supported`]; same file + `external` preset → `direct_play`; H.264 MKV with a PGS track selected → `transcode` [`subtitle_burn_in`]; same with an SRT track selected → not transcoded, subtitle `external`; a 40 Mbps file from a remote IP with `remoteMaxBitrate` 8 Mbps → `transcode` [`bitrate_too_high`] at ≤ 8 Mbps.
- [ ] Given a Direct Play session, when I `GET /stream/{token}/file` with `Range: bytes=1000-1999`, then `206` with exactly those 1,000 bytes, `Content-Range`, and `Accept-Ranges: bytes`; VLC plays the URL and seeks.
- [ ] Given a Direct Stream session on a 2-hour MKV, when the client requests the segment at 1:42:00 first, then it gets it within the seek-time target, and the FFmpeg run started at that segment (checked in the logs), not at 0.
- [ ] Given a Transcode session, then every segment except the last is 6 s (± one frame), and `hls.js` and Safari play through 10 minutes with seeks forward and back without errors.
- [ ] Given a text subtitle (embedded ASS, external `.fr.forced.srt`), when I request its `.vtt`, then valid WebVTT with the right timings; the HLS master playlist lists it as a subtitle rendition; the forced French track is selected by default when the audio is French.
- [ ] Given a session, when the client stops sending requests, then its FFmpeg process is gone within 70 s; when I `DELETE` the session, then it's gone within 6 s and its transcode folder is removed.
- [ ] Given a running transcode, when the PLAID process is killed with `SIGKILL`, then no FFmpeg process remains (Linux), and the transcode folder is emptied at the next start.
- [ ] Given `maxTranscodes` = 1 and one transcode running, when another profile starts a session that needs a transcode, then `503 transcode_limit_reached` with `Retry-After`; a session that is Direct Play or Direct Stream with audio conversion still starts.
- [ ] Given an F003 session with an active playback session, when that F003 session is revoked, then the next stream request with its token returns `404 playback_session_not_found` and its FFmpeg run stops.
- [ ] Given a token, then it appears nowhere in the logs (access log and FFmpeg command lines show `…`).
- [ ] Given `/stream/{token}/video/../../../etc/passwd` or a segment number out of range, then `404`, and no file outside the session folder is read.
- [ ] Given two profiles on one account, when profile A watches half of a movie, then `userState.positionMs` is set for A and empty for B.
- [ ] Given progress reports up to 91% of the duration, then the item is `watched`, `playCount` is 1, the position is reset, and it no longer appears in `/me/continue-watching`.
- [ ] Given a show where S1E1–E3 are watched, then `/me/continue-watching` offers S1E4; `PUT /items/{showId}/watched` marks every episode watched; `DELETE` unmarks them.
- [ ] Given a profile with an age limit and `allowUnrated` false, when it starts an unrated movie, then `403 content_restricted`.
- [ ] Given `hwAccel` = `vaapi` on a machine with a working Intel GPU (manual test, recorded in the PR), then `GET /server/capabilities` lists `h264_vaapi`, a 1080p transcode reports `hardwareAcceleration: "vaapi"`, and its speed is ≥ 3×.
- [ ] Given `hwAccel` = `vaapi` with no `/dev/dri`, then the settings update returns `422 hw_accel_unavailable`; with `auto`, the server falls back to software with a warning and transcodes still work.
- [ ] Given FFmpeg is missing, then Direct Play works and a session needing HLS returns `503 ffmpeg_missing`.
- [ ] Given the throttled Pi-like test profile (`--cpus 0.5`), then a Direct Stream of a 4K HEVC file starts in under 3 s, and a 1080p → 720p software transcode keeps up at ≥ 1.0×.
- [ ] Given the transcode folder over `transcodeDirMaxBytes`, then the oldest segments go first, and a new transcode is refused with `503 transcode_storage_full` only if that isn't enough.
- [ ] Given a member session, when I `GET /playback` or `PATCH /settings/playback`, then `403 forbidden`; when I `DELETE` another user's playback session, then `404 playback_session_not_found`.
- [ ] Given `curl` only, then a developer can sign in, start a session on a movie, and play the returned URL in VLC: phase 1's end goal.

## Revisions
_None yet._
