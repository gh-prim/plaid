import { CODE, LINKS, ambient, btn, cmd, eyebrow, glyph, page, poster, POSTERS, status } from '../layout.mjs';

const still = (img, alt, sizes, cls = '') =>
  `<img${cls ? ` class="${cls}"` : ''} src="/img/${img}.jpg" srcset="/img/${img}-800.jpg 800w, /img/${img}.jpg 1600w" sizes="${sizes}" width="1600" height="900" alt="${alt}" loading="lazy" decoding="async">`;

export default {
  id: 'home',
  slug: '',
  render(ctx) {
    const { t, attr } = ctx;

    // The web app, drawn: a browser window on the home screen.
    const appWindow = `<div class="appwin-wrap"><figure class="appwin" data-tilt role="img" aria-label="${attr('home.app.alt')}">
<div class="appwin__glow" aria-hidden="true"></div>
<div class="appwin__chrome" aria-hidden="true"><i></i><i></i><i></i><span class="appwin__url">${CODE.lanHost}</span></div>
<div class="appwin__hero" aria-hidden="true"><img src="/img/still-tears-of-steel.jpg" srcset="/img/still-tears-of-steel-800.jpg 800w, /img/still-tears-of-steel.jpg 1600w" sizes="(min-width: 1260px) 1080px, 100vw" width="1600" height="900" alt="" decoding="async">
<div class="appwin__tabs"><span class="is-on">${t('home.app.tabs.home')}</span><span>${t('home.app.tabs.movies')}</span><span>${t('home.app.tabs.shows')}</span></div>
<div class="appwin__now"><p>${t('home.app.continue')}</p><b>${t('films.tears-of-steel')}</b><span class="appwin__resume">${t('home.app.resume')}</span></div></div>
<p class="appwin__row-title" aria-hidden="true">${t('home.app.recent')}</p>
<div class="appwin__row" aria-hidden="true">${POSTERS.map((id) => poster(ctx, id, 'appwin__poster')).join('')}</div>
</figure></div>`;

    const cards = [
      ['library', 'still-big-buck-bunny'],
      ['plays', 'still-tears-of-steel-bridge'],
      ['yours', 'still-sprite-fright'],
    ]
      .map(
        ([k, img]) =>
          `<article class="whatis__card"><div class="whatis__img rvi">${still(img, '', '(min-width: 900px) 33vw, 100vw')}</div><h3 class="whatis__title rv">${t(`home.what.${k}.title`)}</h3><p class="whatis__body rv">${t(`home.what.${k}.body`)}</p></article>`,
      )
      .join('');

    const tv = `<div class="tv rv" role="img" aria-label="${attr('home.tv.alt')}"><div class="tv__screen" aria-hidden="true">${still('still-big-buck-bunny', '', '(min-width: 1100px) 700px, 100vw')}<b class="tv__title">${t('films.big-buck-bunny')}</b><span class="tv__bar"><i></i></span><span class="tv__pause">${glyph('pause', 14)}</span></div><div class="tv__foot" aria-hidden="true"></div></div>`;

    const devices = ['tv', 'laptop', 'tablet', 'phone']
      .map(
        (d, i) =>
          `<li class="dev dev--${d} dev--i${i}"><div class="dev__screen" aria-hidden="true"><img src="/img/still-caminandes-800.jpg" width="800" height="450" alt="" loading="lazy" decoding="async"><span class="dev__bar"><i></i></span></div><span class="dev__name">${t(`home.party.devices.${d}.name`)}</span><span class="dev__spec">${t(`home.party.devices.${d}.spec`)}</span></li>`,
      )
      .join('');

    const faq = ['free', 'account', 'pi', 'video', 'party']
      .map(
        (k, i) =>
          `<div class="fq hl${i === 0 ? ' open' : ''}" data-fq><h3 class="fq__h"><button type="button" class="fq__q" aria-expanded="${i === 0}" aria-controls="fa-${k}" id="fq-${k}"><span>${t(`home.faq.${k}.q`)}</span><span class="fq__pm" aria-hidden="true"></span></button></h3><div class="fa" id="fa-${k}" role="region" aria-labelledby="fq-${k}"><div><p>${t(`home.faq.${k}.a`)}</p></div></div></div>`,
      )
      .join('');

    const body = `<div class="hero-zone">
${ambient(['still-tears-of-steel', 'still-big-buck-bunny', 'still-sprite-fright'], 'ambient--home')}
<section class="hero" aria-labelledby="hero-title">
<h1 class="hero__title" id="hero-title"><span class="hero__line">${t('home.hero.line1')}</span> <span class="hero__line">${t('home.hero.line2')}</span> <span class="hero__line">${t('home.hero.line3')}</span></h1>
<p class="hero__lead hero-in">${t('home.hero.lead')}</p>
<div class="actions actions--center hero-in">${btn(ctx.href('install'), t('home.hero.install'), 'cream', 'lg')}${btn(LINKS.github, t('home.hero.github'), 'glass', 'lg')}</div>
</section>
${appWindow}
</div>

<section class="wrap whatis" aria-labelledby="what-title">
${eyebrow(t('home.what.eyebrow'), 'rv')}
<h2 class="h2 whatis__h" id="what-title" data-words>${t('home.what.title')}</h2>
<div class="whatis__grid">${cards}</div>
</section>

<section class="atv" aria-labelledby="atv-title">
<div class="atv__bg" aria-hidden="true"><img src="/img/still-big-buck-bunny-800.jpg" width="800" height="450" alt="" loading="lazy" decoding="async"></div>
<div class="wrap atv__in">
<div class="atv__text">${eyebrow(t('home.tv.eyebrow'), 'rv')}<h2 class="h2" id="atv-title" data-words>${t('home.tv.title')}</h2><p class="lead rv">${t('home.tv.body')}</p>
<ul class="ticks">${['pins', 'hdr', 'lang'].map((k) => `<li class="rv">${t(`home.tv.points.${k}`)}</li>`).join('')}</ul></div>
${tv}
</div>
</section>

<section class="wrap teaser" aria-labelledby="party-title">
<p class="badge rv">${status(ctx, 'planned')}</p>
<h2 class="h2 teaser__h" id="party-title" data-words>${t('home.party.title')}</h2>
<p class="lead teaser__lead rv">${t('home.party.body')}</p>
<ul class="devices" data-devices>${devices}</ul>
</section>

<section class="wrap install-teaser" aria-labelledby="install-title">
<div class="install-teaser__text">${eyebrow(t('home.install.eyebrow'), 'rv')}<h2 class="h2" id="install-title" data-words>${t('home.install.title')}</h2><p class="lead rv">${t('home.install.body')}</p></div>
${cmd(ctx, CODE.script, { variant: 'terminal', note: t('home.install.note') })}
</section>

<section class="wrap duo" aria-label="${attr('home.duo.label')}">
<div class="duo__col hl rv">${eyebrow(t('home.duo.account.eyebrow'))}<h3 class="h3">${t('home.duo.account.title')}</h3><p>${t('home.duo.account.body')}</p></div>
<div class="duo__col hl rv">${eyebrow(t('home.duo.remote.eyebrow'))}<h3 class="h3">${t('home.duo.remote.title')}</h3><p>${t('home.duo.remote.body')}</p><p class="duo__code"><a href="${ctx.href('remote')}"><code>${CODE.short}</code></a></p></div>
</section>

<section class="wrap faqmini" aria-labelledby="faq-title">
<h2 class="h2" id="faq-title" data-words>${t('home.faq.title')}</h2>
<div class="fqs">${faq}</div>
</section>`;

    return page(ctx, { title: attr('home.meta.title'), description: attr('home.meta.description'), body });
  },
};
