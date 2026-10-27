# Claude Design prompt — Server onboarding (first run)

> Paste into Claude Design **after** the PLAID design system (F002) exists, and tell it to use that design system.
> Covers only the first-run wizard of F007. The admin area gets its own prompt later.
> Watch-outs to settle in the F007 spec are listed at the bottom; they are not part of the prompt.

---

## The prompt

Design the **first-run onboarding** of PLAID, a self-hosted media server (an alternative to Plex). Use the PLAID design system: dark mode by default, deep night blue backgrounds, tartan burgundy for primary actions, lamp-light amber for focus and progress, woolly cream text, a very subtle tartan texture. Copy tone: warm, casual, playful — "plaid & chill". Never technical jargon unless it's unavoidable, and then explained in one friendly line.

### Context
Someone just installed the PLAID server on their machine (an Ubuntu box, a NAS, a Raspberry Pi, or Docker). They open `http://<their-server>:32500` in a browser — often **from their phone, on the couch**. The server is empty: no account, no library. This wizard takes them from "blank server" to "my movies are showing up" in about 3 minutes. It's the first impression of PLAID: it must feel like settling under a blanket, not configuring a router.

Design for **mobile first (375 px)**, then **desktop (1440 px)**. Not TV.

### The flow
A progress indicator shows where they are (5 steps after the welcome). Every step has Back, and the wizard can be left and resumed where they stopped.

**0. Welcome**
- Big, cozy hero moment: PLAID wordmark, tartan texture, a short line: "Get comfy, we'll handle the rest." (« Installe-toi, on s'occupe du reste. »)
- Language picker right here, pre-selected from the browser (English, Français), each shown in its own language. Switching it updates the whole screen instantly.
- One primary button: "Let's get started" (« C'est parti »).

**1. Prove it's your server**
- Explain simply: to make sure nobody else on the network grabs this server first, enter the setup code shown where PLAID was installed.
- 6-character code input (large, easy to type on a phone, paste-friendly).
- A collapsible "Where do I find it?" with three short tabs: Ubuntu (`sudo journalctl -u plaid | grep "setup code"`), Docker (`docker logs plaid`), other (the terminal where PLAID started). Code blocks with a copy button.
- States: wrong code (gentle shake + message), too many attempts (wait a bit).

**2. Create your account (admin)**
- Fields: display name, username, password (with a friendly strength meter), confirm password.
- Also: name this server, with a playful placeholder like "The living-room plaid" (« Le plaid du salon »). It's what friends will see later.
- Reassurance line: this account stays on your server, nothing is sent to us.

**3. Add your libraries**
- Choose a library type with big friendly cards: Movies, Shows. (Music and Photos visible but marked "coming later".)
- Library name (prefilled: "Movies", "Shows") and metadata language.
- **Folder picker that browses the server's disks** (not the phone's files): breadcrumb path, folder list with icons, a hint of how many video files each folder holds, "Select this folder". A library can have several folders (chips, removable).
- **Unreadable folder state** (very important, it's the #1 problem on Linux): the folder shows a lock icon and a friendly explanation: "PLAID can't open this folder yet. It's a permissions thing, easy to fix." with an expandable fix (a command to copy, e.g. `sudo setfacl -R -m u:plaid:rx /path/to/movies`) and a "Check again" button.
- Empty folder / no video found state.
- The list of libraries added so far, with "Add another library". They can also continue with zero libraries ("I'll do it later"), but the next step then shows a softer empty version.

**4. A few preferences** (short, everything has a good default)
- Hardware acceleration: show what was detected ("Intel Quick Sync detected: smooth streaming to all your devices" / "No GPU detected: PLAID will use the CPU, perfect for most homes"). Read-only info card, with a link to settings.
- Watching from outside your home: an info card explaining it's off by default and linking to a guide. No configuration here.
- Anonymous usage stats: none. Just a reassuring line that PLAID doesn't phone home.

**5. First scan, live**
- The emotional peak: their library fills up in real time.
- Overall progress (files analyzed / total), per-library progress, live counters (movies found, shows, episodes).
- A grid where **posters fade in one by one** as titles get identified, on the cozy background.
- A small "Couldn't identify X files — we'll let you fix them later" note, never alarming.
- They can leave at any moment: "Start watching, we'll keep going in the background."
- Long-scan state (big library, slow NAS): a calm message and an estimated time.

**6. All set**
- "Your plaid is ready." (« Ton plaid est prêt. ») with a celebratory but soft animation (a blanket unfolding?).
- Primary button: "Go to my library".
- Secondary cards: "Install PLAID on your TV and phone" (coming soon state), "Invite friends" (coming later state).

### States to show for every step
Loading, validation errors inline (not just on submit), server unreachable (connection lost banner with retry), and how the screen looks with a **long translation** (German-length text, ~35 % longer than English) so nothing breaks.

### Deliverables
1. The full flow on **mobile** (every step + key states).
2. The full flow on **desktop**: centered card layout on a full-bleed cozy background; the scan step can use the full width for the poster grid.
3. One step in **light mode** ("Sunday morning") to check the theme holds.
4. The new components this flow needs, with their states, so they join the design system: stepper / progress indicator, setup-code input, password strength meter, library type card, server folder browser (row states: normal, hover, selected, unreadable, empty), folder chip, detected-hardware card, scan progress block with counters, live poster grid item (placeholder → identified).

All UI copy must be written in English (source locale) with the French translation next to it; the app is fully internationalized, so never bake text into images.

---

## Watch-outs for the F007 spec (not part of the prompt)
- **Setup code (step 1)** is not in the F007 intent yet. Without it, anyone on the same network who opens a fresh server first can claim the admin account (a known issue with self-hosted servers). It needs server support in F003 (code generated at first start, printed in the logs, single use, rate-limited). Decide in the F003/F007 specs; the design screen is included so the choice can be made with a mockup in hand.
- **Language first**: the intent orders the wizard as admin → libraries → language → scan. The prompt moves the language picker to the welcome screen so the rest of the wizard is already in the right language. The F007 spec should confirm this order.
- **Server name** (step 2) is new; it will be visible to friends (F009). Confirm in the spec.
- **Hardware detection and remote-access cards (step 4)** depend on F006 and ADR 0003; they're informational only in v1.
