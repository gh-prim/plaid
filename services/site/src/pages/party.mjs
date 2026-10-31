import { LINKS, beat, button, night, opening, page, partyRoom, tag } from '../layout.mjs';

export default {
  id: 'party',
  slug: 'party/',
  render(ctx) {
    const { t, attr } = ctx;
    const social = ['reactions', 'chat', 'pause', 'control']
      .map((k) => `<div class="facts__row"><dt>${t(`party.social.${k}.name`)}</dt><dd>${t(`party.social.${k}.text`)}</dd></div>`)
      .join('');
    const body = `<div class="wrap">
${opening(ctx, {
  kicker: `<p class="opening__kicker">${tag(ctx, 'planned')}<span>${t('party.kicker', { intent: LINKS.partyIntent })}</span></p>`,
  title: t('party.title'),
  lead: t('party.lead'),
  actions: button(LINKS.github, t('party.follow')),
})}
${night([
  beat({ id: 'start', time: '21:00', datetime: '21:00', title: t('party.start.title'), body: `<p>${t('party.start.body')}</p>`, layout: 'text' }),
  beat({
    id: 'joined',
    time: '21:02',
    datetime: '21:02',
    title: t('party.joined.title'),
    body: `<p>${t('party.joined.body')}</p>`,
    aside: partyRoom(ctx),
    layout: 'wide',
  }),
  beat({ id: 'play', time: '21:03', datetime: '21:03', title: t('party.play.title'), body: `<p>${t('party.play.body')}</p>`, layout: 'right' }),
  beat({
    id: 'streams',
    time: '21:40',
    datetime: '21:40',
    title: t('party.streams.title'),
    body: `<p>${t('party.streams.body')}</p>`,
    layout: 'big',
  }),
  beat({ id: 'drift', time: '22:15', datetime: '22:15', title: t('party.drift.title'), body: `<p>${t('party.drift.body')}</p>`, layout: 'text' }),
  beat({
    id: 'pause',
    time: '1:12:48',
    datetime: 'PT1H12M48S',
    title: t('party.pause.title'),
    body: `<p>${t('party.pause.body')}</p><dl class="facts facts--plain">${social}</dl>`,
    aside: partyRoom(ctx, { toast: true }),
    layout: 'flip',
  }),
  beat({ id: 'call', time: '23:51', datetime: '23:51', title: t('party.call.title'), body: `<p>${t('party.call.body')}</p>`, layout: 'right' }),
])}
</div>`;
    return page(ctx, { title: attr('party.meta.title'), description: attr('party.meta.description'), body });
  },
};
