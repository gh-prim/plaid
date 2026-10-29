---
id: F007
title: Web — onboarding and administration
owner: gh-prim
reviewer: TBD
status: in-progress      # draft | approved | in-progress | done
spec: ./spec.md
branch: feat/F007-onboarding-admin
---

# F007 — Plan: Web — onboarding and administration

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

Three PRs, each green on its own, each with `make test` output and screenshots:
- **PR A — Foundations**: the API changes and the three server additions (R27–R29), then the web foundations every screen needs (design tokens, fonts, primitives, hooks). No new screen yet.
- **PR B — Onboarding** (R1–R16): the wizard replaces the F003 `Setup` page.
- **PR C — Admin** (R17–R26).

The screens come from the Claude Design canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz). Its boards are named in the steps (`P07Unreadable`, `AD03`…) so each component can be checked against its mockup. Each PR attaches PNG exports of the boards it builds (spec watch-out "Canvas access").

## Files touched

### API
| File | Action | Why |
|------|--------|-----|
| `api/openapi.yaml` | modify | `ScanEventType` gains `matched`; new `MatchedItem` schema and `ScanEvent.item` (R27). `POST /server/capabilities/hardware-test` → `HardwareStatus`, `409 hw_test_running` (R28). `GET /problems` → `{problems: Problem[]}`, new `Problem` and `ProblemCode` schemas (R29). |
| `server/internal/api/api.gen.go`, `web/src/api/schema.d.ts` | regenerate | `make generate`, never by hand. |

### Server
| File | Action | Why |
|------|--------|-----|
| `server/internal/db/migrations/0006_problems.sql` | create | `problems` table and index (spec, Data model). |
| `server/internal/problems/problems.go` | create | `Recorder` with `Report(ctx, code, params)` and `Resolve(ctx, code, params)`; dedupe key = code + params sorted as JSON; `count`/`last_seen` bumped on repeat; `Resolve` sets `resolved_at`, a later `Report` reopens it. `List(ctx, since)`. A `Reporter` interface (`Report`, `Resolve`) with a no-op value for tests and nil options. |
| `server/internal/problems/janitor.go` | create | Hourly: delete rows resolved more than 7 days ago (R29). Started from `main`. |
| `server/internal/problems/problems_test.go` | create | Dedupe, reopen, resolve, purge, params never containing a value under a `token`/`key` name (guard). |
| `server/internal/metadata/matched.go` | create | Per-library hub of matched items, same shape as the counters hub in `admin.go` but unthrottled, buffered (64) and dropping the oldest when full. `SubscribeMatched(libraryID)`. |
| `server/internal/metadata/service.go` | modify | In `Process`, after a movie or show ends `matched`, load its `Summary` (`read.go`) and publish `{id, kind, title, year, poster, backdrop}` (R27). On `httpx.ErrInvalidKey` report `provider_key_refused` (provider in params); resolve it after the next successful call. New option `Problems problems.Reporter`. |
| `server/internal/metadata/matched_test.go` | create | Publish after match with artwork ids, nothing for seasons/episodes or `needs_review`, slow subscriber drops oldest. |
| `server/internal/handlers/scan_events.go` | modify | Subscribe to matched items next to counters and write `event: matched` (R27). |
| `server/internal/handlers/libraries_e2e_test.go` | modify | A scan whose items match sends `matched` events on the stream with the poster image id. |
| `server/internal/playback/service.go` | modify | `RetestHardware(ctx)`: runs `ProbeHardware` with the current config, clears `hwFails` and `DisabledReason`, stores the status; a mutex-guarded `testing` flag returns `ErrHWTestRunning` (R28). `hwFailed` reports `hw_fallback`; a successful retest or settings change resolves it. New option `Problems`. |
| `server/internal/playback/hls.go` | modify | At the "hardware transcode failed; restarting in software" warning (line ~204), report `ffmpeg_restarted` with the item title, never the token or command line. |
| `server/internal/playback/hwaccel_test.go` | modify | Retest clears a fallback; a second retest during one returns `ErrHWTestRunning`. |
| `server/internal/handlers/capabilities.go` | modify | `PostServerHardwareTest` handler, admin only, maps `ErrHWTestRunning` → `409 hw_test_running`. |
| `server/internal/handlers/problems.go` | create | `GetProblems`, admin only, last 7 days. |
| `server/internal/handlers/playback_admin_e2e_test.go` | modify | Hardware-test endpoint: member gets 403, admin gets the status, concurrent call 409. |
| `server/internal/handlers/problems_e2e_test.go` | create | A folder going unavailable twice gives one row with `count: 2`; member gets 403. |
| `server/internal/handlers/handlers.go` | modify | `Deps.Problems`. |
| `server/internal/library/engine.go`, `reconcile.go` | modify | Option `Problems`. At `reconcile.go:154` report `folder_unavailable`/`folder_unreadable` (path), resolve when the folder is `ok` again. At the watch-limit warnings (`engine.go:219`, `:255`) report `watch_limit` (path). |
| `server/internal/library/reconcile_test.go` | modify | Status change reports and resolves. |
| `server/internal/handlers/capabilities.go` (error code) | modify | `hw_test_running` built with `apierr.New(409, …)` like the other codes. |
| `server/cmd/plaid/main.go` | modify | Build the `problems.Recorder`, pass it to metadata, library, playback and handlers; start the janitor. |

### Web — foundations (PR A)
| File | Action | Why |
|------|--------|-----|
| `web/src/design/tokens.json` | create | Copy of the PLAID design system's `tokens.json` (version recorded in a `_source` field), the input of the generator. |
| `web/scripts/build-tokens.mjs` | create | Turns `tokens.json` into `web/src/design/tokens.css`: CSS custom properties for color (dark on `:root`, light under `[data-theme="light"]` and `prefers-color-scheme`), spacing, radius, shadow, duration, easing, type. Run by `pnpm -C web generate` (so `make generate`); `scripts/check-codegen.sh` gains a third diff for `tokens.css`. |
| `web/src/design/tokens.css` | generate | Output above, committed. |
| `web/public/fonts/InstrumentSans-*.woff2`, `InstrumentSerif-*.woff2`, `OFL.txt` | create | Self-hosted, open-licensed fonts (design system constraint), from the upstream Google Fonts release; no CDN. |
| `web/src/design/base.css` | create | `@font-face`, body, focus ring, reduced motion, the woven thread pattern (SVG data in CSS). Replaces most of `web/src/styles.css`. |
| `web/src/styles.css` | modify | Keep only what existing pages still use; remove the rest as screens move to the design system. |
| `web/src/ui/Button.tsx`, `TextField.tsx` (extends `components/Field.tsx`), `CodeBlock.tsx` (copy + "Copied"), `Banner.tsx`, `Toast.tsx`, `Dialog.tsx` (sheet on phone), `Chip.tsx`, `Thread.tsx` (woven progress), `PosterTile.tsx` (with missing-artwork state), `Backdrop.tsx` (blurred, crossfading), `SettingsGroup.tsx` (own Save + "Saved."), `index.ts` | create | Primitives used by both areas, built on the tokens. Each with a test of its states. Back-ported to F002 later (backlog #120). |
| `web/src/hooks/useScanEvents.ts` | create | `EventSource` on `/api/v1/libraries/{id}/scan/events` (same-origin cookie, GET needs no CSRF header), typed from `schema.d.ts`, reconnects, exposes progress, metadata counters and the latest matched items (R11, R12, R27). |
| `web/src/hooks/usePolling.ts` | create | Interval fetch that pauses on `document.hidden` (R18, R19 NFR). |
| `web/src/hooks/useConnection.ts` | create | `GET /health` retry ladder 2 s, 4 s, 8 s, then 15 s, with "try now" (R16). |
| `web/src/lib/format.ts` | create | `Intl` helpers: numbers, bytes, bitrate, durations, relative time, language and region names (`Intl.DisplayNames`) (R32). |
| `web/src/lib/shellQuote.ts` + test | create | Single-quote a path, escaping `'` (R9). |
| `web/src/lib/cidr.ts` + test | create | Validate one CIDR per line, IPv4 and IPv6 (R24). |
| `web/src/lib/errors.ts` (from `auth/errors.ts`) | modify | One mapping from API error code and `validation_failed` details to i18n keys, with field overrides and the `max` limit read from the schema (R34). |
| `web/src/locales/en/common.json`, `fr/common.json` | modify | Keys from the spec's i18n table, added PR by PR as screens land (so `check-i18n` never sees unused keys). |
| `web/src/test/fakeApi.ts`, `web/src/test/fakeEventSource.ts` | modify / create | Fake SSE for scan tests. |

### Web — onboarding (PR B)
| File | Action | Why |
|------|--------|-----|
| `web/src/onboarding/step.ts` + test | create | Derives the wizard step from `GET /setup`, `/me`, `/libraries` and the local "later" flag (R1, R10). Extends `auth/RequireAuth.tsx`'s `currentStep`. |
| `web/src/onboarding/Start.tsx` | create | Language and the serif start line (R3). Boards `P01Start`, `D01Start`. |
| `web/src/onboarding/SetupCode.tsx` | create | R4, R5. Boards `P03Code`, `P04Locked`, `P05Done`. |
| `web/src/onboarding/Account.tsx` | create | R6. Board `P02Account`. Replaces `pages/Setup.tsx`. |
| `web/src/onboarding/Libraries.tsx`, `LibraryForm.tsx` | create | R7. Boards `P06Libraries`, `P11Added`, `P14Zero`. |
| `web/src/onboarding/FolderPicker.tsx`, `FolderRow.tsx` | create | R8, R9 (shared with the admin). Boards `P07`–`P10`, `CmpPicker`. |
| `web/src/onboarding/Scan.tsx`, `PosterWall.tsx`, `ServerFacts.tsx` | create | R11–R13. Boards `ScanBAfter1440`, `P12Scan`, `D12Scan`, `Motion`. |
| `web/src/onboarding/ScanEnd.tsx` | create | R14, R15. Boards `End375`, `End1440`, `P13End`. |
| `web/src/onboarding/*.test.tsx` | create | The onboarding acceptance criteria, one test file per screen. |
| `web/src/App.tsx` | modify | `/setup/*` routes; `/setup` no longer renders `pages/Setup`. |
| `web/src/auth/RequireAuth.tsx` | modify | Route onboarding steps; loading text uses a new `common.loading` key (R32, see Log). |
| `web/src/pages/Setup.tsx`, `Setup.test.tsx`, `Welcome.tsx`, `Welcome.test.tsx` | delete | Replaced by the wizard; `Home` keeps its own content until F008. |
| `web/src/pages/Home.tsx` | modify | Empty state and the admin link (R15, R17). |

### Web — admin (PR C)
| File | Action | Why |
|------|--------|-----|
| `web/src/admin/AdminLayout.tsx` | create | Admin guard, "Back to the library", lazy-loaded chunk (R17, NFR bundle). |
| `web/src/admin/Home.tsx` | create | R18. Boards `AP01`, `AP02`, `AD01`, `AD02`. |
| `web/src/admin/Playing.tsx`, `StreamCard.tsx`, `reasons.ts` + test | create | R19: reason code + params → sentence key. Boards `AP03`, `AP04`, `AD03`. |
| `web/src/admin/Libraries.tsx`, `Library.tsx` | create | R20, reusing `FolderPicker`. Boards `AP05`, `AP06`. |
| `web/src/admin/Unrecognized.tsx`, `Review.tsx`, `TmdbSearch.tsx` | create | R21, R22. Boards `AP07`, `AP08`. |
| `web/src/admin/People.tsx`, `Person.tsx`, `PairTv.tsx` | create | R23; `PairTv` wraps the logic of `pages/Pair.tsx`. Boards `AP09`–`AP11`. |
| `web/src/admin/Transcoding.tsx` | create | R24, one `SettingsGroup` per setting. Boards `AP12`–`AP14`. |
| `web/src/admin/Metadata.tsx` | create | R25 with attributions. Board `AP15`. |
| `web/src/admin/Problems.tsx`, `problems.ts` | create | R26: problem code + params → what happened / what to do. Board `AP16`. |
| `web/src/admin/*.test.tsx` | create | The admin acceptance criteria. |
| `web/src/App.tsx` | modify | `/admin/*` routes (lazy). |

### Docs
| File | Action | Why |
|------|--------|-----|
| `scripts/check-codegen.sh` | modify | Regenerates `tokens.css` into the temp dir and diffs it (step 6). |
| `docs/install.md` | modify | The setup-code command (`grep first_run_setup_code`) matches the wizard (R5). |
| `sdlc/roadmap.md` | modify | Plan status. |

## Steps, in order

### PR A — Foundations
1. [x] `api/openapi.yaml`: `matched` event and `MatchedItem`, hardware-test route, `/problems` and its schemas, `hw_test_running`. `make generate`. (R27, R28, R29)
2. [x] Migration `0006_problems.sql` and `internal/problems` with its tests first (dedupe, reopen, resolve, purge). (R29)
3. [x] Wire `Problems` into library (folder status, watch limit), playback (`hw_fallback`, `ffmpeg_restarted`), metadata (`provider_key_refused`), with a unit test at each call site; `GET /problems` handler and e2e test. (R29)
4. [x] `RetestHardware` + handler + tests (403, 200, 409 concurrent, fallback cleared and resolved). (R28)
5. [x] Matched hub in metadata, publish from `Process`, forward on the scan stream; unit test and e2e test on the stream. (R27)
6. [x] Web: `tokens.json` → `build-tokens.mjs` → `tokens.css`, hooked into `make generate` and the codegen drift check. Fonts and `base.css`. (R33)
7. [x] Web primitives in `src/ui/` with their state tests, then `useScanEvents` (with `fakeEventSource`), `usePolling`, `useConnection`, `format`, `shellQuote`, `cidr`, error mapping, all test-first. (R9, R16, R24, R32, R34)
8. [ ] `make test`; PR A with a screenshot of a primitives page in dark and light (dev-only route, removed in PR B).

### PR B — Onboarding
9. [x] `onboarding/step.ts` with tests for every resume case, then routes in `App.tsx` and `RequireAuth`. (R1, R2, R10)
10. [x] Start and language, tests for preselection and instant switch. (R3)
11. [x] Setup code: tests for wrong code, `Retry-After`, `setup_complete`, the exact copy-pasted command. Update `docs/install.md`. (R4, R5)
12. [x] Account: client-side checks and server error mapping, tests first. Delete `pages/Setup*`, `pages/Welcome*` and their keys. (R6)
13. [x] Folder picker and row: drill-down, roots, filter over 20, unreadable fix with quoted command and Check again, empty folder, type a path; tests first from the acceptance criteria. (R8, R9)
14. [x] Libraries step: form, chips, overlap and not-found errors, several libraries, "later". (R7)
15. [x] Scan: counters from fake SSE, thread fill formula, poster wall capped at 48, backdrop crossfade, voice line throttles (3 s visual, 10 s `aria-live`), reduced motion, server facts. (R11–R13)
16. [x] End and zero-library end, oldest/newest from items. (R14, R15)
17. [x] Connection banner on every step. (R16)
18. [x] Locale pass: French and a pseudo-locale at +35 % (test helper that pads every string) render at 375 px with no overflow (jsdom can't measure layout: this is a manual check with screenshots, see Evidence). (R33)
19. [ ] `make test`; real run against `make dev` with a 40-movie folder; PNG exports of `P01`–`P15`, `D01`–`D15` next to screenshots; PR B.

### PR C — Admin
20. [ ] `AdminLayout`, lazy routes, member gets forbidden with no admin request (test). (R17)
21. [ ] Playing now and `reasons.ts` (every F006 reason has a sentence, test), stop with confirmation. (R19)
22. [ ] Admin home from playback, libraries, capabilities and problems, every status variant tested. (R18)
23. [ ] Libraries and one library, reusing the picker; scan now, full rescan, language change, delete with confirmation. (R20)
24. [ ] Not recognized and to review with TMDB search. (R21, R22)
25. [ ] People, a person, devices, pair a TV, `last_admin`. (R23)
26. [ ] Transcoding: one group per setting, only the changed field sent, CIDR validation, hardware test and fallback. (R24, R28)
27. [ ] Metadata with keys write-only and attributions. (R25)
28. [ ] Problems. (R26, R29)
29. [ ] Locale pass and light theme as in step 18. (R33)
30. [ ] `make test`; real run on a server with a transcoding stream (the F006 perf clips) and a folder made unavailable; PNG exports `AP01`–`AP17`, `AD01`–`AD17` next to screenshots; PR C. Plan status `done`, roadmap updated.

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| The poster wall is janky on a phone or makes the Pi work (image requests, DOM growth) | medium | DOM capped at 48 tiles, `w342` posters, `loading="lazy"`, CSS transitions only (no JS animation loop), checked on a mid-range phone in step 19. |
| `matched` events missed (buffer full, client reconnect) leave holes in the wall | medium | The wall is a view: counters stay exact from `metadata` events; on reconnect the web fetches the latest matched items once (`GET /libraries/{id}/items`, first page) to refill. |
| Design system tokens drift from the Claude Design source | medium | `tokens.json` is copied with its source version; updating it is one generator run and the codegen drift check keeps `tokens.css` in sync. |
| Problems recorded too often (a flapping NFS mount) bloat the table or the admin | low | Dedupe on code + params bumps `count`; one row per folder. |
| `EventSource` can't send `X-Plaid-Client` | low | The route is a GET; CSRF only applies to state-changing requests (F003-R10). Covered by the e2e test in step 5 and a web test. |
| The wizard and F003's guards loop (setup ↔ login ↔ onboarding) | medium | `step.ts` is the single source of truth, tested for every combination of setup, session, libraries and "later" before any route uses it. |
| Hardware retest while transcodes run disturbs them | low | The test uses its own short FFmpeg runs like F006's startup test; running sessions keep their encoder until they restart. |
| Layout checks can't run in jsdom | high | Manual screenshots at 375 and 1440 in en, fr and the +35 % pseudo-locale are part of the evidence; no claim of automated layout coverage. |

**Riskiest step:** step 15, the scan screen. It combines the SSE stream, two kinds of events at different rates, the poster wall's performance on a phone, the backdrop crossfade, reduced motion and two accessibility throttles, and it is the emotional peak the design was rebuilt around. It gets its tests first and a real-device check before PR B is opened.

## Rejected options
- **Polling items for the poster wall instead of a `matched` event**: `GET /libraries/{id}/items` sorts by title, so new matches can't be found without a new sort; polling 1,240 items every few seconds is also heavier on a Pi than one event per match.
- **Problems from a `slog` handler** (record every warning that has a `code`): no extra call sites, but resolution can't be expressed, params would come from free-form log attributes, and a log rename would silently break the admin.
- **A CSS framework or component library** (Tailwind, Radix themes): the design system already defines tokens and shapes; a framework adds its own defaults, the generic look the design work removed.
- **Building F002's design system in its own feature first**: F002 has no spec yet; doing it first would block F007 for a feature without a plan. Primitives are built here on the tokens and back-ported (backlog #120).
- **One PR for all of F007**: too big to review well (three areas, ~60 files); the split follows F006's.
- **Storing the wizard's position on the server**: needs a new setting and API for a state the server can already derive (R10).

## Expected evidence
- [ ] Tests added: `internal/problems` (dedupe, reopen, resolve, purge), call sites in library/playback/metadata, matched hub, `/problems` and hardware-test e2e, scan stream `matched` e2e; web tests for every acceptance criterion in the spec, `step.ts` resume cases, `shellQuote`, `cidr`, `reasons.ts`, error mapping, `useScanEvents`.
- [ ] `make test` green, output pasted in each PR (last line `✅ all checks passed`).
- [ ] `make check-codegen` green after the tokens generator joins it.
- [ ] Screenshots next to the canvas exports: onboarding `P01`–`P15`, `D01`–`D15`; admin `AP01`–`AP17`, `AD01`–`AD17`; French and +35 % at 375 px; light theme on the scan and the admin home.
- [ ] Manual check: fresh server on the local network, 40-movie folder, from opening the URL to the first poster in under 3 minutes, 3 people timed.
- [ ] Manual check: setup from outside (`curl` through a non-trusted proxy) asks for the code; the documented `journalctl … | grep first_run_setup_code` finds it.
- [ ] Manual check: unreadable folder on a real Linux box, fix command pasted as shown, Check again turns it readable.
- [ ] Manual check: scan of the large library over NFS on a Pi, the wall stays smooth on a mid-range phone.
- [ ] Manual check: a software transcode below 1.0× shows "Can't keep up" and the advice; Stop ends it.
- [ ] Manual check: unplug the NAS share, the folder turns Unavailable, the home lists it, Problems has one entry; plug it back, the entry is Fixed.
- [ ] Manual check: keyboard only and VoiceOver through the wizard; `prefers-reduced-motion` on.

## Log
Deviations from the plan found during the build (date + what + why).
- 2026-10-28 (planning): the spec removes `welcome.status.loading`, but `RequireAuth` shows it while auth loads. Replaced by a new `common.loading` key ("Loading" / "Chargement"); the spec gets a revision line in PR B.
- 2026-10-28 (planning): the web had no design system code (`styles.css` is "deliberately minimal until F002 lands"). PR A brings the tokens and primitives F007 needs; this is the web part of F002 done early, back-ported later.
- 2026-10-29 (PR A): the design system already ships F007's components (React, `window.Plaid`) with their CSS: the admin ones were added on 2026-10-27. PR A vendors `bundle.css` unchanged as `web/src/design/components.css` and ports the components to TypeScript in `web/src/ui/` (`Icon`, `Button`/`Spinner`, `TextField`, `controls` (Chip, SegmentedControl, ChoiceRow), `feedback` (Toast, Dialog, BottomSheet, ProgressBar, EmptyState), `CommandBlock`, `FolderRow`, `FolderPicker`, `scan` (PosterSlot, AmbientBackdrop, ServerFact)) instead of designing primitives. Hardcoded English in the bundle goes through i18n. Admin components are ported in PR C. Backlog #120 shrinks to whatever we change here.
- 2026-10-29 (PR A): `tokens.json` and `components.css` are vendored verbatim, so they're in `web/.prettierignore`; `tokens.css` is generated and formatted. `scripts/check-codegen.sh` diffs it.
- 2026-10-29 (PR A): the janitor is `RunJanitor` in `problems.go` (no `janitor.go`); `hw_test_running` is an `apierr` constructor like the other codes.
- 2026-10-29 (PR A): fix commands single-quote paths (`lib/shellQuote.ts`) where the design system shows double quotes: single quotes don't expand `$` or backticks in a folder name.
- 2026-10-29 (PR A): keys the spec table didn't have, needed by the ported components: `ui.*` (loading, show/hide password, clear, dismiss, progress), `common.close`, `picker.checking`, `picker.readable`, `picker.notAllowed`, `picker.rootsLabel`, `picker.openHint`, `picker.filterCount`, `folder.unavailable.title`, `folder.unavailable.check`; `picker.empty` is split into `.title` and `.body`; `picker.filter` takes `total`, not `count` (i18next treats `count` as a plural).
- 2026-10-29 (PR A): error mapping stays in `web/src/auth/errors.ts` (extended with the new field codes) instead of moving to `lib/errors.ts`: every page already imports it from there.
- 2026-10-29 (PR A): a development-only `/dev/ui` gallery (`web/src/dev/Gallery.tsx`, not routed in production builds) gives the PR A screenshots; PR B removes it.
- 2026-10-29 (PR B): the design puts the setup code on the account screen (board P03, a card beside the form), so there's no `/setup/code` route: `Account.tsx` shows the card when `GET /setup` says `requiresSetupCode`. No `SetupCode.tsx`.
- 2026-10-29 (PR B): the spec's i18n table has keys that are both a leaf and a parent (`onboarding.account.displayName` and `.displayName.help`), which a nested JSON catalog can't hold. The leaves get `.label` (`onboarding.account.{displayName,username,password}.label`, `onboarding.start.language.label`, `library.folders.label`, `library.type.label`, `library.metadata.region.label`) and `scan.complete` becomes `scan.complete.title`. `home.empty` is split into `.title` and `.body` like the design. New keys the screens needed: `onboarding.step`, `onboarding.code.docker`, `onboarding.libraries.added`, `picker.open`, `common.loading`, `library.metadata.language`, `scan.complete.count`, `scan.complete.wall`, `scan.libraries`, `scan.startWatching`, `scan.state.short.scanning`, `scan.wall`, `server.accel.*`. `errors.setup_code_required` takes the spec's new wording.
- 2026-10-29 (PR B): `RequireStep` gains `admin` so `/setup/libraries` and `/setup/scan` send members home; `Home` sends an admin back to the wizard through `adminStep` (`onboarding/step.ts`).
- 2026-10-29 (PR B): the real run on a scratch server found two picker bugs, now fixed with tests: `/` showed `//`, and the parent button was enabled at `/`. "Type a path" is also offered in the desktop panel, not only on phones (R8).
- 2026-10-29 (PR B): the end screen's quiet line about unrecognized files doesn't link to the admin yet: `/admin/libraries` arrives in PR C, which adds the link.
- 2026-10-29 (PR B): the start button of the end screen is `primary`, not `play`: the design system reserves `play` for starting playback.
- 2026-10-29 (PR B): the `/dev/ui` gallery from PR A is removed, as planned.
