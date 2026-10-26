---
id: 0002
title: Monorepo and OpenAPI contract
status: proposed
date: 2026-10-26
deciders: [TBD, TBD]
supersedes: null
---

# 0002 — Monorepo and OpenAPI contract

## Context
Five clients (web, iOS/iPadOS, tvOS, Android, Android TV) talk to the same server. Two devs. We need to keep each app from reinventing the API and versions from drifting apart.

## Options
### A. Monorepo + `api/openapi.yaml` as source of truth, generated clients
- ✅ One PR can change the API and all clients together.
- ✅ SDLC specs, ADRs and code live in the same place.
- ❌ CI has to be split per folder to stay fast.

### B. One repo per app
- ✅ Independent CIs.
- ❌ Painful syncing, scattered specs, overkill for two.

## Decision
Proposed: **A**. The WebSocket events (Plaid Party) are described in `api/events.md` (or AsyncAPI later).

## Consequences
The Go server is written *contract-first* (handlers generated with `oapi-codegen`). The TS, Swift and Kotlin clients are generated and never edited by hand.
