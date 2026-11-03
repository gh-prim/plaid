// Shared markup: <head>, header, footer, and the components the pages compose.
// Every visible string comes from the locale files; no inline styles or
// scripts (the Content Security Policy forbids them), so every variation is a
// class. Motion hooks are explicit: .rv (reveal), .rvi (image reveal),
// .hl (hairline), [data-words] (word-by-word heading), [data-type] (typed
// command), .dev (Plaid Party devices), [data-copy] (copy button).

import { BRAND, CODE, LINKS, LOCALES } from './config.mjs';
import { escapeHtml } from './i18n.mjs';
import { ICON_DEFS, glyph, icon } from './icons.mjs';

export const NAV = ['features', 'party', 'install', 'roadmap', 'faq'];

const ext = (href) => (/^https?:/.test(href) ? ' rel="noopener"' : '');

// ---------- small components ----------

// Pills: cream for the main action, glass for the rest. size: sm | md | lg.
export function btn(href, label, kind = 'cream', size = 'md', cls = '') {
  return `<a class="btn btn--${kind} btn--${size}${cls ? ` ${cls}` : ''}" href="${href}"${ext(href)}>${label}</a>`;
}

export function eyebrow(text, cls = '') {
  return `<p class="eyebrow${cls ? ` ${cls}` : ''}">${text}</p>`;
}

// Status words, one vocabulary site-wide.
export function status(ctx, key) {
  return `<span class="status status--${key}">${ctx.t(`status.${key}`)}</span>`;
}

function copyButton(ctx, kind) {
  return `<button type="button" class="btn btn--${kind} btn--copy" data-copy hidden data-label="${ctx.attr('common.copy')}" data-copied="${ctx.attr('common.copied')}" aria-label="${ctx.attr('common.copyLabel')}">${ctx.t('common.copy')}</button>`;
}

// A shell command. variant: line (dark rounded row, glass Copy) or terminal
// (the home page card: window dots, typed command, cream Copy and a note).
export function cmd(ctx, code, { variant = 'line', note = '', copy = true } = {}) {
  const codeHtml = `<code class="cmd__code"${variant === 'terminal' ? ' data-type' : ''}><span class="cmd__prompt" aria-hidden="true">$ </span><span class="cmd__text">${escapeHtml(code)}</span></code>`;
  const live = '<span class="sr" aria-live="polite"></span>';
  if (variant === 'terminal') {
    return `<div class="term rv" data-cmd><div class="term__dots" aria-hidden="true"><i></i><i></i><i></i></div>${codeHtml}<div class="term__foot">${copy ? copyButton(ctx, 'cream') : ''}${note ? `<span class="term__note">${note}</span>` : ''}</div>${live}</div>`;
  }
  return `<div class="cmd" data-cmd>${codeHtml}${copy ? copyButton(ctx, 'glass') : ''}${live}</div>`;
}

// A value to read, not to run (no prompt, no copy).
export function value(text) {
  return `<div class="cmd cmd--value"><code class="cmd__code"><span class="cmd__text">${escapeHtml(text)}</span></code></div>`;
}

// Full-width rounded still under a page hero, with its credit.
export function banner(ctx, { img, alt, credit, pos = 'center' }) {
  return `<figure class="banner rvi banner--${pos}"><img src="/img/${img}.jpg" srcset="/img/${img}-800.jpg 800w, /img/${img}.jpg 1600w" sizes="(min-width: 1260px) 1200px, 100vw" width="1600" height="900" alt="${alt}" fetchpriority="high" decoding="async"><figcaption class="credit">${credit}</figcaption></figure>`;
}

// Screenshots rendered from the design boards, one file per locale
// (public/img/screen-<name>-<locale>.jpg, plus an -800 variant): [width, height].
const SHOTS = {
  title: [1600, 955],
  ios: [1600, 850],
  'apple-tv': [1600, 1000],
  party: [1600, 833],
  'web-party': [1600, 1000],
  'ipad-party': [1600, 1117],
  remote: [1600, 666],
};

// A screenshot of the apps in the page's language, with an optional caption.
export function shot(ctx, { name, alt, caption = '', sizes = '(min-width: 1260px) 1200px, 100vw', cls = '' }) {
  const [w, h] = SHOTS[name];
  const src = `/img/screen-${name}-${ctx.locale}`;
  return `<figure class="shot rvi${cls ? ` ${cls}` : ''}"><img src="${src}.jpg" srcset="${src}-800.jpg 800w, ${src}.jpg 1600w" sizes="${sizes}" width="${w}" height="${h}" alt="${alt}" loading="lazy" decoding="async">${caption ? `<figcaption class="shot__cap">${caption}</figcaption>` : ''}</figure>`;
}

// The blurred film-still field behind a page's first screen.
export function ambient(imgs, cls = '') {
  return `<div class="ambient${cls ? ` ${cls}` : ''}" aria-hidden="true"><div class="ambient__field" data-ambient>${imgs
    .map((img) => `<img src="/img/${img}-800.jpg" alt="" width="800" height="450" decoding="async">`)
    .join('')}</div><div class="ambient__fade"></div></div>`;
}

// A page's opening: eyebrow (or badge), display title, lead.
export function pageHero(ctx, { eyebrow: eb = '', badge = '', title, lead, cls = '' }) {
  return `<header class="phero${cls ? ` ${cls}` : ''}">${badge}${eb ? eyebrow(eb, 'phero__eyebrow hero-in') : ''}<h1 class="phero__title">${title}</h1>${lead ? `<p class="phero__lead hero-in">${lead}</p>` : ''}</header>`;
}

export const POSTERS = ['sintel', 'nosferatu', 'tears-of-steel', 'metropolis', 'sprite-fright', 'caligari', 'big-buck-bunny', 'voyage-dans-la-lune'];

export function poster(ctx, id, cls = 'poster') {
  return `<span class="${cls}"><img src="/img/poster-${id}-300.jpg" srcset="/img/poster-${id}-300.jpg 300w, /img/poster-${id}.jpg 600w" sizes="160px" width="300" height="450" alt="${ctx.attr('posters.alt', { film: ctx.raw(`films.${id}`) })}" loading="lazy" decoding="async"></span>`;
}

// The staggered poster row that closes install, roadmap and open source.
export function posterParade(ctx, key) {
  return `<section class="parade" aria-label="${ctx.attr(key)}">${eyebrow(ctx.t(key), 'rv')}<div class="parade__row">${POSTERS.slice(0, 7)
    .map((id) => poster(ctx, id, 'poster poster--parade rv'))
    .join('')}</div><p class="parade__credit">${ctx.t('posters.credit')}</p></section>`;
}

// A card with a hairline on top, the site's repeated block.
export function card({ id = '', icon: ic = '', kicker = '', title, body = '', cls = '', titleTag = 'h3' }) {
  return `<div${id ? ` id="${id}"` : ''} class="card hl rv${cls ? ` ${cls}` : ''}">${kicker}${ic ? icon(ic) : ''}<${titleTag} class="card__title">${title}</${titleTag}>${body}</div>`;
}

// A closing call-to-action row.
export function ctaRow({ icon: ic, title, body = '', actions }) {
  return `<div class="cta hl rv"><div class="cta__text">${icon(ic)}<h2 class="cta__title">${title}</h2>${body ? `<p class="cta__body">${body}</p>` : ''}</div><div class="cta__actions">${actions}</div></div>`;
}

// ---------- page frame ----------

function head(ctx, { title, description }) {
  const alternates = LOCALES.map(
    (l) => `<link rel="alternate" hreflang="${l}" href="${ctx.origin}${ctx.href(ctx.pageId, l)}">`,
  ).join('');
  const robots = ctx.noindex ? '<meta name="robots" content="noindex">' : '';
  const canonical = ctx.noindex ? '' : `<link rel="canonical" href="${ctx.url}">`;
  const xdefault = ctx.noindex
    ? ''
    : `<link rel="alternate" hreflang="x-default" href="${ctx.origin}${ctx.href(ctx.pageId, 'en')}">`;
  const other = LOCALES.filter((l) => l !== ctx.locale)
    .map((l) => `<meta property="og:locale:alternate" content="${ctx.ogLocale(l)}">`)
    .join('');
  return `<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title}</title>
<meta name="description" content="${description}">
${robots}${canonical}${ctx.noindex ? '' : alternates + xdefault}
<meta name="theme-color" content="#0b0b0c">
<meta name="color-scheme" content="dark">
<meta property="og:type" content="website">
<meta property="og:site_name" content="PLAID">
<meta property="og:title" content="${title}">
<meta property="og:description" content="${description}">
<meta property="og:url" content="${ctx.url}">
<meta property="og:image" content="${ctx.origin}/og.jpg">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:image:alt" content="${ctx.attr('meta.ogAlt')}">
<meta property="og:locale" content="${ctx.ogLocale(ctx.locale)}">${other}
<meta name="twitter:card" content="summary_large_image">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<link rel="preload" href="/fonts/BricolageGrotesque-latin.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="/fonts/InstrumentSans-Variable-latin.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="${ctx.assets.css}">
<script src="${ctx.assets.boot}"></script>
<script src="${ctx.assets.js}" defer></script>
</head>`;
}

function brand(ctx, cls = 'brand') {
  return `<a class="${cls}" href="${ctx.href('home')}" aria-label="${ctx.attr('header.home')}"><img class="brand__logo" src="/logo.svg" width="34" height="34" alt=""><span class="brand__word" aria-hidden="true">${BRAND}</span></a>`;
}

function langSwitch(ctx) {
  const target = ctx.pageId === 'notFound' ? 'home' : ctx.pageId;
  return `<div class="lang" role="group" aria-label="${ctx.attr('header.language')}">${LOCALES.map((l) =>
    l === ctx.locale
      ? `<span class="lang__item is-current" lang="${l}" aria-current="true">${ctx.t(`header.lang.${l}`)}</span>`
      : `<a class="lang__item" lang="${l}" hreflang="${l}" href="${ctx.href(target, l)}" aria-label="${ctx.attr(`header.langName.${l}`)}">${ctx.t(`header.lang.${l}`)}</a>`,
  ).join('')}</div>`;
}

function navLinks(ctx, cls) {
  return (
    NAV.map((id) => {
      const current = id === ctx.pageId;
      return `<a class="${cls}${current ? ' is-active' : ''}" href="${ctx.href(id)}"${current ? ' aria-current="page"' : ''}>${ctx.t(`nav.${id}`)}</a>`;
    }).join('') + `<a class="${cls}" href="${LINKS.github}" rel="noopener">${ctx.t('nav.github')}</a>`
  );
}

function header(ctx) {
  return `<a class="skip" href="#main">${ctx.t('header.skip')}</a>
<div class="prog" aria-hidden="true"></div>
<header class="hdr" data-hdr><div class="hdr__bar">
${brand(ctx)}
<nav class="nav" aria-label="${ctx.attr('header.nav')}">${navLinks(ctx, 'nav__link')}</nav>
<div class="hdr__end">${langSwitch(ctx)}${btn(ctx.href('install'), ctx.t('header.install'), 'cream', 'sm', 'hdr__cta')}
<details class="menu"><summary class="menu__btn" aria-label="${ctx.attr('header.menu')}">${glyph('menu', 20)}</summary>
<div class="menu__sheet"><nav aria-label="${ctx.attr('header.nav')}">${navLinks(ctx, 'menu__link')}</nav>${btn(ctx.href('install'), ctx.t('header.install'), 'cream', 'md', 'menu__cta')}</div></details></div>
</div></header>`;
}

function footer(ctx) {
  const col = (key, items) =>
    `<div class="foot__col"><h2 class="foot__title">${ctx.t(`footer.${key}`)}</h2><ul>${items
      .map(([label, href]) => `<li><a class="foot__link" href="${href}"${ext(href)}>${ctx.t(label)}</a></li>`)
      .join('')}</ul></div>`;
  return `<footer class="foot"><div class="wrap foot__in">
<div class="foot__brand">${brand(ctx, 'brand brand--foot')}<p class="foot__tagline">${ctx.t('footer.tagline')}</p><p class="foot__credits">${ctx.t('footer.credits')}</p></div>
<div class="foot__cols">
${col('product', [['nav.features', ctx.href('features')], ['nav.party', ctx.href('party')], ['nav.install', ctx.href('install')], ['nav.roadmap', ctx.href('roadmap')]])}
${col('project', [['nav.github', LINKS.github], ['footer.releases', LINKS.releases], ['footer.howWeBuild', ctx.href('openSource')], ['nav.faq', ctx.href('faq')]])}
${col('direct', [['nav.remote', ctx.href('remote')], ['nav.privacy', ctx.href('privacy')], ['nav.protocol', ctx.href('protocol')]])}
${col('docs', [['footer.installGuide', ctx.href('install')], ['footer.api', LINKS.openapi]])}
</div>
</div></footer>`;
}

export function page(ctx, { title, description, body }) {
  return `<!doctype html>
<html lang="${ctx.locale}">
${head(ctx, { title, description })}
<body class="page-${ctx.pageId}">
${ICON_DEFS}
${header(ctx)}
<main id="main">
${body}
</main>
${footer(ctx)}
<div class="veil" aria-hidden="true"></div>
</body>
</html>
`;
}

export { CODE, LINKS, icon, glyph };
