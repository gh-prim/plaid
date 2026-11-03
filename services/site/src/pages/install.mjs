import { CODE, LINKS, ambient, banner, btn, card, cmd, eyebrow, page, pageHero, posterParade, value } from '../layout.mjs';

// What the package installs. Paths are code: never translated.
const FILES = [
  ['bin', '/usr/bin/plaid'],
  ['env', '/etc/plaid/plaid.env'],
  ['data', '/var/lib/plaid'],
  ['service', 'plaid.service'],
];

export default {
  id: 'install',
  slug: 'install/',
  render(ctx) {
    const { t, attr, raw } = ctx;

    const seg = `<nav class="seg rv" aria-label="${attr('install.methods')}" data-seg>${['script', 'deb', 'docker']
      .map((k, i) => `<a class="seg__item${i === 0 ? ' is-on' : ''}" href="#${k}">${t(`install.${k}.name`)}</a>`)
      .join('')}</nav>`;

    const files = FILES.map(
      ([k, path]) => `<div class="kv__row"><code class="kv__k">${path}</code><span class="kv__v">${t(`install.deb.files.${k}`)}</span></div>`,
    )
      .join('');

    const reqs = ['systems', 'arch', 'needs']
      .map((k) => `<div class="kv__row"><span class="kv__k kv__k--label">${t(`install.req.${k}.name`)}</span><span class="kv__v kv__v--strong">${t(`install.req.${k}.value`)}</span></div>`)
      .join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-tears-of-steel-bridge'], 'ambient--short')}
${pageHero(ctx, { eyebrow: t('install.eyebrow'), title: t('install.title'), lead: t('install.lead') })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-tears-of-steel-bridge', alt: attr('install.bannerAlt'), credit: t('credits.tearsOfSteel'), pos: 'center' })}
<section class="methods" aria-label="${attr('install.methods')}">
${seg}
${card({ id: 'script', icon: 'terminal', title: t('install.script.name'), cls: 'card--anchor', body: `<p class="card__lead">${t('install.script.body', { script: LINKS.installScript })}</p>${cmd(ctx, CODE.script)}` })}
<div class="grid2">
${card({ id: 'deb', icon: 'box', title: t('install.deb.name'), cls: 'card--anchor', body: `<p>${t('install.deb.body', { releases: LINKS.latestRelease })}</p>${cmd(ctx, CODE.debVerify)}${cmd(ctx, CODE.debInstall)}<div class="kv kv--files">${files}</div>` })}
${card({ id: 'docker', icon: 'docker', title: t('install.docker.name'), cls: 'card--anchor', body: `<p>${t('install.docker.body')}</p>${cmd(ctx, CODE.dockerClone)}${cmd(ctx, CODE.dockerUp)}` })}
</div>
</section>
<section class="grid2 grid2--gap" aria-label="${attr('install.after')}">
${card({ kicker: eyebrow(t('install.open.kicker'), 'card__kicker'), icon: 'globe', title: t('install.open.title'), cls: 'card--big', body: `<p>${t('install.open.body')}</p>${value(raw('common.serverUrl'))}` })}
${card({ kicker: eyebrow(t('install.folders.kicker'), 'card__kicker'), icon: 'folder', title: t('install.folders.title'), cls: 'card--big', body: `<p>${t('install.folders.body')}</p>` })}
</section>
${card({ id: 'apps', kicker: eyebrow(t('install.apps.kicker'), 'card__kicker'), icon: 'phone', title: t('install.apps.title'), cls: 'card--wide card--apps', body: `<p>${t('install.apps.body')}</p><div class="actions">${btn(LINKS.testflight, t('install.apps.cta'), 'cream', 'md')}</div>` })}
<section class="grid2 grid2--gap" aria-label="${attr('install.ops')}">
${card({ icon: 'chip', title: t('install.req.title'), cls: 'card--big', body: `<div class="kv">${reqs}</div>` })}
${card({ icon: 'terminal', title: t('install.cmds.title'), cls: 'card--big', body: `<p class="card__small">${t('install.cmds.status')}</p>${cmd(ctx, CODE.status)}<p class="card__small">${t('install.cmds.logs')}</p>${cmd(ctx, CODE.logs)}` })}
</section>
</div>
${posterParade(ctx, 'install.parade')}`;
    return page(ctx, { title: attr('install.meta.title'), description: attr('install.meta.description'), body });
  },
};
