// Site-wide constants. Not user-facing text: every visible string lives in locales/*.json.

export const ORIGIN = 'https://getplaid.tv';

export const LOCALES = ['en', 'fr'];
export const DEFAULT_LOCALE = 'en';

const REPO = 'https://github.com/gh-prim/plaid';

export const LINKS = {
  github: REPO,
  releases: `${REPO}/releases/latest`,
  issues: `${REPO}/issues`,
  contributing: `${REPO}/blob/main/sdlc/README.md`,
  installGuide: `${REPO}/blob/main/docs/install.md`,
  remoteGuide: `${REPO}/blob/main/docs/install.md#remote-access`,
  proxyGuide: `${REPO}/blob/main/docs/install.md#behind-a-reverse-proxy`,
  permissionsGuide: `${REPO}/blob/main/docs/install.md#letting-plaid-read-your-media-folders`,
  hwGuide: `${REPO}/blob/main/docs/install.md#hardware-transcoding`,
  roadmap: `${REPO}/blob/main/sdlc/roadmap.md`,
  specs: `${REPO}/tree/main/sdlc/features`,
  partyIntent: `${REPO}/blob/main/sdlc/features/F010-plaid-party/intent.md`,
  openapi: `${REPO}/blob/main/api/openapi.yaml`,
  protocol: `${REPO}/blob/main/services/direct/PROTOCOL.md`,
  license: `${REPO}/blob/main/LICENSE`,
  installScript: `${REPO}/blob/main/packaging/install.sh`,
};

// Commands and paths shown on the site. Code is never translated.
export const CODE = {
  script: 'curl -fsSL https://raw.githubusercontent.com/gh-prim/plaid/main/packaging/install.sh | sudo bash',
  debVerify: 'sha256sum -c --ignore-missing checksums.txt',
  debInstall: 'sudo apt install ./plaid_<version>_amd64.deb',
  dockerClone: 'git clone https://github.com/gh-prim/plaid.git && cd plaid',
  dockerUp: 'docker compose up -d',
  status: 'systemctl status plaid',
  logs: 'journalctl -u plaid -f',
  host: 'living-room-k3x7qa.getplaid.direct',
  short: 'https://getplaid.direct/k3x7qa',
};

// URL prefix of a locale: the default locale lives at the root.
export function localePrefix(locale) {
  return locale === DEFAULT_LOCALE ? '/' : `/${locale}/`;
}
