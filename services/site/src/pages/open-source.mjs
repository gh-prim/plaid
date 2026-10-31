import { LINKS, beat, button, night, opening, page } from '../layout.mjs';

export default {
  id: 'openSource',
  slug: 'open-source/',
  render(ctx) {
    const { t, attr } = ctx;
    const rules = ['plan', 'contract', 'words', 'review']
      .map((k) => `<li><h3>${t(`openSource.rules.${k}.name`)}</h3><p>${t(`openSource.rules.${k}.text`, { openapi: LINKS.openapi })}</p></li>`)
      .join('');
    const body = `<div class="wrap">
${opening(ctx, { title: t('openSource.title'), lead: t('openSource.lead', { license: LINKS.license }) })}
${night(
  [
    beat({ id: 'rules', time: t('openSource.rules.time'), title: t('openSource.rules.title'), body: `<ol class="rules">${rules}</ol>`, layout: 'text' }),
    beat({ id: 'stack', time: t('openSource.stack.time'), title: t('openSource.stack.title'), body: `<p class="stack">${t('openSource.stack.list')}</p>`, layout: 'text' }),
    beat({
      id: 'contribute',
      time: t('openSource.contribute.time'),
      title: t('openSource.contribute.title'),
      body: `<p>${t('openSource.contribute.body')}</p><div class="actions">${button(LINKS.issues, t('openSource.contribute.issue'))}${button(LINKS.contributing, t('openSource.contribute.guide'))}</div>`,
      layout: 'text',
    }),
  ],
  'words',
)}
</div>`;
    return page(ctx, { title: attr('openSource.meta.title'), description: attr('openSource.meta.description'), body });
  },
};
