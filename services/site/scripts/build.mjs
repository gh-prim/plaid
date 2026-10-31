// Renders every page in every locale to dist/, copies public/, fingerprints
// CSS and JS, and writes robots.txt and sitemap.xml. No dependencies.

import { createHash } from 'node:crypto';
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { DEFAULT_LOCALE, LOCALES, ORIGIN, localePrefix } from '../src/config.mjs';
import { loadLocale, makeTranslator } from '../src/i18n.mjs';
import home from '../src/pages/home.mjs';
import download from '../src/pages/download.mjs';
import remoteAccess from '../src/pages/remote-access.mjs';
import privacy from '../src/pages/privacy.mjs';
import notFound from '../src/pages/not-found.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(root, 'dist');
const PAGES = [home, download, remoteAccess, privacy, notFound];
const OG_LOCALES = { en: 'en_US', fr: 'fr_FR' };

function fingerprint(src, name, ext) {
  const content = readFileSync(join(root, 'src', src));
  const hash = createHash('sha256').update(content).digest('hex').slice(0, 10);
  const file = `assets/${name}.${hash}.${ext}`;
  writeFileSync(join(dist, file), content);
  return `/${file}`;
}

function pathFor(pageId, locale) {
  const page = PAGES.find((p) => p.id === pageId);
  if (!page) throw new Error(`Unknown page "${pageId}"`);
  return localePrefix(locale) + page.slug;
}

export function build() {
  rmSync(dist, { recursive: true, force: true });
  mkdirSync(join(dist, 'assets'), { recursive: true });
  cpSync(join(root, 'public'), dist, { recursive: true });

  const assets = {
    css: fingerprint('styles.css', 'styles', 'css'),
    js: fingerprint('site.js', 'site', 'js'),
    theme: fingerprint('theme.js', 'theme', 'js'),
  };

  const written = [];
  for (const locale of LOCALES) {
    const tr = makeTranslator(loadLocale(join(root, 'locales', `${locale}.json`)), locale);
    for (const page of PAGES) {
      const path = pathFor(page.id, locale);
      const ctx = {
        ...tr,
        locale,
        pageId: page.id,
        noindex: Boolean(page.noindex),
        url: ORIGIN + path,
        assets,
        href: (id, l = locale) => pathFor(id, l),
        ogLocale: (l) => OG_LOCALES[l],
      };
      const html = page.render(ctx);
      const out = join(dist, path.endsWith('/') ? `${path}index.html` : path);
      mkdirSync(dirname(out), { recursive: true });
      writeFileSync(out, html);
      written.push(path);
    }
  }

  writeFileSync(join(dist, 'robots.txt'), `User-agent: *\nAllow: /\n\nSitemap: ${ORIGIN}/sitemap.xml\n`);

  const urls = PAGES.filter((p) => !p.noindex).flatMap((page) =>
    LOCALES.map((locale) => {
      const alternates = LOCALES.map(
        (l) => `    <xhtml:link rel="alternate" hreflang="${l}" href="${ORIGIN}${pathFor(page.id, l)}"/>`,
      );
      alternates.push(
        `    <xhtml:link rel="alternate" hreflang="x-default" href="${ORIGIN}${pathFor(page.id, DEFAULT_LOCALE)}"/>`,
      );
      return `  <url>\n    <loc>${ORIGIN}${pathFor(page.id, locale)}</loc>\n${alternates.join('\n')}\n  </url>`;
    }),
  );
  writeFileSync(
    join(dist, 'sitemap.xml'),
    `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">\n${urls.join('\n')}\n</urlset>\n`,
  );

  return written;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const written = build();
  console.log(`Built ${written.length} pages into dist/:\n  ${written.join('\n  ')}`);
}
