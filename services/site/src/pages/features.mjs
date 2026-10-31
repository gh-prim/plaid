import { LINKS, beat, more, night, opening, page, shot, tag } from '../layout.mjs';

// Each group is one moment of the night; each line is a fact with its status.
const GROUPS = [
  { id: 'libraries', time: '19:10', shot: 'scan', items: [['scan', 'available'], ['match', 'available'], ['search', 'available']] },
  {
    id: 'playback',
    time: '21:00',
    items: [['direct', 'available'], ['transcode', 'available'], ['hw', 'available'], ['subs', 'available'], ['resume', 'available']],
  },
  { id: 'screens', time: '21:01', items: [['web', 'available'], ['appletv', 'available'], ['ios', 'planned'], ['android', 'planned']] },
  {
    id: 'people',
    time: '21:02',
    items: [['accounts', 'available'], ['pairing', 'available'], ['remote', 'available'], ['sharing', 'planned'], ['party', 'planned']],
  },
];

export default {
  id: 'features',
  slug: 'features/',
  render(ctx) {
    const { t, attr } = ctx;
    const list = (g) =>
      `<dl class="facts">${g.items
        .map(
          ([key, status]) =>
            `<div class="facts__row"><dt>${t(`features.${g.id}.items.${key}.name`)}</dt><dd>${t(`features.${g.id}.items.${key}.text`, { party: ctx.href('party'), remote: ctx.href('remote'), hw: LINKS.hwGuide })}</dd><dd class="facts__status">${tag(ctx, status)}</dd></div>`,
        )
        .join('')}</dl>`;
    const body = `<div class="wrap">
${opening(ctx, { title: t('features.title'), lead: t('features.lead') })}
${night(
  GROUPS.map((g) =>
    beat({
      id: g.id,
      time: g.time,
      datetime: g.time,
      title: t(`features.${g.id}.title`),
      body: `${list(g)}`,
      aside: g.shot ? shot(ctx, g.shot, { sizes: '(min-width: 1100px) 40vw, 100vw' }) : '',
      layout: g.shot ? 'split' : 'text',
    }),
  ).concat(
    beat({
      id: 'statuses',
      time: t('features.statuses.time'),
      title: t('features.statuses.title'),
      body: `<p>${t('features.statuses.body')}</p>${more(ctx.href('roadmap'), t('features.statuses.more'))}`,
      layout: 'text',
    }),
  ),
)}
</div>`;
    return page(ctx, { title: attr('features.meta.title'), description: attr('features.meta.description'), body });
  },
};
