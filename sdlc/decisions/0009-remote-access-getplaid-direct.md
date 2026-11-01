---
id: 0009
title: Remote access through getplaid.direct
status: accepted
date: 2026-11-01
deciders: [gh-prim]
supersedes: 0003
---

# 0009 — Remote access through getplaid.direct

## Context
ADR 0003 proposed pure self-host for v1: users expose their server themselves (reverse proxy, Tailscale, tunnel). In practice that leaves most households with an IP and a port to type, a self-signed certificate, or no remote access at all, and sharing (F009) and Plaid Party (F010) both need friends to reach the server. Plex solves it with plex.tv and plex.direct; we want the same ease without becoming a relay or an identity provider.

## Options
### A. Pure self-host (ADR 0003)
- ✅ No infrastructure, maximum privacy.
- ❌ An IP and a port to type, no valid certificate, too hard for most users.

### B. A small name and certificate service: getplaid.direct
Each server opens a port on its router (UPnP / NAT-PMP, or manual), registers with getplaid.direct, gets `<name>-<id>.getplaid.direct` pointing at its public IP and its own Let's Encrypt certificate through DNS-01. The short link `getplaid.direct/<id>` serves the web app, which connects to the server's LAN address at home and its public address elsewhere.
- ✅ One link to share, a real certificate, works at home even without NAT loopback.
- ✅ Video, accounts and libraries never touch our service; the certificate's private key stays on the server.
- ❌ A service to run (DNS, a small API, a static page) and a small amount of data per server.

### C. A full Hub with a relay (Plex-style plex.tv)
- ✅ Works behind CGNAT.
- ❌ Media through our infrastructure: bandwidth cost, privacy, operating burden. Too early.

## Decision
We choose **B**. Remote access is opt-in, off by default. getplaid.direct stores per server only: id, slug, token hash, public IPv4/IPv6, external port, version, dates, current ACME values and the optional LAN address. Self-hosters can still use their own domain, reverse proxy or tunnel with getplaid.direct turned off.

## Consequences
- getplaid.direct (services/direct) and getplaid.tv (services/site) live in this repository and are deployed on Coolify.
- Behind CGNAT, remote access can't work; the docs point to a VPN. A relay stays a possible later feature (Plaid Hub).
- The hosted app and the servers share the site getplaid.direct, so the session cookie works across them; adding getplaid.direct to the Public Suffix List would break that and needs a new decision.
- F009 and F010 can rely on a stable HTTPS URL per server.
