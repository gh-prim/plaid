---
id: F013
title: "Web: Pair a TV in the V2 look"
owner: gh-prim
reviewer: gh-prim
status: approved
intent: ./intent.md
decisions: []
---

# F013 — Spec: Web: Pair a TV in the V2 look

## Summary
`/pair` is redrawn after step 16 of the "PLAID Design" canvas (page V2, boards TPair01–TPair06). The flow is F003-R20's, unchanged: type the code the TV shows, pick the profile it opens on, give that profile's PIN if it has one, claim. What changes is how it looks and how each input behaves.

## User journeys
1. Alex opens PLAID on the Apple TV, which shows `K7P-4QX`. On the phone, Alex opens Profile menu ▸ Pair a TV.
2. The page opens with the code field focused. As Alex types, each character lands in a glass box and on the TV drawn above. Characters a code can't have (`0 O 1 I L`, spaces, the dash) are dropped.
3. Alex's profile is picked by default. Alex taps Mia's face instead: Mia has a PIN, so a "Mia's PIN" pill with four dots appears under the faces and takes the focus.
4. Alex types the PIN; "Pair" turns cream. Alex presses it (or Enter).
5. The TV drawn on the page lights up with Mia's colors, a check and "Good evening, Mia". The page says "Apple TV is connected." with "Pair another TV" and "Done".

Errors:
- The code expired or no TV shows it: the six boxes shake and turn red, the server's message replaces the hint in a glass pill. Typing again clears it.
- Wrong PIN or locked profile: only the PIN pill shakes; it empties and keeps the focus. The code stays as typed.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F013-R1 | The code is one text field drawn as six boxes (three, a dash, three). Input is uppercased and filtered to the server's alphabet (`ABCDEFGHJKMNPQRSTUVWXYZ23456789`), at most 6 characters; pasting `k7p-4qx` gives `K7P4QX`. The box after the last character shows a caret while the field has the focus. | must |
| F013-R2 | A TV drawn above the title mirrors the code as typed, empty slots as dots. | should |
| F013-R3 | The profiles are a radio group ("It will open on") of faces with their names, the active profile picked by default; profiles with a PIN carry a lock. | must |
| F013-R4 | Picking a profile with a PIN shows a PIN pill (one numeric password field, four dots) and focuses it. It isn't focused on load or after "Pair another TV", so the code keeps the focus. | must |
| F013-R5 | "Pair" stays disabled until the code has 6 characters and, if needed, the PIN 4 digits; it shows a spinner while the claim is in flight. | must |
| F013-R6 | A failed claim shakes the field it's about: `invalid_pin` and `profile_locked` on the PIN pill (emptied, message under it), anything else on the code boxes (red, message in a pill instead of the hint). Editing that field clears the error. | must |
| F013-R7 | After a claim, the "connected" screen shows the TV with the profile's ambient backdrop (when it has one), a check and a greeting by time of day, the title "{device} is connected.", where it opens, and for admins where to find it (Settings, Devices). "Pair another TV" goes back to an empty code; "Done" goes home. | must |
| F013-R8 | A top bar with a glass "Home" pill and the active profile's face. The page is lit by the picked profile's ambient backdrop, like the profile picker. | should |
| F013-R9 | At phone width (< 768 px) the same page in one column: smaller TV and boxes (44 × 51), the done buttons stacked. | must |

## Design

### Data model
None.

### API
None. `POST /pairing/claim` (F003-R20) as is; `Profile.ambient` from `GET /me` (V2) lights the page and the done TV.

### Behavior per app
- **server**: unchanged.
- **web**: `pages/Pair.tsx` rewritten; styles in `browse/browse.css` under `.pa__*`, reusing the profile picker's `.pf` page, faces and PIN dots.
- **apple / android**: out of scope.

### UI / Art direction
Canvas "PLAID Design", page V2, row "Step 16. Pair a TV (web, /pair)": TPair01 typing, TPair02 profile with PIN, TPair03 code expired, TPair04 done, TPair05–06 phone. Deliberate differences: the web's Instrument Sans instead of Bricolage Grotesque, the profiles' party colors instead of the board's pastels, and the theme's `--status-error` for errors. The TV screen stays dark in the light theme.

### i18n
| Key | en | fr |
|-----|----|----|
| `pairing.lead` | Open PLAID on the TV, then type the code it shows here. | Ouvre PLAID sur la TV, puis tape ici le code qu’elle affiche. |
| `pairing.codeHint` | 6 characters, no 0, O, 1, I or L. The code expires after 10 minutes. | 6 caractères, sans 0, O, 1, I ni L. Le code expire au bout de 10 minutes. |
| `pairing.startProfile` (changed) | It will open on | Elle s’ouvrira sur |
| `pairing.pin` | {{name}}’s PIN | Code du profil de {{name}} |
| `pairing.hello.morning/afternoon/evening` | Good morning/afternoon/evening, {{name}} | Bonjour / Bon après-midi / Bonsoir {{name}} |
| `pairing.done.title` (was `pairing.done`) | {{device}} is connected. | {{device}} est connectée. |
| `pairing.done.profile` | It opens on {{name}}’s profile. | Elle s’ouvre sur le profil de {{name}}. |
| `pairing.done.devices` | You’ll find it in Settings, Devices. | Tu la retrouves dans Réglages, Appareils. |
| `pairing.done.another` | Pair another TV | Associer une autre TV |
| `pairing.done.finish` | Done | Terminé |

`profiles.enterPin`, used only by the old page, is removed.

## Non-functional requirements
- Accessibility: one labelled field for the code and one for the PIN (screen readers announce each once, paste and password managers work); profiles are real radio inputs in a `fieldset`; errors are `role="alert"`; the done title is `role="status"`.
- Motion: the entrance rises in with `--ease-glide`; `prefers-reduced-motion` stops the glow and the caret blink.
- No horizontal scroll at 390 px.

## ⚠️ Watch-outs
- [x] A profile with a PIN has no `ambient` (the server hides what it watches), so its done TV is the plain dark screen. Accepted.
- [x] The done title names the TV ("Apple TV is connected."): the claim response already carries `deviceName`, so no API change.
- [x] `PinInput` (`components/PinInput.tsx`) loses its only user. Left in place for now; removing it is a separate cleanup.

## Acceptance criteria
- [ ] Given the page, when I type `k0o-1il7p4qx9`, then the field holds `K7P4QX`.
- [ ] Given a profile without a PIN and a valid code, when I press Pair, then the claim sends `{code, profileId}` and the page says "{device} is connected." and where it opens.
- [ ] Given a profile with a PIN, when I pick it, then Pair stays disabled until 4 digits, and the claim sends the PIN.
- [ ] Given a wrong PIN, when I press Pair, then the alert is under the PIN, the PIN field is empty and the code field isn't marked invalid.
- [ ] Given an expired code, then the alert reads "This code expired. Show a new one on the TV."
- [ ] Given the done screen, when I press "Pair another TV", then the code field is empty.

## Revisions
- 2026-11-01: written after the code, at the owner's request, and reviewed with it in one PR.
