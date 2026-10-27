---
id: F003
title: Accounts and authentication
owner: gh-prim
reviewer: TBD
status: accepted
phase: 1
apps: [server, web]
created: 2026-10-26
---

# F003 — Accounts and authentication

## Problem
The server holds someone's personal media library. We need to know who is signing in, and the admin must stay in control without relying on an external service.

## Expected outcome
On first launch, the admin account is created. The admin can then create member accounts, and each account can have several profiles ("Me", "Kids"…). All apps authenticate the same way.

## Users and systems affected
- Admin, members, profiles.
- Server (storage, sessions); all client apps.

## Success criteria
- [ ] Admin created on first launch only.
- [ ] Password sign-in, revocable sessions ("sign out this device").
- [ ] Profiles with an optional PIN and parental restrictions.
- [ ] TV pairing via a short code (you type the code on your phone, no remote keyboard).

## Constraints
- Global identifiers (UUID) and a format compatible with a future Plaid Hub (ADR 0003).
- Passwords hashed with argon2id.

## Out of scope
SSO / external OIDC and federated accounts (Hub): later.

## Open questions
- [ ] Sessions: short-lived JWT + refresh, or opaque tokens stored in the database?
- [ ] Is the profile PIN asked on every use, or only once per device?
