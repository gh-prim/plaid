// Locale formatting for every number, size, duration and name on screen
// (F007 R32). Never concatenate units by hand.

export function formatNumber(locale: string, n: number): string {
  return new Intl.NumberFormat(locale).format(n)
}

/** Bytes as GB, MB… with the locale's units, e.g. "12,4 Go" in French. */
export function formatBytes(locale: string, bytes: number): string {
  const units = ['byte', 'kilobyte', 'megabyte', 'gigabyte', 'terabyte'] as const
  let v = bytes
  let i = 0
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000
    i++
  }
  return new Intl.NumberFormat(locale, {
    style: 'unit',
    unit: units[i],
    unitDisplay: 'short',
    maximumFractionDigits: v < 10 && i > 0 ? 1 : 0,
  }).format(v)
}

/** Bits per second as Mbps ("8 Mbps", "8 Mbit/s"). */
export function formatMbps(locale: string, bps: number): string {
  return new Intl.NumberFormat(locale, {
    style: 'unit',
    unit: 'megabit-per-second',
    unitDisplay: 'short',
    maximumFractionDigits: 1,
  }).format(bps / 1e6)
}

/** A media position or runtime: 1:12:48, 47:05. */
export function formatClock(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = String(s % 60).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`
}

/** "3 minutes ago", "il y a 3 minutes". */
export function formatRelative(locale: string, when: Date, now = new Date()): string {
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const sec = Math.round((when.getTime() - now.getTime()) / 1000)
  const steps: [Intl.RelativeTimeFormatUnit, number][] = [
    ['day', 86400],
    ['hour', 3600],
    ['minute', 60],
  ]
  for (const [unit, size] of steps) {
    if (Math.abs(sec) >= size) return rtf.format(Math.round(sec / size), unit)
  }
  return rtf.format(sec, 'second')
}

export function formatTime(locale: string, when: Date): string {
  return new Intl.DateTimeFormat(locale, { hour: 'numeric', minute: '2-digit' }).format(when)
}

/** "French", "français": a language name in the UI's language. */
export function languageName(locale: string, code: string): string {
  return new Intl.DisplayNames([locale], { type: 'language' }).of(code) ?? code
}

export function regionName(locale: string, code: string): string {
  return new Intl.DisplayNames([locale], { type: 'region' }).of(code) ?? code
}

/** A language named in its own language, for the language picker (F007 R3). */
export function nativeLanguageName(code: string): string {
  const name = new Intl.DisplayNames([code], { type: 'language' }).of(code) ?? code
  return name.charAt(0).toLocaleUpperCase(code) + name.slice(1)
}
