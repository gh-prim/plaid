import { LINKS } from '../config.mjs';
import { layout, status } from '../layout.mjs';

const FEATURES = [
  { id: 'party', status: 'planned' },
  { id: 'sharing', status: 'planned' },
  { id: 'remote', status: 'inProgress', link: 'remoteAccess' },
  { id: 'private', status: 'available' },
];

const SCREENS = [
  { id: 'web', status: 'available' },
  { id: 'appleTv', status: 'available' },
  { id: 'ios', status: 'planned' },
  { id: 'android', status: 'planned' },
];

const HARDWARE = ['pi', 'nas', 'docker', 'pc'];

export default {
  id: 'home',
  slug: '',
  render(ctx) {
    const { t, attr } = ctx;
    const features = FEATURES.map(
      (f) => `<li class="feature">
        <div class="feature-head"><h3>${t(`home.features.${f.id}.title`)}</h3>${status(ctx, f.status)}</div>
        <p>${t(`home.features.${f.id}.body`)}</p>
        ${f.link ? `<p><a class="more" href="${ctx.href(f.link)}">${t(`home.features.${f.id}.link`)}</a></p>` : ''}
      </li>`,
    ).join('');
    const screens = SCREENS.map(
      (s) => `<li><span>${t(`home.screens.items.${s.id}`)}</span>${status(ctx, s.status)}</li>`,
    ).join('');
    const hardware = HARDWARE.map((h) => `<li>${t(`home.hardware.items.${h}`)}</li>`).join('');

    const body = `
<section class="hero">
  <div class="wrap hero-inner">
    <div class="hero-copy">
      <p class="eyebrow">${t('home.hero.eyebrow')}</p>
      <h1>${t('home.hero.title')}<span class="hero-sub">${t('home.hero.subtitle')}</span></h1>
      <p class="lead">${t('home.hero.lead')}</p>
      <div class="actions">
        <a class="button button--primary" href="${ctx.href('download')}">${t('home.hero.ctaPrimary')}</a>
        <a class="button button--ghost" href="${LINKS.github}" rel="noopener">${t('home.hero.ctaSecondary')}</a>
      </div>
      <p class="hero-note">${t('home.hero.status')}</p>
    </div>
    <div class="hero-art" aria-hidden="true">
      <div class="lamp"></div>
      <img src="/favicon.svg" alt="" width="320" height="320">
    </div>
  </div>
</section>

<section class="section pitch">
  <div class="wrap narrow">
    <h2>${t('home.pitch.title')}</h2>
    <p class="lead">${t('home.pitch.body')}</p>
  </div>
</section>

<section class="section" aria-labelledby="features-title">
  <div class="wrap">
    <h2 id="features-title" class="section-title">${t('home.features.title')}</h2>
    <ul class="features">${features}</ul>
  </div>
</section>

<section class="section split" aria-labelledby="screens-title">
  <div class="wrap split-inner">
    <div>
      <h2 id="screens-title">${t('home.screens.title')}</h2>
      <p>${t('home.screens.body')}</p>
      <ul class="status-list">${screens}</ul>
    </div>
    <div>
      <h2>${t('home.hardware.title')}</h2>
      <p>${t('home.hardware.body')}</p>
      <ul class="chips">${hardware}</ul>
      <p><a class="more" href="${ctx.href('download')}">${t('home.hardware.link')}</a></p>
    </div>
  </div>
</section>

<section class="section closing">
  <div class="wrap narrow center">
    <h2>${t('home.closing.title')}</h2>
    <p>${t('home.closing.body')}</p>
    <div class="actions actions--center">
      <a class="button button--primary" href="${ctx.href('download')}">${t('home.closing.cta')}</a>
    </div>
  </div>
</section>`;
    return layout(ctx, {
      title: attr('home.meta.title'),
      description: attr('home.meta.description'),
      body,
      pageClass: 'page-home',
    });
  },
};
