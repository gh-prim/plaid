# getplaid.tv

The public website of PLAID: a static, bilingual site (English at `/`, French at `/fr/`) with no runtime framework, no analytics and no cookies.

Pages: home, `download/`, `remote-access/` (how getplaid.direct works), `privacy/`, plus a 404 page per locale.

## Layout

| Path | What |
|------|------|
| `locales/en.json`, `locales/fr.json` | Every user-facing string. English is the source. |
| `src/pages/*.mjs` | One template per page, plain template literals. |
| `src/layout.mjs` | `<head>` (Open Graph, hreflang, canonical), header, footer, code blocks, status labels. |
| `src/i18n.mjs` | Locale loading and the inline markup allowed in strings: `` `code` ``, `**strong**`, `[label](url)`, `{placeholder}`. |
| `src/styles.css`, `src/site.js`, `src/theme.js` | Styles, and the only scripts (theme toggle, copy buttons). Fingerprinted at build time. |
| `public/` | Copied as is: fonts (Instrument Sans and Serif, SIL OFL), favicon (the app icon), Open Graph image. |
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

To add a page: create `src/pages/<name>.mjs` (see `privacy.mjs`), register it in `PAGES` in `scripts/build.mjs`, and add its nav label under `nav` and its id to `NAV` in `src/layout.mjs` if it belongs in the menu.

## Docker

```bash
docker build -t plaid-site .
docker run --rm -p 8080:80 plaid-site   # http://localhost:8080
```

The image builds the site with Node, then serves `dist/` with nginx on port 80: gzip, `no-cache` on HTML, one-year immutable cache on fingerprinted `/assets/`, a strict Content Security Policy, a health check at `/healthz`, and French 404s under `/fr/`.

## Deploy on Coolify

1. **New resource → Public/Private repository**, pick this repository and the `main` branch.
2. **Build pack:** Dockerfile. **Base directory:** `services/site`. **Dockerfile location:** `/Dockerfile`. **Ports exposes:** `80`.
3. **Domains:** `https://getplaid.tv,https://www.getplaid.tv`. Coolify gets the Let's Encrypt certificates.
4. **Redirect:** in the resource's settings, set the direction to **redirect to non-www**, so `www.getplaid.tv` answers 301 to `getplaid.tv`. (`nginx.conf` also redirects `www.getplaid.tv` itself, in case the proxy forwards it.)
5. **Health check:** path `/healthz`, port `80`.
6. Point the DNS of `getplaid.tv` and `www.getplaid.tv` (A/AAAA) to the Coolify server, then **Deploy**. Enable automatic deployment on push if you want every merge to `main` to publish the site.
