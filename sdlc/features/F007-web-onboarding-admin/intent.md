---
id: F007
title: Web — onboarding and administration
owner: TBD
reviewer: TBD
status: draft
phase: 2
apps: [web]
created: 2026-10-26
---

# F007 — Web: onboarding and administration

## Problem
Right after installation, the admin lands on a blank page. They should be able to configure everything from their browser, without touching a config file.

## Expected outcome
A warm first-run wizard ("Get comfy, we'll handle the rest" ("Installe-toi, on s'occupe du reste")): admin account creation, adding folders, language, then a first scan you can follow live. After that, an admin area to manage everything.

## Users and systems affected
- Admin.
- Web; server API from F003, F004, F005 and F006.

## Success criteria
- [ ] Wizard: admin account → libraries (server-side folder picker) → language → first scan.
- [ ] Real-time scan progress.
- [ ] Admin: users, libraries, transcoding (detected hardware acceleration), connected devices, active streams, logs.

## Constraints
Usable on mobile: people often set up a server from their couch.

## Out of scope
Managing sharing with friends (F009).

## Open questions
- [ ] Show server health (CPU, disk, GPU) as early as v1?
