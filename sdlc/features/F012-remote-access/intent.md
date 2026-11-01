---
id: F012
title: Remote access through getplaid.direct
owner: gh-prim
reviewer: gh-prim
status: accepted
phase: 3
apps: [server, web]
created: 2026-10-31
---

# F012 — Remote access through getplaid.direct

## Problem
Watching away from home, or letting a friend watch, means typing an IP address and a port, opening the router by hand and accepting a certificate warning. Most households give up. Sharing (F009) and Plaid Party (F010) can't work until a server is easy to reach from outside.

## Expected outcome
The admin flips one switch. PLAID opens a port on the router, gets a name like `living-room-k3x7qa.getplaid.direct` with a real certificate, and shows one short link, `getplaid.direct/k3x7qa`. Anyone who opens it lands on the server's sign-in, at home or away, and the video goes straight from the server to their screen.

## Users and systems affected
- Server admin: turns remote access on and off, picks automatic or manual port, sees each step's state.
- Members and invited friends: open the short link from any browser.
- Server (port mapping, registration, certificate, HTTPS listener, CORS), web app (admin page, hosted mode, remote sign-in), a new service getplaid.direct (API, authoritative DNS, hosted app), the public site getplaid.tv.

## Success criteria
- [x] With UPnP or NAT-PMP on the router, turning remote access on reaches "ok" with no other action.
- [x] The hostname resolves on public resolvers and serves a Let's Encrypt certificate for it and its subdomains.
- [x] From outside the home network, the short link reaches the server over HTTPS with a valid certificate.
- [x] At home, the short link reaches the server through its LAN address, even when the router has no NAT loopback.
- [x] Turning remote access off closes the port and removes the name within seconds.
- [x] Video never goes through getplaid.direct.

## Constraints
- Opt-in, off by default. No account with us; nothing about libraries, accounts or viewing is sent.
- Runs on a Raspberry Pi: the certificate is issued once and renewed 30 days before expiry, the heartbeat is every 15 minutes.
- getplaid.direct stores the minimum (ADR 0009) and runs on the project's Coolify server.

## Out of scope
- Working behind CGNAT (needs a relay; the docs point to a VPN).
- A Plaid Hub with global identities.
- Native apps using the hosted connection logic (the Apple TV app finds the server over Bonjour at home).

## Open questions
- [x] Pure self-host or a service? Settled by ADR 0009.
- [ ] Public Suffix List for getplaid.direct: blocked by the PSL's user threshold, and it would break the shared session cookie; revisit with a new ADR.
