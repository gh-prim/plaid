import { ambient, banner, card, page, pageHero, shot } from '../layout.mjs';

export default {
  id: 'remote',
  slug: 'remote-access/',
  render(ctx) {
    const { t, attr } = ctx;

    const steps = [
      ['port', 'router'],
      ['name', 'tag'],
      ['cert', 'shield'],
      ['link', 'link'],
    ]
      .map(([k, ic], i) =>
        card({
          kicker: `<span class="card__num" aria-hidden="true">${i + 1}</span>`,
          icon: ic,
          title: t(`remote.steps.${k}.title`),
          body: `<p>${t(`remote.steps.${k}.body`)}</p>`,
          cls: 'card--step',
        }),
      )
      .join('');

    const node = (k) =>
      `<li class="flow__node flow__node--${k}"><span class="flow__dot" aria-hidden="true"></span><b>${t(`remote.flow.${k}.title`)}</b><p>${t(`remote.flow.${k}.body`)}</p></li>`;

    const flow = `<figure class="flow rv"><ol class="flow__nodes">${node('friend')}${node('direct')}${node('server')}</ol>
<svg class="flow__lines rvs" viewBox="0 0 1200 300" preserveAspectRatio="none" aria-hidden="true" focusable="false"><path class="flow__dash" d="M260 90 C 360 60, 400 60, 470 60"/><path class="flow__dash" d="M730 60 C 800 60, 850 70, 940 90"/><path class="flow__video" d="M940 150 C 700 260, 500 260, 260 150"/><path class="flow__video" d="M272 146 l-14 2 l8 12"/></svg>
<span class="flow__label flow__label--dns" aria-hidden="true">${t('remote.flow.dns')}</span><span class="flow__label flow__label--ip" aria-hidden="true">${t('remote.flow.ip')}</span>
<figcaption class="flow__video-label">${t('remote.flow.video')}</figcaption></figure>`;

    const cant = [
      ['cgnat', 'home'],
      ['docker', 'docker'],
      ['own', 'gear'],
    ]
      .map(([k, ic]) => card({ icon: ic, title: t(`remote.cant.${k}.title`), body: `<p>${t(`remote.cant.${k}.body`)}</p>` }))
      .join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-tears-of-steel'], 'ambient--short')}
${pageHero(ctx, { eyebrow: t('remote.eyebrow'), title: t('remote.title'), lead: t('remote.lead') })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-tears-of-steel', alt: attr('remote.bannerAlt'), credit: t('credits.tearsOfSteel'), pos: 'center' })}
<section class="grid4 steps" aria-label="${attr('remote.steps.label')}">${steps}</section>
${shot(ctx, { name: 'remote', alt: attr('shots.remote.alt'), caption: t('shots.remote.cap'), cls: 'shot--wide' })}
<section class="block" aria-labelledby="never-title">
<h2 class="h2 h2--md" id="never-title" data-words>${t('remote.never.title')}</h2>
<p class="lead lead--sm rv">${t('remote.never.body')}</p>
${flow}
</section>
${card({ icon: 'power', title: t('remote.off.title'), body: `<p>${t('remote.off.body')}</p>`, cls: 'card--wide', titleTag: 'h2' })}
<section class="block" aria-labelledby="cant-title">
<h2 class="h2 h2--md" id="cant-title" data-words>${t('remote.cant.title')}</h2>
<div class="grid3">${cant}</div>
</section>
</div>`;
    return page(ctx, { title: attr('remote.meta.title'), description: attr('remote.meta.description'), body });
  },
};
