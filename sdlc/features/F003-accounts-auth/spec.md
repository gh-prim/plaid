---
id: F003
title: Accounts and authentication
owner: gh-prim
reviewer: TBD
status: approved         # draft | approved
intent: ./intent.md
decisions: [0002, 0003, 0005]
---

# F003 — Spec: Accounts and authentication

## Summary
F003 makes the server know who is there. On first launch someone creates the admin account; the admin then creates member accounts. Every account has one or more profiles (a family shares one account, each person picks a profile), optionally protected by a PIN and limited by an age rating. Every client (web today, Apple and Android apps later) signs in the same way and gets a revocable session tied to a device. TVs sign in by showing a short code that you confirm from a phone or computer. Everything stays on the server: no cloud account (ADR 0003), no external identity provider.

## User journeys

**Admin, first launch**
1. Opens `http://<server>:32500`. The server has no account, so the web app shows "Create the admin account".
2. Enters a display name, a username and a password (twice). The account is created with a default profile named after them, and they're signed in.
3. Any later visit to the setup endpoint is refused: there's already an admin.

**Admin, adding a member**
1. Admin goes to Settings → Users → Add user, enters a display name, username, temporary password, role (member or admin).
2. The member signs in with it and is asked to choose a new password.

**Member, signing in on the web**
1. Enters username and password. Wrong credentials: "Wrong username or password." (never says which one was wrong).
2. If the account has one profile, lands on the home page. If it has several: "Who's watching?" with one tile per profile.
3. Picks a profile. A PIN-protected profile asks for its 4-digit PIN; wrong PIN: "Wrong PIN. Try again."

**Family, profiles**
1. Signed in as the account owner, "Manage profiles" lets them add a profile ("Kids"), set an age limit (for example 12+) and an optional PIN on their own profile so kids can't switch to it.

**TV, pairing (server side now; TV apps in phases 4 and 5)**
1. The TV app asks the server for a pairing code and shows it: "Go to `<server>/pair` and enter K7P-4QX".
2. On their phone, a signed-in person opens `/pair`, types the code, picks which profile the TV starts on, confirms.
3. Within a couple of seconds the TV is signed in. The code expires after 10 minutes or once used.

**Anyone, signing out a device**
1. Settings → Devices lists every signed-in device (name, type, last seen, approximate location by IP).
2. "Sign out" on a row ends that session immediately; that device's next request gets `401`.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F003-R1 | Accounts have a role: `admin` or `member`. The role enum is extensible (F009 adds `friend`). At least one active admin must always exist: demoting, disabling or deleting the last admin is refused. | must |
| F003-R2 | First-run setup: while no account exists, `POST /setup/admin` creates the first admin and signs them in. Once any account exists it returns `409 setup_complete`. Creation is atomic: two concurrent setup requests can't both succeed. | must |
| F003-R3 | First-run setup is only accepted from a private or loopback address (RFC 1918, RFC 4193, 127.0.0.0/8, ::1), unless the request carries the one-time setup code the server prints in its log at startup while unconfigured. | must |
| F003-R4 | Usernames: 3–32 chars, `[a-z0-9._-]`, unique case-insensitively, normalized to lowercase. Display names: 1–64 chars, any script. | must |
| F003-R5 | Passwords: 8–128 chars, no composition rules (NIST SP 800-63B), checked against a small embedded list of the most common passwords. Hashed with argon2id (m = 19 MiB, t = 2, p = 1), salted, parameters stored in the encoded hash so they can be raised later. | must |
| F003-R6 | Sign-in by username + password returns a session. Errors never reveal whether the username exists; the server does the same amount of hashing work for unknown usernames. | must |
| F003-R7 | Brute-force protection: per username and per source IP, failed attempts trigger an increasing delay (1 s after 5 failures, doubling, capped at 15 min). Lockouts are in memory and reset on success or restart. | must |
| F003-R8 | Sessions use opaque random tokens (32 bytes, base64url). Only a SHA-256 hash of the token is stored. A session belongs to an account, records the device (name, type, client version), the active profile, created/last-seen time and IP. | must |
| F003-R9 | Native apps send `Authorization: Bearer <token>`. The web app uses an `HttpOnly`, `SameSite=Lax` cookie `plaid_session` (`Secure` when the request is HTTPS), never readable by JavaScript. | must |
| F003-R10 | CSRF: state-changing requests authenticated by cookie must carry `X-Plaid-Client: web` and a same-origin `Origin` header; otherwise `403 csrf`. Bearer requests are exempt. | must |
| F003-R11 | Session lifetime: expires after 90 days without use (sliding); `last_seen_at` is written at most once every 5 minutes per session to spare SQLite. Changing a password ends every other session of that account. | must |
| F003-R12 | Every `/api/v1` route requires a valid session except: `GET /health`, `GET /server`, `GET /setup`, `POST /setup/admin`, `POST /auth/login`, `POST /pairing`, `GET /pairing/{id}`. Enforced by default in middleware (deny unless listed), mirrored by OpenAPI `security`. | must |
| F003-R13 | Admin-only routes (user management, all future library and server settings) return `403 forbidden` to members. | must |
| F003-R14 | Admin user management: list, create (with temporary password, `must_change_password` set), update display name/role/locale, disable/enable, reset password, delete. Deleting an account deletes its profiles and sessions. | must |
| F003-R15 | Self-service: read and update own display name and locale; change own password (requires the current one); temporary passwords force a change before anything else (`403 password_change_required`). | must |
| F003-R16 | Profiles: each account has 1–8 profiles, exactly one marked default, created with the account. Fields: name (1–32 chars), avatar color index (0–7, maps to `party-*` design tokens), optional 4-digit PIN (argon2id), optional age limit, optional locale override. The last profile can't be deleted. | must |
| F003-R17 | Selecting a profile sets it on the current session. A PIN-protected profile needs its PIN on every selection; 5 wrong PINs lock that profile for 5 minutes on that session. | must |
| F003-R18 | Age limit is stored as a minimum-age number (`0`, `7`, `10`, `12`, `13`, `16`, `18`) plus `allowUnrated` (default false when a limit is set). F003 stores and exposes it; enforcement on content belongs to the browsing and playback features (F005, F006, F008). | must |
| F003-R19 | Sessions: list own sessions (current one flagged), revoke one, revoke all others. Admin can list and revoke any account's sessions. | must |
| F003-R20 | TV pairing: `POST /pairing` (no auth) returns a 6-character code from an unambiguous alphabet (no `0 O 1 I L`), shown as `XXX-XXX`, a pairing id and a poll secret; expires in 10 min. A signed-in user claims it with `POST /pairing/claim` (code + profile). The TV polls `GET /pairing/{id}` with its poll secret and receives its session token once, after the claim. Claims are rate-limited per user. | should |
| F003-R21 | `GET /server` (no auth) returns server name, version and whether setup is complete, so clients can tell they reached a PLAID server. | must |
| F003-R22 | Web: "Create the admin account" page (shown only while setup is incomplete), sign-in page, "Who's watching?" profile picker with PIN entry, sign-out, and a `/pair` page to claim a TV code. Unstyled beyond existing CSS until the design system lands. F007 owns the full onboarding wizard and the admin screens. | must |
| F003-R23 | Every auth event (sign-in success/failure, password change, session revoked, user created/deleted, pairing claimed) is logged with `slog` at info level, without passwords, tokens or PINs. | must |
| F003-R24 | Identifiers are UUIDv7 strings (sortable, global), ready for a future Plaid Hub (ADR 0003). | must |

## Design

### Data model
Migration `0002_accounts.sql`. All ids `TEXT` UUIDv7, timestamps `TEXT` RFC 3339 UTC.

```sql
CREATE TABLE users (
  id                    TEXT PRIMARY KEY,
  username              TEXT NOT NULL UNIQUE COLLATE NOCASE,
  display_name          TEXT NOT NULL,
  password_hash         TEXT NOT NULL,              -- argon2id encoded ($argon2id$v=19$m=...)
  role                  TEXT NOT NULL CHECK (role IN ('admin','member')),
  locale                TEXT NOT NULL DEFAULT 'en',
  must_change_password  INTEGER NOT NULL DEFAULT 0,
  disabled_at           TEXT,
  created_at            TEXT NOT NULL,
  updated_at            TEXT NOT NULL
);

CREATE TABLE profiles (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name            TEXT NOT NULL,
  avatar_color    INTEGER NOT NULL DEFAULT 0 CHECK (avatar_color BETWEEN 0 AND 7),
  pin_hash        TEXT,                              -- argon2id, NULL = no PIN
  max_age         INTEGER,                           -- NULL = no limit
  allow_unrated   INTEGER NOT NULL DEFAULT 1,
  locale          TEXT,                              -- NULL = account locale
  is_default      INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
CREATE UNIQUE INDEX profiles_one_default ON profiles(user_id) WHERE is_default = 1;

CREATE TABLE sessions (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  profile_id      TEXT REFERENCES profiles(id) ON DELETE SET NULL,
  token_hash      BLOB NOT NULL UNIQUE,              -- sha256(token)
  device_name     TEXT NOT NULL,
  device_type     TEXT NOT NULL CHECK (device_type IN ('web','ios','ipados','tvos','android','androidtv','other')),
  client_version  TEXT,
  last_ip         TEXT,
  created_at      TEXT NOT NULL,
  last_seen_at    TEXT NOT NULL,
  expires_at      TEXT NOT NULL,                     -- last_seen_at + 90 days
  revoked_at      TEXT
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE pairing_codes (
  id                TEXT PRIMARY KEY,
  code              TEXT NOT NULL UNIQUE,            -- 6 chars, normalized uppercase, no dash
  poll_secret_hash  BLOB NOT NULL,
  device_name       TEXT NOT NULL,
  device_type       TEXT NOT NULL,
  created_at        TEXT NOT NULL,
  expires_at        TEXT NOT NULL,
  claimed_at        TEXT,
  claimed_session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
  token_delivered_at TEXT                            -- the token is handed over once
);
```
The pairing token is held in memory between claim and the TV's next poll (never stored in clear); if the server restarts in between, the TV starts a new pairing. Expired pairing rows are purged hourly.

### API
All routes under `/api/v1`, added to `api/openapi.yaml` first. Shared error shape `{"error": {"code", "message"}}` (F001).

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/server` | none | `{name, version, setupComplete}` |
| GET | `/setup` | none | `{setupComplete, requiresSetupCode}` |
| POST | `/setup/admin` | none (R3) | `{displayName, username, password, locale, setupCode?}` → `201` session (cookie for web, token for others) |
| POST | `/auth/login` | none | `{username, password, device: {name, type, clientVersion}}` → `200 {session, user, profiles}` |
| POST | `/auth/logout` | session | Ends the current session → `204` |
| GET | `/me` | session | `{user, activeProfile, profiles}` |
| PATCH | `/me` | session | `{displayName?, locale?}` |
| POST | `/me/password` | session | `{currentPassword, newPassword}` → ends other sessions |
| GET | `/me/profiles` | session | List |
| POST | `/me/profiles` | session | `{name, avatarColor, pin?, maxAge?, allowUnrated?, locale?}` |
| PATCH | `/me/profiles/{id}` | session | Partial update; `pin: null` removes the PIN (needs the current PIN or the account password) |
| DELETE | `/me/profiles/{id}` | session | Refused for the last profile |
| POST | `/session/profile` | session | `{profileId, pin?}` → sets active profile |
| GET | `/me/sessions` | session | List, current one flagged |
| DELETE | `/me/sessions/{id}` | session | Revoke |
| DELETE | `/me/sessions` | session | Revoke all others |
| GET | `/users` | admin | List users |
| POST | `/users` | admin | `{displayName, username, temporaryPassword, role, locale}` |
| PATCH | `/users/{id}` | admin | `{displayName?, role?, locale?, disabled?}` |
| POST | `/users/{id}/password` | admin | `{temporaryPassword}` → ends that user's sessions |
| DELETE | `/users/{id}` | admin | Delete account |
| GET | `/users/{id}/sessions` | admin | List |
| DELETE | `/users/{id}/sessions/{sid}` | admin | Revoke |
| POST | `/pairing` | none | `{device: {name, type, clientVersion}}` → `{id, code, pollSecret, expiresAt}` |
| GET | `/pairing/{id}` | poll secret (`X-Pairing-Secret`) | `202 {status: "pending"}` or `200 {token, user, activeProfile}` once |
| POST | `/pairing/claim` | session | `{code, profileId}` → `204` |

Session payload returned to native clients: `{"token": "…", "expiresAt": "…", "session": {"id", "deviceName", "deviceType"}}`. The web gets the same body without `token`, plus the `Set-Cookie`.

New error codes (stable keys, translated by clients): `unauthorized`, `forbidden`, `csrf`, `invalid_credentials`, `too_many_attempts` (with `Retry-After`), `setup_complete`, `setup_code_required`, `password_change_required`, `weak_password`, `username_taken`, `last_admin`, `last_profile`, `invalid_pin`, `profile_locked`, `pairing_expired`, `pairing_not_found`, `validation_failed` (with `details: [{field, code}]`).

OpenAPI: two security schemes, `bearerAuth` (HTTP bearer) and `cookieAuth` (API key in cookie `plaid_session`), applied globally; the public routes set `security: []`.

### Behavior per app
- **server**: new packages `internal/auth` (hashing, tokens, middleware, rate limiter), `internal/accounts` (users, profiles, sessions, pairing; SQL), handlers implementing the new operations. Middleware order: recover → access log → session lookup → CSRF check → route. Request context carries `auth.Principal{UserID, Role, ProfileID, SessionID}` for every later feature.
- **web**: `AuthProvider` loading `/me`, route guards (setup → sign-in → profile picker → app), the four pages in R22. All copy through i18n.
- **apple / android**: out of scope; they'll use bearer tokens and pairing.

### UI / Art direction
Plain pages until the design system is adopted (F002 → F008). Copy follows the design system content rules: functional text is plain and short, no jokes, no em dash. One warm line allowed on the first-run page.

States: loading (button disabled with spinner), validation errors inline under each field, server errors in one line above the form, locked (`too_many_attempts`) shows when to retry.

### i18n
| Key | en | fr |
|-----|----|----|
| `setup.title` | Welcome to PLAID. | Bienvenue sur PLAID. |
| `setup.subtitle` | Create the admin account for this server. | Crée le compte administrateur de ce serveur. |
| `auth.displayName` | Name | Nom |
| `auth.username` | Username | Nom d'utilisateur |
| `auth.password` | Password | Mot de passe |
| `auth.confirmPassword` | Confirm password | Confirme le mot de passe |
| `auth.signIn` | Sign in | Se connecter |
| `auth.signOut` | Sign out | Se déconnecter |
| `auth.createAdmin` | Create account | Créer le compte |
| `profiles.whoIsWatching` | Who's watching? | Qui regarde ? |
| `profiles.enterPin` | Enter the PIN for {{name}} | Saisis le code PIN de {{name}} |
| `pairing.title` | Pair a TV | Associer une TV |
| `pairing.codeLabel` | Code shown on the TV | Code affiché sur la TV |
| `pairing.startProfile` | Start on profile | Profil de départ |
| `pairing.confirm` | Pair | Associer |
| `pairing.done` | {{device}} is signed in. | {{device}} est connectée. |
| `errors.invalid_credentials` | Wrong username or password. | Nom d'utilisateur ou mot de passe incorrect. |
| `errors.invalid_pin` | Wrong PIN. Try again. | Code PIN incorrect. Réessaie. |
| `errors.too_many_attempts` | Too many attempts. Try again in {{minutes, number}} min. | Trop de tentatives. Réessaie dans {{minutes, number}} min. |
| `errors.weak_password` | Choose a longer or less common password. | Choisis un mot de passe plus long ou moins courant. |
| `errors.username_taken` | This username is already taken. | Ce nom d'utilisateur est déjà pris. |
| `errors.password_mismatch` | Passwords don't match. | Les mots de passe ne correspondent pas. |
| `errors.pairing_expired` | This code has expired. Ask the TV for a new one. | Ce code a expiré. Demande-en un nouveau sur la TV. |
| `errors.generic` | Something went wrong. Try again. | Une erreur s'est produite. Réessaie. |

Every error `code` from the API maps to `errors.<code>`; unknown codes fall back to `errors.generic`.

## Non-functional requirements
- **Performance**: one argon2id hash ≤ 250 ms on a Raspberry Pi 4 core; at most 2 hashes run concurrently (semaphore) so a burst of logins can't exhaust a 1 GB device (19 MiB each). Session lookup ≤ 1 ms (indexed hash lookup, small in-memory LRU of recent sessions invalidated on revoke).
- **Security**: constant-time comparisons for tokens, poll secrets and PINs; tokens from `crypto/rand`; no secrets in logs or URLs; cookies `HttpOnly`; rate limiting on login, setup, PIN and pairing claim.
- **Accessibility**: labels on every field, errors linked with `aria-describedby`, PIN input usable with a keyboard and screen reader (4 separate digits announced as one field).
- **i18n**: all strings as above, ICU plurals where counts appear.

## ⚠️ Watch-outs
Settled by gh-prim on 2026-10-27. ✅ = resolved, ⏸ = tracked elsewhere.

- [x] **Sessions: opaque tokens vs JWT** — ✅ Opaque random tokens, stored as SHA-256 hashes (R8): instant revocation, no signing keys.
- [x] **Profile PIN: every time or once per device** — ✅ Asked every time the profile is selected (R17).
- [x] **First-run hijack** — ✅ Setup only from private/loopback addresses unless the one-time setup code from the log is given (R3).
- [x] **Cookie `Secure` and client IP behind a reverse proxy** — ✅ `X-Forwarded-*` trusted only from `PLAID_TRUSTED_PROXIES` (default: none); `Secure` set when the server sees HTTPS.
- [x] **Overlap with F007** — ⏸ F003 ships a minimal setup page; F007's wizard and admin screens reuse the same API. Update F007's intent when it's specified.
- [x] **Enforcing age limits** — ⏸ Stored by F003, enforced by F005/F006/F008. Don't advertise parental controls before then.
- [x] **Argon2id cost on small hardware** — ⏸ Measure on a Pi 4 (or the throttled `make test-deb` profile) during the build; raise parameters if fast enough.
- [x] **Parallel work with F004** — ⏸ F003's plan delivers `auth.Principal` and `auth.RequireAdmin` first.
- [x] **Backlog items touched** (SSO/LDAP, 2FA, activity log, kids UI, user import) — ⏸ Out of scope; the model doesn't block them.

## Acceptance criteria
- [ ] Given a fresh server, when I `GET /setup`, then `setupComplete` is false; when I `POST /setup/admin` from localhost with valid fields, then I get `201`, a session, and `GET /me` shows role `admin` and one default profile.
- [ ] Given an admin exists, when anyone `POST /setup/admin`, then `409 setup_complete`.
- [ ] Given a fresh server and two concurrent `POST /setup/admin`, then exactly one succeeds.
- [ ] Given a fresh server and a request from a public IP without the setup code, then `403 setup_code_required`; with the code printed in the log, then `201`.
- [ ] Given a user, when I sign in with a wrong password or an unknown username, then both return `401 invalid_credentials` with the same body, and their response times differ by less than 50 ms on average.
- [ ] Given 5 failed sign-ins for one username, when I try again immediately, then `429 too_many_attempts` with a `Retry-After` header.
- [ ] Given a valid session, when I call any `/api/v1` route except the public ones without a token or cookie, then `401 unauthorized`; with it, then the route works.
- [ ] Given a member session, when I `GET /users`, then `403 forbidden`.
- [ ] Given a cookie session, when I `PATCH /me` without `X-Plaid-Client: web` or from another `Origin`, then `403 csrf`; with both, then `200`. Given a bearer session, the header isn't needed.
- [ ] Given two sessions for one user, when I revoke the other one, then its next request returns `401`, immediately.
- [ ] Given I change my password, then all my other sessions are revoked and the current one keeps working.
- [ ] Given a user created with a temporary password, when they sign in and call anything but `POST /me/password` or `GET /me`, then `403 password_change_required`.
- [ ] Given the last admin, when an admin tries to demote, disable or delete them, then `409 last_admin`.
- [ ] Given an account with one profile, when I delete it, then `409 last_profile`; given 8 profiles, creating a 9th returns `422 validation_failed`.
- [ ] Given a PIN-protected profile, when I select it with a wrong PIN 5 times, then the 6th attempt returns `423 profile_locked` for 5 minutes on that session, and the right PIN works after that.
- [ ] Given a TV calls `POST /pairing`, when a signed-in user claims the code with a profile, then the TV's next `GET /pairing/{id}` returns a token once, a second poll returns `404 pairing_not_found`, and the TV's session shows that profile as active.
- [ ] Given a pairing code older than 10 minutes, when it's claimed, then `410 pairing_expired`.
- [ ] Given the browser is set to French, when I open a fresh server, then the setup page is in French; the sign-in errors are translated from their `code`.
- [ ] Given the logs during these scenarios, then no password, PIN, token or poll secret appears in them.
- [ ] Given a Raspberry Pi 4 (or the throttled `make test-deb` profile), a sign-in completes in under 500 ms.

## Revisions
_None yet._
