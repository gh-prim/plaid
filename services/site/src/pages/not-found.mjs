import { button, page } from '../layout.mjs';

export default {
  id: 'notFound',
  slug: '404.html',
  noindex: true,
  render(ctx) {
    const { t, attr } = ctx;
    const body = `<div class="wrap lost">
<p class="lost__time"><time datetime="03:12">03:12</time></p>
<div class="lost__text"><h1 class="opening__title">${t('notFound.title')}</h1><p class="opening__lead">${t('notFound.body')}</p><div class="actions">${button(ctx.href('home'), t('notFound.home'), 'cream')}</div></div>
</div>`;
    return page(ctx, { title: attr('notFound.meta.title'), description: attr('notFound.meta.description'), body });
  },
};
