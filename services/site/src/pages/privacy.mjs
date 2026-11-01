import { banner, card, eyebrow, page, pageHero } from '../layout.mjs';

const KEPT = ['id', 'name', 'token', 'ips', 'lan', 'port', 'version', 'dates', 'acme'];

export default {
  id: 'privacy',
  slug: 'privacy/',
  render(ctx) {
    const { t, attr } = ctx;
    const kept = `<p class="card__lead card__lead--sm">${t('privacy.kept.body')}</p><dl class="kv">${KEPT.map(
      (k) => `<div class="kv__row"><dt class="kv__k kv__k--label">${t(`privacy.kept.rows.${k}.name`)}</dt><dd class="kv__v kv__v--strong">${t(`privacy.kept.rows.${k}.value`)}</dd></div>`,
    ).join('')}</dl>`;

    const k = (key) => eyebrow(t(key), 'card__kicker');

    const body = `<div class="hero-zone hero-zone--page hero-zone--plain">
${pageHero(ctx, { eyebrow: t('privacy.eyebrow'), title: t('privacy.title'), lead: t('privacy.lead') })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-big-buck-bunny-close', alt: attr('privacy.bannerAlt'), credit: t('credits.bigBuckBunny'), pos: 'bbb' })}
<div class="privacy">
${card({ kicker: k('privacy.kept.kicker'), icon: 'archive', title: t('privacy.kept.title'), body: kept, cls: 'card--big', titleTag: 'h2' })}
<div class="privacy__side">
${card({ kicker: k('privacy.never.kicker'), icon: 'eyeOff', title: t('privacy.never.title'), body: `<p>${t('privacy.never.body')}</p>`, cls: 'card--mid', titleTag: 'h2' })}
${card({ kicker: k('privacy.gone.kicker'), icon: 'trash', title: t('privacy.gone.title'), body: `<p>${t('privacy.gone.body')}</p>`, cls: 'card--mid', titleTag: 'h2' })}
</div>
</div>
<div class="grid3 privacy__more">
${card({ kicker: k('privacy.server.kicker'), icon: 'server', title: t('privacy.server.title'), body: `<p>${t('privacy.server.body')}</p>`, titleTag: 'h2' })}
${card({ kicker: k('privacy.site.kicker'), icon: 'globe', title: t('privacy.site.title'), body: `<p>${t('privacy.site.body')}</p>`, titleTag: 'h2' })}
${card({ kicker: k('privacy.questions.kicker'), icon: 'question', title: t('privacy.questions.title'), body: `<p>${t('privacy.questions.body', { protocol: ctx.href('protocol') })}</p>`, titleTag: 'h2' })}
</div>
</div>`;
    return page(ctx, { title: attr('privacy.meta.title'), description: attr('privacy.meta.description'), body });
  },
};
