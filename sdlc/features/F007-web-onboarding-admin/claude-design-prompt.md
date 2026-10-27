# Claude Design prompts — F007 onboarding and administration

> Three prompts, sent in order in **one** Claude Design project with the PLAID design system selected:
> 1. **Prompt A**: first-run onboarding (send first, iterate until it holds).
> 2. **Prompt B**: the admin area (same project, so it inherits what A settled).
> 3. **Review prompt**: paste after every round of A or B. It's the critique loop.
>
> Attach the material listed under "What I'm giving you" before sending A. Without it, the result drifts back to generic.
> Replaces `claude-design-prompt-onboarding.md`, which described the pre-revision design system (night blue, burgundy, tartan texture, "blanket unfolding"), all of which the revised system now forbids.

## Why the prompts look like this
Research on avoiding "AI slop" (sources at the bottom) and our own F002 and website passes agree on the same causes and fixes:
- **Generic output is the statistical average.** Without direction, a model picks the most common choice in its training data ("distributional convergence"). For our screens that average is a known shape: a wizard with numbered circles and an illustration, an admin with a sidebar of icons and a row of KPI cards with sparklines. So the prompts **name those defaults and forbid them**, and give concrete alternatives.
- **Adjectives don't steer, material does.** "Cozy", "clean", "premium" produce the average. Real data (our API's actual fields, real titles from `metadata.csv`, real terminal output, real error codes) forces specific layouts. So the prompts describe **jobs and data**, not a layout skeleton. Our first website brief listed "eyebrow, H1, lead, two buttons, three cards" and got exactly that.
- **The first draft becomes the template.** Asking for **three divergent directions** on the hardest screens before committing avoids locking into the first idea.
- **A model is a lenient judge of its own work.** A separate, skeptical **review pass** with explicit criteria (design quality and originality weighted above craft and functionality, AI tells penalized) improves results over several rounds. That's the review prompt.
- **Wording leaks into style.** Words like "museum quality" or "cozy blanket" push a look. The prompts keep the design system's own vocabulary and avoid new metaphors.

---

## Prompt A — first-run onboarding

Design the first-run onboarding of PLAID's web app. Use the PLAID design system exactly as it is: its tokens, type, components, signature move and kill list. Do not invent colors, fonts or components. If the flow needs something the system lacks, build it from existing tokens and list it as a proposed addition.

### The moment
Someone just installed the PLAID server: a `.deb` on an Ubuntu box, a Docker container on a NAS, or a Raspberry Pi 4 under the TV. The install script ended by printing `http://<their-server>:32500`. They open it, very often on their phone, on the couch, on the home Wi-Fi. The server has no account and no library. In about three minutes they should go from "blank server" to "my movies are here". They are usually the household's tech person (self-hoster, home lab), comfortable with a terminal, allergic to marketing, and they will later invite less technical people. Treat them as competent: no hand-holding, no exclamation marks, no mascots.

### What the flow must get done (jobs, not screens)
Decide yourself how many screens this takes and how they're laid out. Fewer is better. These are the jobs, with what the server really does:

1. **Pick the language.** English or French, each named in its own language, preselected from the browser. Everything after it is in that language. It becomes the admin's locale and the default metadata language of their libraries.
2. **Prove it's their server, only when needed.** The server accepts setup without any code from a private or loopback address (home network, `localhost`). Only when the request comes from outside (a public IP, a reverse proxy without trusted headers) does it require a **one-time setup code** that it printed in its log at startup. So most people never see this step. When they do: a code field that pastes well on a phone, and where to find the code: `sudo journalctl -u plaid | grep "setup code"` on Ubuntu, `docker logs plaid` on Docker, otherwise the terminal where PLAID started. Errors the server returns: `setup_code_required`, `too_many_attempts` (with a wait time), `setup_complete` (someone else just finished setup: explain and send them to sign in).
3. **Create the admin account.** Display name, username, password. Server errors: `weak_password`, `username_taken`, `validation_failed` per field. The account lives on their server only; say it once, plainly.
4. **Add libraries.** A library is a type (Movies or Shows; Music and Photos don't exist yet, don't show them), a name, a metadata language and region, and 1 to 16 folders **on the server's disks**, not the phone's. The folder picker browses server folders through an API that returns, per folder, only its `name`, `path` and whether PLAID can `readable` it. It never lists files, so there are no file counts to show. Paths look like `/srv/media/movies`, `/mnt/nas/Films`, `/media/pi/USB-4TB/Séries`, `/volume1/video`; some admins restrict browsing to a few roots. An unreadable folder is the number one problem on Linux (the `plaid` service user can't read it): it must be obvious why, with the fix to copy (`sudo setfacl -R -m u:plaid:rX /srv/media/movies`) and a way to check again. A library can also be skipped for now.
5. **Watch the first scan.** The server streams scan events. Per library: a state (`queued`, `scanning`, `analyzing`, `idle`), the folder being scanned, and counters `found`, `new`, `analyzed`, `failed`, `unrecognized`; then metadata counters `pending`, `matched`, `needsReview`, `unmatched`. Titles and posters arrive as items get matched. A small library finishes in under a minute; 8,000 episodes on a NAS over NFS can take an hour. Nothing here is alarming: unrecognized files and items to review are handled later in the admin area. They can leave at any time and the scan continues.
6. **Know what the server can do.** Facts the server already has: whether FFmpeg and ffprobe were found (with versions), which hardware acceleration passed its test (`vaapi`, `qsv`, `nvenc`, `v4l2m2m`, or none, then software), and whether some folders can't be watched live (network file systems, Linux watch limit reached, so they're rescanned periodically). Show only what changes something for the admin, at the moment it matters. A Raspberry Pi with no hardware encoder and a box with an NVIDIA GPU should not get the same reassurance text.
7. **Land in the library.** End where the value is: their posters. Pointers to apps for TV and phone and to inviting people are honest about not existing yet, or are left out.

Every step can be left and resumed. The server can become unreachable at any moment (it restarts, the Wi-Fi drops).

### Data to design with
- Titles, posters and backdrops: `metadata.csv`. Use it as the library that fills up during the scan, including its stress cases (very long titles, non-Latin original titles, missing artwork, an unidentified file).
- A small library: 37 movies on a Pi over USB. A large one: 1,240 movies and 312 shows (8,064 episodes) on a NAS over NFS, with 23 unrecognized files.
- Folder trees with real names, accents and spaces, at least one unreadable folder, one empty folder, a root with 40 sub-folders.

### Forbidden in this flow (on top of the design system's kill list)
Numbered circles joined by a line as the progress indicator · a hero illustration, mascot, rocket, party popper or confetti · emoji anywhere · "Welcome to PLAID! 👋"-style headings · a centered card floating over a gradient glow · three feature cards explaining what PLAID does (they already installed it) · a password strength meter in traffic-light colors · a fake progress bar that doesn't come from the server's counters · posters that are gradients with a title on top · a "Skip" link on every screen · cards inside cards · status dots with no label · generic copy that would work for Jellyfin or Home Assistant.

### Method
1. **Copy first.** Write every string before any layout: English source, French next to it, using "tu". Functional text (labels, errors, the permission fix) is plain and exact. Warmth is allowed in two places only: the first screen and the end of the scan. Cut until each sentence passes "so what?" and "could only PLAID say this?".
2. **Three directions on the two hardest moments**: the folder picker with an unreadable folder, on a phone; and the first scan of the large library, on desktop. Make them genuinely different (different structure, not different colors). Show them side by side, say which one you pick and why, then build the whole flow in that direction.
3. **Grayscale first**: type and space only. The hierarchy has to work before amber, posters or the tartan come in.
4. **Amber budget**: at most one amber element per viewport (the primary action, focus or progress). The tartan appears only where the design system allows it.
5. **One motion**, the one that carries the flow (likely posters arriving during the scan), slow, respecting `prefers-reduced-motion`.

### Deliverables
- The full flow at 375 px (phone) and 1440 px (desktop), including: setup code required and wrong, server unreachable, setup already completed elsewhere, unreadable folder, empty folder, browsing restricted to roots, zero libraries, small and large scan, scan with unrecognized files, Pi without hardware acceleration vs NVIDIA box.
- One screen in French at its longest and one with German-length strings (about 35 % longer than English) to prove nothing breaks.
- The light theme on the scan screen.
- The directions you rejected, and why.
- Every new or extended component, with its states, listed as proposed additions to the design system.

### What I'm giving you
- `metadata.csv`: 49 real titles with poster and backdrop URLs (TMDB CDN, mockups only).
- `screens/`: the current, unstyled F003 setup, sign-in and profile pages, so you see what exists.
- `terminal/`: the real end of the install script and `systemctl status plaid` output, where the URL and setup code appear.
- The F003, F004, F005 and F006 specs, as the only source of what the server does.

---

## Prompt B — administration

Same project, same design system, same rules as the onboarding, including its kill list and amber budget. Design the admin area of PLAID's web app. Before starting, list what the onboarding settled (components, patterns, tone) that you will reuse.

### Who and when
The same admin, weeks later. They come to the admin area for a reason, rarely to browse: a friend says "it's buffering", a new hard drive got plugged in, a movie shows the wrong poster, the Pi feels hot. Most visits are short and from a phone. The admin area answers "is something wrong, and what do I do about it", then gets out of the way. It sits next to the library, not in a separate product: going back to the posters is always one step away.

### The jobs, with what the server really provides
1. **Who's watching right now.** Per active stream: user, profile, device, title, method (`direct_play`, `direct_stream`, `transcode`) with machine-readable reasons (`video_codec_not_supported`, `bitrate_too_high`, `subtitle_burn_in`, …), quality, hardware or software, transcode speed (e.g. 0.8× means it can't keep up), position, local or remote. The admin can stop a stream. This is where "it's buffering" gets answered: translate reasons into one plain sentence each.
2. **Libraries.** Name, type, folders (each `ok`, `unavailable` or `unreadable`, watched live or not, file system type), counts (movies, shows, episodes, files, unrecognized), last scan, scan state. Actions: scan now, full rescan, add or remove folders, change metadata language, delete. Files whose name couldn't be understood, and items that need review (several candidates, not found, duplicates), with the candidates from TMDB to pick from.
3. **Users and their devices.** Users with role (admin or user), disabled or not, must change password; their profiles; their signed-in sessions (device, last seen) which can be revoked. Create a user, reset a password, disable. Pair a TV with a code. The last admin can't be removed (`last_admin`).
4. **Transcoding.** What the hardware test found (accelerator, device, working encoders and decoders, tone mapping), a hardware fallback warning when the GPU failed 3 times, FFmpeg found or not. Settings: hardware acceleration (`auto` or a specific one), max simultaneous transcodes, remote bitrate limit, extra local networks (CIDR), encoder preset (`fast`, `balanced`, `quality`), transcode folder size cap.
5. **Metadata providers.** Which TMDB and fanart.tv keys are in use (custom, from the environment, built in, none), never showing the keys; per-library metadata progress.
6. **Logs.** No logs API exists yet. Design the screen for errors and warnings a non-expert can act on (an FFmpeg failure, a folder that disappeared, the watch limit), and flag it as "needs an API".
7. **Server health** (CPU, disk, GPU) is an open question for v1. Propose the smallest version worth having, or argue it isn't worth it.

### Forbidden in the admin (on top of the onboarding's list)
A row of four KPI cards with big numbers and sparklines · charts that answer no question · a left sidebar with a dozen icons on desktop and a hamburger drawer on mobile, by default, without trying something better · tables that become unreadable horizontal scrollers on a phone · every setting in one long form with a sticky "Save" bar · green / amber / red dots without words · "Dashboard" as a page title · raw enum values shown to the user (`v4l2m2m`, `direct_stream`) without a human label next to them.

### Method
Same as the onboarding: copy first, grayscale first, then **three directions for the admin's home** (what you see when nothing is wrong, and when something is), and for the active-streams view on a phone. Pick one, explain why, build the rest.

### Deliverables
- Every job above at 375 px and 1440 px, with real data: a quiet evening (nothing playing, everything fine), a busy Saturday (4 streams, one transcode at 0.8× on a Pi, one remote stream capped by the bitrate limit), a NAS folder gone `unavailable`, 23 unrecognized files, an item with three TMDB candidates, a hardware fallback, FFmpeg missing.
- Empty states for every list.
- One screen in French and one with German-length strings.
- Rejected directions, and new components as proposed design system additions.

---

## Review prompt (after each round)

Switch roles: you are now a skeptical design reviewer who did not make these screens and is paid to find what's wrong. Don't fix anything yet.

1. **Audit** every screen against the design system's kill list and this prompt's forbidden list. Table: pattern → screen → what to do.
2. **Swap test**: put the Jellyfin, Plex or Home Assistant logo in place of PLAID's. Every screen that still works as is is generic: say what would make it only PLAID's.
3. **Squint and grayscale tests** at 375 and 1440: does the one thing that matters on each screen still stand out?
4. **Data test**: is every number, state and label something the server actually provides (see the specs)? List anything invented.
5. **Amber count** per viewport, and every use of the tartan.
6. **Copy**: every string that is cute in a functional place, vague, or longer than needed.
7. Score each screen 1–5 on design quality, originality, craft, function. Weigh the first two double. Anything under 4 in originality gets a concrete change, not "make it more distinctive".

Then propose the changes and wait for my go.

---

## Notes (not part of the prompts)
- **Material to collect before sending A:**
  - Screens: `make dev`, open http://localhost:5173 on a blank data dir, screenshot setup, sign-in and profile picker at 2x.
  - Terminal: `make test-deb` (or a VM install), keep the end of the install script and `systemctl status plaid`.
  - `metadata.csv`: `sdlc/features/F002-design-system/design-inputs/`.
  - Specs: export F003, F004, F005, F006 `spec.md`.
- **Watch-outs for the F007 spec**, surfaced while writing this:
  - The setup code is only asked from non-private addresses (F003-R3), so the code screen is an edge case, not a step.
  - Language first: the intent orders admin → libraries → language → scan. Putting the language first means the rest is already translated; confirm in the spec.
  - A **server name** is returned by `GET /server`, but nothing lets the admin set it yet. Decide whether onboarding names the server (it will matter for F009).
  - The folder picker can't show file counts: `/fs/browse` returns folder names only. Adding counts is an API change.
  - Logs and server health have no API; the spec decides whether v1 adds them.
  - Live posters during the scan need the items as they're matched; scan events only carry counters. The spec decides how the web gets them (polling items, or a new event).
- **The checks that matter come from outside Claude Design**: try the result on a real phone on the couch, and time 3 people from "URL opened" to "posters visible".

### Sources
- Anthropic, [Improving frontend design through Skills](https://claude.com/blog/improving-frontend-design-through-skills): distributional convergence, name the defaults and give alternatives, work at the right altitude.
- Anthropic Engineering, [Harness design for long-running apps](https://www.anthropic.com/engineering/harness-design-long-running-apps): separate skeptical evaluator, design quality and originality weighted above craft and function, wording side effects.
- The Fountain Institute, [7 signs a UI has been vibe coded](https://www.thefountaininstitute.com/blog/signs-vibe-coded-ui): glow on dark mode, emoji icons, cards in cards, undefined status dots, color as a shared resource.
- MindStudio, [Claude Design: avoid AI slop with a design system](https://www.mindstudio.ai/blog/claude-design-avoid-ai-slop-design-system): design system first, real screens next to tokens, name edge states explicitly.
- [Hallmark](https://instagit.com/Nutlope/hallmark/hallmark-anti-patterns-detected/): catalogue of AI UI tells (three-card grids, centered full-viewport heroes, gradient text).
