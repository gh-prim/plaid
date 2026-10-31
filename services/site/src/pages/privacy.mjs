import { LINKS } from '../config.mjs';
import { layout } from '../layout.mjs';

const STORED = ['id', 'slug', 'token', 'ip', 'port', 'version', 'dates', 'acme'];

// Date of the last meaningful change to this policy.
const UPDATED = new Date(Date.UTC(2026, 9, 31));

export default {
  id: 'privacy',
  slug: 'privacy/',
  render(ctx) {
    const { t, attr, locale } = ctx;
    const updated = new Intl.DateTimeFormat(locale, { dateStyle: 'long', timeZone: 'UTC' }).format(UPDATED);
    const rows = STORED.map(
      (k) => `<tr><th scope="row">${t(`privacy.direct.rows.${k}.what`)}</th><td>${t(`privacy.direct.rows.${k}.detail`)}</td><td>${t(`privacy.direct.rows.${k}.why`)}</td></tr>`,
    ).join('');

    const body = `
<header class="page-head">
  <div class="wrap narrow">
    <h1>${t('privacy.title')}</h1>
    <p class="lead">${t('privacy.lead')}</p>
  </div>
</header>

<div class="wrap narrow prose">
  <section aria-labelledby="pv-site">
    <h2 id="pv-site">${t('privacy.site.title')}</h2>
    <p>${t('privacy.site.body')}</p>
    <p>${t('privacy.site.theme')}</p>
    <p>${t('privacy.site.logs')}</p>
  </section>

  <section aria-labelledby="pv-direct">
    <h2 id="pv-direct">${t('privacy.direct.title')}</h2>
    <p>${t('privacy.direct.intro', { remote: ctx.href('remoteAccess') })}</p>
    <div class="table-wrap" tabindex="0" role="region" aria-labelledby="pv-direct">
      <table>
        <thead><tr><th scope="col">${t('privacy.direct.headWhat')}</th><th scope="col">${t('privacy.direct.headDetail')}</th><th scope="col">${t('privacy.direct.headWhy')}</th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
    <h3>${t('privacy.direct.notStoredTitle')}</h3>
    <p>${t('privacy.direct.notStored')}</p>
    <h3>${t('privacy.direct.retentionTitle')}</h3>
    <p>${t('privacy.direct.retention')}</p>
  </section>

  <section aria-labelledby="pv-server">
    <h2 id="pv-server">${t('privacy.server.title')}</h2>
    <p>${t('privacy.server.body')}</p>
  </section>

  <section aria-labelledby="pv-contact">
    <h2 id="pv-contact">${t('privacy.contact.title')}</h2>
    <p>${t('privacy.contact.body', { issues: LINKS.issues })}</p>
    <p class="muted">${t('privacy.updated', { date: updated })}</p>
  </section>
</div>`;
    return layout(ctx, {
      title: attr('privacy.meta.title'),
      description: attr('privacy.meta.description'),
      body,
    });
  },
};
