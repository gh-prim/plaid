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

## Decisions (from the V2 design, steps 19 and 20, 2026-11-02)
- **Groups.** Access is set per group: libraries, best quality, streams at the same time per person, downloads, Plaid Party (no / can join / can start), can create profiles. A person can be in several groups; their access is the union of their groups.
- **Exceptions per person only add rights** on top of their groups (one more library, a higher quality, more streams). They never take one away.
- **Libraries only.** Sharing per collection is dropped: a library is the unit of sharing.
- **Suspend or delete.** Suspending stops active streams, signs the person out everywhere and keeps their account and history; it can be undone. Deleting removes the account.
- **Existing members** go into a "Family" group with every library at migration. Admins are never filtered.
- **Household-only libraries** (home videos): a library flag; such a library can't be shared with any group or person.
- **Best quality** is enforced by playback (F006): above the cap, the file is transcoded down.
- **Downloads and Plaid Party** don't exist yet: the settings are stored now and read by those features later.
- **Profiles for a friend:** a group right, "can create profiles", on for Family, off by default for new groups.
- **Invitations** (step 20): single-use link, expires (7 days by default), carries a name and the groups. The invited person creates their own account from the link and must set a 4-digit PIN.
- **Avatars** (step 20): a profile shows an avatar built in PLAID, a photo, or its initial on its color. The builder uses a CC0 style of DiceBear (MIT engine); the server stores the options and serves the rendered image to every app.
- **Admin visibility:** the admin sees who is watching now, last seen and devices (People screen).
- **One PR** for steps 19 and 20, avatars included.

## Open questions
None for now.
