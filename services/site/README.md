# getplaid.tv

The public website of PLAID: a static, bilingual site (English at `/`, French at `/fr/`) with no runtime framework, no analytics and no cookies.

Pages: home, `features/`, `party/`, `install/`, `remote-access/` (how getplaid.direct works), `protocol/` (the getplaid.direct protocol), `roadmap/`, `open-source/`, `faq/`, `privacy/`, plus a 404 page per locale. `/download/` (the first site's install page) redirects to `/install/`.

## Design

The site reproduces the Claude Design mockup of getplaid.tv as real, responsive HTML and CSS:

- One dark theme: `#0b0b0c` background, cream `#f4f0e9` text, Bricolage Grotesque (600 to 700) for display headings with tight negative tracking, Instrument Sans for text. Both fonts are self-hosted in `public/fonts` (SIL OFL).
- A liquid-glass pill header that turns solid after the first screen and hides while scrolling down; cream pills for the main action, glass pills for the rest.
- Each page opens on a blurred field of film stills, a display title and a lead, then a rounded still with its credit. The home page draws the web app in a tilted browser window, the Apple TV app and the Plaid Party devices.
- Imagery: stills and posters of public-domain films and Blender open movies (CC BY), credited in the footer and under each still. No commercial posters.
- Motion is progressive: `src/js/boot.js` marks the page as scripted before first paint, and only then do scroll reveals hide content until it is seen. Hooks are explicit classes (`.rv`, `.rvi`, `.hl`, `[data-words]`, `[data-type]`, `[data-devices]`, `[data-copy]`). `prefers-reduced-motion` turns it all off.
- The Content Security Policy forbids inline styles and scripts, so templates never use `style=""`; every variation is a class, and scripts only touch styles through the CSSOM.

## Layout

| Path | What |
|------|------|
| `locales/en.json`, `locales/fr.json` | Every user-facing string. English is the source; French uses "tu". |
| `src/pages/*.mjs` | One template per page, plain template literals. |
| `src/layout.mjs` | `<head>` (Open Graph, hreflang, canonical), header, footer, and the shared components (`btn`, `cmd`, `banner`, `ambient`, `pageHero`, `card`, `ctaRow`, `posterParade`). |
| `src/icons.mjs` | The line icons, stroked with the cream-to-blue gradient. |
| `src/i18n.mjs` | Locale loading and the inline markup allowed in strings: `` `code` ``, `**strong**`, `[label](url)`, `{placeholder}`. |
| `src/css/` | `fonts.css` and `site.css`, bundled and fingerprinted at build time. |
| `src/js/boot.js`, `src/js/site.js` | The only scripts: the `js` class before first paint; header, reveals, typed command, copy buttons, FAQ accordion, page transitions. The site works without them. |
| `public/` | Copied as is: fonts, `logo.svg` (the tartan play button), favicon, film stills and posters (`img/`), Open Graph image. |
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
