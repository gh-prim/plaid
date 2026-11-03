import { LINKS, ambient, banner, btn, ctaRow, icon, page, pageHero, posterParade, status } from '../layout.mjs';

// [feature id or '', key, status]
const COLUMNS = [
  [
    'today',
    'check',
    [
      ['F001', 'f001', 'available'],
      ['F002', 'f002', 'progress'],
      ['F003', 'f003', 'available'],
      ['F004', 'f004', 'available'],
      ['F005', 'f005', 'available'],
      ['F006', 'f006', 'available'],
      ['F007', 'f007', 'available'],
      ['F008', 'f008', 'available'],
      ['F009', 'f009', 'available'],
      ['F010', 'f010', 'available'],
      ['F011', 'f011', 'available'],
      ['F012', 'f012', 'available'],
      ['', 'ios', 'available'],
    ],
  ],
  [
    'designed',
    'pencil',
    [
      ['', 'android', 'planned'],
    ],
  ],
  [
    'later',
    'list',
    [
      ['', 'offline', 'later'],
      ['', 'hub', 'later'],
      ['', 'media', 'later'],
      ['', 'intro', 'later'],
    ],
  ],
];

export default {
  id: 'roadmap',
  slug: 'roadmap/',
  render(ctx) {
    const { t, attr } = ctx;
    const cols = COLUMNS.map(
      ([col, ic, items]) =>
        `<section class="rcol card hl rv" aria-labelledby="r-${col}">${icon(ic)}<h2 class="card__title" id="r-${col}">${t(`roadmap.cols.${col}.title`)}</h2><p class="rcol__sub">${t(`roadmap.cols.${col}.sub`)}</p><ul class="ritems">${items
          .map(
            ([id, key, st]) =>
              `<li class="ritem"><span class="ritem__id">${id}</span><span class="ritem__name">${t(`roadmap.items.${key}`)}</span>${status(ctx, st)}</li>`,
          )
          .join('')}</ul></section>`,
    ).join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-elephants-dream'], 'ambient--short')}
${pageHero(ctx, { eyebrow: t('roadmap.eyebrow'), title: t('roadmap.title'), lead: t('roadmap.lead') })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-elephants-dream', alt: attr('roadmap.bannerAlt'), credit: t('credits.elephantsDream'), pos: 'ed' })}
<div class="rcols">${cols}</div>
${ctaRow({ icon: 'doc', title: t('roadmap.written.title'), body: t('roadmap.written.body'), actions: btn(LINKS.features, t('roadmap.written.action'), 'cream', 'md') })}
</div>
${posterParade(ctx, 'roadmap.parade')}`;
    return page(ctx, { title: attr('roadmap.meta.title'), description: attr('roadmap.meta.description'), body });
  },
};
