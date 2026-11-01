// Fails when locale files differ in keys or {placeholders}, or contain empty strings.
// Also fails when a page template uses a key that the source locale lacks.

import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { DEFAULT_LOCALE, LOCALES } from '../src/config.mjs';
import { flatten, loadLocale } from '../src/i18n.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const flat = Object.fromEntries(
  LOCALES.map((l) => [l, flatten(loadLocale(join(root, 'locales', `${l}.json`)))]),
);
const errors = [];
const source = flat[DEFAULT_LOCALE];
const placeholders = (s) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',');

for (const locale of LOCALES) {
  for (const [key, value] of Object.entries(flat[locale])) {
    if (typeof value !== 'string' || value.trim() === '') {
      errors.push(`${locale}: "${key}" is empty or not a string`);
    }
  }
  if (locale === DEFAULT_LOCALE) continue;
  const target = flat[locale];
  for (const key of Object.keys(source)) {
    if (!(key in target)) errors.push(`${locale}: missing key "${key}"`);
    else if (placeholders(source[key]) !== placeholders(target[key])) {
      errors.push(`${locale}: placeholders differ for "${key}"`);
    }
  }
  for (const key of Object.keys(target)) {
    if (!(key in source)) errors.push(`${locale}: extra key "${key}" (not in ${DEFAULT_LOCALE})`);
  }
}

// Static keys used in templates: t('a.b') / attr('a.b'). Dynamic keys built
// from template strings are covered by the build, which throws on a miss.
const srcFiles = ['layout.mjs', ...readdirSync(join(root, 'src/pages')).map((f) => `pages/${f}`)];
for (const file of srcFiles) {
  const code = readFileSync(join(root, 'src', file), 'utf8');
  for (const m of code.matchAll(/\b(?:t|attr)\('([\w.]+)'/g)) {
    if (!(m[1] in source)) errors.push(`src/${file}: unknown key "${m[1]}"`);
  }
}

if (errors.length) {
  console.error(`i18n check failed:\n  ${errors.join('\n  ')}`);
  process.exit(1);
}
console.log(`i18n OK: ${Object.keys(source).length} keys in ${LOCALES.join(', ')}.`);
