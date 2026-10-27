---
id: 0007
title: Metadata provider API keys (TMDB, fanart.tv)
status: proposed
date: 2026-10-27
deciders: [gh-prim]
supersedes: null
---

# 0007 — Metadata provider API keys (TMDB, fanart.tv)

## Context
F005 fetches metadata from TMDB and extra artwork from fanart.tv. Both need an API key. TMDB's terms allow free non-commercial use, need an attribution notice ("This product uses the TMDB API but is not endorsed or certified by TMDB.") with their logo, and tie each key to one application. fanart.tv gives each project a key, and users can add an optional personal key that unlocks newer images. The intent wants metadata to work with no setup, so PLAID needs a key of its own, and the admin can override it with theirs. PLAID is MIT-licensed and public (ADR 0004), so anything committed is public.

## Options
### A. Project keys committed in the source
The usual open-source approach (Jellyfin and Kodi ship their TMDB key in the repository).
- ✅ Works the same in every build: dev, release, Docker, forks.
- ❌ The key is public. Anyone can reuse it, and if TMDB revokes it for abuse, every PLAID install breaks at once.

### B. Project keys injected at release build time
The keys live in GitHub Actions secrets and are set through `-ldflags -X …/internal/metadata.builtinTMDBKey=…` by GoReleaser and the Docker build. Dev builds and forks have no key and use `PLAID_TMDB_API_KEY`.
- ✅ The key isn't in the repository, though anyone can still pull it from a release binary.
- ✅ A fork that publishes its own builds needs its own key, which matches TMDB's one-key-per-application rule.
- ❌ CI needs secrets. The Docker build passes them as build secrets, never as plain `ARG`s, because those stay in the image history. `make dev` without a key shows metadata as disabled.

### C. The admin always provides keys
- ✅ No shared key at all.
- ❌ Onboarding gets an "open an account with TMDB" step, and metadata stays empty until the admin does it. That goes against the intent.

## Decision
_Proposed:_ **B**, with an admin override. Order of use: the key saved by the admin (database setting) → `PLAID_TMDB_API_KEY` / `PLAID_FANART_API_KEY` → the built-in key. Keys are write-only in the API and never logged.

## Consequences
- Two repository secrets, `TMDB_API_KEY` and `FANART_API_KEY`, used by the release workflow and `docker.yml`, never exposed to workflows that run on pull requests from forks.
- Tests never call the real providers: they replay recorded responses.
- The web About page (F007) shows the TMDB and fanart.tv attribution.
- If a built-in key is revoked, a patch release with a new key fixes every install that updates. Admins with their own key aren't affected.
