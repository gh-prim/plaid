import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { Icon, Spinner } from '../ui'

/*
 * The admin section pages in V2 (canvas page "V2", Step 9 second pass,
 * boards TA01–TA12 and TQ01–TQ03): like Apple TV's settings, a big tile on
 * the left that says where you are, with the state in words, and grouped
 * glass rows on the right, each with its value in grey and a chevron. On a
 * phone the tile goes and the rows join into one inset list per group.
 */

export type Tone = 'ok' | 'warning' | 'error'

/** The big glyphs of the left tile, one per section (boards TA01–TA12). */
const glyphs = {
  root: 'M5.8 3.5h12.4a1.8 1.8 0 0 1 1.8 1.8v3.4a1.8 1.8 0 0 1-1.8 1.8H5.8A1.8 1.8 0 0 1 4 8.7V5.3a1.8 1.8 0 0 1 1.8-1.8zM5.8 13.5h12.4a1.8 1.8 0 0 1 1.8 1.8v3.4a1.8 1.8 0 0 1-1.8 1.8H5.8A1.8 1.8 0 0 1 4 18.7v-3.4a1.8 1.8 0 0 1 1.8-1.8zM11 7h6M11 17h6M7.5 7h.01M7.5 17h.01',
  libraries:
    'M5.5 6h13a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2zM6 3.5h12M8.5 6v13M15.5 6v13M3.5 12.5h17',
  library:
    'M5.7 5h12.6a2.2 2.2 0 0 1 2.2 2.2v9.6a2.2 2.2 0 0 1-2.2 2.2H5.7a2.2 2.2 0 0 1-2.2-2.2V7.2A2.2 2.2 0 0 1 5.7 5zM8 5v14M16 5v14M3.5 12h17',
  unmatched: 'M6 3h8l4 4v14H6zM14 3v4h4M10 11.5a2 2 0 1 1 2.7 1.9c-.5.2-.7.6-.7 1.1v.5M12 17.5v.1',
  people:
    'M9 5.5a3 3 0 1 0 0 6a3 3 0 0 0 0-6zM3.5 19c.6-3 2.8-4.8 5.5-4.8s4.9 1.8 5.5 4.8M16.5 7.1a2.4 2.4 0 1 0 0 4.8a2.4 2.4 0 0 0 0-4.8zM15.5 14.4c2.5-.3 4.4 1.3 5 4.1',
  devices:
    'M4 5h10a1.5 1.5 0 0 1 1.5 1.5v6A1.5 1.5 0 0 1 14 14H4a1.5 1.5 0 0 1-1.5-1.5v-6A1.5 1.5 0 0 1 4 5zM6 17h6M18.2 8h2.6A1.2 1.2 0 0 1 22 9.2v7.6a1.2 1.2 0 0 1-1.2 1.2h-2.6a1.2 1.2 0 0 1-1.2-1.2V9.2A1.2 1.2 0 0 1 18.2 8z',
  transcoding: 'M4 8h12l-3-3M20 16H8l3 3',
  metadata:
    'M6 3.5h12a2 2 0 0 1 2 2v13a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2zM8 8h8M8 12h8M8 16h5',
  problems: 'M12 3.5L21 19.5H3zM12 10v4.5M12 17v.1',
  playing: 'M8 5.5v13a1 1 0 0 0 1.5.86l10.5-6.5a1 1 0 0 0 0-1.72L9.5 4.64A1 1 0 0 0 8 5.5z',
  review: 'M11 4a7 7 0 1 0 0 14a7 7 0 0 0 0-14zM20 20l-4-4M8.5 11l1.8 1.8 3.2-3.4',
  remote:
    'M12 3.5a8.5 8.5 0 1 0 0 17a8.5 8.5 0 0 0 0-17zM3.5 12h17M12 3.5c2.3 2.3 3.4 5.2 3.4 8.5s-1.1 6.2-3.4 8.5M12 3.5C9.7 5.8 8.6 8.7 8.6 12s1.1 6.2 3.4 8.5',
} as const

export type GlyphName = keyof typeof glyphs

/** Each section's tile has its own tint, from the boards. */
const tints: Record<GlyphName, string> = {
  root: 'oklch(0.3 0.02 70)',
  libraries: 'oklch(0.32 0.03 250)',
  library: 'oklch(0.32 0.04 40)',
  unmatched: 'oklch(0.33 0.04 85)',
  people: 'oklch(0.32 0.04 300)',
  devices: 'oklch(0.3 0.03 220)',
  transcoding: 'oklch(0.3 0.04 160)',
  metadata: 'oklch(0.3 0.03 280)',
  problems: 'oklch(0.32 0.05 25)',
  playing: 'oklch(0.3 0.03 200)',
  review: 'oklch(0.33 0.04 85)',
  remote: 'oklch(0.3 0.04 190)',
}

export function Glyph({ name, size = 150 }: { name: GlyphName; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.3}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={glyphs[name]} />
    </svg>
  )
}

export type Status = { text: string; tone?: Tone }

/** A status in words with its dot, in the tile's pill or under a phone title. */
export function StatusPill({ status }: { status: Status }) {
  return (
    <span className={`adm-pill${status.tone ? ` is-${status.tone}` : ''}`}>
      <span aria-hidden="true" />
      {status.text}
    </span>
  )
}

/**
 * A section page: the back pill and the title on top, the tile with the
 * section's state on the left, the rows on the right. `back: null` for
 * Settings itself; by default it goes back to Settings.
 */
export function Section({
  title,
  back,
  glyph,
  tile,
  tileBg,
  paneTitle,
  paneSub,
  status,
  phoneTitle = 'title',
  children,
}: {
  title: string
  back?: { to: string; label: string } | null
  glyph?: GlyphName
  /** Text in the tile instead of a glyph (an initial). */
  tile?: ReactNode
  tileBg?: string
  paneTitle?: ReactNode
  paneSub?: ReactNode
  status?: Status
  /** On a phone, the big title is the page title, or the pane's (Settings: the server's name). */
  phoneTitle?: 'title' | 'pane'
  children: ReactNode
}) {
  const { t } = useTranslation()
  const to = back === undefined ? { to: '/admin/settings', label: t('admin.tab.settings') } : back
  return (
    <div className={`adm-sec adm-sec--${phoneTitle}`}>
      <header className="adm-sec__head">
        {to && (
          <Link
            to={to.to}
            className="adm-sec__back br-glass pl-focusable"
            aria-label={t('admin.backTo', { name: to.label })}
          >
            <Icon name="chevronLeft" size={18} />
            <span>{to.label}</span>
          </Link>
        )}
        <h1 className="adm-sec__title">{title}</h1>
      </header>
      <div className="adm-sec__body">
        <aside className="adm-sec__pane">
          <span
            className="adm-sec__tile"
            aria-hidden="true"
            style={{ '--tile': tileBg ?? (glyph ? tints[glyph] : tints.root) } as CSSProperties}
          >
            {tile ?? (glyph ? <Glyph name={glyph} /> : null)}
          </span>
          {paneTitle != null && <p className="adm-sec__pane-title">{paneTitle}</p>}
          {paneSub != null && <p className="adm-sec__pane-sub">{paneSub}</p>}
          {status && <StatusPill status={status} />}
        </aside>
        <div className="adm-sec__list">{children}</div>
      </div>
    </div>
  )
}

/** Data on its way: a spinner where the rows will be, never a blank. */
export function Loading({ label }: { label?: string }) {
  return (
    <div className="adm__empty" aria-busy="true">
      <Spinner size={28} />
      {label && <p>{label}</p>}
    </div>
  )
}

/** A group of rows, with an optional heading above and a note below. */
export function Group({
  title,
  foot,
  children,
}: {
  title?: string
  foot?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="adm-group" aria-label={title}>
      {title && <h2 className="adm-group__h">{title}</h2>}
      <ul className="adm-group__rows">{children}</ul>
      {foot && <p className="adm-group__foot">{foot}</p>}
    </section>
  )
}

/**
 * One row: a label (and a line under it), its value in grey or in a tone,
 * then a chevron for a link, or a control (a switch, a small button).
 * A link (`to`), a button (`onClick`) or plain. `children` open under it.
 */
export function Row({
  label,
  sub,
  value,
  tone,
  subTone,
  to,
  onClick,
  disabled,
  busy,
  avatar,
  control,
  danger,
  muted,
  mono,
  children,
}: {
  label: ReactNode
  sub?: ReactNode
  value?: ReactNode
  tone?: Tone
  subTone?: Tone
  to?: string
  onClick?: () => void
  disabled?: boolean
  busy?: boolean
  avatar?: ReactNode
  control?: ReactNode
  danger?: boolean
  muted?: boolean
  mono?: boolean
  children?: ReactNode
}) {
  const cls = [
    'adm-row',
    to || onClick ? 'pl-focusable' : '',
    danger ? 'is-danger' : '',
    muted ? 'is-muted' : '',
  ]
    .filter(Boolean)
    .join(' ')
  const inner = (
    <>
      {avatar}
      <span className="adm-row__text">
        <span className={`adm-row__label${mono ? ' is-mono' : ''}`}>{label}</span>
        {sub != null && sub !== '' && (
          <span className={`adm-row__sub${subTone ? ` is-${subTone}` : ''}`}>{sub}</span>
        )}
      </span>
      {value != null && value !== '' && (
        <span className={`adm-row__value${tone ? ` is-${tone}` : ''}`}>{value}</span>
      )}
      {busy && <Spinner size={16} />}
      {control && <span className="adm-row__control">{control}</span>}
      {to && <Icon name="chevronRight" size={16} className="adm-row__chev" />}
    </>
  )
  return (
    <li className={`adm-row-item${children ? ' has-more' : ''}`}>
      {to ? (
        <Link to={to} className={cls}>
          {inner}
        </Link>
      ) : onClick ? (
        <button
          type="button"
          className={cls}
          onClick={onClick}
          disabled={disabled || busy}
          aria-busy={busy || undefined}
        >
          {inner}
        </button>
      ) : (
        <div className={cls}>{inner}</div>
      )}
      {children && <div className="adm-row__more">{children}</div>}
    </li>
  )
}

/** An on/off switch, as on the boards' Access and toggles rows. */
export function Switch({
  label,
  on,
  disabled,
  onChange,
}: {
  label: string
  on: boolean
  disabled?: boolean
  onChange: (on: boolean) => void
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      disabled={disabled}
      className={`adm-switch pl-focusable${on ? ' is-on' : ''}`}
      onClick={() => onChange(!on)}
    >
      <span aria-hidden="true" />
    </button>
  )
}

/** A number that rolls in when it changes (TM25 "Live dashboard", 400 ms). */
export function Roll({ value }: { value: string }) {
  return (
    <span key={value} className="adm-roll">
      {value}
    </span>
  )
}

/**
 * Keeps an item that left the list for `ms` more, flagged as leaving, so
 * it can fade before its slot closes (TM25: a stream that ends fades).
 */
export function useLeaving<T>(
  items: T[],
  key: (item: T) => string,
  ms = 320,
): { item: T; leaving: boolean }[] {
  const previous = useRef(items)
  const [gone, setGone] = useState<{ item: T; index: number }[]>([])
  useEffect(() => {
    const now = new Set(items.map(key))
    const left = previous.current
      .map((item, index) => ({ item, index }))
      .filter((x) => !now.has(key(x.item)))
    previous.current = items
    if (!left.length) return
    const ids = new Set(left.map((x) => key(x.item)))
    // An item leaving is a change from outside (a poll), shown for a moment.
    setGone((g) => [...g.filter((x) => !ids.has(key(x.item))), ...left])
    setTimeout(() => setGone((g) => g.filter((x) => !ids.has(key(x.item)))), ms)
    // The key function is stable in practice; the list is what changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items, ms])
  const out = items.map((item) => ({ item, leaving: false }))
  const present = new Set(items.map(key))
  for (const g of gone) {
    if (present.has(key(g.item))) continue
    out.splice(Math.min(g.index, out.length), 0, { item: g.item, leaving: true })
  }
  return out
}
