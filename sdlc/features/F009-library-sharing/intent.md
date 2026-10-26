---
id: F009
title: Library sharing
owner: TBD
reviewer: TBD
status: draft
phase: 3
apps: [server, web]
created: 2026-10-26
---

# F009 — Library sharing

## Problem
People want to share their media library with friends and family without handing over the keys to the whole server, and without relying on a central service.

## Expected outcome
The admin invites a friend by link. The friend creates an account on the server and can only access the shared libraries (or collections), with the chosen permissions.

## Users and systems affected
- Admin, invited friend.
- Server (permissions, quotas); web; later all apps.

## Success criteria
- [ ] Single-use, time-limited invitation link.
- [ ] Sharing per library or per collection.
- [ ] Permissions: max quality, downloads allowed or not, number of simultaneous streams.
- [ ] Immediate revocation, including active streams.
- [ ] The friend never sees non-shared libraries, neither in the API nor in search.

## Constraints
- ADR 0003: must work with a simple public URL.
- Security-critical: every endpoint must filter by access permissions.

## Out of scope
Server-to-server sharing (federation / Plaid Hub).

## Open questions
- [ ] Can a friend have multiple profiles?
- [ ] Showing the admin what their friends are watching: useful or intrusive?
