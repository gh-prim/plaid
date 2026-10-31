import { codeBlock, layout, status } from '../layout.mjs';

const STEPS = ['enable', 'register', 'router', 'connect'];
const PORT_OPTIONS = ['auto', 'manual', 'cgnat'];

const EXAMPLE = `living-room-k3x7qa.getplaid.direct
https://getplaid.direct/living-room-k3x7qa`;

export default {
  id: 'remoteAccess',
  slug: 'remote-access/',
  render(ctx) {
    const { t, attr } = ctx;
    const steps = STEPS.map(
      (s) => `<li><h3>${t(`remote.how.steps.${s}.title`)}</h3><p>${t(`remote.how.steps.${s}.body`)}</p></li>`,
    ).join('');
    const ports = PORT_OPTIONS.map(
      (p) => `<div class="option"><h3>${t(`remote.ports.${p}.title`)}</h3><p>${t(`remote.ports.${p}.body`)}</p></div>`,
    ).join('');

    const flowRow = (kind, from, label1, label2, to) => `
      <div class="flow-row flow-row--${kind}">
        <div class="node">${from}</div>
        <div class="link"><span>${label1}</span><span class="link-line" aria-hidden="true"></span><span>${label2}</span></div>
        <div class="node">${to}</div>
      </div>`;

    const body = `
<header class="page-head">
  <div class="wrap narrow">
    <p class="eyebrow">${status(ctx, 'inProgress')}</p>
    <h1>${t('remote.title')}</h1>
    <p class="lead">${t('remote.lead')}</p>
  </div>
</header>

<div class="wrap narrow prose">
  <section aria-labelledby="ra-what">
    <h2 id="ra-what">${t('remote.what.title')}</h2>
    <p>${t('remote.what.body')}</p>
    ${codeBlock(ctx, EXAMPLE, t('remote.what.caption'))}
    <p>${t('remote.what.https')}</p>
  </section>

  <section aria-labelledby="ra-how">
    <h2 id="ra-how">${t('remote.how.title')}</h2>
    <ol class="steps">${steps}</ol>
  </section>

  <section aria-labelledby="ra-privacy">
    <h2 id="ra-privacy">${t('remote.privacy.title')}</h2>
    <p>${t('remote.privacy.body')}</p>
    <figure class="flow">
      ${flowRow('dns', t('remote.diagram.device'), t('remote.diagram.question'), t('remote.diagram.answer'), t('remote.diagram.service'))}
      ${flowRow('media', t('remote.diagram.device'), t('remote.diagram.media'), t('remote.diagram.direct'), t('remote.diagram.server'))}
      <figcaption>${t('remote.diagram.caption')}</figcaption>
    </figure>
    <p>${t('remote.privacy.stored', { privacy: ctx.href('privacy') })}</p>
    <p>${t('remote.privacy.home')}</p>
  </section>

  <section aria-labelledby="ra-ports">
    <h2 id="ra-ports">${t('remote.ports.title')}</h2>
    <p>${t('remote.ports.intro')}</p>
    <div class="options">${ports}</div>
  </section>

  <section aria-labelledby="ra-off">
    <h2 id="ra-off">${t('remote.off.title')}</h2>
    <p>${t('remote.off.body')}</p>
    <p>${t('remote.off.optional')}</p>
  </section>
</div>`;
    return layout(ctx, {
      title: attr('remote.meta.title'),
      description: attr('remote.meta.description'),
      body,
    });
  },
};
