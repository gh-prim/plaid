import { layout } from '../layout.mjs';

// Rendered to 404.html (and fr/404.html); not listed in the sitemap.
export default {
  id: 'notFound',
  slug: '404.html',
  noindex: true,
  render(ctx) {
    const { t, attr } = ctx;
    const body = `
<header class="page-head page-head--center">
  <div class="wrap narrow center">
    <h1>${t('notFound.title')}</h1>
    <p class="lead">${t('notFound.body')}</p>
    <div class="actions actions--center">
      <a class="button button--primary" href="${ctx.href('home')}">${t('notFound.cta')}</a>
    </div>
  </div>
</header>`;
    return layout(ctx, {
      title: attr('notFound.meta.title'),
      description: attr('notFound.meta.description'),
      body,
    });
  },
};
