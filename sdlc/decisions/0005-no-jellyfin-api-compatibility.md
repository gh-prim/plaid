---
id: 0005
title: No Jellyfin API compatibility, PLAID ships its own apps
status: accepted
date: 2026-10-27
deciders: [gh-prim]
supersedes: null
---

# 0005 — No Jellyfin API compatibility, PLAID ships its own apps

## Context
The backlog (item 25) flagged Jellyfin API compatibility as the most strategic item: if the PLAID server also answered the Jellyfin API (`/System/Info/Public`, `/Users/AuthenticateByName`, `/Items`, `PlaybackInfo`, HLS streams, session progress, SyncPlay), existing clients such as Swiftfin, Infuse, Findroid and Jellyfin Android TV would work with PLAID on day one, long before our own apps (phases 4 and 5). The question had to be settled before the phase 1 specs (F003, F004, F005, F006), because compatibility would constrain their data model, auth and playback negotiation.

## Options
### A. No compatibility layer: PLAID's own API and its own apps only
- ✅ One API to design, secure and test: `api/openapi.yaml`, free to evolve.
- ✅ Every screen carries the PLAID experience and design system, Plaid Party included, as defined in the [vision](../vision.md).
- ✅ Phase 1 specs are shaped by PLAID's needs only.
- ❌ No iPhone, iPad or TV client until phases 4 and 5. Until then: the web app.

### B. Full Jellyfin API compatibility
- ✅ The whole Jellyfin client ecosystem works.
- ❌ A large, partly undocumented API that evolves without us; third-party apps show their own UI, not PLAID's.

### C. Partial compatibility (browse, play, resume, later SyncPlay)
- ✅ Most real-world use with a fraction of B's effort; TVs covered early.
- ❌ A second API to maintain, client-specific quirks to reproduce, and phase 1 specs constrained by another product's model.

## Decision
We choose **A**. PLAID is a product with its own apps (web, SwiftUI for iPhone, iPad and Apple TV, Compose for Android and Android TV), as the vision defines. We don't implement the Jellyfin API, now or as a planned feature.

## Consequences
- Phase 1 specs (F003 accounts, F004 libraries, F005 metadata, F006 playback) are designed for PLAID's own API only; no Jellyfin-shaped IDs, auth headers or item model.
- The native apps (phases 4 and 5) are the only path to phones and TVs. Until they ship, the web app (F007, F008) is the client.
- Plaid Party (F010) uses its own protocol (`api/events.md`), not SyncPlay.
- Backlog item 25 is closed by this ADR. Subsonic compatibility for third-party music players may be reconsidered on its own when music is scoped; it would need a new ADR.
- Importing users and watch history from Jellyfin (backlog item 18) is unaffected: it reads another server's data, it doesn't make PLAID speak its API.
