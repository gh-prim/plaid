import { ambient, btn, page, pageHero } from '../layout.mjs';

export default {
  id: 'notFound',
  slug: '404.html',
  noindex: true,
  render(ctx) {
    const { t, attr } = ctx;
    const body = `<div class="hero-zone hero-zone--page hero-zone--lost">
${ambient(['still-elephants-dream'], 'ambient--short')}
${pageHero(ctx, { eyebrow: t('notFound.eyebrow'), title: t('notFound.title'), lead: t('notFound.body') })}
<div class="actions actions--center hero-in">${btn(ctx.href('home'), t('notFound.home'), 'cream', 'lg')}</div>
</div>`;
    return page(ctx, { title: attr('notFound.meta.title'), description: attr('notFound.meta.description'), body });
  },
};
