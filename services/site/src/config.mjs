// Site-wide constants. Not user-facing text: every visible string lives in locales/*.json.

export const ORIGIN = 'https://getplaid.tv';

export const LOCALES = ['en', 'fr'];
export const DEFAULT_LOCALE = 'en';

export const LINKS = {
  github: 'https://github.com/gh-prim/plaid',
  releases: 'https://github.com/gh-prim/plaid/releases/latest',
  issues: 'https://github.com/gh-prim/plaid/issues',
  installGuide: 'https://github.com/gh-prim/plaid/blob/main/docs/install.md',
  license: 'https://github.com/gh-prim/plaid/blob/main/LICENSE',
  appleSource: 'https://github.com/gh-prim/plaid/tree/main/apple',
};

// URL prefix of a locale: the default locale lives at the root.
export function localePrefix(locale) {
  return locale === DEFAULT_LOCALE ? '/' : `/${locale}/`;
}
