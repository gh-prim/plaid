---
id: F013
title: "Web: Pair a TV in the V2 look"
owner: gh-prim
reviewer: gh-prim
status: accepted
phase: 4
apps: [web]
created: 2026-11-01
---

# F013 — Web: Pair a TV in the V2 look

## Problem
Pairing the Apple TV (F011) goes through `/pair` on the web, and that page is still the F003 placeholder: a bare text field, a `<select>` of profiles and a plain button, on a white form. Every other screen around it (profile picker, home, title pages) moved to V2: glass, a page lit by colors, generous type. Pairing is the first thing someone does after installing the TV app, and it looks unfinished.

## Expected outcome
`/pair` matches step 16 of the "PLAID Design" canvas (page V2): a drawn TV that echoes the code as it's typed, the code in six glass boxes, the profile the TV opens on as faces, its PIN inline, and a "connected" screen with the TV lit.

## Users and systems affected
- Any signed-in user who pairs a TV (admin or member).
- Web only: `web/src/pages/Pair.tsx` and its styles. The pairing API (F003-R20) doesn't change.

## Success criteria
- [ ] `/pair` at 1440 and 390 px looks like the canvas boards TPair01 to TPair06.
- [ ] A real Apple TV pairs from it, on a profile with and without a PIN.
- [ ] An expired or unknown code and a wrong PIN each show their error where it belongs.
- [ ] Every string in English and French.

## Constraints
- No API change. Only what `GET /me` and `POST /pairing/claim` already return.
- Same accessibility as the other V2 screens: one real field per input, labels, keyboard only.

## Out of scope
- The TV side of pairing (F011).
- Bricolage Grotesque, used by the canvas boards: the web keeps its own fonts (F002).
- A per-profile backdrop for profiles that don't have one yet.

## Open questions
None left.
