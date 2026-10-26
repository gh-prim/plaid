---
id: 0003
title: Remote access in v1
status: proposed
date: 2026-10-26
deciders: [TBD, TBD]
supersedes: null
---

# 0003 — Remote access in v1

## Context
Library sharing and the Plaid Party with friends require the server to be reachable from outside. Plex relies on a central service for this (plex.tv).

## Options
### A. Pure self-host: users expose their server themselves
Reverse proxy, Tailscale, Cloudflare Tunnel… We document the methods and operate no infra.
- ✅ Zero infra, zero cost, maximum privacy.
- ❌ More friction for non-technical users.

### B. Plaid Hub from v1
A central service that handles identity, server discovery and a relay.
- ✅ "It just works" experience.
- ❌ Infra to operate and secure, which delays the core product.

## Decision
Proposed: **A in v1**, but with accounts and invitations designed so a Hub (option B) can plug in for v2 without migration (UUID-style global identifiers, signed-token invitations).

## Consequences
Feature F009 (sharing) must work with a plain public server URL. The Hub will get its own intent later.
