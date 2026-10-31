// Shared markup: <head>, header, footer, and the few components the pages
// compose. Components mirror the web app's: CommandBlock (.pl-cmd), the
// onboarding pills (.pl-btn + cream/glass), status labels (.pl-tag), the
// player timeline (.pl-tl), avatars (.pl-avatar), toasts (.pl-toast).
// Every visible string comes from the locale files.

import { CODE, LINKS, LOCALES } from './config.mjs';
import { escapeHtml } from './i18n.mjs';

export const NAV = ['features', 'party', 'install', 'roadmap', 'faq'];

// Foxy, the fox of the brand: the dot on the i of the wordmark (web/src/browse/Wordmark.tsx).
const FOXY = `<svg class="foxy" viewBox="0 0 100 100" aria-hidden="true" focusable="false"><path d="M25 47 C14 33 9 15 15 4 C30 3 42 14 46 28 Z" fill="var(--fx-tan)"/><path d="M28 41 C20 31 17 19 20 11 C29 12 37 20 40 29 Z" fill="var(--fx-cream)" opacity="0.22"/><path d="M25 47 C14 33 9 15 15 4 C30 3 42 14 46 28" fill="none" stroke="var(--fx-ink)" stroke-opacity="0.6" stroke-width="2.2"/><path d="M55 28 C61 14 77 7 92 11 C98 24 93 39 79 47 Z" fill="var(--fx-tan)"/><path d="M61 29 C66 20 77 15 88 16 C91 25 87 34 79 40 Z" fill="var(--fx-cream)" opacity="0.22"/><path d="M55 28 C61 14 77 7 92 11 C98 24 93 39 79 47" fill="none" stroke="var(--fx-ink)" stroke-opacity="0.6" stroke-width="2.2"/><path d="M50 24 C68 24 80 36 81 52 C82 66 74 76 64 82 C59 86 54 88 50 88 C46 88 41 86 36 82 C26 76 18 66 19 52 C20 36 32 24 50 24 Z" fill="var(--fx-tan)"/><path d="M28 33 C34 27 42 24 50 24 C58 24 66 27 72 33 C65 31 57 32 50 36 C43 32 35 31 28 33 Z" fill="var(--fx-ink)" opacity="0.16"/><path d="M50 53 C58 53 63 60 64 67 C65 78 58 87 50 87 C42 87 35 78 36 67 C37 60 42 53 50 53 Z" fill="var(--fx-cream)"/><path d="M27 72 C30 79 36 84 42 86 C38 80 36 76 36 71 Z M73 72 C70 79 64 84 58 86 C62 80 64 76 64 71 Z" fill="var(--fx-cream)" opacity="0.7"/><ellipse cx="50" cy="64" rx="12" ry="8" fill="var(--fx-ink)" opacity="0.1"/><circle cx="35" cy="48.5" r="6.3" fill="var(--fx-ink)"/><circle cx="65" cy="48.5" r="6.3" fill="var(--fx-ink)"/><circle cx="36.8" cy="46.4" r="1.8" fill="var(--fx-cream)"/><circle cx="66.8" cy="46.4" r="1.8" fill="var(--fx-cream)"/><path d="M41.5 64.5 C41.5 58.5 58.5 58.5 58.5 64.5 C58.5 69.5 53.5 72.5 50 72.5 C46.5 72.5 41.5 69.5 41.5 64.5 Z" fill="var(--fx-ink)"/><path d="M50 72.5 L50 75.5 M46.5 77 C48.5 78.2 51.5 78.2 53.5 77" fill="none" stroke="var(--fx-ink)" stroke-width="1.8" stroke-linecap="round"/></svg>`;

// The wordmark is the brand, never translated: "plaid" with Foxy on a dotless i.
const WORDMARK = `<span class="wordmark" aria-hidden="true">pla<span class="wordmark__i">ı${FOXY}</span>d</span>`;

const ICONS = {
  theme:
    '<svg class="pl-icon" width="20" height="20" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/></svg>',
  check:
    '<svg class="pl-icon" width="16" height="16" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M5 12.5l4.5 4.5L19 7.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  arrow:
    '<svg class="pl-icon" width="16" height="16" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M5 12h13m-5-6 6 6-6 6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
};

export function icon(name) {
  return ICONS[name];
}

// ---------- components ----------

// A shell command the reader copies (web/src/ui/CommandBlock.tsx).
export function cmd(ctx, code) {
  return `<div class="pl pl-cmd"><code class="pl-cmd__code">${escapeHtml(code)}</code><button type="button" class="pl-cmd__btn pl-focusable" hidden data-copy data-label="${ctx.attr('common.copy')}" data-copied="${ctx.attr('common.copied')}">${ctx.t('common.copy')}</button><span class="pl-sr" aria-live="polite"></span></div>`;
}

// A value to read, not to run: a hostname, a URL.
export function value(code) {
  return `<code class="value">${escapeHtml(code)}</code>`;
}

// Buttons: the app's onboarding pills. kind: cream (one per view) or glass.
export function button(href, label, kind = 'glass', extra = '') {
  const ext = /^https?:/.test(href) ? ' rel="noopener"' : '';
  return `<a class="pl-btn pl-focusable btn-${kind}${extra}" href="${href}"${ext}>${label}</a>`;
}

export function more(href, label) {
  const ext = /^https?:/.test(href) ? ' rel="noopener"' : '';
  return `<a class="more" href="${href}"${ext}>${label}${icon('arrow')}</a>`;
}

// Status label (the app's .pl-tag), one vocabulary site-wide.
export function tag(ctx, status) {
  return `<span class="pl-tag status status--${status}">${ctx.t(`status.${status}`)}</span>`;
}

// A real screenshot of the running app, cropped, at two widths.
export const SHOTS = {
  player: { w: [960, 1600], ratio: [1600, 300] },
  account: { w: [680, 1200], ratio: [1200, 970] },
  scan: { w: [760, 1400], ratio: [1400, 764] },
  remote: { w: [680, 1200], ratio: [1200, 920] },
  appletv: { w: [700, 1250], ratio: [1250, 720] },
};

export function shot(ctx, name, { sizes = '(min-width: 1100px) 60vw, 100vw', eager = false, mod = '' } = {}) {
  const s = SHOTS[name];
  const [small, big] = s.w;
  const h = Math.round((big * s.ratio[1]) / s.ratio[0]);
  return `<figure class="shot${mod ? ` shot--${mod}` : ''}"><div class="shot__frame"><img src="/shots/${name}-${small}.jpg" srcset="/shots/${name}-${small}.jpg ${small}w, /shots/${name}-${big}.jpg ${big}w" sizes="${sizes}" width="${big}" height="${h}" alt="${ctx.attr(`shots.${name}.alt`)}"${eager ? ' fetchpriority="high"' : ' loading="lazy"'} decoding="async"></div><figcaption>${ctx.t(`shots.${name}.caption`)}</figcaption></figure>`;
}

// ---------- the night: the site's one idea ----------
// A timeline down the left of the page. Section markers are times of one
// movie night; the played part of the thread (woven, as in the player)
// follows the reader down the page.

export function night(beats, mod = '') {
  return `<div class="night${mod ? ` night--${mod}` : ''}" data-night><div class="night__track" aria-hidden="true"><div class="night__played"></div></div>${beats.join('')}</div>`;
}

// time: '21:02' (clock), '1:12:48' (film position) or a word; layout: split | wide | flip | text.
export function beat({ id, time, datetime, title, body, aside = '', layout = 'split' }) {
  const marker = datetime
    ? `<time datetime="${datetime}">${time}</time>`
    : `<span>${time}</span>`;
  return `<section class="beat beat--${layout}" id="${id}" aria-labelledby="${id}-title"><p class="beat__time">${marker}</p><div class="beat__text"><h2 class="beat__title" id="${id}-title">${title}</h2>${body}</div>${aside ? `<div class="beat__aside">${aside}</div>` : ''}</section>`;
}

// The Plaid Party room, drawn with the design system's components. Concept:
// the feature doesn't exist yet, and the label says so.
export function partyRoom(ctx, { toast = false } = {}) {
  const people = [
    ['ada', 1, 'ready'],
    ['sam', 4, 'ready'],
    ['jo', 6, 'loading'],
    ['lou', 8, 'ready'],
  ];
  const crowd = people
    .map(
      ([p, c, state]) =>
        `<li class="crowd__p"><span class="pl-avatar pc-${c}"><span>${ctx.t(`party.room.people.${p}.initial`)}</span><i class="pl-avatar__status pl-avatar__status--${state}" aria-hidden="true"></i></span><span class="crowd__who"><span class="crowd__name">${ctx.t(`party.room.people.${p}.name`)}</span><span class="crowd__dev">${ctx.t(`party.room.people.${p}.device`)}</span></span></li>`,
    )
    .join('');
  const toastHtml = toast
    ? `<div class="pl-toast room__toast" role="presentation"><span class="pl-toast__text">${ctx.t('party.room.toast')}</span><span class="pl-btn btn-cream btn-sm">${ctx.t('party.room.accept')}</span><span class="pl-btn btn-glass btn-sm">${ctx.t('party.room.keep')}</span></div>`
    : '';
  return `<figure class="room"><div class="room__screen is-dark" role="img" aria-label="${ctx.attr(toast ? 'party.room.altToast' : 'party.room.alt')}"><span class="pl-tag room__concept">${ctx.t('status.concept')}</span>${toastHtml}<div class="room__bottom"><p class="room__title">${ctx.t('party.room.film')}</p><div class="pl-tl"><div class="pl-tl__track"><div class="pl-tl__seg room__seg"><div class="pl-tl__buf room__buf"></div><div class="pl-tl__played room__played"></div></div></div></div><p class="room__times"><span>1:12:48</span><span>${ctx.t('party.room.left')}</span></p><ul class="crowd">${crowd}</ul></div></div><figcaption>${ctx.t('party.room.caption')}</figcaption></figure>`;
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
<meta name="theme-color" content="#151311">
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
<link rel="preload" href="/fonts/InstrumentSans-Variable-latin.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="${ctx.assets.css}">
<script src="${ctx.assets.theme}"></script>
<script src="${ctx.assets.js}" defer></script>
</head>`;
}

function langSwitch(ctx, cls) {
  return `<div class="${cls}" role="group" aria-label="${ctx.attr('header.language')}">${LOCALES.map((l) =>
    l === ctx.locale
      ? `<span class="lang__item is-current" lang="${l}" aria-current="true">${ctx.t(`header.lang.${l}`)}</span>`
      : `<a class="lang__item pl-focusable" lang="${l}" hreflang="${l}" href="${ctx.href(ctx.pageId === 'notFound' ? 'home' : ctx.pageId, l)}" aria-label="${ctx.attr(`header.langName.${l}`)}">${ctx.t(`header.lang.${l}`)}</a>`,
  ).join('')}</div>`;
}

function themeToggle(ctx) {
  return `<button type="button" class="iconbtn pl-focusable theme-toggle" hidden aria-pressed="false" aria-label="${ctx.attr('header.theme')}">${icon('theme')}</button>`;
}

function header(ctx) {
  const links = (cls) =>
    NAV.map((id) => {
      const current = id === ctx.pageId;
      return `<a class="${cls}${current ? ' is-active' : ''}" href="${ctx.href(id)}"${current ? ' aria-current="page"' : ''}>${ctx.t(`nav.${id}`)}</a>`;
    }).join('');
  return `<a class="skip" href="#main">${ctx.t('header.skip')}</a>
<header class="top"><div class="top__in">
<a class="top__brand pl-focusable" href="${ctx.href('home')}" aria-label="${ctx.attr('header.home')}">${WORDMARK}</a>
<nav class="nav" aria-label="${ctx.attr('header.nav')}">${links('nav__tab pl-focusable')}</nav>
<div class="top__end">${langSwitch(ctx, 'lang')}${themeToggle(ctx)}${button(LINKS.github, ctx.t('header.github'), 'glass', ' btn-sm')}</div>
<details class="menu"><summary class="pl-btn btn-glass btn-sm pl-focusable">${ctx.t('header.menu')}</summary>
<div class="menu__sheet"><nav aria-label="${ctx.attr('header.nav')}">${links('menu__link')}</nav>
<div class="menu__end">${langSwitch(ctx, 'lang')}${themeToggle(ctx)}${button(LINKS.github, ctx.t('header.github'), 'glass')}</div></div></details>
</div></header>`;
}

function footer(ctx) {
  const col = (key, items) =>
    `<div class="foot__col"><h2 class="foot__title">${ctx.t(`footer.${key}.title`)}</h2><ul>${items
      .map(([label, href]) => {
        const ext = /^https?:/.test(href) ? ' rel="noopener"' : '';
        return `<li><a href="${href}"${ext}>${ctx.t(label)}</a></li>`;
      })
      .join('')}</ul></div>`;
  return `<footer class="foot"><div class="wrap foot__in">
<p class="foot__tagline">${ctx.t('footer.tagline')}</p>
<div class="foot__cols">
${col('product', [['nav.features', ctx.href('features')], ['nav.party', ctx.href('party')], ['nav.install', ctx.href('install')], ['nav.roadmap', ctx.href('roadmap')]])}
${col('project', [['nav.openSource', ctx.href('openSource')], ['nav.faq', ctx.href('faq')], ['footer.project.github', LINKS.github], ['footer.project.releases', LINKS.releases]])}
${col('direct', [['nav.remote', ctx.href('remote')], ['nav.privacy', ctx.href('privacy')]])}
${col('docs', [['footer.docs.guide', LINKS.installGuide], ['footer.docs.api', LINKS.openapi], ['footer.docs.protocol', LINKS.protocol]])}
</div>
<div class="foot__bottom"><p>${ctx.t('footer.bottom', { license: LINKS.license })}</p>${langSwitch(ctx, 'lang')}${themeToggle(ctx)}</div>
</div></footer>`;
}

export function page(ctx, { title, description, body }) {
  return `<!doctype html>
<html lang="${ctx.locale}" data-theme="dark">
${head(ctx, { title, description })}
<body>
${header(ctx)}
<main id="main">
${body}
</main>
${footer(ctx)}
</body>
</html>
`;
}

// A page opening: a title in the app's display voice, a short lead, actions.
export function opening(ctx, { kicker = '', title, lead, actions = '', aside = '', mod = '' }) {
  return `<header class="opening${mod ? ` opening--${mod}` : ''}"><div class="opening__text">${kicker}<h1 class="opening__title">${title}</h1>${lead ? `<p class="opening__lead">${lead}</p>` : ''}${actions ? `<div class="actions">${actions}</div>` : ''}</div>${aside ? `<div class="opening__aside">${aside}</div>` : ''}</header>`;
}

export { CODE, LINKS };
