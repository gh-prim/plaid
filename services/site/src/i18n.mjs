// Locale loading and a tiny inline markup renderer for translated strings.
//
// Strings may use three inline forms, rendered after HTML escaping:
//   `code`   **strong**   [label](url)
// and {name} placeholders filled from the vars argument (before the markup,
// so a placeholder can be a link target).

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
    .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_, label, href) =>
      /^https?:/.test(href) ? `<a href="${href}" rel="noopener">${label}</a>` : `<a href="${href}">${label}</a>`,
    );
}

// French typography: a narrow no-break space before ; : ! ? » and after «,
// so punctuation never wraps onto its own line. URLs and code are left alone.
function frenchSpacing(text) {
  return text
    .split(/(`[^`]*`|\]\([^)]*\)|https?:\/\/\S+)/)
    .map((part, i) => (i % 2 ? part : part.replace(/ ([;:!?»])/g, ' $1').replace(/« /g, '« ')))
    .join('');
}

export function makeTranslator(messages, locale) {
  const flat = flatten(messages);
  const lookup = (key, vars = {}) => {
    const value = flat[key];
    if (typeof value !== 'string') {
      throw new Error(`Missing translation "${key}" in locale "${locale}"`);
    }
    const spaced = locale === 'fr' ? frenchSpacing(value) : value;
    return spaced.replace(/\{(\w+)\}/g, (match, name) => (name in vars ? String(vars[name]) : match));
  };
  return {
    // HTML-safe text with inline markup, for element content.
    t: (key, vars) => inline(escapeHtml(lookup(key, vars))),
    // Escaped plain text without markup, for attributes and <title>.
    attr: (key, vars) => escapeHtml(lookup(key, vars)).replace(/`/g, ''),
    // Plain text, unescaped, for values that markup escapes later (code samples).
    raw: (key, vars) => lookup(key, vars),
    has: (key) => typeof flat[key] === 'string',
  };
}
