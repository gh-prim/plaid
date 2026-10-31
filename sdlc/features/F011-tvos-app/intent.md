---
id: F011
title: Apple TV app
owner: gh-prim
reviewer: gh-prim
status: accepted
phase: 4
apps: [server, apple]
created: 2026-10-31
---

# F011 — Apple TV app

## Problem
PLAID is only watchable in a browser. On the living-room TV, the place people actually watch movies and shows, there is no PLAID app: you have to cast a laptop or go back to Plex. The server can't be found on the TV either: nothing tells a TV app where it is without typing an address with a remote.

## Expected outcome
On an Apple TV, you open PLAID, it finds the server on the home network, you pair it with a code from the web app, and you browse and play your library with the same look, flow and features as the web app, the way you would in Apple's own TV app.

## Users and systems affected
- Members of a server (and their profiles) watching on an Apple TV.
- Server admin: nothing to set up; the server announces itself on the local network.
- Apps: `server` (local network discovery, intro/credits timestamps), `apple` (new tvOS app).

## Success criteria
- [ ] A fresh Apple TV on the same network lists the server without any address typed, and pairs with the existing TV pairing code (F003-R20).
- [ ] The home, libraries, search, title pages, person and collection pages, and settings match what the web app offers (F008).
- [ ] Playback works on Apple TV (HLS, direct or transcoded per F006), resumes where you left off, and remembers audio and subtitle choices.
- [ ] At the end of an episode, the next episode is offered before the credits run, on every episode, whether or not anyone has published timestamps for it.
- [ ] The intro can be skipped when its timestamps are known.
- [ ] The UI follows the profile's language (English, French).
- [ ] What you're watching shows up in the Apple TV's Top Shelf.

## Constraints
- tvOS 18+, SwiftUI, focus-engine navigation with the Siri Remote only.
- The server stays light on modest hardware (Pi, NAS): any media analysis runs at low priority, once per file.
- No new user-facing string is hardcoded (String Catalogs, en + fr).
- No third-party service is required for the app to work; external lookups degrade gracefully.

## Out of scope
- iPhone, iPad and macOS apps (rest of phase 4).
- Playing trailers inside the app: YouTube can't be embedded without bypassing its protections; the trailer button opens the YouTube app instead.
- Series theme music (no usable source).
- App Store distribution, TestFlight, SharePlay, Plaid Party on TV.
- Remote access outside the home network (ADR 0003).

## Open questions
- [x] Where do intro and credits timestamps come from? → TheIntroDB, plus local credits detection (ADR 0008).
- [x] How does the TV find the server? → Bonjour `_plaid._tcp`.
