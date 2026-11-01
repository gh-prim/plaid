---
id: F012
title: Remote access through getplaid.direct
owner: gh-prim
reviewer: gh-prim
status: done
spec: ./spec.md
branch: feat/F012-remote-access
---

# F012 — Plan: Remote access through getplaid.direct

> Written after the build, at the user's request: it records what was done.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` (+ generated files) | modify | `/remote-access`, `RemoteAccess*` schemas, `ServerInfo.remoteUrl` (R1, R14) |
| `server/internal/remoteaccess/` | create | port mapping, getplaid.direct client, ACME issuer, LAN address, service loop (R2–R6) |
| `server/internal/handlers/remote_access.go` | create | admin endpoints (R1, R3) |
| `server/internal/settings/settings.go` | modify | remote access keys |
| `server/internal/config/config.go` | modify | `PLAID_TLS_PORT`, `PLAID_DIRECT_URL`, `PLAID_ACME_DIRECTORY`, `PLAID_APP_ORIGINS` |
| `server/internal/httpserver/cors.go`, `internal/auth/middleware.go` | create / modify | CORS and CSRF for the hosted app (R11) |
| `server/cmd/plaid/main.go` | modify | wire the service, HTTPS listener (R4) |
| `services/direct/` | create | getplaid.direct: API, DNS, store, probe, rate limits, hosted app, Dockerfile, PROTOCOL.md (R7–R10) |
| `web/src/admin/RemoteAccess.tsx` and routes | create | admin page (R1–R3) |
| `web/src/api/server.ts`, `web/src/hosted/` | create | hosted mode, gate, remote sign-in (R12, R13) |
| `web/src/api/client.ts`, media and player files | modify | server URLs in hosted mode (R12) |
| `services/site/` | create | getplaid.tv (R15) |
| `.github/workflows/services.yml`, `ci.yml` | create / modify | CI for services |
| `docs/install.md`, `packaging/`, `Dockerfile`, `docker-compose.yml` | modify | docs and port 32501 |
| `sdlc/decisions/0009-…` | create | supersedes 0003 |

## Steps, in order
1. [x] OpenAPI contract and codegen (R1, R14).
2. [x] `remoteaccess`: NAT-PMP / UPnP mapper with port fallback (R2, R3).
3. [x] getplaid.direct client and protocol, then the service: API, store, DNS, probe, rate limits (R5, R7, R8).
4. [x] ACME DNS-01 issuer, certificate storage and renewal, HTTPS listener with self-signed fallback (R4, R6).
5. [x] Service loop with states and error codes; admin endpoints; `remoteUrl` (R1, R14).
6. [x] Admin page in the web app, en/fr.
7. [x] Deploy getplaid.direct and getplaid.tv on Coolify; delegate the zone (NS + glue) at the registrar.
8. [x] End-to-end test on a real network (NAT-PMP, Let's Encrypt, access from the VPS).
9. [x] LAN address in the heartbeat, `connections` endpoint (R5, R9).
10. [x] CORS and CSRF for the hosted app; hosted build served on the short link (R10, R11).
11. [x] Hosted mode in the web app: find the server, route URLs, cookie (R12).
12. [x] Remote sign-in, V2 step 15 (R13).
13. [x] getplaid.tv rebuilt on the PLAID website design, protocol and privacy pages (R15).

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Router without UPnP / NAT-PMP | medium | manual mode, clear error code |
| No NAT loopback at home | high | LAN connection in the hosted app |
| Let's Encrypt rate limits on getplaid.direct | low | per-server certificates, wildcard per host, renewal only 30 days before expiry |
| Chrome blocks local network requests | medium | CORS private-network preflight; Chrome's one-time prompt documented |
| CGNAT | medium | documented: use a VPN |

**Riskiest step:** the hosted app's cross-origin session (step 10–11): it depends on getplaid.direct staying one site (not on the PSL).

## Rejected options
- Short link as a plain redirect: works, but at home without NAT loopback it needs a redirect to an IP-shaped name, and the address bar changes. Kept only as the fallback when the app isn't built in.
- Bearer tokens in the hosted app: images and EventSource can't send headers; the shared-site cookie works for all of them.
- A relay for CGNAT: media through our infrastructure (ADR 0009, option C).

## Expected evidence
- [x] Tests: `remoteaccess` (fake mapper, directory and issuer), handlers, CORS and CSRF, getplaid.direct API (registration, LAN validation, connections, hosted app, short links), DNS, store, probe; web: admin page, `api/server.ts`, remote sign-in.
- [x] `make test` green.
- [x] Screenshots: admin page, hosted gate, remote sign-in desktop and tablet, getplaid.tv.
- [x] Manual check: real network test on 2026-11-01 (NAT-PMP mapping, certificate, public resolvers, access from the VPS with certificate verification, LAN connection from Chrome at home, turn-off cleanup).

## Log
- 2026-10-31: SDLC skipped at the user's request; code first, documents afterwards.
- 2026-11-01: the box under test has no NAT loopback: added the LAN address and, at the user's request, the Plex-style hosted app on the short link instead of a redirect.
- 2026-11-01: the user redesigned the remote sign-in (V2 step 15); implemented as designed.
