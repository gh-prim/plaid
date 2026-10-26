---
id: F001
title: Monorepo foundations
owner: TBD
reviewer: TBD
status: draft
phase: 0
apps: [server, web, apple, android]
created: 2026-10-26
---

# F001 — Monorepo foundations

## Problem
Two of us are starting four apps at the same time. Without a shared structure, CI and an API contract, each of us will head off in a different direction and we'll spend our time gluing the pieces back together.

## Expected outcome
A repo where every app has its place, `make test` checks everything, the API is described in a single place, and `docker compose up` starts an empty server reachable from the browser.

## Users and systems affected
- Both devs, and Claude, which reads `CLAUDE.md`.
- Every folder: `api/`, `server/`, `web/`, `apple/`, `android/`.

## Success criteria
- [ ] `server/` skeleton (Go) that answers `GET /api/v1/health`.
- [ ] `web/` skeleton (React + Vite) served by the server.
- [ ] `api/openapi.yaml` v0, with TS client generation wired in.
- [ ] `make test` and `make dev` at the root.
- [ ] GitHub Actions CI triggered per changed folder.
- [ ] Dockerfile + `docker-compose.yml`.
- [ ] `CLAUDE.md` filled in with the real commands.
- [ ] i18n wired into the web skeleton (`en` + `fr` catalogs), with a CI check that catches missing keys or hard-coded strings.

## Constraints
Depends on ADR 0001 (server language) and 0002 (monorepo + OpenAPI), which must be accepted first.

## Out of scope
The Xcode and Gradle projects: they are created in their own phases (4 and 5).

## Open questions
- [ ] Default server port (Plex uses 32400; we suggest 32500)?
- [ ] Web package manager: pnpm or bun?
- [ ] Project license?
