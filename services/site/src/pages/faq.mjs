import { LINKS, beat, more, night, opening, page } from '../layout.mjs';

const GROUPS = [
  { id: 'basics', qs: ['free', 'today', 'fork'] },
  { id: 'data', qs: ['account', 'sends', 'files'] },
  { id: 'hardware', qs: ['pi', 'docker'] },
  { id: 'friends', qs: ['outside', 'install'] },
];

export default {
  id: 'faq',
  slug: 'faq/',
  render(ctx) {
    const { t, attr } = ctx;
    const vars = {
      roadmap: ctx.href('roadmap'),
      remote: ctx.href('remote'),
      privacy: ctx.href('privacy'),
      proxy: LINKS.proxyGuide,
      hw: LINKS.hwGuide,
    };
    const body = `<div class="wrap">
${opening(ctx, { title: t('faq.title'), lead: t('faq.lead') })}
${night(
  GROUPS.map((g) =>
    beat({
      id: g.id,
      time: t(`faq.${g.id}.time`),
      title: t(`faq.${g.id}.title`),
      body: g.qs
        .map(
          (q) =>
            `<details class="qa" id="${q}"><summary class="qa__q pl-focusable">${t(`faq.${g.id}.${q}.q`)}</summary><div class="qa__a"><p>${t(`faq.${g.id}.${q}.a`, vars)}</p></div></details>`,
        )
        .join(''),
      layout: 'text',
    }),
  ).concat(
    beat({
      id: 'ask',
      time: t('faq.ask.time'),
      title: t('faq.ask.title'),
      body: more(LINKS.issues, t('faq.ask.link')),
      layout: 'text',
    }),
  ),
  'words',
)}
</div>`;
    return page(ctx, { title: attr('faq.meta.title'), description: attr('faq.meta.description'), body });
  },
};
