import { LINKS, beat, more, night, opening, page, tag } from '../layout.mjs';

// From sdlc/roadmap.md. "available": merged and in the releases.
const PHASES = [
  {
    id: 'tonight',
    items: [
      ['F001', 'available'],
      ['F003', 'available'],
      ['F004', 'available'],
      ['F005', 'available'],
      ['F006', 'available'],
      ['F007', 'available'],
      ['F008', 'available'],
      ['F011', 'available'],
      ['F012', 'available'],
      ['F002', 'progress'],
    ],
  },
  { id: 'next', items: [['F009', 'planned'], ['F010', 'planned'], ['ios', 'planned'], ['android', 'planned']] },
  { id: 'later', items: [['offline', 'later'], ['hub', 'later'], ['music', 'later'], ['live', 'later'], ['intros', 'later']] },
];

export default {
  id: 'roadmap',
  slug: 'roadmap/',
  render(ctx) {
    const { t, attr } = ctx;
    const body = `<div class="wrap">
${opening(ctx, { title: t('roadmap.title'), lead: t('roadmap.lead') })}
${night(
  PHASES.map((p) =>
    beat({
      id: p.id,
      time: t(`roadmap.${p.id}.time`),
      title: t(`roadmap.${p.id}.title`),
      body: `<p>${t(`roadmap.${p.id}.body`)}</p><ul class="ledger">${p.items
        .map(
          ([key, status]) =>
            `<li class="ledger__row">${/^F\d/.test(key) ? `<span class="ledger__id">${key}</span>` : '<span class="ledger__id"></span>'}<span class="ledger__name">${t(`roadmap.items.${key}`)}</span>${tag(ctx, status)}</li>`,
        )
        .join('')}</ul>`,
      layout: 'text',
    }),
  ).concat(
    beat({
      id: 'how',
      time: t('roadmap.how.time'),
      title: t('roadmap.how.title'),
      body: `<p>${t('roadmap.how.body')}</p>${more(LINKS.specs, t('roadmap.how.more'))}`,
      layout: 'text',
    }),
  ),
  'words',
)}
</div>`;
    return page(ctx, { title: attr('roadmap.meta.title'), description: attr('roadmap.meta.description'), body });
  },
};
