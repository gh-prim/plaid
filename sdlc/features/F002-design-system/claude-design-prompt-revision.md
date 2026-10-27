# Claude Design prompt — Design system revision (depth pass)

> Paste into Claude Design **inside the existing PLAID design system**, so it revises it instead of starting over.
> Before sending, attach the material listed under "What I'm giving you" (it's what makes this work).

---

## The prompt

Revise the PLAID design system. Don't add components. Make it deeper: the current version reads as AI-generated — warm palette, soft serif plus rounded sans, pills everywhere, a literal "cozy blanket" metaphor applied to everything. I want something a senior product designer at Apple TV, Mubi or the Criterion Channel would sign: restrained, confident, premium, where the warmth comes from craft and not from decoration.

### 1. Start with a point of view, not tokens
Before touching any value, write a half-page design stance in the README:
- What PLAID is, in one sentence, and three things it is **not** (e.g. not a toy, not a theme park of tartan, not another dark-grey Plex clone).
- **The content is the color.** Posters and backdrops are loud, varied and colorful; the interface recedes. State how the UI stays quiet next to a neon Pixar poster, a black-and-white noir still and a white minimal A24 poster.
- **One or two signature moves at most**, used with discipline, where the brand lives. Everything else stays neutral. Propose 3 candidates (for example: the tartan only appears as a woven texture inside the progress/timeline fill; or a single warm "lamp" light source that every focus state comes from; or the twill weave angle as the only diagonal in the system), pick one, and explain why.
- A **kill list**: what this system now forbids. At minimum: tartan as a background wallpaper, pill shapes as the default for everything, oversized radii on large surfaces, decorative serif in UI controls, cute copy in functional places (buttons, errors, settings), emoji-like illustrations, gradients that don't come from a real light source, any "blanket unfolding" style animation.

### 2. Work from the raw material, not from your own previous output
- **Real posters and backdrops**: `metadata.csv` (attached) has a `poster_url` (2:3) and a `backdrop_url` (16:9) for 46 real titles, served by the TMDB image CDN. Load them and build every surface, card and the hero against them. The set already holds the extremes: near-black (*Dune: Part Two*, *The Substance*), blown-out pink and green (*Barbie*, *Wicked*), hyper-saturated (*Inside Out 2*), black and white (*Casablanca*, *Seven Samurai*, *The Seventh Seal*), very old art (*Metropolis*), busy ensemble posters (*Everything Everywhere All at Once*). The `stress_case` column says what each row tests.
- **Real, messy metadata** (same file): very long titles (*Borat…*, *Dr. Strangelove…*), Korean, Japanese, Chinese and Telugu original titles, runtimes from 81 to 263 minutes, editions, a show with 36 seasons, a home video with no year, an unidentified file, a show with no metadata. The rows with `poster` = `missing` have no artwork on purpose: they are the missing-artwork states. Every component must hold with this data, not with "Movie Title".
- **Real tartan**: base the sett on an actual registered tartan (pick one whose palette fits, cite it and its thread count from the Scottish Register of Tartans), woven as a 2/2 twill: show how colors mix where warp and weft cross, at thread scale. Not a drawn grid.
- **Physical references over app screenshots**: cinema interiors at the moment the lights dim, Criterion packaging, wool blankets (Faribault, Pendleton, Johnstons of Elgin), warm lamp light in a dark living room. Name what you took from each. Don't reference other AI-made UI kits.

### 3. Method
- **Screens first, tokens second.** Design the five hardest screens with the real material, then derive the tokens from what they needed. The five: TV home with focus on a poster in a row, movie detail over a bright backdrop, the player with controls on a white frame, the Plaid Party overlay during a dark scene, the first-run onboarding on a phone outdoors.
- **Color in OKLCH**: rebuild the ramps with even perceptual steps. Neutrals carry 90% of the interface; check they don't look muddy next to saturated posters. Keep one action color.
- **Contrast with APCA**, not only WCAG 2: body text Lc ≥ 75, UI labels Lc ≥ 60, TV text Lc ≥ 75 at its size. Publish the table.
- **10-foot sizing by viewing angle**: a 55" TV seen from 3 m. Derive the TV type scale and focus ring thickness from that, show the math.
- **Typography**: challenge the current Fraunces + Nunito pairing (it's the default "warm" combo). Test at least two alternatives, including one where a single well-drawn family does all the work and the display face only appears on titles. Keep the license constraint (open-licensed, self-hosted). Justify the choice with specimens at real sizes on TV and phone.
- **Shape**: one radius scale tied to size (small things slightly rounded, large surfaces nearly square). Pills only where the shape means something (toggles, avatars).
- **Motion**: fewer, slower, purposeful transitions. Every animation answers "what changed and where did it go". Specify them frame by frame for focus, backdrop crossfade and player controls.
- **Copy**: warmth only in moments that are emotional (welcome, empty states, the end of a scan, a Plaid Party joining). Functional text is plain, short and precise. Rewrite the reference lines with that rule.
- **Subtraction pass**: for every component, ask "what can I remove". Delete any token or component no screen uses.
- **Squint and grayscale tests** on the five screens: the hierarchy must survive both.

### 4. Proof, not adjectives
Deliver, next to the revised system:
- The five screens, with the real material, on their real device frames: TV (1920×1080 with safe area), iPhone, desktop.
- A **poster stress sheet**: the poster card, the hero and the detail header rendered over at least 24 posters and 12 backdrops from `metadata.csv` (all the extremes included), side by side, plus the missing-artwork rows.
- The **APCA contrast table** for every text/background pair in both themes.
- The **10-foot math** for the TV scale.
- A **before / after** of three components (button, poster card, toast), with what was removed and why.
- The **kill list** and the chosen signature move, written into the README so future work follows them.

Constraints that don't change: dark ("Movie night") is the default theme, light ("Sunday morning") exists, the player is always dark, everything is localized (English source, strings up to 35% longer), focus must be visible from 3 m, fonts are open-licensed and self-hosted. Brand terms PLAID and Plaid Party stay.

### What I'm giving you
- `metadata.csv`: 49 real titles (36 movies, 13 shows) with type, title, original title, year, runtime, seasons, genres, edition, the stress case each row tests, and TMDB poster and backdrop URLs. Poster and backdrop images come from TMDB; they're only used for mockups. This product uses the TMDB API but is not endorsed or certified by TMDB.

---

## Notes (not part of the prompt)
- Attach `design-inputs/metadata.csv` when sending. To refresh the URLs: `TMDB_TOKEN=… python3 design-inputs/tmdb_enrich.py design-inputs/metadata.csv design-inputs/metadata.csv` (the key lives in 1Password, item "TMDB").
- The images stay on TMDB's CDN: never download them into the repo.
- The external checks that matter come after: TV screenshots on a real TV at 3 m, a phone outdoors, and 3–5 people timed on "find and play a movie" and "join a Plaid Party". Record them in the F002 spec as acceptance criteria.
