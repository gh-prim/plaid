import { CODE, LINKS, beat, more, night, opening, page, shot, value } from '../layout.mjs';

export default {
  id: 'remote',
  slug: 'remote-access/',
  render(ctx) {
    const { t, attr } = ctx;
    // Who talks to whom: the only diagram on the site, in words.
    const paths = `<ul class="paths">${['video', 'dns', 'cert']
      .map(
        (k) =>
          `<li class="paths__row paths__row--${k}"><span class="paths__what">${t(`remote.paths.${k}.what`)}</span><span class="paths__route">${t(`remote.paths.${k}.route`)}</span></li>`,
      )
      .join('')}</ul>`;
    const body = `<div class="wrap">
${opening(ctx, { title: t('remote.title'), lead: t('remote.lead') })}
${night([
  beat({
    id: 'port',
    time: '20:40',
    datetime: '20:40',
    title: t('remote.port.title'),
    body: `<p>${t('remote.port.body')}</p><p class="beat__note">${t('remote.port.note')}</p>`,
    aside: shot(ctx, 'remote', { sizes: '(min-width: 1100px) 40vw, 100vw' }),
    layout: 'split',
  }),
  beat({
    id: 'name',
    time: '20:42',
    datetime: '20:42',
    title: t('remote.name.title'),
    body: `<p class="addr">${value(CODE.host)}</p><p>${t('remote.name.body')}</p>`,
    layout: 'right',
  }),
  beat({
    id: 'cert',
    time: '20:43',
    datetime: '20:43',
    title: t('remote.cert.title'),
    body: `<p>${t('remote.cert.body')}</p>`,
    layout: 'text',
  }),
  beat({
    id: 'send',
    time: '20:47',
    datetime: '20:47',
    title: t('remote.send.title'),
    body: `<p class="addr">${value(CODE.short)}</p><p>${t('remote.send.body')}</p>`,
    layout: 'big',
  }),
  beat({
    id: 'through',
    time: t('remote.through.time'),
    title: t('remote.through.title'),
    body: `<p>${t('remote.through.body')}</p>${paths}${more(ctx.href('privacy'), t('remote.through.more'))}`,
    layout: 'text',
  }),
  beat({
    id: 'off',
    time: '23:59',
    datetime: '23:59',
    title: t('remote.off.title'),
    body: `<p>${t('remote.off.body')}</p>`,
    layout: 'right',
  }),
  beat({
    id: 'limits',
    time: t('remote.limits.time'),
    title: t('remote.limits.title'),
    body: `<dl class="facts facts--plain">${['cgnat', 'docker', 'own']
      .map(
        (k) =>
          `<div class="facts__row"><dt>${t(`remote.limits.${k}.name`)}</dt><dd>${t(`remote.limits.${k}.text`, { proxy: LINKS.proxyGuide })}</dd></div>`,
      )
      .join('')}</dl>${more(LINKS.remoteGuide, t('remote.limits.guide'))}`,
    layout: 'text',
  }),
])}
</div>`;
    return page(ctx, { title: attr('remote.meta.title'), description: attr('remote.meta.description'), body });
  },
};
