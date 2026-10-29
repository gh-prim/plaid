# PLAID website: Claude Design correction prompt (depth pass)

> Paste into Claude Design **inside the existing website project**, so it revises the pages instead of starting over.
> Before sending, attach the material listed under "What I'm giving you". Without it, the result will drift back to generic.

---

## The prompt

Revise the PLAID website. Do not add pages, sections or components. Make it deeper. Right now it reads as AI-generated: a template with PLAID's words poured in. I want a site an art director at Mubi, the Criterion Collection or A24 would sign: editorial, specific, restrained, where the warmth comes from craft and true details, not from decoration.

One correction to my first brief: its copy is the content, not the layout. I listed it as "eyebrow, H1, lead, two buttons, three cards" and you followed that literally, which is half of the problem. Keep the substance, drop that skeleton.

### 1. Audit before you change anything
Go through every page and list every generic pattern it uses, with the page and section. Check at least for:
- an eyebrow line above every heading
- a centered hero: heading, lead, two buttons, device mockup
- radial glows or gradient blobs that don't come from a real light source
- devices floating at an angle
- grids of three or four cards with an icon in a rounded square
- every section in the same rhythm (centered H2, subtitle, grid)
- status pills on everything
- a "final CTA" band
- a checkmark comparison table
- fade-up-on-scroll for every block, and cards that lift on hover
- an italic serif second line used just to look "soft"
- invented posters that are gradients with a title on top
- words like seamless, effortless, powerful, beautiful, all-in-one

Then run the **swap test**: put the Jellyfin or Plex logo in place of PLAID's. Every section that still works is generic. Rewrite it so it can only be about PLAID, or cut it.

Deliver this audit as a table (pattern → where → what you'll do) before the redesign.

### 2. One idea for the whole site
Propose three concepts, pick one, and explain why. Apply it with discipline: it is the only decorative idea on the site. Candidates:
- **The lights go down.** The page opens in the light theme, a lit living room. As you scroll it dims to the movie-night theme until the product's screen is the only light source left.
- **One night, timestamped.** The site follows a single real movie night. Section markers are times: 20:47 the link is sent, 21:02 everyone has joined, 1:12:48 Sam asks for a pause. A single amber thread, woven from the Gordon tartan at thread scale, runs down the page as the night's timeline.
- **The programme.** An editorial layout like a cinema programme or a Criterion booklet: typographic, numbered, with credits. The install page reads like end credits.

### 3. Work from the raw material, not from my digested brief
- **Real product screens**: the attached screenshots of the running PLAID web app. Use them as they are, cropped and framed, not redrawn. Screens that don't exist yet (player, Plaid Party) come from the PLAID design system and carry a visible "Concept" label. Nothing fake should pass as shipped.
- **The real terminal**: the attached output of the install script and of `systemctl status plaid`. The Install page shows real output, not a stylized code block with invented lines.
- **Real facts only**: every claim on the site must trace to the attached README, `docs/install.md` or a feature spec (port 32500, one Go binary, SQLite, FFmpeg, amd64/arm64, Raspberry Pi 4/5, MIT, sync gap under 500 ms, two developers). No invented numbers, testimonials or user counts.
- **The real tartan**: the registered Gordon tartan, woven as a 2/2 twill, shown at thread scale. It lives in the logo and in the played part of timelines, and nowhere else.
- **Posters**: use `metadata.csv` to stress-test layouts in mockups only. A published page cannot show real commercial posters. For final art, use films that are actually in the public domain (for example *Metropolis*, *Nosferatu*, *The General*, *Night of the Living Dead*), and flag each one for me to verify.
- **The audience's own words**: the attached quotes from r/selfhosted and similar forums about Plex accounts, paywalls and watch-together hacks. The copy answers what these people actually complain about, in their vocabulary, not in marketing vocabulary.
- **Anti-references**: the attached screenshots of the plex.tv, jellyfin.org and emby.media home pages. List what all three do, and avoid it wherever PLAID has something different to say.
- **References, none of them software**: Criterion booklets, Mubi's editorial pages, cinema programmes, Saul Bass title sequences, a living room at the moment the lamp goes off. Name what you took from each.

### 4. Method
- **Copy before layout.** Rewrite first and cut at least 40%. Each sentence must pass two questions: "so what?" and "could only PLAID say this?". Warmth only in emotional moments (the Party, empty states, the 404). Functional text (install steps, FAQ answers, buttons) stays plain and precise. Apply the same cut to the French copy, and keep "tu".
- **Grayscale wireframes first.** Type and space only, no images, no color. The hierarchy has to work before anything else is added.
- **An editorial grid.** Twelve columns, left-aligned, asymmetric. Instrument Serif at true display sizes for a few headlines only, Instrument Sans for everything else, one type scale. No two consecutive sections share a layout, and the whole site gets at most one card grid.
- **Amber budget.** At most one amber element per viewport: the primary action, focus or progress.
- **One motion.** The motion that carries the chosen concept, and nothing else. It respects `prefers-reduced-motion`.
- **Kill list.** Write it at the top of the project and follow it: every pattern from the audit that you removed.
- **Tests on every page.** Squint test, grayscale test and swap test, at 1440, 768 and 375, in English and in French.

### 5. Proof, not adjectives
Deliver, next to the revised pages:
- The audit table and the kill list.
- The concept you chose, with the two you rejected and why.
- Before/after views of the Home hero, the Plaid Party page and the Install page, saying what was removed and why.
- Swap test results, section by section.
- A claims sheet: every factual sentence on the site with its source.
- A WCAG AA and APCA contrast table for every text/background pair, in both themes.
- A five-second test script I can run with five self-hosters: show the Home page for five seconds, then ask "What is this? Who is it for? What does it do that others don't?" Write the answers you expect, so we can compare.

Constraints that don't change: 8 pages, English and French, the PLAID design system's tokens and fonts, dark by default with a light mode, honest about early development, no hardcoded text in images.

### What I'm giving you
- `screens/`: screenshots of the running PLAID web app (login, onboarding, libraries), at 2x.
- `terminal/`: the real output of the install script and `systemctl status plaid`.
- `README.md`, `docs/install.md`, and the F006 and F010 specs, as the only source for claims.
- `metadata.csv`: 49 real titles with TMDB poster and backdrop URLs, for layout stress tests only.
- `quotes.md`: 10–15 real quotes from self-hosting forums, with links.
- `anti-references/`: home page screenshots of plex.tv, jellyfin.org and emby.media.

---

## Notes (not part of the prompt)
- Collect the material before sending:
  - **Screens:** run `make dev`, open http://localhost:5173, and screenshot each existing page at 2x.
  - **Terminal output:** run `make test-deb`, or install in a VM.
  - **Posters:** `metadata.csv` is in `sdlc/features/F002-design-system/design-inputs/`.
  - **Quotes:** pick them yourself on r/selfhosted and r/PleX, and keep the links.
- The checks that matter come from outside Claude Design:
  - Run the five-second test with real self-hosters, not the team.
  - Compare the result side by side with the Plex and Jellyfin home pages.
  - Check every claim against the claims sheet.
  - Once the site is live, measure clicks on "Install" and "GitHub".
