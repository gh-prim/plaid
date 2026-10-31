import { LINKS, beat, night, opening, page } from '../layout.mjs';

export default {
  id: 'privacy',
  slug: 'privacy/',
  render(ctx) {
    const { t, attr } = ctx;
    const stored = ['id', 'slug', 'token', 'ip', 'port', 'version', 'seen', 'acme']
      .map((k) => `<li>${t(`privacy.stored.items.${k}`)}</li>`)
      .join('');
    const body = `<div class="wrap">
${opening(ctx, { title: t('privacy.title'), lead: t('privacy.lead') })}
${night(
  [
    beat({ id: 'stored', time: t('privacy.stored.time'), title: t('privacy.stored.title'), body: `<p>${t('privacy.stored.body')}</p><ul class="plain">${stored}</ul>`, layout: 'text' }),
    beat({ id: 'never', time: t('privacy.never.time'), title: t('privacy.never.title'), body: `<p>${t('privacy.never.body')}</p>`, layout: 'text' }),
    beat({ id: 'deleted', time: t('privacy.deleted.time'), title: t('privacy.deleted.title'), body: `<p>${t('privacy.deleted.body')}</p>`, layout: 'text' }),
    beat({ id: 'server', time: t('privacy.server.time'), title: t('privacy.server.title'), body: `<p>${t('privacy.server.body')}</p>`, layout: 'text' }),
    beat({ id: 'site', time: t('privacy.site.time'), title: t('privacy.site.title'), body: `<p>${t('privacy.site.body')}</p>`, layout: 'text' }),
    beat({ id: 'contact', time: t('privacy.contact.time'), title: t('privacy.contact.title'), body: `<p>${t('privacy.contact.body', { issues: LINKS.issues, protocol: LINKS.protocol })}</p>`, layout: 'text' }),
  ],
  'words',
)}
</div>`;
    return page(ctx, { title: attr('privacy.meta.title'), description: attr('privacy.meta.description'), body });
  },
};
