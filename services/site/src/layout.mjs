// Shared page shell: <head>, header, footer. Pages provide their <main> content.

import { LINKS, LOCALES, ORIGIN } from './config.mjs';

export const NAV = ['home', 'download', 'remoteAccess', 'privacy'];

const ICON_THEME = `<svg class="icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path class="moon" d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z"/><g class="sun"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></g></svg>`;

export function codeBlock(ctx, code, label) {
  const { attr } = ctx;
  const escaped = code
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');
  return `<figure class="code">
  ${label ? `<figcaption>${label}</figcaption>` : ''}
  <pre tabindex="0"><code>${escaped}</code></pre>
  <button type="button" class="copy" hidden aria-live="polite" data-copied="${attr('common.copied')}">${attr('common.copy')}</button>
</figure>`;
}

export function status(ctx, kind) {
  return `<span class="status status--${kind}">${ctx.t(`status.${kind}`)}</span>`;
}

function head(ctx, { title, description }) {
  const { locale, attr, pageId, url, assets } = ctx;
  const alternates = LOCALES.map(
    (l) => `<link rel="alternate" hreflang="${l}" href="${ORIGIN}${ctx.href(pageId, l)}">`,
  ).join('\n  ');
  const ogAlt = LOCALES.filter((l) => l !== locale)
    .map((l) => `<meta property="og:locale:alternate" content="${ctx.ogLocale(l)}">`)
    .join('\n  ');
  const robots = ctx.noindex ? '\n  <meta name="robots" content="noindex">' : '';
  return `<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>${title}</title>
  <meta name="description" content="${description}">${robots}
  <meta name="color-scheme" content="dark light">
  <meta name="theme-color" content="#0e141e">
  <link rel="canonical" href="${url}">
  ${alternates}
  <link rel="alternate" hreflang="x-default" href="${ORIGIN}${ctx.href(pageId, 'en')}">
  <link rel="icon" href="/favicon.svg" type="image/svg+xml">
  <link rel="apple-touch-icon" href="/apple-touch-icon.png">
  <link rel="preload" href="/fonts/InstrumentSans-Variable-latin.woff2" as="font" type="font/woff2" crossorigin>
  <link rel="preload" href="/fonts/InstrumentSerif-Regular-latin.woff2" as="font" type="font/woff2" crossorigin>
  <link rel="stylesheet" href="${assets.css}">
  <script src="${assets.theme}"></script>
  <script src="${assets.js}" defer></script>
  <meta property="og:type" content="website">
  <meta property="og:site_name" content="${attr('meta.siteName')}">
  <meta property="og:title" content="${title}">
  <meta property="og:description" content="${description}">
  <meta property="og:url" content="${url}">
  <meta property="og:image" content="${ORIGIN}/og.png">
  <meta property="og:image:width" content="1200">
  <meta property="og:image:height" content="630">
  <meta property="og:image:alt" content="${attr('meta.ogImageAlt')}">
  <meta property="og:locale" content="${ctx.ogLocale(locale)}">
  ${ogAlt}
  <meta name="twitter:card" content="summary_large_image">
</head>`;
}

function header(ctx) {
  const { t, attr, locale, pageId } = ctx;
  const other = LOCALES.find((l) => l !== locale);
  const links = NAV.filter((id) => id !== 'home')
    .map((id) => {
      const current = id === pageId ? ' aria-current="page"' : '';
      return `<li><a href="${ctx.href(id)}"${current}>${t(`nav.${id}`)}</a></li>`;
    })
    .join('');
  return `<a class="skip" href="#main">${t('nav.skip')}</a>
<header class="site-header">
  <div class="wrap header-inner">
    <a class="brand" href="${ctx.href('home')}" aria-label="${attr('nav.homeAria')}">
      <img src="/favicon.svg" alt="" width="36" height="36">
      <span>${t('meta.siteName')}</span>
    </a>
    <nav aria-label="${attr('nav.label')}">
      <ul>${links}</ul>
    </nav>
    <div class="header-tools">
      <a class="lang" href="${ctx.href(pageId, other)}" hreflang="${other}" lang="${other}">${t('nav.otherLanguage')}</a>
      <button type="button" class="theme-toggle" hidden aria-label="${attr('nav.themeToggle')}" aria-pressed="false">${ICON_THEME}</button>
    </div>
  </div>
</header>`;
}

function footer(ctx) {
  const { t, attr } = ctx;
  const pages = NAV.map((id) => `<li><a href="${ctx.href(id)}">${t(`nav.${id}`)}</a></li>`).join('');
  return `<footer class="site-footer">
  <div class="tartan-band" aria-hidden="true"></div>
  <div class="wrap footer-grid">
    <div class="footer-brand">
      <p class="footer-tagline">${t('footer.tagline')}</p>
      <p>${t('footer.license')}</p>
      <p>${t('footer.noTracking', { privacy: ctx.href('privacy') })}</p>
    </div>
    <nav aria-label="${attr('footer.pagesLabel')}">
      <h2>${t('footer.pagesLabel')}</h2>
      <ul>${pages}</ul>
    </nav>
    <nav aria-label="${attr('footer.projectLabel')}">
      <h2>${t('footer.projectLabel')}</h2>
      <ul>
        <li><a href="${LINKS.github}" rel="noopener">${t('footer.github')}</a></li>
        <li><a href="${LINKS.releases}" rel="noopener">${t('footer.releases')}</a></li>
        <li><a href="${LINKS.installGuide}" rel="noopener">${t('footer.installGuide')}</a></li>
        <li><a href="${LINKS.issues}" rel="noopener">${t('footer.issues')}</a></li>
      </ul>
    </nav>
  </div>
</footer>`;
}

export function layout(ctx, { title, description, body, pageClass = '' }) {
  return `<!doctype html>
<html lang="${ctx.locale}">
${head(ctx, { title, description })}
<body class="${pageClass}">
${header(ctx)}
<main id="main" tabindex="-1">
${body}
</main>
${footer(ctx)}
</body>
</html>
`;
}
