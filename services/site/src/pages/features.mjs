import { ambient, banner, icon, page, pageHero, shot, status } from '../layout.mjs';

// Each group lists [key, icon, status].
const GROUPS = [
  ['libraries', [['scan', 'scan', 'available'], ['match', 'image', 'available'], ['search', 'search', 'available']]],
  [
    'playback',
    [
      ['direct', 'play', 'available'],
      ['transcode', 'swap', 'available'],
      ['hardware', 'chip', 'available'],
      ['subtitles', 'subtitles', 'available'],
      ['resume', 'resume', 'available'],
    ],
  ],
  ['screens', [['web', 'globe', 'available'], ['appletv', 'tv', 'available'], ['ios', 'phone', 'available'], ['android', 'android', 'planned']]],
  [
    'people',
    [
      ['accounts', 'people', 'available'],
      ['pairing', 'pair', 'available'],
      ['remote', 'link', 'available'],
      ['sharing', 'share', 'available'],
      ['party', 'party', 'available'],
    ],
  ],
];

export default {
  id: 'features',
  slug: 'features/',
  render(ctx) {
    const { t, attr } = ctx;
    // Screenshots shown under a group, as designed.
    const SHOTS = {
      playback: [{ name: 'title', alt: attr('shots.title.alt'), caption: t('shots.title.cap') }],
      screens: [
        { name: 'ios', alt: attr('shots.ios.alt'), caption: t('shots.ios.cap') },
        { name: 'apple-tv', alt: attr('home.tv.alt'), caption: t('shots.appletv.cap') },
      ],
      people: [{ name: 'party', alt: attr('shots.party.alt'), caption: t('shots.party.cap') }],
    };
    const groups = GROUPS.map(
      ([g, items]) =>
        `<section class="fgroup" aria-labelledby="g-${g}"><h2 class="fgroup__title rv" id="g-${g}">${t(`features.groups.${g}`)}</h2><ul class="frows">${items
          .map(
            ([k, ic, st]) =>
              `<li class="frow hl rv">${icon(ic, 30)}<h3 class="frow__name">${t(`features.items.${k}.name`)}</h3><p class="frow__body">${t(`features.items.${k}.body`)}</p>${status(ctx, st)}</li>`,
          )
          .join('')}</ul>${(SHOTS[g] || []).map((s) => shot(ctx, { ...s, cls: 'shot--group' })).join('')}</section>`,
    ).join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-big-buck-bunny-close'], 'ambient--features')}
${pageHero(ctx, { eyebrow: t('features.eyebrow'), title: t('features.title'), lead: t('features.lead'), cls: 'phero--features' })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-big-buck-bunny', alt: attr('features.bannerAlt'), credit: t('credits.bigBuckBunny'), pos: 'bbb' })}
<div class="fgroups">${groups}</div>
<p class="legend rv">${t('features.legend')}</p>
</div>`;
    return page(ctx, { title: attr('features.meta.title'), description: attr('features.meta.description'), body });
  },
};
