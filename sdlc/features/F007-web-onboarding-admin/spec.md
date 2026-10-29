---
id: F007
title: Web — onboarding and administration
owner: gh-prim
reviewer: TBD
status: approved
intent: ./intent.md
decisions: [0003, 0007]
---

# F007 — Spec: Web — onboarding and administration

## Summary
F007 gives PLAID's web app its first two real surfaces. The **first-run wizard** takes the admin from a blank server to their own posters on screen: language, the setup code only when they come from outside the local network, the admin account, libraries picked from the server's own folders (with the fix for an unreadable folder right there), then a first scan where matched posters fill a wall and the last one's backdrop lights the room. The **admin area** opens on one sentence about the server's state and what needs the admin, then six sections: Playing now (with why each stream is converted), Libraries (folders, unrecognized files, items to review), People (users, profiles, devices, TV pairing), Transcoding, Metadata and Problems. Screens and copy come from the Claude Design canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz) (pages Onboarding and Admin). F007 is mostly web on top of the F003–F006 API; it adds three small server pieces the design needs: matched items in the scan stream, a GPU re-test and a problems feed.

## User journeys

**Admin, first run on the computer they installed from (local network)**
1. The install script printed `http://mediabox:32500`. They open it. `GET /setup` says setup isn't complete and no code is required.
2. Start screen: "Tonight's movies will come from mediabox." English or Français, preselected from the browser; switching it changes the screen at once.
3. Admin account: display name, username, password. `POST /setup/admin` with the chosen `locale`. They're signed in.
4. Libraries: Movies, name "Movies", metadata language preset to their locale. "Add folder" opens the picker on the server's folders: `/srv` → `media` → `movies` shows "No access", with why, the command to copy and "Check again". They run it over SSH, check again, the folder is readable, "Use this folder". "Add library" creates it and starts its scan.
5. Scan: posters arrive one by one in the wall, the backdrop of the latest matched title glows behind, one serif line says "Dune: Part Two just arrived. 412 of your movies are here." Counters stay exact in one line. They can close the page.
6. End: "From Metropolis to Sinners, 1,240 movies are home." One amber button: "Start watching". A quiet line: 23 files weren't recognized and 18 titles need a second look, in Settings, Libraries.

**Admin, first run from outside (public IP, reverse proxy not trusted)**
1. `GET /setup` returns `requiresSetupCode: true`. After the language, a screen explains they're connecting from outside and asks for the setup code, with where to find it (journalctl, Docker logs, the terminal).
2. A wrong code shakes nothing: the field says the code doesn't match. After too many attempts: "Try again in 5 min".
3. If someone else finished setup meanwhile (`409 setup_complete`): "This server is already set up", with "Sign in".

**Admin, a friend says "it's buffering"**
1. From the library, the server strip opens the admin home: "mediabox. 3 people are watching. One stream can't keep up."
2. Playing now lists streams worst first. Sam's stream: Converted (Transcode), In software, "Can't keep up (0.8×): it will buffer", the reason in one sentence ("This device can't play HEVC video, so it's converted."), and the advice to pick a lower quality or another device.
3. They can stop the stream after a confirmation.

**Admin, a NAS share went down**
1. Admin home: "One thing needs you." Libraries shows `/mnt/nas/Films` "Unavailable": its movies are hidden until it's back, nothing is deleted, after 30 days missing they leave the library, and what to check.

**Admin, a wrong poster**
1. Libraries → Movies → To review (18): an item with three TMDB candidates, each with poster, year and match percentage. "This one" locks the match. "Search TMDB" accepts a title, a year or a TMDB link.

**Admin, adds a person and pairs the living-room TV**
1. People → "Add a person": display name, username, temporary password, role. They'll choose their own at first sign-in.
2. "Pair a TV": the code shown on the TV, then the profile to start on.

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| **Onboarding** | | |
| F007-R1 | While `GET /setup` says setup isn't complete, every web route leads to the wizard at `/setup`. Once it's complete, `/setup` leads to sign-in, or to the onboarding's next step for a signed-in admin (R10). | must |
| F007-R2 | Step order: language → setup code (only when `requiresSetupCode`) → admin account → libraries → scan → end. Every step has Back except the scan and the end. | must |
| F007-R3 | Language: English and Français, each named in its own language, preselected from the saved choice, then the browser, then English (existing `createI18n` order). Changing it re-renders at once. It's sent as `locale` with `POST /setup/admin`, which makes it the admin's locale and, through F005-R10, the default metadata language of the libraries they create. | must |
| F007-R4 | Setup code: shown only when `GET /setup` returns `requiresSetupCode: true`. One text field, case-insensitive, paste-friendly, no auto-submit. "Where to find it" gives the exact commands (R5). `setup_code_required` after submit → field error; `too_many_attempts` → disabled with the minutes from `Retry-After`; `setup_complete` → the "already set up" screen with Sign in. | must |
| F007-R5 | The setup-code commands match what the server logs (`first_run_setup_code=<8 characters>`, a new code at each start while unconfigured): `sudo journalctl -u plaid \| grep first_run_setup_code` (.deb), `docker logs plaid 2>&1 \| grep first_run_setup_code` (Docker), otherwise the terminal where PLAID started. Commands are constants in code, not translated, with a Copy button. | must |
| F007-R6 | Admin account: display name, username, password with Show/Hide, client-side checks that mirror F003-R4/R5 (so most errors show before submit), server errors mapped per field (`username_taken`, `weak_password`, `validation_failed` details). No password confirmation field and no strength meter: the server's rule (8+ characters, not common) is the help text. | must |
| F007-R7 | Libraries step: type (Movies, Shows; nothing else shown), name prefilled from the type, metadata language and region (defaults: the admin's locale and the browser's region, F005-R10), 1–16 folders as removable chips. "Add library" calls `POST /libraries`, which starts its first scan. Several libraries can be added; "Add libraries later" skips to the end with the zero-libraries variant. | must |
| F007-R8 | Folder picker on `GET /fs/browse`: one level per screen (drill-down), a parent button and the current path, folder rows that open on tap, "Use this folder" for the current folder, "Type a path" for an absolute path, a filter field when a folder has more than 20 sub-folders, and the roots list when `PLAID_BROWSE_ROOTS` is set. It never shows files or file counts: the API doesn't list them. | must |
| F007-R9 | Unreadable folder (`readable: false`, or `folder_unreadable` from `POST /libraries`): the row says "No access" and expands under itself with why, the fix command for the exact path (single-quoted, with any `'` escaped), the Docker variant, and "Check again" which re-browses that folder and says whether it's readable now. Errors `folder_not_found` and `folder_overlap` show on the chip. | must |
| F007-R10 | Resume: the wizard's position is derived from the server, not stored in the browser: setup incomplete → language/account; signed-in admin with no library and no "later" choice on this browser → libraries; a library scanning for the first time → scan. Leaving and coming back lands on the right step. "Later" is remembered in `localStorage` only (another browser lands on the home empty state, which offers to add a library). | must |
| F007-R11 | Scan screen, from `GET /libraries/{id}/scan/events` (one stream per library being scanned): per library its state and current folder (`scan.state.*`), one line of exact counters (found, analyzed, failed, unrecognized), one line of metadata counters (matched, to review, not found), a progress thread whose fill is `analyzed / found` during analysis, then `1 − pending / (pending + matched + needsReview + unmatched)` while metadata is fetched. The admin can leave at any moment ("You can close this page…"). | must |
| F007-R12 | The poster wall: each item matched during the scan (R27) appears in the wall as its poster arrives, newest first, with its title below. The backdrop of the latest matched title, blurred and dimmed, sits behind the screen and crossfades when it changes. The serif voice line names the latest title and the matched count, updated at most every 3 s. Items without artwork show the design system's missing-artwork state. At most the 48 latest posters are in the DOM. | must |
| F007-R13 | Server facts during the scan, only when they apply, from `GET /server/capabilities` and the library's folders: hardware acceleration in use or not (text differs for a GPU and for software only), FFmpeg missing, ffprobe missing, folders not watched live (network file system, watch limit reached, with the `sysctl` command). | must |
| F007-R14 | End of scan, when every library of the onboarding is `idle` with metadata `pending = 0`: the wall of their posters, the serif line with the oldest and newest movie by year (taken from `GET /libraries/{id}/items`; with one movie, the count line only), the shows and episodes line when there are shows, "Start watching" (amber, to `/`), and the unrecognized/review line linking to Libraries when either count is above zero. | must |
| F007-R15 | Zero libraries end: no wall; the home empty-state line and "Add a library". | must |
| F007-R16 | Connection lost on any step (network error or `GET /health` failing): a banner, not a page, with automatic retry (2 s, 4 s, 8 s, then every 15 s), "Try now", and "Connected again." when it's back. Finished steps are on the server, so nothing is lost. | must |
| **Admin** | | |
| F007-R17 | Admin area under `/admin`, admin-only (members get the existing forbidden handling, F003-R13). Entry: a server strip in the library header (F008 owns the library; until then, a link on `Home`) and the account menu. Every admin screen has "Back to the library". | must |
| F007-R18 | Admin home: the server name as title, one status sentence (quiet, N watching, one stream can't keep up, N things need you), the "needs you" list only when something does (unavailable or unreadable folders, a stream slower than 1.0×, hardware fallback, FFmpeg missing, open problems), then the six sections with a live one-line summary each. Data: `GET /playback`, `GET /libraries`, `GET /server/capabilities`, `GET /problems`. Polled every 10 s while visible. | must |
| F007-R19 | Playing now (`GET /playback`, every 5 s while visible): one card per stream, slowest first: who (user · profile on device), what (poster, title, position of duration), method with its human label, hardware or software, speed ("Keeps up" / "Can't keep up (0.8×): it will buffer"), quality, at home or from outside, and each reason as one sentence (`reason.*`). Under a slow software transcode, the advice line. "Stop" asks for confirmation, then `DELETE /playback/{id}`. Header: conversions in use out of `maxTranscodes`. Empty state when nothing plays. | must |
| F007-R20 | Libraries: each library with type, counts, last scan, scan state and progress, and its folders with status (`ok`, `unavailable`, `unreadable`), watched live or checked every N minutes, and file system type. Actions: Scan now, Full rescan (with what it does), add folder (the onboarding picker), remove folder, change metadata language and region (with what it refetches), add a library, delete a library (confirmation names what is forgotten and that files stay on disk). | must |
| F007-R21 | Not recognized: `GET /libraries/{id}/unrecognized`, paginated, file path and size; "Identify" opens a TMDB search (`GET /metadata/search`) and assigns with `POST /media-files/{id}/identify`. | must |
| F007-R22 | To review: `GET /libraries/{id}/review`, each item with its reason (several candidates, not found on TMDB, duplicate), what was parsed from the file name, and its candidates (poster, title, year, `confidence` as a percentage). "This one" → `POST /items/{id}/match`; "Search TMDB" accepts a title, a year or a TMDB link; "None of these" leaves it unmatched. The same "Fix match" is reachable from any item's admin view. | must |
| F007-R23 | People: users (`GET /users`) with role, disabled, must change password, profiles count; one person's view with their signed-in devices (`GET /users/{id}/sessions`) and Sign out per device; Add a person (temporary password, role); reset password, disable/enable, delete; `last_admin` explained. "Pair a TV" reuses the F003 `/pair` flow inside the admin. | must |
| F007-R24 | Transcoding (`GET`/`PATCH /settings/playback`, `GET /server/capabilities`): what the hardware test found (accelerator, device, encoders, decoders, tone mapping), the hardware fallback warning with "Test the GPU again" (R28), FFmpeg missing. Settings in small groups, each saved on its own with "Saved.": hardware acceleration (`auto` or one accelerator or off, with "Testing the hardware" while the server tests), conversions at once (default shown from `maxTranscodesDefault`), limit for remote streams, extra home networks (one CIDR per line, validated), encoder speed, space for conversions (default from `transcodeDirMaxBytesDefault`). Raw enum values are never shown alone. | must |
| F007-R25 | Metadata (`GET`/`PATCH /metadata/settings`, `GET /metadata/status`): per provider the key source (built-in, environment, yours, none) and state; "Use my own key" stores a key that is never shown again; per-library metadata progress; the provider attributions from `GET /server` (F005-R37). | must |
| F007-R26 | Problems (`GET /problems`, R29): the last 7 days, newest first, each entry saying what happened and what to do, resolved entries marked "Fixed {time}". Empty state. | should |
| **Server additions** | | |
| F007-R27 | Scan events gain a `matched` event: `{type: "matched", item: {id, kind, title, year, posterImageId?, backdropImageId?}}`, sent on the library's existing `/scan/events` stream when F005 matches a movie or a show (not seasons or episodes) during or after that library's scan. Buffered per connection like other events; a slow client may miss some (the wall is a view, the counters stay exact). | must |
| F007-R28 | `POST /server/capabilities/hardware-test` (admin) re-runs the hardware test (F006-R24) and clears a fallback, returning the new `HardwareStatus`. `409 hw_test_running` while one runs. | should |
| F007-R29 | Problems feed: the server records actionable events in a `problems` table (code, params, first/last seen, resolved at): hardware fallback (F006-R24), FFmpeg run failed and restarted in software, a library folder unavailable or unreadable (F004), watch limit reached, provider key refused (F005). Same code + params updates the existing row instead of adding one. Rows older than 7 days after resolution are deleted. `GET /problems` (admin) lists them. | should |
| F007-R30 | ~~Server name: `PATCH /server`~~. Moved to the backlog (#117) at approval: the design never asks for a name. | won't (v1) |
| F007-R31 | ~~Health: `GET /server/health`~~. Moved to the backlog (#118) at approval. | won't (v1) |
| **Cross-cutting** | | |
| F007-R32 | Every string comes from the i18n catalogs (table below); plurals with i18next `_one`/`_other`, numbers, dates, durations and sizes with `Intl` through i18next formatters; language names with `Intl.DisplayNames`. The copy decks on the canvas are the source; the catalogs win over the canvas once merged. | must |
| F007-R33 | Every screen works at 375 px and 1440 px with no horizontal page scroll, in the dark theme (default) and the light theme, and with German-length strings (+35 %). | must |
| F007-R34 | Every server error code the screens can receive has a translated message; unknown codes fall back to `errors.generic`. `validation_failed` details map to `fieldErrors.<code>` (existing: `required`, `too_short`, `too_long`, `invalid`, `too_common`; added: `out_of_range`, `too_many`, `not_found`), with field-specific overrides where the design wrote one. | must |

## Design

### Data model
Migration `000N_problems.sql` (next free number):

```sql
CREATE TABLE problems (
  id           TEXT PRIMARY KEY,           -- UUIDv7
  code         TEXT NOT NULL,              -- hw_fallback, ffmpeg_restarted, folder_unavailable, folder_unreadable, watch_limit, provider_key_refused
  params       TEXT NOT NULL,              -- JSON object, e.g. {"path":"/mnt/nas/Films"}; never tokens or keys
  dedupe_key   TEXT NOT NULL UNIQUE,       -- code + canonical params
  first_seen   TEXT NOT NULL,
  last_seen    TEXT NOT NULL,
  count        INTEGER NOT NULL DEFAULT 1,
  resolved_at  TEXT
);
CREATE INDEX problems_last_seen ON problems(last_seen);
```

No other schema change: the onboarding's position is derived (R10).

### API
Carried into `api/openapi.yaml` first, then `make generate`.

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/libraries/{id}/scan/events` | admin | **Changed**: `ScanEvent.type` gains `matched`, with `item: MatchedItem` (R27) |
| POST | `/server/capabilities/hardware-test` | admin | Re-run the hardware test, `200 HardwareStatus`, `409 hw_test_running` (R28) |
| GET | `/problems` | admin | `{problems: [{id, code, params, firstSeen, lastSeen, count, resolvedAt?}]}`, last 7 days (R29) |

Everything else F007 uses already exists: `/setup`, `/setup/admin`, `/server`, `/fs/browse`, `/libraries*`, `/metadata/*`, `/items/{id}/match`, `/media-files/{id}/identify`, `/users*`, `/pairing/claim`, `/playback*`, `/settings/playback`, `/server/capabilities`.

New error code: `hw_test_running`.

### Behavior per app
- **server**: the `matched` scan event (hook where F005 sets an item `matched`, publish on the library's event hub), the hardware re-test endpoint (reuses F006's test), the problems recorder (a small `internal/problems` package called from the places that already log those events with `slog`).
- **web**: routes `/setup/*` (wizard) and `/admin/*`; components from the canvas built on the design system tokens; a typed SSE hook for scan events; polling hooks for Playing now and the admin home that pause when the tab is hidden; the existing `Setup` page replaced by the wizard; `Pair` reused inside People.
- **apple / android**: out of scope.

### Web routes
| Route | Screen |
|-------|--------|
| `/setup` | Start (language) |
| `/setup/code` | Setup code (only when required) |
| `/setup/account` | Admin account |
| `/setup/libraries` | Libraries, folder picker as a full-screen sheet on phone, a dialog on desktop |
| `/setup/scan` | Scan and end |
| `/admin` | Home |
| `/admin/playing` | Playing now |
| `/admin/libraries`, `/admin/libraries/:id` | Libraries, one library |
| `/admin/libraries/:id/unrecognized`, `/admin/libraries/:id/review` | Not recognized, to review |
| `/admin/people`, `/admin/people/:id`, `/admin/people/pair` | People, a person, pair a TV |
| `/admin/transcoding`, `/admin/metadata`, `/admin/problems` | Transcoding, metadata, problems |

### UI / Art direction
The canvas holds every screen at 375 and 1440 px, the directions explored and the one picked, the motion spec, the locale checks, the light theme on the scan, and the components:
- **Onboarding**: folder picker A (drill-down, fix expands under the row), scan B (the wall fills), the end of scan, 15 states per width (start, account with weak password, code wrong, code locked, done elsewhere, add a library, unreadable, empty, roots, 40 sub-folders filtered, libraries added, small scan on a Pi, end, zero libraries, server unreachable).
- **Admin**: home A (status sentence, "needs you", index of sections; entered from the library's server strip), Playing now A (one card per stream, worst first, reason in one sentence), and 17 screens per width with real data (quiet evening, busy Saturday, NAS folder unavailable, 23 not recognized, three TMDB candidates, people, last admin, TV code expired, transcoding on a Pi with an unsaved change, hardware fallback, FFmpeg missing, metadata, problems, empty states).
- **Rules from the design system**: warm near-black ground, the posters bring the color, one amber element per viewport (primary action, focus, progress), the tartan only in the woven progress thread, Instrument Serif only for the three emotional lines (start, scan voice, end), Instrument Sans everywhere else, one motion (posters arriving, backdrop crossfade) that respects `prefers-reduced-motion`.
- **Copy rule**: warmth on the start, the scan and the end; functional text plain and exact everywhere else.

### i18n
Keys below come from the canvas copy decks, converted to the repo's i18next format: `{{var}}`, `{{n, number}}`, plurals as `_one`/`_other` with `count`. Strings marked "replaces existing" change a key already in `common.json`. Commands (setup code, `setfacl`, `sysctl`) are not translated and live as constants. Keys removed with the old `Setup` and `Welcome` pages: `setup.title`, `setup.subtitle`, `setup.setupCode`, `setup.setupCodeHint`, `welcome.status.loading`, `welcome.status.ok` (the last two break the design system's no-cute-copy rule). F005's reference line `metadata.state.pending` ("…grab a blanket") is replaced by `scan.metadata.pending`. Dates and times use `Intl.DateTimeFormat` (relative times with `Intl.RelativeTimeFormat`), sizes and bitrates use `Intl.NumberFormat` with units. German-length checks are on the canvas (L02, L03, AL02, AL03).

**Language and start** (Emotional moment, one warm line)

| Key | en | fr |
|-----|----|----|
| `onboarding.start.title` (serif, warm) | Tonight’s movies will come from {{serverName}}. | Ce soir, tes films viendront de {{serverName}}. |
| `onboarding.start.body` | Three steps: your language, an account that stays on this server, and the folders where your movies live. | Trois étapes : ta langue, un compte qui reste sur ce serveur, et les dossiers où vivent tes films. |
| `onboarding.start.language` | Language | Langue |
| `onboarding.start.language.help` | Also the default language for titles and synopses in your libraries. | C’est aussi la langue par défaut des titres et résumés de tes bibliothèques. |
| `onboarding.start.continue` | Continue | Continuer |
| `onboarding.start.version` | PLAID {{version}} · {{serverName}} | PLAID {{version}} · {{serverName}} |

**Setup code** (Only when the request is not from a private or loopback address)

| Key | en | fr |
|-----|----|----|
| `onboarding.code.title` | You’re connecting from outside your network. | Tu te connectes depuis l’extérieur de ton réseau. |
| `onboarding.code.body` | To prove this server is yours, enter the setup code PLAID wrote to its log when it started. | Pour prouver que ce serveur est à toi, saisis le code de configuration que PLAID a écrit dans son journal au démarrage. |
| `onboarding.code.label` | Setup code | Code de configuration |
| `onboarding.code.where` | Where to find it | Où le trouver |
| `onboarding.code.deb` | Ubuntu or Debian (.deb) | Ubuntu ou Debian (.deb) |
| `onboarding.code.other` | Anything else: the terminal where PLAID started. | Sinon : le terminal où PLAID a démarré. |
| `common.copy` | Copy | Copier |
| `common.copied` | Copied | Copié |
| `errors.setup_code_required` (replaces existing) | This code doesn’t match. Copy the latest one from the log. | Ce code ne correspond pas. Copie le plus récent depuis le journal. |
| `errors.too_many_attempts` (existing) | Too many attempts. Try again in {{minutes, number}} min. | Trop de tentatives. Réessaie dans {{minutes, number}} min. |
| `onboarding.doneElsewhere.title` | This server is already set up. | Ce serveur est déjà configuré. |
| `onboarding.doneElsewhere.body` | Someone finished setup a moment ago. Sign in with the account they created. | Quelqu’un vient de terminer la configuration. Connecte-toi avec le compte créé. |
| `onboarding.doneElsewhere.action` | Sign in | Se connecter |

**Admin account** (Functional)

| Key | en | fr |
|-----|----|----|
| `onboarding.account.title` | Create the admin account | Crée le compte administrateur |
| `onboarding.account.body` | It lives on {{serverName}} only. There’s no PLAID account online. | Il existe uniquement sur {{serverName}}. Il n’y a pas de compte PLAID en ligne. |
| `onboarding.account.displayName` | Display name | Nom affiché |
| `onboarding.account.displayName.help` | Shown to the people you add later. | Visible par les personnes que tu ajouteras. |
| `onboarding.account.username` | Username | Nom d’utilisateur |
| `onboarding.account.username.help` | You’ll sign in with it. | Tu t’en serviras pour te connecter. |
| `onboarding.account.password` | Password | Mot de passe |
| `onboarding.account.password.help` | 8 characters or more. A long phrase beats symbols. | 8 caractères minimum. Une longue phrase vaut mieux que des symboles. |
| `common.show` | Show | Afficher |
| `common.hide` | Hide | Masquer |
| `onboarding.account.submit` | Create account | Créer le compte |
| `onboarding.account.submitting` | Creating account | Création du compte |
| `errors.weak_password` (existing) | Choose a longer or less common password. | Choisis un mot de passe plus long ou moins courant. |
| `errors.username_taken` (existing) | This username is already taken. | Ce nom d’utilisateur est déjà pris. |
| `fieldErrors.required` (existing) | Required. | Obligatoire. |
| `fieldErrors.too_long_max` (`max` from the field's limit in the API schema) | {{max, number}} characters at most. | {{max, number}} caractères maximum. |
| `onboarding.account.username.invalid` | Use letters, numbers, dots, dashes or underscores. | Utilise des lettres, chiffres, points, tirets ou tirets bas. |

**Libraries** (Functional)

| Key | en | fr |
|-----|----|----|
| `onboarding.libraries.title` | Where are your movies and shows? | Où sont tes films et séries ? |
| `onboarding.libraries.body` | Pick folders on {{serverName}}’s disks, not on this device. | Choisis des dossiers sur les disques de {{serverName}}, pas sur cet appareil. |
| `library.type.movies` | Movies | Films |
| `library.type.shows` | Shows | Séries |
| `library.name` | Name | Nom |
| `library.folders` | Folders | Dossiers |
| `library.folders.count_one` | {{count, number}} folder of 16 | {{count, number}} dossier sur 16 |
| `library.folders.count_other` | {{count, number}} folders of 16 | {{count, number}} dossiers sur 16 |
| `library.folders.add` | Add folder | Ajouter un dossier |
| `library.folders.remove` | Remove {{path}} | Retirer {{path}} |
| `library.metadata.summary` | Titles and synopses in {{language}}, {{region}} | Titres et résumés en {{language}}, {{region}} |
| `library.metadata.region.help` | The region sets age ratings and release dates. | La région fixe les classifications d’âge et les dates de sortie. |
| `common.change` | Change | Modifier |
| `library.add` | Add library | Ajouter la bibliothèque |
| `onboarding.libraries.another` | Add another library | Ajouter une autre bibliothèque |
| `onboarding.libraries.next` | Follow the scan | Suivre l’analyse |
| `onboarding.libraries.later` | Add libraries later | Ajouter les bibliothèques plus tard |
| `errors.folder_overlap` | This folder is already part of {{library}}. | Ce dossier fait déjà partie de {{library}}. |
| `errors.folder_not_found` | This folder doesn’t exist anymore. | Ce dossier n’existe plus. |

**Folder picker** (Folders on the server; the API never lists files)

| Key | en | fr |
|-----|----|----|
| `picker.title` | Folders on {{serverName}} | Dossiers sur {{serverName}} |
| `picker.parent` | Parent folder | Dossier parent |
| `picker.filter` | Filter {{count, number}} folders | Filtrer {{count, number}} dossiers |
| `picker.typePath` | Type a path | Saisir un chemin |
| `picker.noAccess` | No access | Pas d’accès |
| `errors.folder_unreadable` | PLAID can’t read this folder. | PLAID ne peut pas lire ce dossier. |
| `picker.unreadable.why` | PLAID runs as the plaid user, which can’t open {{name}}. Run this on the server: | PLAID tourne sous l’utilisateur plaid, qui ne peut pas ouvrir {{name}}. Lance ceci sur le serveur : |
| `picker.unreadable.docker` | On Docker, mount the folder into the container instead, then restart it. | Sur Docker, monte plutôt le dossier dans le conteneur, puis redémarre-le. |
| `picker.checkAgain` | Check again | Vérifier à nouveau |
| `picker.fixed` | PLAID can read it now. | PLAID peut le lire maintenant. |
| `picker.stillUnreadable` | Still no access. Check that the command ran without errors. | Toujours pas d’accès. Vérifie que la commande s’est exécutée sans erreur. |
| `picker.empty` | No sub-folders. Files aren’t listed here, so this can still be the right folder. | Aucun sous-dossier. Les fichiers ne sont pas affichés ici : ce dossier peut quand même être le bon. |
| `picker.roots` | Browsing is limited to these folders (PLAID_BROWSE_ROOTS). | La navigation est limitée à ces dossiers (PLAID_BROWSE_ROOTS). |
| `picker.noMatch` | No folder matches “{{query}}”. | Aucun dossier ne correspond à « {{query}} ». |
| `picker.use` | Use this folder | Utiliser ce dossier |

**First scan** (Functional while running, one warm line at the end)

| Key | en | fr |
|-----|----|----|
| `scan.title` (label) | Scanning your libraries | Analyse de tes bibliothèques |
| `scan.voice_one` (serif, live) | {{latestTitle}} just arrived. {{count, number}} of your movies is here. | {{latestTitle}} vient d’arriver. {{count, number}} de tes films est là. |
| `scan.voice_other` (serif, live) | {{latestTitle}} just arrived. {{count, number}} of your movies are here. | {{latestTitle}} vient d’arriver. {{count, number}} de tes films sont là. |
| `scan.voice.small` (serif, live) | {{latestTitle}} just arrived. {{matched, number}} of your {{total, number}} movies are here. | {{latestTitle}} vient d’arriver. {{matched, number}} de tes {{total, number}} films sont là. |
| `scan.leave` | You can close this page. The scan keeps running on {{serverName}}. | Tu peux fermer cette page. L’analyse continue sur {{serverName}}. |
| `scan.started` | Started at {{time}} | Commencée à {{time}} |
| `scan.state.queued` | Queued | En attente |
| `scan.state.scanning` | Looking for files in {{path}} | Recherche de fichiers dans {{path}} |
| `scan.state.analyzing` | Reading files with ffprobe | Lecture des fichiers avec ffprobe |
| `scan.state.idle` | Done | Terminé |
| `scan.found_one` | {{count, number}} file found | {{count, number}} fichier trouvé |
| `scan.found_other` | {{count, number}} files found | {{count, number}} fichiers trouvés |
| `scan.new_one` | {{count, number}} new | {{count, number}} nouveau |
| `scan.new_other` | {{count, number}} new | {{count, number}} nouveaux |
| `scan.analyzed` | {{done, number}} of {{total, number}} analyzed | {{done, number}} sur {{total, number}} analysés |
| `scan.failed_one` | {{count, number}} file couldn’t be read | {{count, number}} fichier illisible |
| `scan.failed_other` | {{count, number}} files couldn’t be read | {{count, number}} fichiers illisibles |
| `scan.unrecognized_one` | {{count, number}} not recognized | {{count, number}} non reconnu |
| `scan.unrecognized_other` | {{count, number}} not recognized | {{count, number}} non reconnus |
| `scan.metadata.pending` | Fetching posters and synopses from TMDB. | Récupération des affiches et résumés sur TMDB. |
| `scan.metadata.summary` | {{matched, number}} matched · {{needsReview, number}} to review · {{unmatched, number}} not found | {{matched, number}} identifiés · {{needsReview, number}} à vérifier · {{unmatched, number}} introuvables |
| `scan.slowShare` | On a network share, a library this size can take a while. Posters appear as titles are matched. | Sur un partage réseau, une bibliothèque de cette taille peut prendre un moment. Les affiches apparaissent au fil des identifications. |
| `scan.complete_one` (serif, warm) | From {{oldestTitle}} to {{newestTitle}}, {{count, number}} movie is home. | De {{oldestTitle}} à {{newestTitle}}, {{count, number}} film est à la maison. |
| `scan.complete_other` (serif, warm) | From {{oldestTitle}} to {{newestTitle}}, {{count, number}} movies are home. | De {{oldestTitle}} à {{newestTitle}}, {{count, number}} films sont à la maison. |
| `scan.complete.shows` | And {{shows}}, {{episodes}}, on {{serverName}}. | Et {{shows}}, {{episodes}}, sur {{serverName}}. |
| `scan.complete.showCount_one` | {{count, number}} show | {{count, number}} série |
| `scan.complete.showCount_other` | {{count, number}} shows | {{count, number}} séries |
| `scan.complete.episodeCount_one` | {{count, number}} episode | {{count, number}} épisode |
| `scan.complete.episodeCount_other` | {{count, number}} episodes | {{count, number}} épisodes |
| `scan.complete.watch` | Start watching | Commencer |
| `scan.later` | {{unrecognized, number}} files weren’t recognized and {{review, number}} titles need a second look. Both wait in Settings, Libraries. | {{unrecognized, number}} fichiers n’ont pas été reconnus et {{review, number}} titres sont à vérifier. Tout t’attend dans Réglages, Bibliothèques. |
| `scan.open` | Open {{library}} | Ouvrir {{library}} |

**Server facts** (Shown during the scan, only the ones that change something)

| Key | en | fr |
|-----|----|----|
| `server.hw.active` | Video conversion uses the {{device}} ({{method}}): up to {{max, number}} at once when a device can’t play a file as it is. | La conversion vidéo utilise {{device}} ({{method}}) : jusqu’à {{max, number}} à la fois quand un appareil ne lit pas un fichier tel quel. |
| `server.hw.none` | No hardware encoder. Files your devices play as they are stream fine. Converting happens in software, one video at a time, and 4K may not keep up. | Pas d’encodeur matériel. Les fichiers lus tels quels passent sans problème. La conversion se fait en logiciel, une vidéo à la fois, et la 4K peut ne pas suivre. |
| `server.ffmpeg.missing` | FFmpeg isn’t installed. Files your devices play as they are still work; the others won’t. | FFmpeg n’est pas installé. Les fichiers lus tels quels fonctionnent, les autres non. |
| `server.ffprobe.missing` | ffprobe isn’t installed, so files are listed but not analyzed. | ffprobe n’est pas installé : les fichiers sont listés mais pas analysés. |
| `server.watch.network` | {{path}} is on a network share, so new files aren’t seen right away. PLAID checks it every {{minutes, number}} minutes. | {{path}} est sur un partage réseau : les nouveaux fichiers n’apparaissent pas tout de suite. PLAID le vérifie toutes les {{minutes, number}} minutes. |
| `server.watch.limit` | Linux’s file watch limit is reached, so {{path}} is checked every {{minutes, number}} minutes instead of live. To raise the limit: | La limite de surveillance de fichiers de Linux est atteinte : {{path}} est vérifié toutes les {{minutes, number}} minutes au lieu d’en direct. Pour l’augmenter : |

**Landing** (Where the value is)

| Key | en | fr |
|-----|----|----|
| `home.recentlyAdded` | Recently added | Ajoutés récemment |
| `home.empty` (warm, design system line) | Nothing to watch yet. Add a folder and PLAID will find your movies and shows. | Rien à regarder pour l’instant. Ajoute un dossier et PLAID trouvera tes films et séries. |
| `home.empty.action` | Add a library | Ajouter une bibliothèque |
| `home.apps` | PLAID plays in this browser. TV and phone apps aren’t out yet. | PLAID se regarde dans ce navigateur. Les apps TV et mobile ne sont pas encore sorties. |
| `home.people` | Add people in Settings, Users. | Ajoute des personnes dans Réglages, Utilisateurs. |

**Connection lost** (Any step)

| Key | en | fr |
|-----|----|----|
| `offline.title` | Can’t reach {{serverName}}. | Impossible de joindre {{serverName}}. |
| `offline.body` | Check that it’s on and on the same network. Steps you finished are saved on the server. | Vérifie qu’il est allumé et sur le même réseau. Les étapes terminées sont enregistrées sur le serveur. |
| `offline.retrying` | Trying again in {{seconds, number}} s | Nouvel essai dans {{seconds, number}} s |
| `offline.now` | Try now | Réessayer |
| `offline.back` | Connected again. | Connexion rétablie. |
| `common.details` | Details | Détails |

**Admin home** (Title = server name, then one sentence)

| Key | en | fr |
|-----|----|----|
| `admin.home.title` | {{serverName}} | {{serverName}} |
| `admin.home.status.quiet` | All good. Nothing is playing. | Tout va bien. Rien en lecture. |
| `admin.home.status.playing_one` | {{count, number}} person is watching. All streams keep up. | {{count, number}} personne regarde. Tous les flux suivent. |
| `admin.home.status.playing_other` | {{count, number}} people are watching. All streams keep up. | {{count, number}} personnes regardent. Tous les flux suivent. |
| `admin.home.status.slow_one` | {{count, number}} person is watching. One stream can’t keep up. | {{count, number}} personne regarde. Un flux n’arrive pas à suivre. |
| `admin.home.status.slow_other` | {{count, number}} people are watching. One stream can’t keep up. | {{count, number}} personnes regardent. Un flux n’arrive pas à suivre. |
| `admin.home.status.problems_one` | One thing needs you. | Une chose demande ton attention. |
| `admin.home.status.problems_other` | {{count, number}} things need you. | {{count, number}} choses demandent ton attention. |
| `admin.home.library` | {{movies, number}} movies · {{shows, number}} shows · last scan {{time}} | {{movies, number}} films · {{shows, number}} séries · dernière analyse {{time}} |
| `admin.back` | Back to the library | Retour à la bibliothèque |
| `admin.nav.playing` | Playing now | En lecture |
| `admin.nav.libraries` | Libraries | Bibliothèques |
| `admin.nav.people` | People | Personnes |
| `admin.nav.transcoding` | Transcoding | Transcodage |
| `admin.nav.metadata` | Metadata | Métadonnées |
| `admin.nav.problems` | Problems | Problèmes |
| `admin.health.disk` (not in v1, backlog #118: don't add to the catalogs) | Less than {{free}} left on the disk that holds PLAID’s data. New conversions may fail. | Il reste moins de {{free}} sur le disque des données de PLAID. Les nouvelles conversions peuvent échouer. |
| `admin.health.heat` (not in v1, backlog #118: don't add to the catalogs) | The Pi is slowing itself down because it’s hot ({{temp}}). Conversions will be slower until it cools. | Le Pi ralentit parce qu’il chauffe ({{temp}}). Les conversions seront plus lentes le temps qu’il refroidisse. |

**Playing now** (Answers “it’s buffering”)

| Key | en | fr |
|-----|----|----|
| `playing.title` | Playing now | En lecture |
| `playing.empty` (kind) | Nothing is playing. When someone presses Play, you’ll see how it reaches them here. | Rien en lecture. Quand quelqu’un lance un film, tu verras ici comment il lui parvient. |
| `playing.row` | {{user}} · {{profile}} on {{device}} | {{user}} · {{profile}} sur {{device}} |
| `playing.position` | {{position}} of {{duration}} | {{position}} sur {{duration}} |
| `playing.where.local` | At home | À la maison |
| `playing.where.remote` | From outside | De l’extérieur |
| `playing.method.direct_play` | Plays as is (Direct Play) | Lu tel quel (Direct Play) |
| `playing.method.direct_stream` | Repackaged (Direct Stream) | Réemballé (Direct Stream) |
| `playing.method.transcode` | Converted (Transcode) | Converti (Transcode) |
| `playing.hw.hardware` | On the GPU ({{accelerator}}) | Sur le GPU ({{accelerator}}) |
| `playing.hw.software` | In software | En logiciel |
| `playing.speed.ok` | Keeps up ({{speed}}×) | Suit ({{speed}}×) |
| `playing.speed.slow` | Can’t keep up ({{speed}}×): it will buffer. | N’arrive pas à suivre ({{speed}}×) : ça va saccader. |
| `playing.slow.advice.software` | {{server}} can’t convert this file in real time. Ask {{user}} to pick a lower quality, or play it on a device that reads {{codec}}. | {{server}} ne peut pas convertir ce fichier en temps réel. Demande à {{user}} de baisser la qualité, ou de le lire sur un appareil qui lit le {{codec}}. |
| `playing.quality` | {{resolution}} · {{bitrate}} Mbps | {{resolution}} · {{bitrate}} Mb/s |
| `playing.stop` | Stop | Arrêter |
| `playing.stop.confirm` | Stop {{title}} for {{user}}? They’ll see that the admin stopped it. | Arrêter {{title}} pour {{user}} ? |
| `playing.stop.confirm.action` | Stop stream | Arrêter le flux |
| `playing.stop.confirm.cancel` | Keep playing | Laisser jouer |
| `playing.stopped` (toast) | Stopped {{title}} for {{user}}. | {{title}} arrêté pour {{user}}. |

**Why it’s converted** (One plain sentence per reason, shown under the method)

| Key | en | fr |
|-----|----|----|
| `reason.container_not_supported` | This device can’t open the file format, so it’s repackaged. No quality is lost. | Cet appareil n’ouvre pas ce format de fichier : il est réemballé, sans perte de qualité. |
| `reason.video_codec_not_supported` | This device can’t play {{codec}} video, so it’s converted. | Cet appareil ne lit pas la vidéo {{codec}} : elle est convertie. |
| `reason.video_profile_not_supported` | This device can’t play this kind of {{codec}}, so it’s converted. | Cet appareil ne lit pas cette variante de {{codec}} : elle est convertie. |
| `reason.resolution_too_high` | The video is bigger than this device can show, so it’s scaled down. | La vidéo est plus grande que ce que l’appareil affiche : elle est réduite. |
| `reason.bit_depth_not_supported` | This device can’t play 10-bit video, so it’s converted to 8-bit. | Cet appareil ne lit pas la vidéo 10 bits : elle est convertie en 8 bits. |
| `reason.hdr_not_supported` | This screen doesn’t show HDR, so colors are converted to SDR. | Cet écran n’affiche pas le HDR : les couleurs sont converties en SDR. |
| `reason.audio_codec_not_supported` | This device can’t play {{codec}} sound, so only the audio is converted. | Cet appareil ne lit pas le son {{codec}} : seul l’audio est converti. |
| `reason.audio_channels_too_many` | This device can’t play {{channels}} channels, so the sound is mixed down. | Cet appareil ne lit pas {{channels}} canaux : le son est réduit. |
| `reason.audio_track_switch_unsupported` | This device can’t switch audio tracks inside a file, so the chosen track is repackaged. | Cet appareil ne change pas de piste audio dans un fichier : la piste choisie est réemballée. |
| `reason.subtitle_burn_in` | These subtitles are images, so they’re drawn into the video. | Ces sous-titres sont des images, ils sont donc incrustés dans la vidéo. |
| `reason.bitrate_too_high` | Lowered to fit the connection. | Qualité réduite pour la connexion. |
| `reason.bitrate_too_high.remote_limit` | Watching from outside: lowered to your {{limit}} Mbps limit for remote streams. | Lecture depuis l’extérieur : réduit à ta limite de {{limit}} Mb/s pour les flux à distance. |
| `reason.keyframe_index_missing` | This file has no index (often an old AVI or TS), so it’s converted to allow seeking. | Ce fichier n’a pas d’index (souvent un vieux AVI ou TS) : il est converti pour pouvoir avancer. |
| `reason.forced_by_client` | The app asked for a conversion. | L’application a demandé une conversion. |
| `reason.colors_may_be_wrong` | Dolby Vision without an HDR10 layer: colors may look off. | Dolby Vision sans couche HDR10 : les couleurs peuvent paraître fausses. |

**Libraries** (Folders reuse the onboarding FolderRow)

| Key | en | fr |
|-----|----|----|
| `libraries.title` | Libraries | Bibliothèques |
| `libraries.counts.movies` | {{movies, number}} movies · {{files, number}} files | {{movies, number}} films · {{files, number}} fichiers |
| `libraries.counts.shows` | {{shows, number}} shows · {{episodes, number}} episodes · {{files, number}} files | {{shows, number}} séries · {{episodes, number}} épisodes · {{files, number}} fichiers |
| `libraries.lastScan` | Last scan {{time}} | Dernière analyse {{time}} |
| `folder.status.ok` | Reachable | Accessible |
| `folder.status.unavailable` | Unavailable | Indisponible |
| `folder.unavailable.body` | {{path}} can’t be reached. Its {{count, number}} movies are hidden until it’s back; nothing is deleted. After 30 days missing, they leave the library. | {{path}} est injoignable. Ses {{count, number}} films sont masqués jusqu’à son retour ; rien n’est supprimé. Après 30 jours d’absence, ils quittent la bibliothèque. |
| `folder.unavailable.check` | Check that the NAS is on and the share is mounted: | Vérifie que le NAS est allumé et le partage monté : |
| `folder.status.unreadable` | No access (same fix as the onboarding) | Pas d’accès (même correction que l’onboarding) |
| `folder.watch.live` | Watched live · {{fs}} | Surveillé en direct · {{fs}} |
| `folder.watch.periodic` | Checked every {{minutes}} min · {{fs}} | Vérifié toutes les {{minutes}} min · {{fs}} |
| `folder.fs` | Local disk (ext4) · Network share (NFS) · Network share (SMB) | Disque local (ext4) · Partage réseau (NFS) · Partage réseau (SMB) |
| `libraries.scanNow` | Scan now | Analyser |
| `libraries.fullRescan` | Full rescan | Tout réanalyser |
| `libraries.fullRescan.help` | Reads every file again with ffprobe. On {{name}} that can take a while. | Relit chaque fichier avec ffprobe. Sur {{name}}, ça peut prendre un moment. |
| `libraries.addFolder` | Add folder | Ajouter un dossier |
| `libraries.removeFolder` | Remove folder | Retirer le dossier |
| `libraries.language` | Titles and synopses in {{language}}, {{region}} · Change | Titres et résumés en {{language}}, {{region}} · Modifier |
| `libraries.language.help` | Titles and synopses are fetched again in the background. Matches you fixed stay. | Les titres et résumés sont récupérés à nouveau en arrière-plan. Les correspondances corrigées restent. |
| `libraries.delete` | Delete library | Supprimer la bibliothèque |
| `libraries.delete.confirm` | Delete {{name}}? Your files stay on disk. PLAID forgets this library, its matches and its watch history. | Supprimer {{name}} ? Tes fichiers restent sur le disque. PLAID oublie cette bibliothèque, ses correspondances et l’historique de visionnage. |
| `libraries.empty` (kind) | No libraries yet. Add a folder and PLAID will find your movies and shows. | Aucune bibliothèque. Ajoute un dossier et PLAID trouvera tes films et séries. |

**To identify and to review** (23 unrecognized files, an item with three candidates)

| Key | en | fr |
|-----|----|----|
| `review.unrecognized.title` | Not recognized ({{count, number}}) | Non reconnus ({{count, number}}) |
| `review.unrecognized.body` | PLAID couldn’t read a title from these file names. Identify them, or rename the files and scan again. | PLAID n’a pas pu lire de titre dans ces noms de fichiers. Identifie-les, ou renomme les fichiers et relance l’analyse. |
| `review.identify` | Identify | Identifier |
| `review.needs.title` | To review ({{count, number}}) | À vérifier ({{count, number}}) |
| `review.reason.candidates` | Several possible matches | Plusieurs correspondances possibles |
| `review.reason.unmatched` | Not found on TMDB | Introuvable sur TMDB |
| `review.reason.duplicate` | Same title twice | Même titre en double |
| `review.parsed` | From the file name: {{parsed}} | D’après le nom de fichier : {{parsed}} |
| `review.pick` | Which one is it? | Lequel est-ce ? |
| `review.candidate` | {{title}} ({{year}}) · {{score, percent}} match | {{title}} ({{year}}) · correspond à {{score, percent}} |
| `review.choose` | This one | Celui-ci |
| `review.search` | Search TMDB | Chercher sur TMDB |
| `review.search.help` | A title, a year, or a TMDB link. | Un titre, une année ou un lien TMDB. |
| `review.none` | None of these | Aucun de ceux-là |
| `review.fixMatch` (wrong poster) | Fix match | Corriger la correspondance |
| `review.matched` (toast) | {{title}} is matched and locked. | {{title}} est identifié et verrouillé. |
| `review.empty` (kind) | Nothing to review. Every file found its movie. | Rien à vérifier. Chaque fichier a trouvé son film. |

**People** (Users, profiles, signed-in devices, TV pairing)

| Key | en | fr |
|-----|----|----|
| `people.title` | People | Personnes |
| `people.role.admin` | Admin | Admin |
| `people.role.user` | Member | Membre |
| `people.disabled` | Disabled: can’t sign in | Désactivé : ne peut pas se connecter |
| `people.mustChange` | Chooses a new password at next sign-in | Choisira un nouveau mot de passe à la prochaine connexion |
| `people.profiles_one` | {{count, number}} profile | {{count, number}} profil |
| `people.profiles_other` | {{count, number}} profiles | {{count, number}} profils |
| `people.devices` | Signed-in devices | Appareils connectés |
| `people.device.row` | {{device}} · last seen {{time}} | {{device}} · vu {{time}} |
| `people.device.signOut` | Sign out | Déconnecter |
| `people.add` | Add a person | Ajouter une personne |
| `people.tempPassword.help` | A temporary password. They choose their own at first sign-in. | Un mot de passe temporaire. Il ou elle choisira le sien à la première connexion. |
| `people.resetPassword` | Reset password | Réinitialiser le mot de passe |
| `people.disable` | Disable | Désactiver |
| `people.enable` | Enable | Réactiver |
| `errors.last_admin` (replaces existing) | You’re the last admin. Make someone else an admin first. | Tu es le dernier admin. Nomme d’abord quelqu’un d’autre admin. |
| `people.pair` | Pair a TV | Associer une TV |
| `people.pair.help` | Open PLAID on the TV and enter the code it shows. | Ouvre PLAID sur la TV et saisis le code affiché. |
| `errors.pairing_expired` (replaces existing) | This code expired. Show a new one on the TV. | Ce code a expiré. Affiche-en un nouveau sur la TV. |
| `errors.pairing_not_found` (existing) | No TV is showing this code. Check it and try again. | Aucune TV n’affiche ce code. Vérifie-le et réessaie. |
| `people.empty` (kind) | Only you so far. Add people so each gets their own profiles and watch history. | Toi seul pour l’instant. Ajoute des personnes pour que chacune ait ses profils et son historique. |
| `people.devices.empty` | Not signed in on any device. | Connecté sur aucun appareil. |

**Transcoding** (Settings in small groups, each saved on its own)

| Key | en | fr |
|-----|----|----|
| `transcoding.title` | Transcoding | Transcodage |
| `transcoding.test.ok` | {{accelerator}} on {{device}} passed the hardware test. | {{accelerator}} sur {{device}} a réussi le test matériel. |
| `transcoding.test.none` | No hardware encoder passed the test. Conversions run in software. | Aucun encodeur matériel n’a réussi le test. Les conversions se font en logiciel. |
| `transcoding.encoders` | Converts to {{list}} | Convertit en {{list}} |
| `transcoding.decoders` | Reads {{list}} | Lit {{list}} |
| `transcoding.tonemap` | HDR to SDR: on the GPU · in software | HDR vers SDR : sur le GPU · en logiciel |
| `transcoding.fallback` | The GPU failed 3 times in 10 minutes, so PLAID switched to software until the next restart. Streams keep working, more slowly. | Le GPU a échoué 3 fois en 10 minutes : PLAID est passé en logiciel jusqu’au prochain redémarrage. Les flux continuent, plus lentement. |
| `transcoding.retest` | Test the GPU again | Retester le GPU |
| `transcoding.ffmpeg.missing` | FFmpeg isn’t installed. Files your devices play as they are still work; the others won’t. | FFmpeg n’est pas installé. Les fichiers lus tels quels fonctionnent, les autres non. |
| `transcoding.hw.label` | Hardware acceleration | Accélération matérielle |
| `transcoding.hw.options` | Automatic · Intel Quick Sync (qsv) · VA-API (vaapi) · NVIDIA (nvenc) · Raspberry Pi (v4l2m2m) · Off | Automatique · Intel Quick Sync (qsv) · VA-API (vaapi) · NVIDIA (nvenc) · Raspberry Pi (v4l2m2m) · Désactivée |
| `transcoding.hw.testing` | Testing the hardware | Test du matériel en cours |
| `errors.hw_accel_unavailable` | This accelerator didn’t pass the test. PLAID keeps using {{current}}. | Cet accélérateur n’a pas réussi le test. PLAID continue avec {{current}}. |
| `transcoding.max` | Conversions at once | Conversions simultanées |
| `transcoding.max.help` | Default here: {{default}}. Streams that play as is don’t count. | Par défaut ici : {{default}}. Les flux lus tels quels ne comptent pas. |
| `transcoding.remote` | Limit for remote streams | Limite des flux à distance |
| `transcoding.remote.options` | No limit · {{n}} Mbps | Aucune limite · {{n}} Mb/s |
| `transcoding.remote.help` | Applies to people watching from outside your network. Set it below your upload speed. | S’applique aux personnes hors de ton réseau. Mets-la sous ton débit montant. |
| `transcoding.networks` | Also count as home | Compter aussi comme la maison |
| `transcoding.networks.help` | One network per line, like 10.8.0.0/24 for a VPN. No limit applies to them. | Un réseau par ligne, comme 10.8.0.0/24 pour un VPN. Aucune limite ne s’y applique. |
| `transcoding.preset` | Encoder speed | Vitesse d’encodage |
| `transcoding.preset.options` | Fast · Balanced · Best quality | Rapide · Équilibré · Meilleure qualité |
| `transcoding.preset.help` | Fast uses less CPU. Best quality looks better at the same size but needs a stronger machine. | Rapide utilise moins de CPU. Meilleure qualité est plus belle à taille égale mais demande une machine plus puissante. |
| `transcoding.cap` | Space for conversions | Espace pour les conversions |
| `transcoding.cap.help` | Old segments are deleted past this size. | Les anciens segments sont supprimés au-delà de cette taille. |
| `common.save` | Save | Enregistrer |
| `common.saved` | Saved. | Enregistré. |

**Metadata** (Keys are never shown)

| Key | en | fr |
|-----|----|----|
| `metadata.title` | Metadata | Métadonnées |
| `metadata.source.builtin` | Built-in key | Clé intégrée |
| `metadata.source.env` | From the environment ({{var}}) | Depuis l’environnement ({{var}}) |
| `metadata.source.custom` | Your key | Ta clé |
| `metadata.source.none` | No key | Aucune clé |
| `metadata.state.ok` | Working | Fonctionne |
| `metadata.state.unreachable` | Can’t reach {{provider}} right now. PLAID tries again on its own. | Impossible de joindre {{provider}} pour l’instant. PLAID réessaie tout seul. |
| `metadata.state.invalid_key` | {{provider}} refused this key. Check it, or go back to the built-in one. | {{provider}} a refusé cette clé. Vérifie-la, ou reviens à la clé intégrée. |
| `metadata.state.no_api_key` | No key: artwork from {{provider}} isn’t fetched. | Aucune clé : les images de {{provider}} ne sont pas récupérées. |
| `metadata.useOwn` | Use my own key | Utiliser ma clé |
| `metadata.key.help` | Stored on the server and never shown again. | Enregistrée sur le serveur et plus jamais affichée. |
| `metadata.progress` | {{matched, number}} matched · {{needsReview, number}} to review · {{unmatched, number}} not found | {{matched, number}} identifiés · {{needsReview, number}} à vérifier · {{unmatched, number}} introuvables |

**Problems** (Needs a logs API · each entry: what happened, what to do)

| Key | en | fr |
|-----|----|----|
| `problems.title` | Problems | Problèmes |
| `problems.ffmpeg` | Converting {{title}} failed on the GPU at {{time}}. PLAID restarted it in software. | La conversion de {{title}} a échoué sur le GPU à {{time}}. PLAID l’a relancée en logiciel. |
| `problems.folderGone` | {{path}} disappeared at {{time}}. Its movies are hidden until it’s back. | {{path}} a disparu à {{time}}. Ses films sont masqués jusqu’à son retour. |
| `problems.watchLimit` | Linux’s file watch limit is reached, so {{path}} is checked every {{minutes}} minutes instead of live. | La limite de surveillance de fichiers de Linux est atteinte : {{path}} est vérifié toutes les {{minutes}} minutes au lieu d’en direct. |
| `problems.whatToDo` | What to do | Que faire |
| `problems.resolved` | Fixed {{time}} | Réglé {{time}} |
| `problems.empty` (kind) | No problems in the last 7 days. | Aucun problème ces 7 derniers jours. |
**Added by the spec** (error codes the screens can receive that the decks didn't cover)

| Key | en | fr |
|-----|----|----|
| `fieldErrors.out_of_range` | Out of range. | Hors limites. |
| `fieldErrors.too_many` | Too many. | Trop nombreux. |
| `fieldErrors.not_found` | Not found. | Introuvable. |
| `errors.hw_test_running` | A hardware test is already running. | Un test matériel est déjà en cours. |
| `errors.library_type_immutable` | A library's type can't change. Create a new library instead. | Le type d’une bibliothèque ne change pas. Crée plutôt une nouvelle bibliothèque. |
| `transcoding.networks.invalid` | One network per line, like 10.8.0.0/24. | Un réseau par ligne, comme 10.8.0.0/24. |

## Non-functional requirements
- **Time to posters**: on a local network, a first-time admin with one 40-movie folder reaches the first poster on the scan screen in under 3 minutes from opening the URL (measured with 3 people, R-acceptance below).
- **Modest hardware**: the web app never makes the server do more than F003–F006 already do: scan events are pushed, Playing now polls every 5 s and the admin home every 10 s only while the tab is visible. The poster wall caps the DOM at 48 posters and requests the `w342` poster size (F005 images).
- **Bundle**: the admin area is a lazy-loaded route chunk; the wizard doesn't load it.
- **Accessibility**: WCAG 2.2 AA; real buttons, links and labeled inputs; visible focus ring; the scan voice line in an `aria-live="polite"` region updated at most every 10 s (the visual line every 3 s); motion off with `prefers-reduced-motion`; every status says its meaning in words, never color alone; touch targets ≥ 44 px.
- **Security**: admin screens rely on the server's checks (F003-R13), never on hiding; provider keys are write-only (F005); the fix commands quote paths so a folder name can't inject shell; `problems.params` never hold tokens, keys or full stream URLs.
- **i18n**: no hardcoded string (`make check-i18n`); German-length layouts hold.

## ⚠️ Watch-outs
Settled at approval (2026-10-28).

- [x] **Live posters need a server change (R27)** — ✅ Kept: the `matched` event on the scan stream.
- [x] **Problems feed (R26, R29)** — ✅ Kept in F007.
- [x] **Server name (R30)** — ⏸ Backlog #117. Everything shows `GET /server`'s name (`PLAID_SERVER_NAME`, then the hostname).
- [x] **Health (R31)** — ⏸ Backlog #118. The `admin.health.*` keys stay out of the catalogs.
- [x] **The design's setup-code command was wrong** — ✅ R5 uses `grep first_run_setup_code`, the key the server logs.
- [x] **Stopping a stream** — ✅ Copy adapted: the confirmation no longer promises the person will see why. An end reason in F006 is backlog #119.
- [x] **"About an hour"** — ✅ Copy adapted to "can take a while"; no estimate in v1.
- [x] **`fieldErrors.too_long_max`** — ✅ The web takes the limit from the generated schema per field; the server's details stay `{field, code}`.
- [x] **F005's artwork picker** — ⏸ Moves to F008's detail page. Attributions are on the Metadata screen (R25).
- [x] **Design system additions** — ✅ Built in `web/` on the design system tokens, then back-ported to F002 (backlog #120).
- [x] **`sdlc/vision.md` §5 out of date** — ⏸ Backlog #121, its own change. The revised design system governs F007.
- [x] **Canvas access** — ✅ Each implementation PR attaches PNG exports of the screens it builds; `plan.md` evidence never depends on the private link.
- [x] **Remote setup over plain HTTP** — ⏸ Backlog #122 (warn on the setup-code screen when the page is `http:` from a public address). ADR 0003 leaves TLS to the reverse proxy.
- [x] **Several libraries scanning at once** — ✅ Accepted: one SSE stream per library in the wizard, polling in the admin.

## Acceptance criteria
**Onboarding**
- [ ] Given a fresh server reached from a private address, when I open `/`, then I land on the start screen in my browser's language, with no setup-code step.
- [ ] Given the start screen, when I switch to Français, then every string on screen changes without a reload, and `POST /setup/admin` is later sent with `locale: "fr"`.
- [ ] Given `GET /setup` returns `requiresSetupCode: true`, when I submit a wrong code, then the field says "This code doesn't match…"; after `too_many_attempts` with `Retry-After: 300`, the submit is disabled and says "Try again in 5 min".
- [ ] Given the setup-code screen, then the .deb command shown is `sudo journalctl -u plaid | grep first_run_setup_code` and Copy puts exactly that on the clipboard.
- [ ] Given another browser completes setup first, when I submit my account, then I see "This server is already set up." with Sign in.
- [ ] Given the account form, when I type a 2-character username, then the field error shows before submit; when the server returns `username_taken`, then it shows under Username.
- [ ] Given the folder picker on `/srv/media` where `movies` has `readable: false`, then the row shows "No access", expands with the command `sudo setfacl -R -m u:plaid:rX '/srv/media/movies'`, and "Check again" re-browses and reports the new state.
- [ ] Given a folder named `Bob's "films"`, then the fix command quotes it so the shell reads one argument.
- [ ] Given `PLAID_BROWSE_ROOTS=/srv,/mnt`, when the picker opens, then it lists `/srv` and `/mnt` with the roots note, and has no parent button at a root.
- [ ] Given a folder with 40 sub-folders, then a filter field appears and typing narrows the list; no match shows "No folder matches …".
- [ ] Given `POST /libraries` returns `409 folder_overlap`, then the chip says which library already holds it.
- [ ] Given I close the tab during the libraries step and reopen `/`, then I'm back on the libraries step, signed in.
- [ ] Given a scan running, when `progress` events arrive, then the counters line shows exactly the event's numbers, formatted for the locale.
- [ ] Given a `matched` event with a poster, then that poster appears first in the wall and its backdrop crossfades behind the screen; with `prefers-reduced-motion`, it appears without animation.
- [ ] Given 1,240 matched movies, then the DOM never holds more than 48 poster tiles.
- [ ] Given `GET /server/capabilities` reports no hardware encoder, then the scan shows the software-only line; with `nvenc` active, the GPU line with the `maxTranscodes` value.
- [ ] Given a library folder on NFS, then the "network share… checked every 15 minutes" line shows with that path.
- [ ] Given every onboarding library is idle with nothing pending, then the end screen names the oldest and newest movie by year, and "Start watching" goes to `/`.
- [ ] Given 23 unrecognized files and 18 to review, then the end screen's quiet line says so and links to Libraries.
- [ ] Given "Add libraries later", then the end screen shows the zero-libraries variant.
- [ ] Given the server stops answering on any step, then the banner appears, retries on its own, and says "Connected again." when `GET /health` answers.

**Admin**
- [ ] Given a member account, when it opens `/admin`, then it gets the forbidden screen and no admin data is requested.
- [ ] Given nothing playing and no problem, then the admin home says "All good. Nothing is playing." and shows no "needs you" list.
- [ ] Given four streams including a software transcode at 0.8×, then Playing now lists it first with "Can't keep up (0.8×): it will buffer", its reasons as sentences, and the advice line; the home says "One stream can't keep up."
- [ ] Given a remote stream with reason `bitrate_too_high` capped by `remoteMaxBitrate`, then its reason reads "Watching from outside: lowered to your 8 Mbps limit for remote streams."
- [ ] Given I press Stop and confirm, then `DELETE /playback/{id}` is sent, the card leaves, and a toast confirms.
- [ ] Given a folder with status `unavailable`, then Libraries marks it Unavailable with its hidden-movies count and what to check, and the home lists it under "needs you".
- [ ] Given an item to review with three candidates, when I choose one, then `POST /items/{id}/match` is sent with its TMDB id and the item leaves the list.
- [ ] Given an unrecognized file, when I search "Metropolis 1927" and pick a result, then `POST /media-files/{id}/identify` is sent and the file leaves the list.
- [ ] Given I try to disable the last admin, then the server's `last_admin` is shown as "You're the last admin. Make someone else an admin first."
- [ ] Given a person with two signed-in devices, when I sign one out, then `DELETE /users/{id}/sessions/{sessionId}` is sent and the row leaves.
- [ ] Given a pairing code that expired, then "This code expired. Show a new one on the TV." shows.
- [ ] Given I change "Conversions at once" and save that group, then only that setting is sent in `PATCH /settings/playback` and the group says "Saved."; other groups keep their unsaved state.
- [ ] Given "Also count as home" with `10.8.0.0/33`, then the field error shows and nothing is sent.
- [ ] Given a hardware fallback, when I press "Test the GPU again", then `POST /server/capabilities/hardware-test` is sent and the screen shows the new result.
- [ ] Given `PATCH /metadata/settings` with my TMDB key, then the key field empties, the source reads "Your key", and the key is never displayed again.
- [ ] Given a hardware fallback 2 days ago, then Problems lists it with what to do; given nothing in 7 days, then "No problems in the last 7 days."

**Server**
- [ ] Given a scan where F005 matches a movie, then the library's event stream sends `matched` with the item's id, title, year and poster image id.
- [ ] Given the same folder goes unavailable twice in a day, then `GET /problems` has one row with `count: 2`.
- [ ] Given a hardware test already running, when `POST /server/capabilities/hardware-test` is called again, then `409 hw_test_running`.

**Cross-cutting**
- [ ] Given every screen at 375 px in French and with German-length strings, then nothing overflows and there's no horizontal page scroll.
- [ ] Given the light theme, then the scan and the admin home stay legible (contrast AA).
- [ ] Given `make check-i18n`, then no key is missing in `fr` and no screen has a hardcoded string.

## Revisions
- 2026-10-29: PR B renames the keys that were both a leaf and a parent (`.label`, `scan.complete.title`), splits `home.empty` into `.title` and `.body`, and adds the keys listed in the plan Log. The setup code sits on the account screen, as designed; there is no separate route.
- 2026-10-29: i18n keys added by PR A for the ported design system components (see plan Log); `picker.empty` split into `.title` and `.body`; `picker.filter` interpolates `total`.
- 2026-10-28: approved. R30 and R31 moved to the backlog; three strings adapted so the copy doesn't promise what the API can't do (`playing.stop.confirm`, `scan.slowShare`, `libraries.fullRescan.help`).
