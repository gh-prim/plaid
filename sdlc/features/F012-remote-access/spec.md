---
id: F012
title: Remote access through getplaid.direct
owner: gh-prim
reviewer: gh-prim
status: approved
intent: ./intent.md
decisions: [0009]
---

# F012 — Spec: Remote access through getplaid.direct

## Summary
An admin turns on remote access in Settings. The server maps a port on the router (NAT-PMP, then UPnP; or a manual port), serves HTTPS on `PLAID_TLS_PORT` (32501), registers with getplaid.direct and gets `<slug>-<id>.getplaid.direct` pointing at its public IP, then a Let's Encrypt certificate for that name and its wildcard through DNS-01, published by getplaid.direct. getplaid.direct checks from outside that the server answers. The short link `getplaid.direct/<id>` serves the PLAID web app (hosted mode): it asks getplaid.direct for the server's connections, tries the LAN one first and the public one after, keeps the first that answers and runs against it, with the session cookie. The service's contract is `services/direct/PROTOCOL.md`.

## User journeys
1. **Admin turns it on.** Settings → Remote access → switch on. Steps show pending, then ok: port mapping (method shown), certificate (expiry), reachability. The short link appears with a Copy button. If the router refuses, the step says why and the admin can choose Manual and type the external port they forwarded to 32501.
2. **Friend away from home.** Opens `getplaid.direct/k3x7qa` → "Connecting to the server…" → the remote sign-in (server name over a film still) → signs in → the app, URL still `getplaid.direct/k3x7qa/…`. Playback streams from the server's public address.
3. **Member at home.** Same link; Chrome asks once to let getplaid.direct reach the local network; the app connects to `a-b-c-d.<label>.getplaid.direct` (the LAN address, same certificate). The video stays in the house.
4. **Admin turns it off.** The port mapping is removed, the registration deleted: the short link answers 404, the name NXDOMAIN.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F012-R1 | Remote access is off by default and only an admin can change it (`GET/PATCH /remote-access`, `POST /remote-access/test`). | must |
| F012-R2 | In automatic mode the server maps its HTTPS port with NAT-PMP, else UPnP IGD (WANIPConnection2/1, WANPPPConnection1), on the same external port when free, else a random high one; the lease is 1 h, renewed every 15 min, removed when turned off. | must |
| F012-R3 | In manual mode the admin gives the external port; the router is not touched. A manual mode without a port is a 422. | must |
| F012-R4 | The server serves HTTPS on `PLAID_TLS_PORT` (default 32501, 0 disables), with the issued certificate, or a self-signed one until it exists. | must |
| F012-R5 | The server registers with getplaid.direct (name, external port, version, LAN IPv4 and HTTPS port when private), keeps the id and token in settings, and heartbeats every 15 min; an unknown registration registers again. | must |
| F012-R6 | The server gets a Let's Encrypt certificate for `host` and `*.host` through DNS-01 published by getplaid.direct, stores it with its key (0600) under the data dir, and renews it 30 days before expiry, backing off 1 h up to 24 h on failure. | must |
| F012-R7 | getplaid.direct takes the public IP from the request's source only, stores the token as SHA-256, rate-limits registration (5/h per IP, /64 for IPv6) and reachability checks (10/h per server), and removes servers unseen for 30 days. | must |
| F012-R8 | getplaid.direct is the authoritative DNS for the zone: A/AAAA for each label, `a-b-c-d.<label>` only for private, CGNAT and link-local addresses, `_acme-challenge` TXT values, CAA for letsencrypt.org, NS with glue. | must |
| F012-R9 | `GET /servers/{id}/connections` lists the LAN connection (when sent) then the public one. | must |
| F012-R10 | `GET getplaid.direct/<id>` and `/<id>/<route>` serve the hosted web app with a CSP limited to the zone; `/<label>` redirects to `/<id>`. Without the app built in, the link redirects to the server (to the LAN name for a visitor behind the server's public IP). | must |
| F012-R11 | The server answers CORS with credentials, and private-network preflights, only for `PLAID_APP_ORIGINS` (default `https://getplaid.direct`); the CSRF check accepts that origin. | must |
| F012-R12 | The hosted app finds the server (LAN first with a head start), then sends every API call, image, preview, subtitle and stream to it with the session cookie, under the router basename `/<id>`. It shows a loading state, and an error with Retry when no connection answers. | must |
| F012-R13 | On the short link, sign-in is the remote sign-in screen (V2 step 15, boards TR01–TR04). | must |
| F012-R14 | `GET /server` returns `remoteUrl` when remote access is ok. | should |
| F012-R15 | getplaid.tv explains the feature, the protocol and what is stored (privacy page). | should |

## Design

### Data model
- Server settings: `remote.enabled`, `remote.port_mode`, `remote.manual_port`, `remote.external_port`, `remote.direct.{id,token,hostname,url,short_url}`. Certificate and ACME account key under `<data>/remote/`.
- getplaid.direct (SQLite): `servers` (id, slug, token_hash, ipv4/ipv6 with seen times, port, version, created_at, last_seen, lan_ip, lan_port) and `acme_values` (server_id, value, expires_at).

### API
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/remote-access` | State: enabled, mode, ports, hostname, URLs, public IP, steps (admin) |
| PATCH | `/api/v1/remote-access` | Turn on/off, choose mode and manual port (admin) |
| POST | `/api/v1/remote-access/test` | Run the reachability check again (admin) |
| GET | `/api/v1/server` | Adds `remoteUrl` |

getplaid.direct (`https://getplaid.direct/api/v1`, see PROTOCOL.md): `POST /servers`, `PUT/DELETE /servers/{id}`, `PUT/DELETE /servers/{id}/acme-challenge`, `POST /servers/{id}/reachability`, `GET /servers/{id}/connections`, `GET /ip`.

### Behavior per app
- **server**: `internal/remoteaccess` (port mapping, getplaid.direct client, ACME issuer, service loop), HTTPS listener, CORS middleware, settings, handlers.
- **web**: admin page Settings → Remote access; hosted mode (`pnpm build:hosted`, base `/app/`): `HostedGate`, `api/server.ts`, remote sign-in.
- **services/direct**: Go service with API, DNS, store, probe; embeds the hosted app.
- **services/site**: getplaid.tv, static, en/fr.
- **apple / android**: out of scope (the Apple TV app finds the server over Bonjour).

### UI / Art direction
- Admin page: switch, short link in a command block, three steps with states, automatic/manual port.
- Hosted gate: the onboarding card with "Connecting to the server…" and a spinner; unknown link and unreachable server cards with Retry.
- Remote sign-in: V2 step 15 (TR01 empty, TR02 wrong password with shake, TR03 signing in, TR04 phone).

### i18n
New keys in `remoteAccess.*` (admin page), `admin.nav.remoteAccess`, `errors.remote_access_off`, `hosted.*` (finding, retry, unknown, unreachable) and `remoteSignIn.*` (kicker, lead, hint, signingIn, forgot, forgotShort, secure, credit), in en and fr. The server name and the short id are never translated.

## Non-functional requirements
- No media through getplaid.direct; the certificate key never leaves the server.
- Heartbeat 15 min, DNS TTL 60 s: a new home IP is followed within about 15 minutes.
- The hosted page's CSP allows only `https://*.getplaid.direct:*` besides itself.

## ⚠️ Watch-outs
- [x] Routers without NAT loopback: solved by the LAN connection in the hosted app (R9, R12).
- [x] Chrome's local network access prompt at home: documented in docs/install.md; it is shown once per browser.
- [x] Cookies across the app and servers rely on getplaid.direct not being on the PSL: recorded in ADR 0009.
- [x] The LAN IP is public through `connections`: private addresses only, same as the DNS LAN names; listed on the privacy page.
- [ ] CGNAT: can't work; documented, a relay is a later feature.

## Acceptance criteria
- [x] Given UPnP or NAT-PMP, when the admin turns remote access on, then port mapping, certificate and reachability reach ok.
- [x] Given remote access on, when anyone resolves the hostname on a public resolver, then it returns the server's public IP.
- [x] Given remote access on, when a client outside the LAN opens the short link, then it reaches the server over HTTPS with a valid certificate.
- [x] Given a router without NAT loopback, when a member at home opens the short link, then the app connects through the LAN name.
- [x] Given a cookie session from the hosted app, when another origin sends a state-changing request with it, then the server answers 403.
- [x] Given remote access on, when the admin turns it off, then the short link answers 404, the name NXDOMAIN and the port is closed from outside.

## Revisions
- 2026-11-01: written after the implementation, at the user's request; the short link went from a redirect to the hosted app (Plex-style) during the build.
