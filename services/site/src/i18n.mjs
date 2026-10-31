// Locale loading and a tiny inline markup renderer for translated strings.
//
// Strings may use three inline forms, rendered after HTML escaping:
//   `code`   **strong**   [label](https://url)
// and {name} placeholders filled from the vars argument.

import { readFileSync } from 'node:fs';

export function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

export function loadLocale(file) {
  return JSON.parse(readFileSync(file, 'utf8'));
}

// Flattens {"a": {"b": "x"}} into {"a.b": "x"}.
export function flatten(obj, prefix = '', out = {}) {
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      flatten(value, path, out);
    } else {
      out[path] = value;
    }
  }
  return out;
}

function inline(escaped) {
  return escaped
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_, label, href) => {
      const external = /^https?:/.test(href);
      return external
        ? `<a href="${href}" rel="noopener">${label}</a>`
        : `<a href="${href}">${label}</a>`;
    });
}

// French typography: a narrow no-break space before ; : ! ? » and after «,
// so punctuation never wraps onto its own line.
function frenchSpacing(text) {
  return text.replace(/ ([;:!?»])/g, ' $1').replace(/« /g, '« ');
}

export function makeTranslator(messages, locale) {
  const flat = flatten(messages);
  const lookup = (key, vars = {}) => {
    const value = flat[key];
    if (typeof value !== 'string') {
      throw new Error(`Missing translation "${key}" in locale "${locale}"`);
    }
    const filled = value.replace(/\{(\w+)\}/g, (match, name) =>
      name in vars ? String(vars[name]) : match,
    );
    return locale === 'fr' ? frenchSpacing(filled) : filled;
  };
  return {
    // HTML-safe text with inline markup, for element content.
    t: (key, vars) => inline(escapeHtml(lookup(key, vars))),
    // Escaped plain text without markup, for attributes and <title>.
    attr: (key, vars) => escapeHtml(lookup(key, vars)),
    has: (key) => typeof flat[key] === 'string',
  };
}
