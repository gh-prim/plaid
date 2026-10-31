import { LINKS } from '../config.mjs';
import { codeBlock, layout, status } from '../layout.mjs';

const SCRIPT = 'curl -fsSL https://raw.githubusercontent.com/gh-prim/plaid/main/packaging/install.sh | sudo bash';

const DEB = `sha256sum -c --ignore-missing checksums.txt
sudo apt install ./plaid_<version>_arm64.deb`;

const COMPOSE_START = `git clone https://github.com/gh-prim/plaid.git && cd plaid
docker compose up -d`;

const COMPOSE = `services:
  plaid:
    build: .
    image: plaid:dev
    container_name: plaid
    restart: unless-stopped
    ports:
      - "32500:32500"
    volumes:
      - plaid-data:/data
      - /path/to/movies:/media/movies:ro
      - /path/to/shows:/media/shows:ro

volumes:
  plaid-data:`;

export default {
  id: 'download',
  slug: 'download/',
  render(ctx) {
    const { t, attr } = ctx;
    const body = `
<header class="page-head">
  <div class="wrap narrow">
    <h1>${t('download.title')}</h1>
    <p class="lead">${t('download.lead')}</p>
  </div>
</header>

<div class="wrap narrow prose">
  <section aria-labelledby="dl-script">
    <h2 id="dl-script">${t('download.script.title')}</h2>
    <p>${t('download.script.body')}</p>
    ${codeBlock(ctx, SCRIPT, t('download.script.caption'))}
  </section>

  <section aria-labelledby="dl-deb">
    <h2 id="dl-deb">${t('download.deb.title')}</h2>
    <p>${t('download.deb.body')}</p>
    <ul class="arch">
      <li><strong>amd64</strong> ${t('download.deb.amd64')}</li>
      <li><strong>arm64</strong> ${t('download.deb.arm64')}</li>
    </ul>
    <p class="actions"><a class="button button--primary" href="${LINKS.releases}" rel="noopener">${t('download.deb.cta')}</a></p>
    ${codeBlock(ctx, DEB, t('download.deb.caption'))}
    <p>${t('download.deb.after')}</p>
  </section>

  <section aria-labelledby="dl-docker">
    <h2 id="dl-docker">${t('download.docker.title')}</h2>
    <p>${t('download.docker.body')}</p>
    ${codeBlock(ctx, COMPOSE_START, t('download.docker.startCaption'))}
    <p>${t('download.docker.compose')}</p>
    ${codeBlock(ctx, COMPOSE, 'docker-compose.yml')}
    <p>${t('download.docker.paths')}</p>
  </section>

  <section aria-labelledby="dl-apps">
    <h2 id="dl-apps">${t('download.apps.title')}</h2>
    <div class="app-row">
      <h3>${t('download.apps.appleTv.title')}</h3>${status(ctx, 'available')}
    </div>
    <p>${t('download.apps.appleTv.body', { source: LINKS.appleSource })}</p>
    <div class="app-row">
      <h3>${t('download.apps.web.title')}</h3>${status(ctx, 'available')}
    </div>
    <p>${t('download.apps.web.body')}</p>
    <div class="app-row">
      <h3>${t('download.apps.mobile.title')}</h3>${status(ctx, 'planned')}
    </div>
    <p>${t('download.apps.mobile.body')}</p>
  </section>

  <section aria-labelledby="dl-next">
    <h2 id="dl-next">${t('download.next.title')}</h2>
    <p>${t('download.next.body', { guide: LINKS.installGuide, remote: ctx.href('remoteAccess') })}</p>
  </section>
</div>`;
    return layout(ctx, {
      title: attr('download.meta.title'),
      description: attr('download.meta.description'),
      body,
    });
  },
};
