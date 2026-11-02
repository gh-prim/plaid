---
id: F009
title: Library sharing
owner: gh-prim
reviewer: TBD
status: in-progress      # written after the code (team's request); approved in PR 77's review
spec: ./spec.md
branch: feat/F009-library-sharing
---

# F009 — Plan: Library sharing

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.
> Written after the code at the team's request (2026-11-02): it records what was built, in the order it was built.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` | modify | Groups, invitations, avatars, access fields, errors (R1–R16) |
| `server/internal/db/migrations/0012_sharing.sql` | create | Tables, household flag, avatar columns, Family (R11) |
| `server/internal/access/access.go` | modify | `Limit.Libraries`, `Allows`, `SQL` (R7) |
| `server/internal/accounts/sharing.go` | create | Groups CRUD, rights union, `Access`, `LastSeen` (R1–R6) |
| `server/internal/accounts/invitations.go` | create | Create, list, revoke, check, accept with backoff (R12–R14) |
| `server/internal/accounts/avatars.go` | create | Avatar options, photo storage, default pictures (R15) |
| `server/internal/accounts/{users,profiles,accounts,janitor}.go` | modify | Group and exception patches, profile creation right, suspension (R3, R10, R16) |
| `server/internal/avatar/{avatar,path,photo}.go`, `lorelei.json` | create | Lorelei renderer, path normalization for oksvg, photo normalization (R15) |
| `server/internal/catalog/*.go`, `library/{items,store}.go`, `metadata/{admin,artwork_sync}.go` | modify | Filter every read by `access.Limit` (R7) |
| `server/internal/playback/{decide,service,watchstate}.go` | modify | Quality cap, stream limit, `Recheck` on access change (R8–R10) |
| `server/internal/handlers/{sharing,avatars}.go` | create | The new endpoints |
| `server/internal/handlers/{users,me,libraries,items,catalog,convert,playback,metadata}.go` | modify | Access fields, `householdOnly`, filters, 429 |
| `server/internal/auth/middleware.go`, `apierr/apierr.go` | modify | Public invite routes; new errors |
| `web/src/admin/{Sharing,People,Libraries}.tsx`, `routes.tsx` | create/modify | Groups, group page, person sharing, invite dialog, library sharing (R18) |
| `web/src/avatar/*`, `web/src/pages/{Invite,MyAvatar}.tsx` | create | Builder, photo crop, invite flow (R13, R15) |
| `web/src/{ui/admin.tsx,pages/ProfilePicker.tsx,browse/*}` | modify | Pictures everywhere |
| `apple/iOS/Avatar/AvatarViews.swift`, `apple/iOS/SignIn/InviteView.swift` | create | iOS builder, photo, invite flow (R19) |
| `apple/Common/{API,App,RemoteImage.swift}`, `apple/iOS/*`, `apple/project.yml` | modify | Models, client calls, deep link, camera permission, pictures |
| locales (`web/src/locales/*`, `Localizable.xcstrings`, `InfoPlist.xcstrings`) | modify | en and fr strings |

## Steps, in order
1. [x] API contract for groups, invitations, avatars and the new fields; `make generate`. (R1–R16)
2. [x] Migration 0012 with Family and every existing member in it; incremental-migration test drops the new tables. (R11)
3. [x] `access.Limit` with libraries; `Allows` and `SQL` kept equal by `TestSQLMatchesAllows`; every catalog, library, metadata, artwork and watch-state read takes the limit. (R7)
4. [x] Groups, rights union, exceptions, household and household-only rules in `accounts`. (R1–R6, R16)
5. [x] Playback: quality cap in `Decide`, stream limit at start, `Recheck` on suspension, deletion or access removed. (R8–R10)
6. [x] Invitations with hashed tokens, backoff and PIN on accept. (R12–R14)
7. [x] Avatar renderer (DiceBear Lorelei definition, masks, oksvg) and photos; `/avatars/{profileId}`. (R15)
8. [x] Web admin: People with groups, group page, person sharing, library sharing, invite dialog. (R18)
9. [x] Web invite flow and avatar builder. (R13, R15)
10. [x] iOS invite, builder, photo; tvOS and iOS pictures. (R19)
11. [x] People redone as boards Share01 and Share03.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| A read path forgets the filter and leaks a library | medium | One `access.Limit` type for every read; e2e test as a member across lists, search, items, home; `TestSQLMatchesAllows` |
| Existing members lose access at upgrade | low | Migration puts them all in Family (household) |
| oksvg misdraws the avatars | happened | `normalizePath` turns minified paths absolute and arcs into cubics; tests draw every variant |
| Invitation links guessed | low | 256-bit tokens, hashed, backoff per address |

**Riskiest step:** step 3, filtering every read: one missed query shows a friend a library they shouldn't see.

## Rejected options
- **Sharing per collection**: dropped with the team; a library is the unit of sharing.
- **Rendering avatars in each app** (DiceBear JS on web, Swift port): three renderers to keep equal; the server renders one PNG for all.
- **Avatar services online** (DiceBear API, Gravatar): a third-party call per picture, against self-hosting.
- **Restrictive exceptions**: exceptions only add, so a person's access is easy to read.

## Expected evidence
- [x] Tests added: `TestGroupsDecideWhatMembersSee`, `TestInvitation`, `TestAvatars`, `TestAllows`, `TestSQLMatchesAllows`, avatar rendering and path tests, the quality-cap case in `TestDecide`, web tests for People, groups and invitations.
- [ ] End-to-end test for the stream limit (429).
- [x] `make test` green (pasted in PR 77).
- [ ] Screenshots attached to the PR.
- [x] Manual check: invite → account → PIN → avatar → home on web and the iOS simulator; deployed to srv-plex-prod.

## Log
- 2026-11-02: coded before the spec at the team's request; decisions recorded in the intent while coding.
- 2026-11-02: point 9 (profiles for friends) settled as a group right, `canCreateProfiles`.
- 2026-11-02: oksvg misread DiceBear's minified paths; `avatar/path.go` normalizes them.
- 2026-11-02: People was rebuilt after the first deploy, the groups weren't findable (boards Share01, Share03).
- 2026-11-02: step 21 (F015) added invitation history and copyable links on top of R12–R14.
