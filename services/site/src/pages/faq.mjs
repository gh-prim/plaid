import { LINKS, ambient, banner, btn, ctaRow, icon, page, pageHero } from '../layout.mjs';

const GROUPS = [
  ['basics', 'sparkle', ['free', 'today', 'fork']],
  ['data', 'lock', ['account', 'sends', 'files']],
  ['hardware', 'chip', ['pi', 'docker']],
  ['friends', 'people', ['reach', 'install']],
];

export default {
  id: 'faq',
  slug: 'faq/',
  render(ctx) {
    const { t, attr } = ctx;
    const groups = GROUPS.map(
      ([g, ic, qs]) =>
        `<section class="qgroup" aria-labelledby="q-${g}"><div class="qgroup__head rv">${icon(ic)}<h2 class="qgroup__title" id="q-${g}">${t(`faq.groups.${g}`)}</h2></div><dl class="qlist">${qs
          .map((q) => `<div class="qitem hl rv"><dt>${t(`faq.items.${q}.q`)}</dt><dd>${t(`faq.items.${q}.a`)}</dd></div>`)
          .join('')}</dl></section>`,
    ).join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-caminandes'], 'ambient--short')}
${pageHero(ctx, { eyebrow: t('faq.eyebrow'), title: t('faq.title'), lead: t('faq.lead') })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-caminandes', alt: attr('faq.bannerAlt'), credit: t('credits.caminandes'), pos: 'center' })}
<div class="qgroups">${groups}</div>
${ctaRow({ icon: 'question', title: t('faq.still.title'), actions: btn(LINKS.issues, t('faq.still.action'), 'cream', 'md') })}
</div>`;
    return page(ctx, { title: attr('faq.meta.title'), description: attr('faq.meta.description'), body });
  },
};
