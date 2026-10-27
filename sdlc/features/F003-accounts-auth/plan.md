---
id: F003
title: Accounts and authentication
owner: gh-prim
reviewer: TBD
status: done             # draft | approved | in-progress | done
spec: ./spec.md
branch: feat/F003-accounts-auth
---

# F003 — Plan: Accounts and authentication

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

**Starting point (checked 2026-10-27):** F001 is in place. The server is a single `handlers.Handlers` struct implementing the generated `api.StrictServerInterface` (oapi-codegen `strict-server` + `std-http-server`), mounted under `/api/v1` by `httpserver.New`, wrapped by `withRecover(withAccessLog(mux))`. Handler errors all become `500 internal_error` (`ResponseErrorHandlerFunc`). One empty migration `0001_init.sql`; SQLite via `modernc.org/sqlite`, foreign keys on. `github.com/google/uuid` v1.6 (has `NewV7`) is already in `go.sum` as an indirect dependency. The web app has one `Welcome` page on `*`, an `openapi-fetch` client, i18next with `en`/`fr` `common.json`, and ESLint `i18next/no-literal-string`.

**Delivery:** three PRs on `feat/F003-*` branches, in order, so F004 is unblocked early:
- **PR 1 — foundations** (steps 1–4): typed API errors, `auth.Principal`, `auth.RequireAdmin`, deny-by-default middleware. Small, merged first. F004 branches from `main` after it.
- **PR 2 — server** (steps 5–13): accounts, sessions, profiles, pairing, all endpoints.
- **PR 3 — web** (steps 14–17): pages and guards.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` | modify | Security schemes, every route of the spec's API table, schemas, `Error.details`, `Set-Cookie` / `Retry-After` response headers (R12, all) |
| `server/internal/api/api.gen.go` | regenerate | From `openapi.yaml` |
| `web/src/api/schema.d.ts` | regenerate | From `openapi.yaml` |
| `server/go.mod`, `go.sum` | modify | `golang.org/x/crypto` (argon2), `github.com/google/uuid` becomes direct |
| `server/internal/apierr/apierr.go` + `_test.go` | create | `apierr.Error{Status, Code, Message, Details, Headers}`; constructors for every new error code. Handlers return it as `error`; the strict error handler writes it (all error codes) |
| `server/internal/httpserver/server.go` | modify | `ResponseErrorHandlerFunc` maps `*apierr.Error` → status + shared shape, else 500; middleware order recover → access log → client info → session → CSRF → password-change gate → mux (spec "Behavior per app") |
| `server/internal/httpserver/errors.go` | modify | `writeError` accepts `details` and extra headers |
| `server/internal/httpserver/middleware.go` | modify | `statusRecorder.Unwrap()` so `http.ResponseController` and cookies keep working through it (also needed by F004 SSE) |
| `server/internal/clientinfo/clientinfo.go` + `_test.go` | create | Client IP and HTTPS from `RemoteAddr`, `X-Forwarded-For`/`X-Forwarded-Proto` only from `PLAID_TRUSTED_PROXIES`; stored in context (watch-out "reverse proxy") |
| `server/internal/auth/principal.go` | create | `Principal{UserID, Role, ProfileID, SessionID, MustChangePassword}`, `WithPrincipal`, `FromContext`, `Role` type (`admin`, `member`, extensible) (R1) |
| `server/internal/auth/middleware.go` + `_test.go` | create | `Authenticate` (bearer or cookie → session lookup), public-route allowlist, `RequireAdmin(ctx) error`, CSRF check, `password_change_required` gate (R9, R10, R12, R13, R15) |
| `server/internal/auth/public_routes_test.go` | create | Parses `api/openapi.yaml`; fails if the operations with `security: []` differ from the middleware allowlist (R12 "mirrored by OpenAPI") |
| `server/internal/auth/password.go` + `_test.go` | create | argon2id hash/verify with encoded params, 2-slot semaphore, dummy hash for unknown users, `NeedsRehash` (R5, R6, NFR performance) |
| `server/internal/auth/common_passwords.txt` + `commonpw.go` | create | Embedded top-10k list (SecLists, MIT), lowercase set lookup (R5) |
| `server/internal/auth/token.go` + `_test.go` | create | 32-byte `crypto/rand` tokens, base64url, SHA-256, constant-time compare (R8) |
| `server/internal/auth/limiter.go` + `_test.go` | create | In-memory backoff per key (username, IP, session+profile, user for pairing claims): 1 s after 5 failures, doubling, cap 15 min; injectable clock (R7, R17, R20) |
| `server/internal/auth/netcheck.go` + `_test.go` | create | `IsLocal(addr)`: loopback, RFC 1918, RFC 4193 via `netip` (R3) |
| `server/internal/db/migrations/0002_accounts.sql` | create | Tables from the spec's data model |
| `server/internal/accounts/store.go` + `_test.go` | create | SQL for users, profiles, sessions, pairing codes; UUIDv7 ids; RFC 3339 UTC times (R24) |
| `server/internal/accounts/users.go` + `_test.go` | create | Validation (R4), create/update/disable/delete, last-admin guard in one transaction (R1, R14) |
| `server/internal/accounts/setup.go` + `_test.go` | create | Setup state, atomic first-admin creation (`BEGIN IMMEDIATE` + count check), one-time setup code (R2, R3) |
| `server/internal/accounts/sessions.go` + `_test.go` | create | Create/lookup/revoke, sliding expiry, `last_seen_at` throttled to 5 min, LRU cache invalidated on revoke (R8, R11, R19) |
| `server/internal/accounts/profiles.go` + `_test.go` | create | 1–8 profiles, one default, PIN hash, age limit rules, selection with PIN + lockout (R16–R18) |
| `server/internal/accounts/pairing.go` + `_test.go` | create | Codes from `ABCDEFGHJKMNPQRSTUVWXYZ23456789`, poll secret, in-memory one-shot token hand-over, expiry (R20) |
| `server/internal/accounts/janitor.go` | create | Hourly purge: expired pairing rows, sessions expired or revoked > 30 days |
| `server/internal/handlers/handlers.go` | modify | `New(Deps)` with a `Deps` struct (`DB`, `Accounts`, `Logger`, …) so F004 only adds a field |
| `server/internal/handlers/server_info.go`, `setup.go`, `auth.go`, `me.go`, `profiles.go`, `sessions.go`, `users.go`, `pairing.go` + `_test.go` each | create | Strict handlers, one file per API group |
| `server/internal/handlers/auth_e2e_test.go` | create | Acceptance criteria through the real `httpserver.New` stack |
| `server/internal/config/config.go` + `_test.go` | modify | `PLAID_TRUSTED_PROXIES` (CIDR list), `PLAID_SERVER_NAME` (default: hostname) |
| `server/cmd/plaid/main.go` | modify | Wiring, setup code generated and logged at startup while unconfigured, janitor goroutine |
| `server/internal/httpserver/server_test.go`, `handlers/health_test.go` | modify | New constructor signatures |
| `packaging/plaid.env` | modify | Commented `PLAID_TRUSTED_PROXIES`, `PLAID_SERVER_NAME` |
| `docs/install.md` | modify | First-run setup, setup code in the log (`journalctl -u plaid`, `docker logs`), reverse proxy + `PLAID_TRUSTED_PROXIES` |
| `web/src/api/client.ts` | modify | Middleware adding `X-Plaid-Client: web` to every request (R10) |
| `web/src/auth/AuthProvider.tsx` + `_test.tsx` | create | Loads `/server` and `/me`, exposes user, active profile, refresh, sign-out |
| `web/src/auth/RequireAuth.tsx` + `_test.tsx` | create | Guards: setup → sign-in → password change → profile picker → app (spec "Behavior per app") |
| `web/src/auth/errors.ts` | create | `errorKey(code)` → `errors.<code>` with `errors.generic` fallback |
| `web/src/pages/Setup.tsx`, `SignIn.tsx`, `ChangePassword.tsx`, `ProfilePicker.tsx`, `Pair.tsx` + `.test.tsx` each | create | R22; `ChangePassword` covers the member journey step 2 (R15) |
| `web/src/components/PinInput.tsx` + `_test.tsx` | create | 4 digits, one accessible field (NFR accessibility) |
| `web/src/App.tsx` | modify | Routes `/setup`, `/login`, `/change-password`, `/profiles`, `/pair`; `Welcome` stays as the signed-in home with a sign-out button |
| `web/src/locales/{en,fr}/common.json` | modify | Spec i18n table + keys below |
| `sdlc/features/F003-accounts-auth/spec.md` | modify | *Revisions*: the two clarifications in "Spec clarifications" |
| `sdlc/roadmap.md` | modify | Plan status |

## Spec clarifications (recorded in the spec's *Revisions* in step 1)
1. **`POST /setup/admin` takes an optional `device`** (same shape as login) so the first session has a real device name. Default `{name: "Web browser", type: "web"}`.
2. **Setup locality is stricter behind a proxy:** a request carrying `X-Forwarded-For` or `Forwarded` from a peer that is *not* in `PLAID_TRUSTED_PROXIES` is treated as non-local and needs the setup code. Without this, a public request through a reverse proxy on `127.0.0.1` would look local and could take over a fresh server (R3).
3. **Disabled accounts sign in with `401 invalid_credentials`** (no account enumeration); disabling revokes all their sessions.
4. **`POST /auth/logout` stays allowed** while `password_change_required` is active, in addition to `GET /me` and `POST /me/password`.

## Steps, in order
Each step ends with a commit `F003: …`. Tests are written in the same step as the code.

### PR 1 — foundations (unblocks F004)
1. [x] **Typed errors** (R12 error codes). `internal/apierr`, wired in `ResponseErrorHandlerFunc`; `Error` schema gains optional `details: [{field, code}]`; `writeError` supports headers (`Retry-After`). Test: a handler returning `apierr.Forbidden()` gives `403 {"error":{"code":"forbidden",…}}`; any other error stays `500 internal_error` and is logged. Spec *Revisions* updated.
2. [x] **Client info** (watch-out reverse proxy). `PLAID_TRUSTED_PROXIES` in config; `clientinfo` middleware computes IP + HTTPS. Tests: forwarded headers ignored from an untrusted peer, honored from a trusted CIDR, rightmost untrusted hop chosen in a chain.
3. [x] **Principal and middleware skeleton** (R1, R12, R13). `auth.Principal`, `FromContext`, `RequireAdmin`; `Authenticate` with the public-route allowlist and a `SessionLookup` interface (stub that always fails until step 7). OpenAPI: `bearerAuth` + `cookieAuth` global, `security: []` on `/health`. `public_routes_test.go`. Tests: `/health` works without a token; any other `/api/v1` path without a session is `401 unauthorized`, including unknown ones (the middleware runs before routing, and not leaking which routes exist is fine); with a valid session an unknown route is the JSON `404`. `go.yaml.in/yaml/v3` (already indirect) becomes a direct dependency for the OpenAPI test.
4. [x] **`statusRecorder.Unwrap`** + test that `http.NewResponseController(w).Flush()` works through the access log. → **open PR 1.**

### PR 2 — server
5. [x] **Migration `0002_accounts.sql` + store** (R24). Tables exactly as the spec; `accounts.Store` with CRUD. Test: migration applies on a fresh DB and on one with `0001` already applied; cascade deletes work.
6. [x] **Passwords and tokens** (R5, R8). argon2id `m=19456 KiB, t=2, p=1`, 16-byte salt, PHC string; params injectable so tests use `m=64 KiB` (otherwise `-race` runs crawl). Semaphore of 2. Common-password list. Tests: round trip, wrong password, params parsed from the hash, `NeedsRehash` when params rise, 8/128 bounds, `"password1"` refused, 3rd concurrent hash waits.
7. [x] **Sessions** (R8, R9, R11, R19). Create returns the clear token once; lookup by SHA-256; sliding expiry; `last_seen_at`/`expires_at`/`last_ip` written at most every 5 min; in-memory LRU (≈1,000 entries) invalidated on revoke. Real `SessionLookup` plugged into the middleware. Cookie `plaid_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` when HTTPS, `Max-Age` 90 days. Tests with a fake clock: expiry, throttled writes, revoke → next request `401` even with a warm cache.
8. [x] **CSRF and password-change gate** (R10, R15). Cookie-authenticated non-GET/HEAD requests need `X-Plaid-Client: web` and an `Origin` whose scheme+host equal the request's (scheme from step 2, host from `Host`). Bearer exempt. Gate returns `403 password_change_required` except for clarification 4. Tests: table of method × auth kind × headers.
9. [x] **Setup** (R2, R3, R21). `GET /server`, `GET /setup`, `POST /setup/admin`. Creation inside `BEGIN IMMEDIATE` with `SELECT COUNT(*) FROM users` → `409 setup_complete`. Setup code: 8 chars from the pairing alphabet, generated at startup only while no user exists, logged once at `warn` (`"first-run setup code", "code", …`; this is the one secret meant for the log, R23 exception documented in `docs/install.md`), compared in constant time. Session returned as cookie when `X-Plaid-Client: web`, else token. Tests: 20 concurrent setups → exactly one `201`; public IP without code `403 setup_code_required`, with code `201`; clarification 2.
10. [x] **Login, logout, lockout** (R6, R7, R23). Unknown username → verify against a precomputed dummy hash (same cost). Limiter keys `user:<name>` and `ip:<addr>`; `429 too_many_attempts` + `Retry-After` (seconds). Success resets both keys. Auth events logged with `user_id`, `session_id`, `ip`, never the secret. Tests: identical body for unknown user / wrong password; a counting fake hasher proves both paths hash exactly once; 6th failure → `429`; timing test (real params, 20 samples each, mean difference < 50 ms) under `//go:build !race` so CI `-race` doesn't flake; log capture asserts no password/token appears.
11. [x] **Me, users, last-admin guard** (R1, R4, R14, R15). `/me`, `PATCH /me`, `POST /me/password` (revokes other sessions), `/users*`. Last-admin check and the update in one `BEGIN IMMEDIATE` transaction. Usernames lowercased, `COLLATE NOCASE` unique → `409 username_taken`. Tests: every acceptance criterion on users; member on `/users` → `403`; demote/disable/delete last admin → `409 last_admin`; temporary password → `must_change_password`.
12. [x] **Profiles** (R16–R18). Default profile created with every account in the same transaction. Limits 1–8, `409 last_profile`, `422 validation_failed` on the 9th. `maxAge ∈ {0,7,10,12,13,16,18}`; `allowUnrated` defaults to false when `maxAge` is set. PIN = 4 digits, argon2id; removing it needs the current PIN or account password. `POST /session/profile`: 5 wrong PINs → `423 profile_locked` for 5 min on (session, profile). Tests: each criterion, lock expires with the fake clock.
13. [x] **Pairing** (R20). `POST /pairing` (rate-limited 10 / 10 min per IP), `POST /pairing/claim` (wrong codes rate-limited per user through the limiter), `GET /pairing/{id}` with `X-Pairing-Secret`: `202 pending`, then `200` with the token exactly once (in-memory map id → token, deleted on delivery), then `404 pairing_not_found`. Expired → `410 pairing_expired`. Janitor goroutine (hourly) started in `main.go`. Tests: full flow, expiry, restart between claim and poll → `404` (TV starts over), code normalization (`k7p-4qx` = `K7P4QX`). → `make test`, **open PR 2.**

### PR 3 — web
14. [x] **Client + AuthProvider + guards** (R9, R10, R22). `X-Plaid-Client: web` middleware; `AuthProvider` reads `/server` then `/me` (`401` = signed out); `RequireAuth` redirects in the order setup → `/login` → `/change-password` → `/profiles` → app. Tests with stubbed `fetch` for each state.
15. [x] **Setup and sign-in pages** (R22). Inline field errors (`aria-describedby`), one-line server error from `errorKey(code)`, `too_many_attempts` shows minutes from `Retry-After` via ICU plural/number. Setup shows the setup-code field only when `requiresSetupCode`. Tests: happy path, mismatch, French rendering.
16. [x] **Change password, profile picker, PIN** (R15, R17, R22). One profile → straight to home; several → "Who's watching?" tiles; PIN modal using `PinInput` (4 inputs, `role="group"` + label, auto-advance, backspace, paste). Tests: keyboard-only flow, `invalid_pin`, `profile_locked`.
17. [x] **Pair page + sign-out** (R20, R22). Code input accepting `XXX-XXX` or 6 chars, profile select, success line `pairing.done`. Sign-out button on `Welcome`. `make check-i18n`, screenshots of the 5 pages in `en` and `fr`. → **open PR 3.**
18. [x] **Pi check** (NFR performance, watch-out argon2id). `make deb && PLAID_TEST_CPUS=0.5 PLAID_TEST_MEMORY=1g make test-deb`, then time 10 sign-ins with `curl` inside the container. If p95 < 250 ms, raise `t` to 3 and record it in the Log; if > 500 ms, stop and discuss (the spec floor is OWASP's minimum).

### New i18n keys (beyond the spec table)
| Key | en | fr |
|-----|----|----|
| `setup.setupCode` | Setup code (shown in the server log) | Code d'installation (affiché dans le journal du serveur) |
| `auth.newPassword` | New password | Nouveau mot de passe |
| `auth.currentPassword` | Current password | Mot de passe actuel |
| `auth.changePassword.title` | Choose a new password | Choisis un nouveau mot de passe |
| `auth.changePassword.submit` | Save password | Enregistrer le mot de passe |
| `errors.<code>` | for every remaining spec code (`unauthorized`, `forbidden`, `csrf`, `setup_complete`, `setup_code_required`, `password_change_required`, `last_admin`, `last_profile`, `profile_locked`, `pairing_not_found`, `validation_failed`) | idem |

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| A route added later (F004, F005…) is public by mistake | medium | Deny by default; `public_routes_test.go` ties the allowlist to OpenAPI `security: []` |
| Setup takeover of a fresh server through a local reverse proxy | medium | Clarification 2 + test |
| CSRF `Origin` check breaks the Vite dev proxy or a reverse proxy rewriting `Host` | medium | Vite proxy keeps `Host: localhost:5173` (default `changeOrigin: false`), checked manually in step 8; `docs/install.md` tells proxy users to forward `Host` |
| argon2id too slow on a Pi or memory-hungry under login bursts | low | Semaphore of 2 (38 MiB max), step 18 measurement |
| Timing test flaky in CI | high if naive | Structural test (hash count) is the real guard; timing test excluded from `-race` and uses means |
| Session cache serves a revoked session | low | Single process; revoke deletes from the cache before returning; test with a warm cache |
| Merge conflicts with F004 on `openapi.yaml`, `api.gen.go`, `handlers.go`, `main.go`, `config.go` | high | PR 1 lands first with `handlers.Deps`; F004 adds its paths under its own tags; generated files are always resolved by `make generate`, never by hand |
| Strict handlers can't set cookies | low | `Set-Cookie` declared as a response header in OpenAPI → generated `Headers` field; verified in step 7 |

**Riskiest step:** step 7–8 (sessions + CSRF in the middleware). Everything later trusts `auth.Principal`; a bug there is an auth bypass for every feature. It gets the table-driven tests and a dedicated review pass.

## Rejected options
- **JWT sessions**: rejected in the spec (no instant revocation).
- **A strict-server middleware per operation for auth**: the allowlist would live in Go only. A plain `net/http` middleware plus the OpenAPI test keeps one source of truth and also covers non-generated routes (F004 SSE).
- **Typed error responses per status in OpenAPI for every operation** (`401`, `403`, `409`… as generated response types): dozens of near-identical types per handler. `apierr` + a `default` `Error` response documents the shape for clients with far less code.
- **`golang.org/x/time/rate` for lockouts**: token buckets don't express "1 s after 5 failures, doubling". A 60-line backoff map with a fake clock is simpler to test.
- **Storing the pairing token in the DB until delivery**: would store a clear token; the spec forbids it.
- **A third-party LRU**: one hand-written map + list for ~1,000 entries is enough and avoids a dependency.

## Expected evidence
- [x] Tests added: `apierr`, `clientinfo`, `auth/*` (incl. `public_routes_test`), `accounts/*`, `handlers/*` + `auth_e2e_test.go` covering every acceptance criterion, web pages and guards
- [ ] `make test` green, output pasted in each PR
- [x] Screenshots: setup, sign-in (with error), change password, "Who's watching?" with PIN, pair; `en` and `fr` (in [`screenshots/`](screenshots/))
- [x] Manual check: full journey in the browser through the Vite proxy (setup → sign-in → profile → sign-out)
- [x] Manual check: TV pairing with `curl` (create, claim from the browser, poll twice)
- [x] Manual check: sign-in timing on the throttled `make test-deb` profile (step 18), numbers in the Log
- [x] Manual check: `grep` of the server log after the e2e run shows no password, PIN, token or poll secret (also asserted by `TestNoSecretsInLogs`)

## Log
- 2026-10-27, step 2: client info lives in its own package `internal/clientinfo` instead of `internal/httpserver/clientinfo.go`. Handlers need to read it, and `httpserver`'s tests import `handlers`, so putting it in `httpserver` would create an import cycle. The untrusted-forward flag (clarification 2) also covers `X-Real-IP`. `httpserver.New` takes an `Options` struct, which step 3 extends with the session lookup.
- 2026-10-27, step 3: `handlers.Deps` (planned with step 5 in the files table) lands in PR 1 as the risk table says, so F004 can add its field without touching the constructor. The middleware guards every `/api/` path, not only `/api/v1/`, so nothing under `/api/` is reachable without a session except the allowlist. A failed session lookup of any kind (database error included) is a `401`.
- 2026-10-27, PR 2 (steps 5–13):
  - **No session cache.** An indexed SQLite lookup takes about 10 µs (`BenchmarkSessionLookup`), a hundred times under the 1 ms budget. A cache would bring invalidation bugs on the riskiest path (revoke, role change, disable) for no measured gain. The "session cache serves a revoked session" risk is gone with it.
  - **Common-password list**: about 140 hand-curated entries of 8+ characters (`internal/auth/common_passwords.txt`) instead of the SecLists top 10k, which would have meant downloading a third-party file. Shorter passwords are refused by length anyway. To revisit if we want the full list.
  - **API decisions** recorded as spec revisions 6–11: profile management only from an unrestricted profile, proof for sensitive profile fields, `403 invalid_credentials` for re-checks, one `AuthResult` shape, `pin` on pairing claims, cookie `Max-Age` 400 days.
  - **Cookies from strict handlers**: instead of a `Set-Cookie` response header in OpenAPI (generated code would always write it, empty for native clients), handlers queue cookies in the request context and the auth middleware's writer sends them before the headers (`internal/auth/cookies.go`).
  - **`nullable-type: true`** in `oapi-codegen.yaml` so PATCH bodies tell an absent field from `null` (`type: [T, "null"]` in OpenAPI 3.1).
  - **`_txlock=immediate`** on the SQLite DSN: every transaction takes the write lock up front. That makes the setup and last-admin check-then-write sequences atomic without an extra mutex, and avoids `SQLITE_BUSY` on lock upgrades.
  - Revoked and expired sessions are kept 30 days before the janitor purges them; expired pairing codes 10 minutes after expiry (a grace for a TV claimed just before expiry).
  - `auth_e2e_test.go` covers the acceptance criteria. The timing test (`auth_timing_test.go`, real argon2id parameters, `!race`) measured 16.2 ms vs 15.8 ms on an M-series Mac. The Pi measurement (step 18) is still to do.
- 2026-10-27, PR 3 (steps 14–17):
  - **The Vite proxy rewrote `Host`**: Vite's string shorthand for `proxy` sets `changeOrigin: true`, so the server saw `Host: localhost:32500` against `Origin: http://localhost:5173` and refused setup with `403 csrf` in `make dev`. This was the risk the plan flagged. `vite.config.ts` now sets `changeOrigin: false`. Production (same origin) was never affected.
  - **PIN as one field**, not four inputs: one `<input inputMode="numeric" maxLength=4>` spaced out by CSS. Screen readers announce it once (the spec asks for "4 digits announced as one field"), and paste and password managers work without special code.
  - **`POST /pairing/claim` returns `200` with the TV's `SessionInfo`** instead of `204`, so the page can say "Living room TV is signed in." (`pairing.done` needs the device name). Recorded as spec revision 12.
  - **API errors in the web app**: every code maps to `errors.<code>` through an explicit `switch` (`src/auth/errors.ts`), so the i18n extractor sees each key; unknown codes fall back to `errors.generic`. Field `details` map to `fieldErrors.*`.
  - The signed-in home is the F001 `Welcome` page under an account bar (profile, switch profile, pair a TV, sign out) until F008.
  - The service now returns every time in UTC: `createdAt` came out with the host's offset (`+01:00`), seen during the manual check.
  - `.claude/launch.json` (Vite on :5173) for the desktop app's browser preview.
- 2026-10-27, step 18 (sign-in timing), `.deb` `0.1.6~snapshot+557280c` arm64, `ubuntu:24.04` container with real systemd, `--cpus 0.5 --memory 1g`, on an Apple Silicon host:
  - `PLAID_TEST_CPUS=0.5 PLAID_TEST_MEMORY=1g make test-deb`: all 24 checks pass, cold start 127 ms, idle RSS 31 MB.
  - 10 sign-ins one at a time (`curl` inside the container, real argon2id m=19 MiB t=2 p=1): min 20 ms, median 42 ms, max 75 ms, mean 45 ms. Setup: 30 ms.
  - 4 sign-ins at once: 96, 97, 198, 200 ms. The 2-slot semaphore serializes them two by two, as designed. Service memory peak 84 MB.
  - First-run setup code present in the journal (`journalctl -u plaid`).
  - Caveat on these numbers: `--cpus 0.5` throttles CPU time, not per-core speed, and an Apple Silicon core runs argon2id several times faster than a Pi 4's Cortex-A72 (Go's argon2 has no arm64 assembly). They are a lower bound, not a Pi measurement.
  - **`t` raised to 3** as step 18's rule says (p95 < 250 ms), at gh-prim's call despite the caveat. Re-measured on the same profile: min 30 ms, median 77 ms, max 88 ms; 4 at once 103, 187, 381, 383 ms; memory peak 87 MB. Spec revision 13. Existing t = 2 hashes are re-hashed at the next sign-in (`NeedsRehash`). If a real Pi 4 shows sign-in over 500 ms, lower it back to 2: hashes keep their own params, so nothing breaks.
- 2026-10-27, delivery: PR 1 merged as gh-prim/plaid#23; PR 2 and PR 3 merged together as gh-prim/plaid#24 (gh-prim/plaid#26 folded into it) to merge the rest of F003 in one go.
