// Line icons (24×24 grid, 1.5 stroke) drawn with the cream-to-blue gradient
// defined once per page (#icg in layout.mjs). Decorative: always aria-hidden.

const PATHS = {
  scan: '<path d="M4 7V5a1 1 0 0 1 1-1h2M17 4h2a1 1 0 0 1 1 1v2M20 17v2a1 1 0 0 1-1 1h-2M7 20H5a1 1 0 0 1-1-1v-2"/><path d="M4 12h16"/>',
  image: '<rect x="4" y="3.5" width="16" height="17" rx="2"/><path d="M4 15l4.5-4 4 3.5 2.5-2 5 4.5"/><circle cx="15" cy="8" r="1.6"/>',
  search: '<circle cx="11" cy="11" r="6.5"/><path d="M20 20l-4.2-4.2"/>',
  play: '<circle cx="12" cy="12" r="9"/><path d="M10 8.5v7l6-3.5z"/>',
  swap: '<path d="M4 8h13l-3-3M20 16H7l3 3"/>',
  chip: '<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx="1"/><path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3"/>',
  subtitles: '<rect x="3" y="5.5" width="18" height="13" rx="3"/><path d="M10.2 10.3a2.4 2.4 0 1 0 0 3.4M17 10.3a2.4 2.4 0 1 0 0 3.4"/>',
  resume: '<path d="M3.5 12a8.5 8.5 0 1 0 2.5-6"/><path d="M3 4v4.5h4.5"/><path d="M10.5 9v6l4.5-3z"/>',
  globe: '<circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17M12 3.5c2.5 2.6 3.6 5.4 3.6 8.5s-1.1 5.9-3.6 8.5c-2.5-2.6-3.6-5.4-3.6-8.5S9.5 6.1 12 3.5z"/>',
  tv: '<rect x="3" y="5" width="18" height="12" rx="2"/><path d="M8.5 20.5h7"/>',
  phone: '<rect x="7" y="3" width="10" height="18" rx="2.5"/><path d="M11 18h2"/>',
  android: '<rect x="5" y="9" width="14" height="10" rx="2.5"/><path d="M8 9a4 4 0 0 1 8 0M8.5 5.5 7 4M15.5 5.5 17 4"/><circle cx="10" cy="13" r=".6" fill="currentColor"/><circle cx="14" cy="13" r=".6" fill="currentColor"/>',
  people: '<circle cx="9" cy="8.5" r="3"/><path d="M3.5 19c.6-3 2.8-4.8 5.5-4.8s4.9 1.8 5.5 4.8"/><circle cx="16.5" cy="9.5" r="2.4"/><path d="M15.5 14.4c2.5-.3 4.4 1.3 5 4.1"/>',
  pair: '<rect x="3" y="5" width="12" height="9" rx="1.5"/><path d="M6.5 17h5"/><rect x="17" y="8" width="4.5" height="9" rx="1.2"/><path d="M8 9.5h2"/>',
  link: '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>',
  share: '<path d="M12 15V4M8 8l4-4 4 4"/><path d="M5 12v6.5A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5V12"/>',
  party: '<path d="M4 20l4-12 8 8z"/><path d="M14 4c0 2 1 3 3 3M19 9c1 0 2 .5 2 2M12 2.5v1M20 4l-1 1"/>',
  smile: '<circle cx="12" cy="12" r="8.5"/><path d="M8.5 14c1 1.5 2.2 2.2 3.5 2.2s2.5-.7 3.5-2.2M9 9.5v.1M15 9.5v.1"/>',
  chat: '<path d="M4 6.5A2.5 2.5 0 0 1 6.5 4h11A2.5 2.5 0 0 1 20 6.5v7a2.5 2.5 0 0 1-2.5 2.5H10l-4.5 3.5V16A2.5 2.5 0 0 1 4 13.5z"/>',
  hand: '<path d="M8 13V5.5a1.5 1.5 0 0 1 3 0V11M11 10V4.5a1.5 1.5 0 0 1 3 0V11M14 10.5V6a1.5 1.5 0 0 1 3 0v7c0 4-2.5 7-6 7s-5.5-2-6.5-4.5L3 12a1.5 1.5 0 0 1 2.6-1.5L8 13"/>',
  crown: '<path d="M4 17l-1-9 5 4 4-6 4 6 5-4-1 9z"/><path d="M4 20h16"/>',
  micOff: '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21M3 3l18 18"/>',
  terminal: '<rect x="3" y="4.5" width="18" height="15" rx="2.5"/><path d="M7 10l3 2.5L7 15M12.5 15h4.5"/>',
  box: '<path d="M12 3l8.5 4.5v9L12 21l-8.5-4.5v-9z"/><path d="M3.5 7.5 12 12l8.5-4.5M12 12v9"/>',
  docker: '<path d="M3 13h15.5a3 3 0 0 0 3-3c-1.5 0-2.5.5-3 1.5"/><path d="M3 13c0 4 3.5 6.5 8 6.5 4.5 0 7-2.5 7.5-6.5"/><path d="M6 10h3v3H6zM9 10h3v3H9zM12 10h3v3h-3zM9 7h3v3H9z"/>',
  folder: '<path d="M3 6.5A1.5 1.5 0 0 1 4.5 5h4.2l2 2h8.8A1.5 1.5 0 0 1 21 8.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z"/>',
  router: '<rect x="3" y="13" width="18" height="6" rx="2"/><path d="M7 16h.1M10.5 16h.1M8 13l-2-6M16 13l2-6"/><path d="M15 6a4 4 0 0 1 6 0"/>',
  tag: '<path d="M3 12V4.5A1.5 1.5 0 0 1 4.5 3H12l9 9-9 9z"/><circle cx="7.5" cy="7.5" r="1.4"/>',
  shield: '<path d="M12 3l7.5 3v6c0 4.5-3.2 7.8-7.5 9-4.3-1.2-7.5-4.5-7.5-9V6z"/><path d="M9 12l2.2 2.2L15.5 10"/>',
  power: '<path d="M12 3v9"/><path d="M6.5 6.5a8 8 0 1 0 11 0"/>',
  home: '<path d="M4 11l8-6.5 8 6.5V20h-5v-5H9v5H4z"/>',
  gear: '<circle cx="12" cy="12" r="3"/><path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1"/>',
  check: '<circle cx="12" cy="12" r="8.5"/><path d="M8 12.3l2.7 2.7L16 9.5"/>',
  pencil: '<path d="M4 20l1-4L16 5l3 3L8 19z"/><path d="M14 7l3 3"/>',
  list: '<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="4.5" cy="6" r="1"/><circle cx="4.5" cy="12" r="1"/><circle cx="4.5" cy="18" r="1"/>',
  doc: '<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v4h4M9 12h6M9 16h6"/>',
  braces: '<path d="M8 4c-2 0-2.5 1-2.5 3v2c0 1.5-1 2.5-2.5 3 1.5.5 2.5 1.5 2.5 3v2c0 2 .5 3 2.5 3M16 4c2 0 2.5 1 2.5 3v2c0 1.5 1 2.5 2.5 3-1.5.5-2.5 1.5-2.5 3v2c0 2-.5 3-2.5 3"/>',
  translate: '<path d="M4 5h10M9 3v2M6 5c0 4 3 7 6 8M12 5c-.5 4-3.5 7-7 8"/><path d="M13 21l3.5-9 3.5 9M14.3 18h4.4"/>',
  merge: '<circle cx="8" cy="8" r="3"/><circle cx="16" cy="16" r="3"/><path d="M8 11v2a3 3 0 0 0 3 3h2M16 13v-2a3 3 0 0 0-3-3h-2"/>',
  heart: '<path d="M12 20s-7.5-4.5-7.5-10A4.5 4.5 0 0 1 12 7a4.5 4.5 0 0 1 7.5 3c0 5.5-7.5 10-7.5 10z"/>',
  archive: '<rect x="3" y="4" width="18" height="5" rx="1"/><path d="M5 9v10a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V9M10 13h4"/>',
  eyeOff: '<path d="M3 3l18 18M10.6 6.1A9.8 9.8 0 0 1 12 6c5 0 8.5 4 9.5 6-.5 1-1.6 2.6-3.2 3.9M6.4 7.6C4.6 8.9 3.2 10.7 2.5 12c1 2 4.5 6 9.5 6 1.4 0 2.7-.3 3.9-.8"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
  trash: '<path d="M4 7h16M9 7V4.5h6V7M6 7l1 13h10l1-13"/>',
  server: '<rect x="4" y="3.5" width="16" height="7" rx="1.8"/><rect x="4" y="13.5" width="16" height="7" rx="1.8"/><path d="M7.5 7h.1M7.5 17h.1"/>',
  question: '<circle cx="12" cy="12" r="8.5"/><path d="M9.6 9.5a2.5 2.5 0 1 1 3.4 2.3c-.6.3-1 .8-1 1.5v.5M12 16.5v.1"/>',
  sparkle: '<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/><path d="M19 16l.7 2 2 .7-2 .7-.7 2-.7-2-2-.7 2-.7z"/>',
  lock: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
  bars: '<path d="M5 18v-3M9.5 18v-6M14 18V9M18.5 18V6"/>',
  clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>',
  arrow: '<path d="M5 12h13m-5-6 6 6-6 6"/>',
  menu: '<path d="M4 8h16M4 16h16"/>',
  pause: '<path d="M9 6v12M15 6v12"/>',
};

// size: the rendered box in px. The gradient stroke and glow come from CSS (.ic).
export function icon(name, size = 36, cls = '') {
  const paths = PATHS[name];
  if (!paths) throw new Error(`Unknown icon "${name}"`);
  return `<span class="ic ic--${size}${cls ? ` ${cls}` : ''}" aria-hidden="true"><svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="url(#icg)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" focusable="false">${paths}</svg></span>`;
}

// A plain glyph in currentColor (UI controls, not decoration).
export function glyph(name, size = 18) {
  return `<svg class="glyph" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${PATHS[name]}</svg>`;
}

// The hidden <defs> every page carries once.
export const ICON_DEFS =
  '<svg class="defs" width="0" height="0" aria-hidden="true" focusable="false"><defs><linearGradient id="icg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ffffff"/><stop offset="1" stop-color="#b9d3ff"/></linearGradient></defs></svg>';
