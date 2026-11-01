import { LINKS, ambient, banner, btn, card, ctaRow, page, pageHero, posterParade } from '../layout.mjs';

const STACK = ['Go', 'SQLite', 'FFmpeg', 'React', 'SwiftUI', 'Kotlin', 'Jetpack Compose'];

export default {
  id: 'openSource',
  slug: 'open-source/',
  render(ctx) {
    const { t, attr } = ctx;
    const how = [
      ['written', 'doc'],
      ['contract', 'braces'],
      ['words', 'translate'],
      ['review', 'merge'],
    ]
      .map(([k, ic], i) =>
        card({
          kicker: `<span class="card__num card__num--sm" aria-hidden="true">0${i + 1}</span>`,
          icon: ic,
          title: t(`openSource.how.${k}.title`),
          body: `<p>${t(`openSource.how.${k}.body`)}</p>`,
          cls: 'card--how',
        }),
      )
      .join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-sprite-fright'], 'ambient--short')}
${pageHero(ctx, { eyebrow: t('openSource.eyebrow'), title: t('openSource.title'), lead: t('openSource.lead') })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-sprite-fright', alt: attr('openSource.bannerAlt'), credit: t('credits.spriteFright'), pos: 'center' })}
<section class="block" aria-labelledby="how-title">
<h2 class="h2 h2--md" id="how-title" data-words>${t('openSource.how.title')}</h2>
<div class="grid2 grid2--how">${how}</div>
</section>
<section class="block block--tight" aria-labelledby="stack-title">
<h2 class="h2 h2--md" id="stack-title" data-words>${t('openSource.stack')}</h2>
<ul class="chips">${STACK.map((s) => `<li class="chip rv">${s}</li>`).join('')}</ul>
</section>
${ctaRow({ icon: 'heart', title: t('openSource.help.title'), body: t('openSource.help.body'), actions: btn(LINKS.newIssue, t('openSource.help.issue'), 'cream', 'md') + btn(LINKS.contributing, t('openSource.help.how'), 'glass', 'md') })}
</div>
${posterParade(ctx, 'openSource.parade')}`;
    return page(ctx, { title: attr('openSource.meta.title'), description: attr('openSource.meta.description'), body });
  },
};
