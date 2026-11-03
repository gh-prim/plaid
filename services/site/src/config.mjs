// Site-wide constants. Not user-facing text: every visible string lives in locales/*.json.

export const ORIGIN = 'https://getplaid.tv';

export const LOCALES = ['en', 'fr'];
export const DEFAULT_LOCALE = 'en';

// The wordmark is the brand, never translated.
export const BRAND = 'plaid';

const REPO = 'https://github.com/gh-prim/plaid';

export const LINKS = {
  github: REPO,
  releases: `${REPO}/releases`,
  latestRelease: `${REPO}/releases/latest`,
  issues: `${REPO}/issues`,
  newIssue: `${REPO}/issues/new`,
  contributing: `${REPO}/blob/main/sdlc/README.md`,
  installGuide: `${REPO}/blob/main/docs/install.md`,
  features: `${REPO}/tree/main/sdlc/features`,
  openapi: `${REPO}/blob/main/api/openapi.yaml`,
  protocol: `${REPO}/blob/main/services/direct/PROTOCOL.md`,
  license: `${REPO}/blob/main/LICENSE`,
  installScript: `${REPO}/blob/main/packaging/install.sh`,
  // The public TestFlight beta of the iPhone, iPad and Apple TV apps.
  testflight: 'https://testflight.apple.com/join/bs3AVwrQ',
};

// Commands, paths and addresses shown on the site. Code is never translated.
export const CODE = {
  script: 'curl -fsSL https://raw.githubusercontent.com/gh-prim/plaid/main/packaging/install.sh | sudo bash',
  debVerify: 'sha256sum -c --ignore-missing checksums.txt',
  debInstall: 'sudo apt install ./plaid_<version>_amd64.deb',
  dockerClone: 'git clone https://github.com/gh-prim/plaid.git && cd plaid',
  dockerUp: 'docker compose up -d',
  status: 'systemctl status plaid',
  logs: 'journalctl -u plaid -f',
  serverUrl: 'http://<your-server>:32500',
  lanHost: 'living-room.local',
  short: 'getplaid.direct/k3x7qa',
  longHost: 'living-room-k3x7qa.getplaid.direct:32501',
  apiBase: 'https://getplaid.direct/api/v1',
};

// URL prefix of a locale: the default locale lives at the root.
export function localePrefix(locale) {
  return locale === DEFAULT_LOCALE ? '/' : `/${locale}/`;
}
