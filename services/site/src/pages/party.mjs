import { ambient, banner, card, page, pageHero } from '../layout.mjs';

const BEATS = [
  ['start', '21:00', '21:00'],
  ['join', '21:02', '21:02'],
  ['play', '21:03', '21:03'],
  ['streams', '21:40', '21:40'],
  ['drift', '22:15', '22:15'],
  ['pause', '1:12:48', ''],
];

const PEOPLE = ['sam', 'mia', 'leo', 'alex'];

export default {
  id: 'party',
  slug: 'party/',
  render(ctx) {
    const { t, attr } = ctx;

    const badge = `<p class="pill-badge hero-in"><i aria-hidden="true"></i>${t('party.badge')}</p>`;

    const night = BEATS.map(
      ([k, time, dt]) =>
        `<li class="beat hl rv">${dt ? `<time class="beat__time" datetime="${dt}">${time}</time>` : `<span class="beat__time">${time}</span>`}<div><h3 class="beat__title">${t(`party.night.${k}.title`)}</h3><p class="beat__body">${t(`party.night.${k}.body`)}</p></div></li>`,
    ).join('');

    const room = `<figure class="room rv"><div class="room__card" role="img" aria-label="${attr('party.room.alt')}"><div class="room__screen" aria-hidden="true"><img src="/img/still-nosferatu-800.jpg" width="800" height="450" alt="" loading="lazy" decoding="async"><b class="room__film">${t('films.nosferatu')}</b><span class="room__time">${t('party.room.time')}</span><span class="room__bar"><i></i></span></div><ul class="room__people" aria-hidden="true">${PEOPLE.map(
      (p, i) =>
        `<li class="person"><span class="person__avatar person__avatar--${i}">${t(`party.room.people.${p}.initial`)}</span><span class="person__who"><b>${t(`party.room.people.${p}.name`)}</b><span>${t(`party.room.people.${p}.device`)}</span></span><span class="person__q">${t(`party.room.people.${p}.quality`)}</span></li>`,
    ).join('')}</ul></div><figcaption class="room__cap">${t('party.room.caption')}</figcaption></figure>`;

    const around = [
      ['reactions', 'smile'],
      ['chat', 'chat'],
      ['requests', 'hand'],
      ['control', 'crown'],
    ]
      .map(([k, ic]) => card({ icon: ic, title: t(`party.around.${k}.title`), body: `<p>${t(`party.around.${k}.body`)}</p>` }))
      .join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-nosferatu'], 'ambient--short')}
${pageHero(ctx, { badge, title: t('party.title'), lead: t('party.lead') })}
</div>
<div class="wrap">
${banner(ctx, { img: 'still-nosferatu', alt: attr('party.bannerAlt'), credit: t('credits.nosferatu'), pos: 'nosferatu' })}
<div class="party">
<ol class="night" aria-label="${attr('party.night.label')}">${night}</ol>
${room}
</div>
<section class="around" aria-labelledby="around-title">
<h2 class="h2 h2--md" id="around-title" data-words>${t('party.around.title')}</h2>
<div class="grid4">${around}</div>
${card({ icon: 'micOff', title: t('party.around.notv1.title'), body: `<p>${t('party.around.notv1.body')}</p>`, cls: 'card--wide' })}
</section>
</div>`;
    return page(ctx, { title: attr('party.meta.title'), description: attr('party.meta.description'), body });
  },
};
