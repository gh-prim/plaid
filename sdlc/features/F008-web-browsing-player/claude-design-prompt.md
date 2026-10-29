# Claude Design prompts — F008 browsing and player

> Result: canvas _to add once made_ (pages Browse and Player).
>
> Prompts sent in order in **one** Claude Design project with the PLAID design system selected, ideally the same project as F007's canvas [PLAID first-run onboarding](https://claude.ai/artifact/RnKuX3Lq3HrjwzQ2nH2hTz), so the browsing screens inherit what F007 settled (header strip, poster tile, missing-artwork state, copy rules):
> 1. **Prompt A**: browsing (profile picker, home, library grid, detail pages, search).
> 2. **Prompt B**: the player (same project, so it inherits what A settled).
> 3. **Review prompt**: paste after every round of A or B. It's F007's critique loop, unchanged.
> 4. **Correction prompt**: F007's, only when a round comes back cold.
>
> Attach the material listed under "What I'm giving you" before sending A. Why the prompts are written this way (name the defaults and forbid them, design with real data, three directions on the hardest screens, a separate skeptical review): see [F007's prompt](../F007-web-onboarding-admin/claude-design-prompt.md#why-the-prompts-look-like-this).

---

## Prompt A — browsing

Design the browsing screens of PLAID's web app. Use the PLAID design system exactly as it is, plus the components F007 added in this project (header server strip, poster wall tile, missing-artwork state, toasts, the woven progress thread). Do not invent colors, fonts or components. If a screen needs something the system lacks, build it from existing tokens and list it as a proposed addition.

### The moment
A household's movie server is set up. Tonight someone opens PLAID in a browser, on the laptop on the couch, on a desktop, or on a phone in bed, to pick something and press Play. The library is theirs: 300 to 5,000 movies and a few hundred shows, matched by TMDB, with real artwork, ratings and genres. Some posters are missing, some titles are in Japanese or Korean, one show has 36 seasons. A child's profile sees less. This is the one screen people open every evening: it must feel like a cinema whose program is your own shelf, and get you to Play in three clicks.

### What the screens must get done (jobs, not layouts)
Decide yourself how the screens are built. These are the jobs, with what the server really returns (see the F008 spec, which is the only source of truth):

1. **Pick who's watching.** Profiles have a name, one of 8 avatar colors (the `party-*` tokens) and maybe a 4-digit PIN. The title is the one warm line of the app: "Who's under the plaid?" ("Qui est sous le plaid ?"). It shows when a tab opens on an account with two or more profiles.
2. **Home.** `GET /home` returns a hero and rows. The hero is either what you were watching (with "Resume" and the time left) or a recently added title with a backdrop and a logo. Rows, in this order: Continue watching (landscape, with progress and time left, episodes named `S1 E4 · Title`), Recently added per library, then up to 4 genres. A row with nothing isn't shown. Empty homes exist: a member on a server with no library, and a first scan still running.
3. **Browse a library.** Every movie (or show) as posters, 1,240 of them, smooth to scroll, with the count. Sort: Title, Recently added, Release date, Rating, Recently watched, Random. Filters: watch state, genres (multi), decades, resolution (4K, 1080p, 720p and lower), HDR. Active filters are visible and removable, and "nothing matches" exists. Shows carry an unwatched-episodes count. Optional letter index when sorted by title.
4. **A movie.** The backdrop and ambient colors the server computes for it (`dominant`, `vibrant`, `muted`, `dark`, `onDominant`), its logo when there is one, original title, year, runtime, certification, TMDB rating, genres, technical facts (4K, HDR10, Dolby Vision, audio languages), tagline, synopsis, director and writers, cast with portraits and characters, collection, trailer (YouTube, loaded only on click). Actions: Play, or Resume from 1:02:13 with Start over; mark watched; choose a version when there are several (Director's Cut, 4K and 1080p). Admins also get an Edit menu: change artwork, fix match, refresh.
5. **A show.** The same header, plus status (Ended, Returning), seasons as tabs (specials last), episodes with stills, number, title, air date, runtime, two lines of synopsis, watched or in progress. The main action is the right episode: "Play S1 E1", "Resume S2 E3", "Play S3 E1".
6. **A person.** Portrait and what they're in on this server, by role.
7. **Search while typing.** From the first letter, grouped Movies, Shows, Episodes, People, keyboard-driven (↑ ↓ Enter Esc), as a dropdown under the header on desktop and a full screen on phone. "Nothing called “zzz” here."
8. **Get around.** A header with the libraries, the search, the profile menu (switch profile, appearance, pair a TV, sign out) and, for admins only, F007's server strip ("mediabox · 2 watching"). On phones, a bottom tab bar.

### Data to design with
- `metadata.csv`: 49 real titles with posters and backdrops. Use its extremes on the same screens: near-black (*Dune: Part Two*), blown-out pink (*Barbie*), black and white (*Casablanca*), busy ensemble (*Everything Everywhere All at Once*), very long titles (*Dr. Strangelove…*), non-Latin original titles, the rows with no artwork, the unidentified file.
- A large library: 1,240 movies, 312 shows. A small one: 37 movies on a Pi. A child's profile (max age 10) whose home is visibly shorter.
- A show with 36 seasons, one with specials, an episode in progress at 12 min of 48.

### Forbidden in these screens (on top of the design system's kill list)
A Netflix clone: red, a giant auto-playing hero video, a "Top 10" row with huge numbers · posters with rounded pill corners or drop shadows that glow · gradient-tinted card backgrounds · hover cards that pop and cover their neighbors · carousels with dots · a sidebar of icons on desktop · star ratings drawn as five stars · genre chips in rainbow colors · "Because you watched…" rows (the server doesn't compute them) · "Trending", "Popular on PLAID", or anything implying data from other users or the internet · skeletons that shimmer in color · emoji · cute copy on buttons, menus and errors.

### Method
1. **Copy first**, English and French with "tu": the F008 spec's i18n table is the starting point; improve it, list every change.
2. **Three directions on the two hardest screens**: the home on desktop over a bright backdrop (*Barbie*) and a dark one (*Dune: Part Two*), and the show page on a phone. Genuinely different structures. Pick one, say why, build everything in it.
3. **Grayscale then color in the same round**, with the real posters and ambient colors; never deliver the grayscale pass as the result.
4. **Amber budget**: one amber element per viewport (Play, focus, progress). The tartan only in the woven progress thread on cards.
5. **One motion per surface**: the hero backdrop crossfade, the card focus lift. Respect `prefers-reduced-motion`.

### Deliverables
- Every screen at 375 px and 1440 px, and the home and a movie page at 2560 px: profile picker (with PIN), home (in progress, nothing in progress, child profile, empty for a member, first scan running), library grid (sorted, filtered, nothing matches, loading skeletons), movie page (bright and dark backdrop, no logo, no artwork at all, several versions, files missing, admin Edit menu, artwork picker), show page (36 seasons, specials, in progress), person page, search (typing, results, no result), unavailable title.
- One screen in French at its longest, one with German-length strings.
- The light theme on the home and a movie page.
- Keyboard focus visible on cards, rows and the grid.
- The directions you rejected, and why. Every new or extended component as a proposed addition.

### What I'm giving you
- `metadata.csv`: 49 real titles with poster and backdrop URLs (TMDB CDN, mockups only).
- `screens/`: the F007 onboarding end and admin home as built, so the browsing screens continue them.
- The F005, F006 and F008 specs, as the only source of what the server does.

---

## Prompt B — the player

Design PLAID's web player, in the direction Prompt A picked. Always dark, whatever the theme. It plays a movie or an episode full screen in the browser, on a laptop, a desktop and a phone (portrait and landscape).

### What it must get done
1. **Play, pause, seek**: elapsed and remaining time, the buffered range, a hover time tooltip; no preview thumbnails and no chapters (the server has none yet). −10 s / +10 s. Volume, mute, picture-in-picture, full screen. Controls appear on mouse move or tap and fade after 3 s.
2. **Choose subtitles, audio and quality** from what the server returns: subtitles by language with forced and SDH variants ("French (forced)", "English SDH"), off; audio tracks by language and channels ("French · 5.1"); qualities "Original", 1080p, 720p… Some changes are instant (text subtitles), some reload the stream at the same position (audio, quality, image subtitles): the design shows the brief reload without a spinner over the face.
3. **Explain itself on demand**: "Playback info" says in words how the video reaches you (Direct play, Direct stream, Converting), why ("This browser can't play HEVC video, so it's converted."), quality and bitrate, GPU or processor.
4. **Resume or start over** when you open something already started.
5. **Next episode**: in the last 30 s, a card with the next episode's still, number and title, "Play now" and "Cancel"; at the end, a 10 s countdown. After three in a row without touching anything: "Still watching?". After the last episode of a show: one warm line, nothing plays.
6. **Fail clearly**: the server busy converting other videos (with Try again and Lower quality), the file's drive unplugged, FFmpeg missing, a video the browser can't play (with "Copy link for VLC"), playback stopped (expired or stopped by the admin).
7. **Keyboard shortcuts** (Space, ← →, J L, ↑ ↓, M, F, C, Shift+N, 0–9, ?) and their help sheet.

### Frames to design over
A white frame (snow, a bright interior), a black frame (space, a night scene), a busy frame with burned-in text: the controls and subtitles must read over all three. Use stills from `metadata.csv` backdrops.

### Forbidden
YouTube's or Netflix's control bar copied · a big red progress bar · circular buttons floating mid-frame · a glowing play button over the picture while it plays · menus as full-screen takeovers on desktop · emoji · a countdown ring that pulses.

### Deliverables
- Controls at 1440 px over the three frames, at 375 px portrait and landscape.
- Each menu open (subtitles, audio, quality, info), each error, resume, next-episode card and countdown, still watching, end of show, the shortcuts sheet.
- Subtitle rendering (size, background, position above the controls when they show).
- The motion spec for the controls fading and the next-episode card.

---

## Review prompt and correction prompt
Use F007's, unchanged: [review](../F007-web-onboarding-admin/claude-design-prompt.md#review-prompt-after-each-round) after each round, [correction](../F007-web-onboarding-admin/claude-design-prompt.md#correction-prompt--when-a-round-comes-back-cold) when a round comes back cold. In the review's data test, the source is the F008 spec.

---

## Notes (not part of the prompts)
- **Material to collect before sending A**: `metadata.csv` from `sdlc/features/F002-design-system/design-inputs/`; screenshots of F007's onboarding end and admin home at 2x (`make dev`); the F005, F006 and F008 specs exported.
- When the canvas exists, add its link at the top of this file and in the spec's UI section. The plan's web steps take it as input, and the PR attaches PNG exports of the screens it builds.
- **The checks that matter come from outside Claude Design**: time 3 people from "home open" to "playing", on the couch, on a laptop and a phone.
