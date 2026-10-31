import { CODE, LINKS, beat, button, cmd, more, night, opening, page, partyRoom, shot, value } from '../layout.mjs';

export default {
  id: 'home',
  slug: '',
  render(ctx) {
    const { t, attr } = ctx;
    const body = `<div class="wrap">
${opening(ctx, {
  title: t('home.title'),
  lead: t('home.lead'),
  actions: button(ctx.href('install'), t('home.install'), 'cream') + button(LINKS.github, t('home.github')),
  aside: `<p class="state">${t('home.state', { roadmap: ctx.href('roadmap') })}</p>`,
  mod: 'home',
})}
${shot(ctx, 'player', { sizes: '(min-width: 1344px) 1248px, 100vw', eager: true, mod: 'hero' })}
${night([
  beat({
    id: 'install',
    time: '18:30',
    datetime: '18:30',
    title: t('home.install18.title'),
    body: `<p>${t('home.install18.body')}</p>${cmd(ctx, CODE.script)}<p class="beat__note">${t('home.install18.note')}</p>${more(ctx.href('install'), t('home.install18.more'))}`,
    layout: 'text',
  }),
  beat({
    id: 'account',
    time: '18:41',
    datetime: '18:41',
    title: t('home.account.title'),
    body: `<p>${t('home.account.body')}</p>`,
    aside: shot(ctx, 'account', { sizes: '(min-width: 1100px) 40vw, 100vw' }),
    layout: 'split',
  }),
  beat({
    id: 'link',
    time: '20:47',
    datetime: '20:47',
    title: t('home.link.title'),
    body: `<p>${t('home.link.body')}</p><p class="addr">${value(CODE.short)}</p><p>${t('home.link.never')}</p>${more(ctx.href('remote'), t('home.link.more'))}`,
    layout: 'right',
  }),
  beat({
    id: 'tv',
    time: '20:55',
    datetime: '20:55',
    title: t('home.tv.title'),
    body: `<p>${t('home.tv.body')}</p>`,
    // The only Apple TV capture is in French: shown on the French page only.
    aside: ctx.locale === 'fr' ? shot(ctx, 'appletv', { sizes: '(min-width: 1100px) 50vw, 100vw' }) : '',
    layout: ctx.locale === 'fr' ? 'flip' : 'text',
  }),
  beat({
    id: 'party',
    time: '21:02',
    datetime: '21:02',
    title: t('home.party.title'),
    body: `<p>${t('home.party.body')}</p>${more(ctx.href('party'), t('home.party.more'))}`,
    aside: partyRoom(ctx),
    layout: 'wide',
  }),
  beat({
    id: 'credits',
    time: '23:58',
    datetime: '23:58',
    title: t('home.credits.title'),
    body: `<p>${t('home.credits.body')}</p><div class="actions">${button(LINKS.github, t('home.credits.star'))}${more(ctx.href('openSource'), t('home.credits.more'))}</div>`,
    layout: 'big',
  }),
])}
</div>`;
    return page(ctx, { title: attr('home.meta.title'), description: attr('home.meta.description'), body });
  },
};
