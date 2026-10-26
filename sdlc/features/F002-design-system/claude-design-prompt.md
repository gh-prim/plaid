# Claude Design prompt — PLAID design system

> Paste as-is into Claude Design. Send it in one go, or section by section (1 → 2 → 3) to iterate.
> Once approved: export to Claude Code, then store the tokens in `design/tokens.json`.

---

## The prompt

Design the complete design system for **PLAID**, a self-hosted movie and TV streaming app (a Plex alternative). It runs on web, iPhone, iPad, Apple TV and Android TV.

### Brand concept
PLAID is the plaid blanket on the couch. The world: movie night, the cozy nest, "Netflix & chill" made warm and personal. The app should make you want to curl up under a blanket. It is never cold or "techy".
- **Keywords:** cozy, warm, soft, woolly, dimmed lighting, a home movie theater, friendly.
- **Avoid:** neon / gaming looks, corporate blue, Netflix red, copying Plex (yellow / black).
- **Visual signature:** a very subtle **tartan** pattern, used as texture (headers, empty states, loading, profile avatars). It never gets in the way of reading movie posters.
- **Copy tone:** casual, warm and playful, like a friend on the couch (the French locale uses informal "tu"). Examples: "Who's under the plaid?" ("Qui est sous le plaid ?"), "Pick up where you dozed off" ("Reprendre là où tu t'es endormi"), "Nothing to watch? Add a folder and come snuggle back in." ("Rien à regarder ? Ajoute un dossier et reviens te caler."), "Plaid Party".

### 1. Tokens

**Colors.** Dark mode is the default (movie theater). Light mode is called "Sunday morning".
- Base palette to explore: deep midnight blue (backgrounds), tartan burgundy (brand color, primary actions), amber / lamplight (focus, accents, progress), woolly cream (text), fir green (secondary accent, positive statuses).
- Structure as **semantic** tokens on top of primitive tokens: `bg/base`, `bg/elevated`, `bg/overlay`, `surface/card`, `text/primary|secondary|muted|inverse`, `brand/primary`, `accent`, `focus/ring`, `border/subtle`, `status/success|warning|error|info`, `party/*` (Plaid Party participant colors, 8 distinct hues).
- WCAG AA contrast minimum, in both modes.
- 2 or 3 variants of the tartan pattern (density, opacity), in palette tones.

**Typography.** A display typeface with character and warmth (headings, title logos) and a highly legible text typeface. Both must be free to use (Google Fonts / OFL), since the app is self-hosted.
- Full type scale: display, h1 → h4, body L/M/S, caption, label, with sizes, weights, line heights and letter-spacing.
- **Two scales**: "standard" (mobile / web) and "10-foot" (TV, read from 3 m away).

**Everything else.**
- Spacing (base 4), radii (generous, soft: nothing sharp), soft warm shadows (no pure black), elevations.
- Motion: durations and easing curves (gentle, a little "squishy"; never an aggressive bounce). TV focus animation.
- Grid and breakpoints: mobile, tablet, desktop, TV (1920×1080 with safe area).
- Iconography: rounded icon style, medium stroke, consistent with the brand's softness.

### 2. Components
For **every component**, show all states: default, hover (web), **focus (TV and keyboard: a highly visible state, with an amber glow and slight scale-up)**, pressed, disabled, loading (skeleton), error. Also show the mobile / web and TV variants when they differ.

**Basics**
- Buttons: primary, secondary, ghost, icon-only, "Play" (large, iconic), "Plaid Party".
- Inputs: text, password, search, PIN entry, TV pairing code entry (6 characters).
- Selectors: toggle, checkbox, radio, segmented control, dropdown menu, slider.
- Chips / tags (genres, quality 4K / HDR / Dolby, language), badges ("New", unwatched episode count).
- Toasts / notifications, modal dialogs, bottom sheet (mobile), tooltip.
- Progress bars (partial playback on a poster, library scan), themed spinner (a plaid folding itself?).
- Profile avatars (unique tartan pattern per profile + initial), avatar stack.
- Illustrated empty states and error screens, in the brand's tone.

**Navigation**
- Tab bar (mobile), sidebar (web / iPad), top nav (TV, on focus).
- Page header with search.
- Full-screen profile picker ("Who's under the plaid?" ("Qui est sous le plaid ?")).

**Media** (the most important)
- **Poster card** (2:3): with progress, watched / unwatched badge, TV focus state that scales up and reveals title + year.
- **Landscape card** (16:9) for "Continue watching" and episodes: thumbnail, title, time remaining.
- **Horizontal row** (carousel) with a title and "See all".
- Home **hero / banner**: full-screen backdrop, movie logo, synopsis, Play and More info buttons.
- Movie and series **detail page**: immersive backdrop, metadata, cast (round person card), season picker, episode list, trailer.
- Library grid with filter bar and sorting.
- Search result.

**Video player**
- Controls overlay: play / pause, timeline with preview thumbnails, time, chapters.
- Audio / subtitles / quality panels.
- "Skip intro" and "Next episode" buttons with countdown.
- TV (remote) version and touch version.

**Plaid Party** (the signature feature, give it extra care)
- "Start a Plaid Party" card and invitation screen (link + code + QR).
- Waiting room: participants arriving, "ready" / "loading" status.
- In-playback overlay: participant avatar stack (indicator of who is out of sync or buffering), floating emoji reactions, collapsible chat, "Ask for a pause" button.
- Toasts: "Camille hit pause" ("Camille a mis en pause") / "Léo joined the party" ("Léo a rejoint la soirée").

**Admin (web)**
- Table (users, devices), library card with scan status, active stream card (who's watching what, direct play or transcoding), server folder picker, onboarding stepper.

### 3. Reference screens
Assemble the components into these screens, in dark mode, and add at least one screen in light mode:
1. Profile selection — TV
2. Home — mobile, web and TV
3. Movie page — web and TV
4. Series page with episodes — iPad
5. Player with controls — TV
6. Plaid Party in progress, with reactions and chat — web
7. Admin onboarding, "Add your folders" step — web
8. "Empty library" empty state — mobile

### Technical constraints for export
- Tokens must be exportable as JSON (W3C Design Tokens format), then converted to CSS variables (web), Swift (SwiftUI) and Kotlin (Compose).
- Web components will be implemented in React, Apple ones in SwiftUI, Android ones in Jetpack Compose / Compose for TV. Stay close to native conventions (navigation, gestures, tvOS focus) while keeping the PLAID identity.
- Use generic or fictional movie posters, no real brands.
- The app is multilingual (i18n: English as the source language, then French, more languages later). Components must handle text 30 to 40% longer (German) without breaking: ellipsis truncation, line wrapping, buttons that widen. Show at least one component with a long label.
