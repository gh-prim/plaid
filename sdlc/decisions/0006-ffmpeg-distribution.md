---
id: 0006
title: How PLAID gets FFmpeg and ffprobe
status: accepted
date: 2026-10-27
deciders: [gh-prim]
supersedes: null
---

# 0006 — How PLAID gets FFmpeg and ffprobe

## Context
F004 needs `ffprobe` to analyze every video file; F006 needs `ffmpeg` to remux and transcode, ideally with hardware acceleration (VAAPI, Intel QSV, NVENC). Today neither the Docker image (Debian trixie slim) nor the `.deb` package provides them. The choice affects image size, which hardware acceleration works out of the box, and how we support users.

## Options
### A. Use the distribution's FFmpeg
`.deb`: `Depends: ffmpeg` (which ships `ffprobe`). Docker: `apt-get install ffmpeg` in the runtime image.
- ✅ Zero build work, security updates come with the OS, trusted packages.
- ✅ Enough for F004 (ffprobe) and for software transcoding.
- ❌ Version and enabled features differ per distro release (Debian 12 ships an older FFmpeg than Ubuntu 26.04), so hardware acceleration and HDR tone mapping behave differently across our supported targets.
- ❌ Adds roughly 100–200 MB of libraries to the Docker image.

### B. Ship our own FFmpeg build
A pinned static build (or a maintained media-server fork, as Jellyfin does with `jellyfin-ffmpeg`), bundled in the image and in a `plaid-ffmpeg` package.
- ✅ Same version and features everywhere; we control hardware acceleration and tone mapping.
- ❌ We build, update and secure FFmpeg ourselves, for amd64 and arm64; larger packages; licensing care (GPL/LGPL components, patent-encumbered codecs).

### C. A now, B when F006 needs it
Use the distro FFmpeg for F004 and early F006 (direct play, remux, software transcode). Revisit with a superseding ADR if hardware acceleration or tone mapping on the distro builds turns out to be insufficient.
- ✅ Unblocks F004 immediately with no build work; decision on B is taken with real data from F006.
- ❌ A possible packaging change later (new dependency, bigger image).

## Decision
We choose **C**. The server already locates the binaries through `PLAID_FFPROBE_PATH` / `PLAID_FFMPEG_PATH` (falling back to `PATH`), so switching to B later doesn't change the code.

## Consequences
- `.goreleaser.yaml` nFPM: add `ffmpeg` to `dependencies`. Docker runtime: install `ffmpeg` (no recommends).
- `make test-deb` and `package-test` check that `ffprobe -version` works after install.
- F006's spec must state which hardware acceleration is supported on each target with the distro builds, and decide whether to supersede this ADR with B.
