---
id: F004
title: Libraries and file scanning
owner: gh-prim
reviewer: TBD
status: accepted
phase: 1
apps: [server]
created: 2026-10-26
---

# F004 — Libraries and file scanning

## Problem
Movies and shows are scattered across folders with more or less tidy names. The server has to find them, understand what they are, and stay up to date when files are added or removed.

## Expected outcome
The admin creates a "Movies" or "Shows" library pointing to one or more folders. The server scans, recognizes every movie and every episode, analyzes the files (codecs, audio and subtitle tracks) and detects changes on its own.

## Users and systems affected
- Admin.
- Server: file system, ffprobe, database.

## Success criteria
- [ ] Common naming conventions recognized: `Movie (2019)/Movie (2019).mkv`, `Show/Season 01/S01E02.mkv`, localized folder names (`Saison 01`, `Staffel 01`…), multi-episode files, editions (Director's Cut).
- [ ] ffprobe analysis stored: container, codecs, resolution, HDR, audio and subtitle tracks (including `.srt` files next to the video).
- [ ] Incremental rescan + file watching (fsnotify), falling back to a periodic scan for network mounts.
- [ ] A moved file keeps its watch history.

## Constraints
Must stay smooth on a NAS with 10,000 files; no full rescan on every startup.

## Out of scope
Music and photos.

## Open questions
- [ ] What do we do with unrecognized files: a manual "to identify" queue?
- [ ] SMB/NFS mounts don't fire events: how often should the periodic scan run?
