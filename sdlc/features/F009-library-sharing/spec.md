---
id: F009
title: Library sharing
owner: gh-prim
reviewer: TBD
status: draft            # written after the code (team's request, 2026-11-02); approved in PR 77's review
intent: ./intent.md
decisions: [0002, 0009]
---

# F009 — Spec: Library sharing

## Summary
The admin shares libraries with friends and family through **groups**. A group says which libraries its members see and what they can do: best quality, streams at the same time, downloads, Plaid Party, adding profiles. A person can be in several groups and gets the best of each; the admin can add exceptions to one person, which only ever add. Household groups see every library, now and later, and a library can be kept to the household (home videos). The admin **invites** someone with a single-use link that names them and their groups; the person creates their own account and PIN from it. Every profile shows an **avatar** built in PLAID (DiceBear's Lorelei style, CC0, rendered by the server), a photo, or its initial. Screens come from the Claude Design canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz), page V2, steps 19 (sharing, boards Share01–Share03) and 20 (invited account, avatars).

## User journeys
**Admin shares with friends**
1. People → New group "Close friends": libraries Films and Kids, best quality 1080p, 2 streams per person, Plaid Party "can join".
2. Invite → "Romain", group Close friends, valid 7 days → Create the link → Copy.
3. Later, People shows Romain in Close friends, last seen and on which device. The admin gives him one more library as an exception, or suspends him.

**Invited friend**
1. Opens `/invite/{token}` (on an iPhone or iPad, the app opens through `plaid://invite`).
2. Sees who invited them and to which server; chooses a username, a password and a 4-digit PIN.
3. Builds an avatar (or takes a photo), lands on the home with Films and Kids only. Search, home rows, item pages and playback never show anything else.

**Household**
1. Existing members were moved into "Family" at migration: nothing changes for them.
2. "Home videos" is set household-only: no group but household ones can get it, and no exception can add it.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| F009-R1 | Groups have a name, a household flag, libraries and rights: best quality (original, 2160p, 1080p, 720p, 480p), streams at the same time per person (1–20 or no limit), downloads, Plaid Party (none, join, start), can create profiles. | must |
| F009-R2 | A person can be in several groups; their access is the union of their groups (more libraries, the best quality, the most streams, any right granted). | must |
| F009-R3 | Exceptions per person (extra libraries, extra rights) only add on top of the groups. | must |
| F009-R4 | A household group sees every library, present and future, household-only ones included. | must |
| F009-R5 | A household-only library can't be given to a non-household group nor as an exception (422). | must |
| F009-R6 | Admins are never filtered. | must |
| F009-R7 | Every read filters by the caller's libraries: library list, items, item pages, search, home rows, facets, collections, people, up next, artwork, watch state and playback. An item outside them answers 404, as if it didn't exist. | must |
| F009-R8 | Playback caps the picture at the person's best quality (transcode down, reason `quality_capped`). | must |
| F009-R9 | Starting a stream past the person's limit answers 429 `stream_limit_reached`. | must |
| F009-R10 | Suspending a person ends their streams, signs them out everywhere and keeps the account; it can be undone. Removing a library from their access ends the streams of that library. | must |
| F009-R11 | Migration: a "Family" household group exists and holds every member already there. | must |
| F009-R12 | An invitation has a name, at least one group, and expires (1–30 days, 7 by default). Its link works once. | must |
| F009-R13 | `POST /invite/check` and `POST /invite/accept` are public; unknown tokens count toward a per-address backoff. Accepting creates a member in the invitation's groups with a 4-digit PIN on its first profile, and signs them in. | must |
| F009-R14 | The admin lists pending invitations and cancels one. | must |
| F009-R15 | A profile's picture is an avatar (Lorelei options), a photo (cropped square, re-encoded as a 512 px JPEG without metadata, 8 MB max) or its initial on its color. The server renders avatars as PNG for every app. | must |
| F009-R16 | Groups can grant "can create profiles"; without it, a member can't add a profile (403). Family has it, new groups don't. | must |
| F009-R17 | Downloads and Plaid Party rights are stored and returned, for those features to read later. | should |
| F009-R18 | People shows each person's groups, last seen and last device; the admin moves people between groups in bulk. | should |
| F009-R19 | iOS opens invitation links natively, builds avatars and takes photos; tvOS and iOS show pictures. | should |

## Design

### Data model
Migration `0012_sharing.sql`:
- `libraries.household_only` (0/1).
- `groups` (id, unique name, household, max_quality, max_streams NULL = no limit, downloads, party, can_create_profiles, created_at), `group_libraries`, `group_members`.
- `user_rights` (one row per person with exceptions: max_quality, max_streams, unlimited_streams, downloads, party, can_create_profiles, all nullable = from the groups) and `user_libraries` (extra libraries).
- `invitations` (id, token_hash sha256, name, created_by, created_at, expires_at, used_at, used_by, revoked_at) and `invitation_groups`.
- `profiles.avatar_kind` (initial, avatar, photo), `avatar_options` (JSON), `avatar_version`; `profile_photos` (profile_id, jpeg BLOB).
- Inserts Family (`01900000-0000-7000-8000-000000000001`, household, can create profiles) with every existing member.

`access.Limit` gains `Libraries []string` (nil: all) next to the age limit, so every catalog query adds one `library_id IN (…)` clause through `Limit.SQL`.

### API
| Method | Route | Description |
|--------|-------|-------------|
| GET, POST | `/api/v1/groups` | List groups (members, libraries, rights); create one |
| PATCH, DELETE | `/api/v1/groups/{groupId}` | Rename, household, libraries, rights; delete (members keep their other groups) |
| GET, POST | `/api/v1/invitations` | Pending invitations; create one (returns the token once) |
| DELETE | `/api/v1/invitations/{invitationId}` | Cancel |
| POST | `/api/v1/invite/check` | Public: who it's for, who invited, expiry |
| POST | `/api/v1/invite/accept` | Public: create the account and sign in |
| PUT | `/api/v1/me/profiles/{profileId}/avatar` | Choose initial or avatar (options) |
| PUT | `/api/v1/me/profiles/{profileId}/avatar/photo` | Upload a photo (`image/*`, 8 MB max) |
| GET | `/api/v1/avatars/{profileId}` | The picture, PNG or JPEG, `?size=` |
| GET | `/api/v1/avatar-builder`, `/api/v1/avatar-builder/render` | The parts and their variants; a preview |

`AdminUser` gains `groupIds`, `extraLibraryIds`, `extraRights`, `access` (effective), `lastSeenAt`, `lastDevice`, `avatar`; `PATCH /users/{id}` takes `groupIds`, `extraLibraryIds`, `extraRights`. `Me` gains `access`; `Library` gains `householdOnly`; `Profile` gains `avatar`. Errors: `invitation_not_found` (404), `invitation_expired` (410), `stream_limit_reached` (429).

### Behavior per app
- **server**: `accounts` computes access (`Access(ctx, userID)`) from groups and exceptions; `access.Limit` filters every read; playback reads the best quality and stream limit at start, and `Recheck` ends streams when access changes; `avatar` renders Lorelei from DiceBear's JSON definition (layers, masks) and rasterizes with oksvg.
- **web**: People with groups beside the table and bulk actions; group page (libraries, rights, members); person sharing (groups, exceptions, effective access); library "who can see it" and household-only; Invite dialog; `/invite/:token`; `/me/avatar` with the builder and photo crop; pictures in the profile picker, menu and admin.
- **apple**: iOS `plaid://invite` and "I have an invitation link", the invite flow, builder, photo crop and camera, "Change the picture"; tvOS and iOS show pictures.
- **android**: out of scope for now.

### UI / Art direction
Boards Share01 (People with groups), Share02 (a group), Share03 (invite) and step 20's onboarding and avatar boards. Loading states show a spinner; empty groups and no invitation have their own lines.

### i18n
Namespaces `sharing.*`, `avatar.*`, `invite.*` and errors `errors.invitation_*`, `errors.stream_limit_reached`, in `en` and `fr` (web catalogs and Apple String Catalogs). Plurals for member, library and stream counts use `_one/_other` (`_many` in French).

## Non-functional requirements
- Filtering adds one indexed `IN` clause; catalog NFRs (F008) still hold.
- Avatars are rendered on request and cached by clients through `avatar_version` in their URL; photos are capped at 8 MB and 50 megapixels (decompression bombs).
- No third-party call: DiceBear's definition is embedded (CC0 style, MIT engine).

## ⚠️ Watch-outs
- [x] A forgotten filter leaks a library: every catalog entry point goes through `access.Limit`, and `TestSQLMatchesAllows` checks the SQL against the Go rule.
- [x] Invitation tokens: stored hashed, single use, backoff on unknown tokens; the PIN is required on accept.
- [x] Uploaded photos: decoded and re-encoded (no EXIF, no polyglot), size-capped.
- [x] Avatar SVG: built from our own definition, no user markup (`TestSVGHasNoScript`).
- [x] Last admin: can't be suspended, demoted or deleted (409 `last_admin`).
- [ ] Downloads and Plaid Party rights do nothing until those features exist (tracked in F010 and the backlog).

## Acceptance criteria
- [x] Given Close friends with Films, when Camille (only in it) lists libraries, searches or opens a Kids item, then she sees Films only and the item answers 404.
- [x] Given a second group with Kids, when Camille joins it, then she sees both.
- [x] Given home videos household-only, when the admin shares it with Close friends, then 422; a household group sees it.
- [x] Given an invitation, when it's accepted once, then the account exists in its groups with a PIN, and a second use answers 410.
- [x] Given a member suspended, then their sessions end and sign-in fails until restored.
- [x] Given a 1080p cap, when a 4K file plays, then it's transcoded to 1080p with `quality_capped`.
- [ ] Given a limit of 1 stream, when a second starts, then 429 `stream_limit_reached` (implemented; no end-to-end test yet).
- [x] Given an avatar chosen, then `/avatars/{profileId}` serves its PNG to every app; a photo is a 512 px JPEG.

## Revisions
- 2026-11-02: written after the code at the team's request; the decisions were recorded in the intent while coding.
- Step 21 (F015): the invitation list also returns accepted and expired invitations for 30 days and a pending link's token (migration 0014).
