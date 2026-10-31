import { CODE, LINKS, beat, cmd, more, night, opening, page, shot, value } from '../layout.mjs';

export default {
  id: 'install',
  slug: 'install/',
  render(ctx) {
    const { t, attr } = ctx;
    const panels = [
      {
        id: 'script',
        body: `<p>${t('install.tabs.script.body')}</p>${cmd(ctx, CODE.script)}<p class="beat__note">${t('install.tabs.script.note', { script: LINKS.installScript })}</p>`,
      },
      {
        id: 'deb',
        body: `<ol class="steps"><li>${t('install.tabs.deb.step1', { releases: LINKS.releases })}</li><li>${t('install.tabs.deb.step2')}${cmd(ctx, CODE.debVerify)}</li><li>${t('install.tabs.deb.step3')}${cmd(ctx, CODE.debInstall)}</li></ol>
<table class="table"><caption>${t('install.tabs.deb.get.caption')}</caption><tbody>${['bin', 'env', 'data', 'service']
          .map((k) => `<tr><th scope="row">${t(`install.tabs.deb.get.${k}.path`)}</th><td>${t(`install.tabs.deb.get.${k}.what`)}</td></tr>`)
          .join('')}</tbody></table>`,
      },
      {
        id: 'docker',
        body: `${cmd(ctx, CODE.dockerClone)}${cmd(ctx, CODE.dockerUp)}<p>${t('install.tabs.docker.body')}</p>`,
      },
    ];
    const tabs = `<div class="tabs" data-tabs><div class="tabs__list pl-seg" role="tablist" aria-label="${attr('install.tabs.label')}" hidden>${panels
      .map(
        (p, i) =>
          `<button type="button" class="pl-seg__item pl-focusable${i === 0 ? ' is-selected' : ''}" role="tab" id="tab-${p.id}" aria-controls="panel-${p.id}" aria-selected="${i === 0}"${i ? ' tabindex="-1"' : ''}>${t(`install.tabs.${p.id}.name`)}</button>`,
      )
      .join('')}</div>${panels
      .map(
        (p) =>
          `<section class="tabs__panel" id="panel-${p.id}" role="tabpanel" aria-labelledby="tab-${p.id}"><h3 class="tabs__heading">${t(`install.tabs.${p.id}.name`)}</h3>${p.body}</section>`,
      )
      .join('')}</div>`;

    const reqs = ['systems', 'arch', 'ffmpeg']
      .map((k) => `<tr><th scope="row">${t(`install.req.${k}.name`)}</th><td>${t(`install.req.${k}.value`)}</td></tr>`)
      .join('');

    const body = `<div class="wrap">
${opening(ctx, { title: t('install.title'), lead: t('install.lead', { url: ctx.raw('common.serverUrl') }) })}
${night([
  beat({ id: 'pick', time: '18:30', datetime: '18:30', title: t('install.pick.title'), body: tabs, layout: 'text' }),
  beat({
    id: 'open',
    time: '18:36',
    datetime: '18:36',
    title: t('install.open.title'),
    body: `<p class="addr">${value(ctx.raw('common.serverUrl'))}</p><p>${t('install.open.body')}</p><p class="beat__note">${t('install.open.note')}</p>`,
    aside: shot(ctx, 'account', { sizes: '(min-width: 1100px) 40vw, 100vw' }),
    layout: 'split',
  }),
  beat({
    id: 'folders',
    time: '18:44',
    datetime: '18:44',
    title: t('install.folders.title'),
    body: `<p>${t('install.folders.body')}</p><p class="beat__note">${t('install.folders.note', { guide: LINKS.permissionsGuide })}</p>`,
    aside: shot(ctx, 'scan', { sizes: '(min-width: 1100px) 50vw, 100vw' }),
    layout: 'flip',
  }),
  beat({
    id: 'needs',
    time: t('install.req.time'),
    title: t('install.req.title'),
    body: `<table class="table"><tbody>${reqs}</tbody></table>`,
    layout: 'text',
  }),
  beat({
    id: 'everyday',
    time: t('install.everyday.time'),
    title: t('install.everyday.title'),
    body: `<p>${t('install.everyday.body')}</p>${cmd(ctx, CODE.status)}${cmd(ctx, CODE.logs)}${more(LINKS.installGuide, t('install.everyday.guide'))}`,
    layout: 'text',
  }),
])}
</div>`;
    return page(ctx, { title: attr('install.meta.title'), description: attr('install.meta.description'), body });
  },
};
