---
id: F014
title: "iPhone and iPad app"
owner: gh-prim
reviewer: gh-prim
status: accepted
phase: 4
apps: [apple, web]
created: 2026-11-02
---

# F014 — iPhone and iPad app

## Problem
PLAID plays on the web and on Apple TV (F011), but not on the device people always have with them. On an iPhone or an iPad, the only way in is the web app in Safari: no Liquid Glass, no Picture in Picture, no AirPlay picker, no lock-screen controls, and the player goes away when the screen locks. Pairing a TV also means typing its code into a web page.

## Expected outcome
A native SwiftUI app for iPhone and iPad that matches step 17 of the "PLAID Design" canvas (page V2, boards IPh01–IPh16 and IPa01–IPa15). It does everything the Apple TV app and the web do for viewers: find and sign in to a server, pick a profile, browse, search, play, and pair a TV. Pairing gets faster: the TV shows a QR code that the app (or the phone's camera, through the web) reads.

## Users and systems affected
- Any viewer with an account, on iPhone (iOS 26+) or iPad (iPadOS 26+).
- `apple/`: a new `PLAID-iOS` target; the code the TV and the phone share moves to `apple/Common/`.
- Apple TV (F011): its pairing screen shows a QR code next to the code.
- Web: `/pair?code=…` comes with the code already typed.
- Server: no change. The app uses the API as it is.

## Success criteria
- [ ] The screens look like the step 17 boards on iPhone 17 Pro and iPad Pro 11".
- [ ] A viewer finds a server on the local network or by its getplaid.direct code, signs in, picks a profile (with its PIN), and plays a movie and the next episode of a show.
- [ ] Subtitles, audio track, quality and speed can be changed while playing; the picture goes to an Apple TV through AirPlay and into Picture in Picture.
- [ ] Every viewer feature of the Apple TV app and the web is there (list in the spec).
- [ ] A TV pairs from the app by scanning its QR code, or by typing its code.
- [ ] Every string in English and French.

## Constraints
- No API change.
- One code base for iPhone and iPad; the TV app keeps working, built from the same shared code.
- Dark only, as the boards.

## Out of scope
- Administration (libraries, users, devices, remote access, matching, artwork): the web does it.
- A light theme.
- Plaid Party (step 18, F010) and downloads for offline viewing.
- Home-screen widgets and Live Activities.

## Open questions
None left.
