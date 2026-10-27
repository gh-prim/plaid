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
Right after installation, the admin lands on a blank page. They should be able to configure everything from their browser, without touching a config file. Later, when a friend says "it's buffering" or a disk disappears, they have no way to see what's wrong or fix it without reading logs over SSH.

## Expected outcome
A first-run wizard that takes the admin from a blank server to their own posters on screen in a few minutes: language, admin account, libraries picked from the server's folders, then a first scan you watch fill the wall with posters. After that, an admin area that answers "is something wrong, and what do I do about it" first, then lets you manage everything.

## Users and systems affected
- Admin, usually on the computer where they just installed PLAID, sometimes on a phone.
- Web; server API from F003, F004, F005 and F006.

## Design
Screens are designed in Claude Design, canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz) (pages Onboarding and Admin), from the prompts in [claude-design-prompt.md](claude-design-prompt.md). Each page holds an EN/FR copy deck, the directions explored and the one picked, every screen at 375 and 1440 px, locale checks and the proposed design system additions.

## Success criteria
- [ ] Wizard, in this order: language (sets the UI, the admin's locale and the default metadata language) → admin account → libraries (server-side folder picker) → first scan.
- [ ] The setup code is asked only when setup comes from outside the local network (F003-R3).
- [ ] An unreadable folder says why and gives the command to fix it, with a way to check again.
- [ ] Real-time scan progress, where matched posters fill the screen; the admin can leave at any time and the scan continues.
- [ ] Admin home: the server's state in one sentence, what needs the admin's attention only when something does, then the sections. Reached from the library.
- [ ] Admin: active streams with why each one is converted, libraries (folders, unrecognized files, items to review), people (users, profiles, signed-in devices, TV pairing), transcoding (detected hardware acceleration), metadata providers, problems.
- [ ] Every screen works at 375 px and 1440 px, in English and French.

## Constraints
- Usable on a phone as well as a computer.
- No new user-facing string outside i18n; the copy decks are the source for the keys.
- Copy decks replace two existing strings that break the design system's rules: F003 `setup.title` and F005 `metadata.state.pending`.

## Out of scope
- Managing sharing with friends (F009).
- Inviting people and installing the TV and phone apps: the wizard doesn't point to them until they exist.

## Open questions
- [ ] **Server name**: `GET /server` returns one, but nothing lets the admin set it. Does onboarding name the server? It matters for F009.
- [ ] **Live posters during the scan**: scan events only carry counters. How does the web get the items as they're matched (polling items, or a new event)?
- [ ] **Problems screen**: no logs API exists. Add one in F007 (what happened, what to do), or ship without the screen?
- [ ] **Server health**: the design proposes two warnings only, low disk space where PLAID keeps its data and a Pi throttling because it's hot. Both need an API. In v1 or later?
- [ ] **Setup code**: does it change at each start? The copy says "the latest one in the log", true either way.
- [ ] **`validation_failed` codes**: F003 doesn't list `details[].code`; the copy assumes `required`, `too_long`, `invalid`.
- [ ] **Design system additions**: the components proposed by the design (folder row, scan thread, stream card…) go into F002 before the code, or live in the web app?
