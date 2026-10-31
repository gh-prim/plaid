# getplaid.tv

The public website of PLAID: a static, bilingual site (English at `/`, French at `/fr/`) with no runtime framework, no analytics and no cookies.

Pages: home, `features/`, `party/`, `install/`, `remote-access/` (how getplaid.direct works), `roadmap/`, `open-source/`, `faq/`, `privacy/`, plus a 404 page per locale. `/download/` (the first site's install page) redirects to `/install/`.

## Design

The site is built from the web app's design system, not next to it:

- `src/css/tokens.css`, `fonts.css` and `components.css` are copied from `web/src/design/` (components: only the blocks the site uses). Update them by copying again; never edit values here.
- Buttons are the app's onboarding pills (cream for the one primary action, glass for the rest), code blocks are its `CommandBlock`, status labels its `.pl-tag`, the Plaid Party room its timeline, avatars and toast.
- **One idea: one movie night, timestamped.** Section markers are times of a single night (18:30 install, 20:47 the link is sent, 21:02 everyone has joined, 1:12:48 Sam asks for a pause). The thread down the left is a timeline whose played part is woven like the player's and follows the reader. It is the only decoration and the only motion.
- Amber is progress and focus only: one amber element per viewport. The tartan appears only in the logo and in the played part of a timeline.
- The kill list (patterns that must not come back) is at the top of `src/css/site.css`.

Imagery is real: `public/shots/` holds cropped screenshots of the running app (web player, first run, remote access settings) and of the Apple TV app (`sdlc/features/F011-tvos-app/screenshots/settings.jpg`). The film in the player is a public-domain title (*Nosferatu*, 1922) whose picture is a test card. Screens that don't exist yet (Plaid Party) are drawn with the design system's components and labelled "Concept". No commercial posters.

## Layout

| Path | What |
|------|------|
| `locales/en.json`, `locales/fr.json` | Every user-facing string. English is the source; French uses "tu". |
| `src/pages/*.mjs` | One template per page, plain template literals. |
| `src/layout.mjs` | `<head>` (Open Graph, hreflang, canonical), header, footer, and the shared components (`cmd`, `button`, `tag`, `shot`, `night`/`beat`, `partyRoom`). |
| `src/i18n.mjs` | Locale loading and the inline markup allowed in strings: `` `code` ``, `**strong**`, `[label](url)`, `{placeholder}`. |
| `src/css/` | Design system (vendored) + `site.css`, bundled and fingerprinted at build time. |
| `src/js/theme.js`, `src/js/site.js` | The only scripts: theme before first paint; theme toggle, copy buttons, install tabs, the night's thread. The site works without them. |
| `public/` | Copied as is: fonts (Instrument Sans and Serif, SIL OFL), favicon (the app icon), screenshots, Open Graph image. |
| `scripts/build.mjs` | Renders every page in every locale to `dist/`, plus `robots.txt` and `sitemap.xml`. |
| `scripts/check-i18n.mjs` | Fails if the locales differ in keys or placeholders, or a template uses an unknown key. |

No dependencies: Node 24 and pnpm 10 are enough.

## Commands

Run from `services/site`:

```bash
pnpm install       # nothing to download, keeps the lockfile honest
pnpm dev           # build, serve on http://localhost:4321 and rebuild on change
pnpm build         # write dist/
pnpm check-i18n    # compare locales/en.json and locales/fr.json
pnpm preview       # serve the current build without watching
```

To add text: add the key to `locales/en.json` and `locales/fr.json`, use it in a template with `t('key')` (element content) or `attr('key')` (attributes), then run `pnpm check-i18n`.

To add a page: create `src/pages/<name>.mjs` (see `privacy.mjs`), register it in `PAGES` in `scripts/build.mjs`, and add its id to `NAV` in `src/layout.mjs` (and its label under `nav`) if it belongs in the menu.

## Docker

```bash
docker build -t plaid-site .
docker run --rm -p 8080:80 plaid-site   # http://localhost:8080
```

The image builds the site with Node, then serves `dist/` with nginx on port 80: gzip, `no-cache` on HTML, one-year immutable cache on fingerprinted `/assets/`, a strict Content Security Policy (no inline scripts or styles), a health check at `/healthz`, and French 404s under `/fr/`.

## Deploy on Coolify

1. **New resource → Public/Private repository**, pick this repository and the `main` branch.
2. **Build pack:** Dockerfile. **Base directory:** `services/site`. **Dockerfile location:** `/Dockerfile`. **Ports exposes:** `80`.
3. **Domains:** `https://getplaid.tv,https://www.getplaid.tv`. Coolify gets the Let's Encrypt certificates.
4. **Redirect:** in the resource's settings, set the direction to **redirect to non-www**, so `www.getplaid.tv` answers 301 to `getplaid.tv`. (`nginx.conf` also redirects `www.getplaid.tv` itself, in case the proxy forwards it.)
5. **Health check:** path `/healthz`, port `80`.
6. Point the DNS of `getplaid.tv` and `www.getplaid.tv` (A/AAAA) to the Coolify server, then **Deploy**. Enable automatic deployment on push if you want every merge to `main` to publish the site.
