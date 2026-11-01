---
id: F013
title: "Web: Pair a TV in the V2 look"
owner: gh-prim
reviewer: gh-prim
status: done
spec: ./spec.md
branch: feat/F013-pair-tv-v2
---

# F013 — Plan: Web: Pair a TV in the V2 look

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `web/src/pages/Pair.tsx` | rewrite | The V2 page: `Tv`, `CodeBoxes`, `PinPill`, `Done` (R1–R9) |
| `web/src/browse/browse.css` | edit | `.pa__*` section at the end, phone rules under 768 px |
| `web/src/pages/Pair.test.tsx` | edit | Radio group, PIN pill, filtering, wrong PIN, pair another |
| `web/src/locales/{en,fr}/common.json` | edit | `pairing.*` keys of the spec; `profiles.enterPin` removed |

## Steps, in order
1. [x] Rewrite `Pair.tsx` on the profile picker's `.pf` page with `PageLight` lit by the picked profile's `ambient` (R8). The `Face` circle is copied from `ProfilePicker.tsx` (8 lines) rather than exported, like the picker keeps its own.
2. [x] `CodeBoxes`: one invisible text field over six boxes, value uppercased, filtered with `/[^ABCDEFGHJKMNPQRSTUVWXYZ2-9]/g`, cut at 6; caret box while focused (R1). `CodeGlyphs` echoes it on the drawn TV (R2).
3. [x] Profiles as `<fieldset>` + radio inputs hidden with `br-visually-hidden`, faces as labels (R3). `PinPill` mounts when the picked profile has a PIN; `autoFocus` only once a profile was picked by hand (`focusPin`), reset by "Pair another TV" (R4).
4. [x] Submit: disabled until ready, `Spinner` while busy (R5). Errors split by `isPinFailure` (`invalid_pin`, `profile_locked`): separate remount counters (`codeTry`, `pinTry`) replay the shake on the right field; a PIN failure also empties the PIN (R6).
5. [x] `Done`: TV lit (`is-lit`), the profile's ambient at 780 px, `pa-pop` check, greeting by hour (before 5 or from 18 evening, before 12 morning, else afternoon), admin-only devices line, "Pair another TV" and "Done" (R7).
6. [x] CSS `.pa__*` from the boards' measures (TV 330 × 184, boxes 64 × 75, faces 60, CTA 440 × 56), colors from theme tokens so the light theme follows; phone rules (R9). The top bar's face overrides the picker's phone `--pf-s` with `!important`.
7. [x] i18n en + fr, `make test`.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| The picker's phone rules (`.pf__face { --pf-s: … !important }`) leak into `/pair` | Seen once | Explicit `--pf-s … !important` on `.pa__top` and `.pa__circle` faces |
| `backdrop-filter` dropped in the prod build | Low | Only the unprefixed property is written (see the CSS build note in memory/CLAUDE) |
| Autofocus fights between the code and the PIN | Seen once | `focusPin` state, covered by the "pair another TV" test |

**Riskiest step:** step 4, keeping the error on the right field without losing what was typed in the other.

## Rejected options
- Six separate inputs for the code: screen readers announce six fields, paste needs custom code.
- Keeping the `<select>` for profiles: the canvas shows faces, and radios keep it keyboard-accessible.
- Bricolage Grotesque from the boards: a new web font for one page; the web stays on its fonts (F002).

## Expected evidence
- [x] Tests added: filtering, PIN profile via radio, wrong PIN stays on the PIN, pair another TV (6 tests in `Pair.test.tsx`)
- [x] `make test` green
- [x] Screenshots at 1440 and 390 compared to TPair01–TPair06 (typing, PIN, error, done, phone)
- [x] Manual check: a real Apple TV paired on srv-plex-prod (build `0.1.19+local.11012135`), and a code claimed on a local test server on a profile with a PIN

## Log
- 2026-11-01: the code was written first, at the owner's request, then this intent, spec and plan; all four ship in one PR.
- 2026-11-01: `PinInput` is no longer used by any page; it stays for now (owner's call), to be removed in a cleanup.
