/** What a key does in the player (F008-R42). */
export type Shortcut =
  | { type: 'toggle' }
  | { type: 'seek'; seconds: number }
  | { type: 'volume'; by: number }
  | { type: 'mute' }
  | { type: 'fullscreen' }
  | { type: 'subtitles' }
  | { type: 'next' }
  | { type: 'escape' }
  | { type: 'jump'; fraction: number }
  | { type: 'start' }
  | { type: 'end' }
  | { type: 'help' }

type Key = Pick<KeyboardEvent, 'key' | 'shiftKey' | 'ctrlKey' | 'metaKey' | 'altKey'>

/** The shortcut of a key press, or null to let the browser have it. */
export function shortcutFor(e: Key): Shortcut | null {
  if (e.ctrlKey || e.metaKey || e.altKey) return null
  const key = e.key.length === 1 ? e.key.toLowerCase() : e.key
  if (e.shiftKey && key === 'n') return { type: 'next' }
  if (/^[0-9]$/.test(key)) return { type: 'jump', fraction: Number(key) / 10 }
  switch (key) {
    case ' ':
    case 'k':
      return { type: 'toggle' }
    case 'ArrowLeft':
    case 'j':
      return { type: 'seek', seconds: -10 }
    case 'ArrowRight':
    case 'l':
      return { type: 'seek', seconds: 10 }
    case 'ArrowUp':
      return { type: 'volume', by: 0.1 }
    case 'ArrowDown':
      return { type: 'volume', by: -0.1 }
    case 'm':
      return { type: 'mute' }
    case 'f':
      return { type: 'fullscreen' }
    case 'c':
      return { type: 'subtitles' }
    case 'Escape':
      return { type: 'escape' }
    case 'Home':
      return { type: 'start' }
    case 'End':
      return { type: 'end' }
    case '?':
      return { type: 'help' }
  }
  return null
}

/** The shortcuts dialog's lines: keys as the platform names them. */
export const shortcutList = [
  { keys: ['Space', 'K'], label: 'playPause' },
  { keys: ['←', '→', 'J', 'L'], label: 'seek' },
  { keys: ['↑', '↓'], label: 'volume' },
  { keys: ['M'], label: 'mute' },
  { keys: ['F'], label: 'fullscreen' },
  { keys: ['C'], label: 'subtitles' },
  { keys: ['Shift', 'N'], label: 'next' },
  { keys: ['0', '…', '9'], label: 'jump' },
  { keys: ['Home', 'End'], label: 'startEnd' },
  { keys: ['Esc'], label: 'close' },
] as const
