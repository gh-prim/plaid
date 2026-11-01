import { CODE, LINKS, ambient, btn, card, ctaRow, eyebrow, page, pageHero } from '../layout.mjs';
import { escapeHtml } from '../i18n.mjs';

const ZONE = '.getplaid.direct';

// [name, type, key]: name parts before the zone, in code.
const NAMES = [
  ['<slug>-<id>', 'A / AAAA', 'public'],
  ['<a-b-c-d>.<slug>-<id>', 'A', 'lan'],
  ['_acme-challenge.<slug>-<id>', 'TXT', 'acme'],
];

const CALLS = [
  ['POST', '/servers', 'register'],
  ['PUT', '/servers/{id}', 'update'],
  ['DELETE', '/servers/{id}', 'unregister'],
  ['PUT', '/servers/{id}/acme-challenge', 'acmeSet'],
  ['DELETE', '/servers/{id}/acme-challenge', 'acmeClear'],
  ['POST', '/servers/{id}/reachability', 'reach'],
  ['GET', '/ip', 'ip'],
];

const RECORDS = [
  ['getplaid.direct', 'SOA, NS', 'zone'],
  ['getplaid.direct', 'CAA', 'caa'],
  ['<label>', 'A / AAAA', 'label'],
  ['<a-b-c-d>.<label>', 'A', 'lan'],
  ['_acme-challenge.<label>', 'TXT', 'acme'],
];

// The register answer, as in PROTOCOL.md. JSON is code: never translated,
// except the token placeholder, which is prose.
function json(ctx) {
  const k = (s) => `<span class="j-k">"${s}"</span>`;
  const v = (s) => `<span class="j-s">"${escapeHtml(s)}"</span>`;
  const lines = [
    `${k('id')}: ${v('k3x7qa')}`,
    `${k('token')}: ${v(ctx.raw('protocol.api.token'))}`,
    `${k('hostname')}: ${v('living-room-k3x7qa.getplaid.direct')}`,
    `${k('url')}: ${v('https://living-room-k3x7qa.getplaid.direct:32501')}`,
    `${k('shortUrl')}: ${v('https://getplaid.direct/k3x7qa')}`,
    `${k('publicIp')}: ${v('203.0.113.7')}`,
    `${k('port')}: <span class="j-n">32501</span>`,
  ];
  return `{\n  ${lines.join(',\n  ')}\n}`;
}

export default {
  id: 'protocol',
  slug: 'protocol/',
  render(ctx) {
    const { t, attr } = ctx;

    const table = (rows, answerKey, prefix, zone) =>
      `<div class="dtable rv" role="table" aria-label="${attr(`${prefix}.tableLabel`)}"><div class="dtable__row dtable__row--head" role="row"><span role="columnheader">${t('protocol.cols.name')}</span><span role="columnheader">${t('protocol.cols.type')}</span><span role="columnheader">${t(answerKey)}</span></div>${rows
        .map(
          ([name, type, key]) =>
            `<div class="dtable__row" role="row"><code role="cell">${escapeHtml(name)}${zone ? `<span class="dim">${ZONE}</span>` : ''}</code><code role="cell">${type}</code><span role="cell">${t(`${prefix}.rows.${key}`)}</span></div>`,
        )
        .join('')}</div>`;

    const section = (key, inner, lead = true) =>
      `<section class="psec" aria-labelledby="p-${key}">${eyebrow(t(`protocol.${key}.eyebrow`), 'rv')}<h2 class="h2 h2--md" id="p-${key}" data-words>${t(`protocol.${key}.title`)}</h2>${lead ? `<p class="lead lead--sm rv">${t(`protocol.${key}.body`, { base: CODE.apiBase })}</p>` : ''}<div class="psec__in">${inner}</div></section>`;

    const calls = `<div class="api"><ul class="calls">${CALLS.map(
      ([m, path, key]) =>
        `<li class="call hl rv"><span class="verb verb--${m.toLowerCase()}">${m}</span><div><code class="call__path">${escapeHtml(path)}</code><p>${t(`protocol.api.calls.${key}`)}</p></div></li>`,
    ).join('')}</ul><div class="api__sample rv"><p class="api__caption">${t('protocol.api.sample')}</p><pre class="json"><code>${json(ctx)}</code></pre></div></div>`;

    const links = `<div class="hop rv"><div class="hop__end hl"><p class="hop__k">${t('protocol.links.from')}</p><p class="hop__v"><code>${CODE.short}</code></p></div><span class="hop__arrow" aria-hidden="true">302 →</span><div class="hop__end hop__end--wide hl"><p class="hop__k">${t('protocol.links.to')}</p><p class="hop__v"><code>${CODE.longHost}</code></p></div></div><p class="psec__note rv">${t('protocol.links.note')}</p>`;

    const rules = [
      ['source', 'server'],
      ['families', 'bars'],
      ['token', 'lock'],
      ['ttl', 'clock'],
      ['keys', 'shield'],
      ['probe', 'home'],
    ]
      .map(([k, ic]) => card({ icon: ic, title: t(`protocol.rules.${k}.title`), body: `<p>${t(`protocol.rules.${k}.body`)}</p>`, cls: 'card--rule' }))
      .join('');

    const body = `<div class="hero-zone hero-zone--page">
${ambient(['still-tears-of-steel'], 'ambient--shorter')}
${pageHero(ctx, { eyebrow: t('protocol.eyebrow'), title: t('protocol.title'), lead: t('protocol.lead') })}
</div>
<div class="wrap protocol">
${section('names', table(NAMES, 'protocol.cols.points', 'protocol.names', true))}
${section('api', calls)}
${section('links', links, false)}
${section('dns', table(RECORDS, 'protocol.cols.answer', 'protocol.dns', false))}
${section('rules', `<div class="grid3 grid3--rules">${rules}</div>`, false)}
${ctaRow({ icon: 'doc', title: t('protocol.read.title'), body: t('protocol.read.body'), actions: btn(LINKS.protocol, t('protocol.read.action'), 'cream', 'md') })}
</div>`;
    return page(ctx, { title: attr('protocol.meta.title'), description: attr('protocol.meta.description'), body });
  },
};
