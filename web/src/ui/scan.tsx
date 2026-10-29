// Ported from the PLAID design system: PosterSlot, AmbientBackdrop,
// ServerFact, the pieces of the first scan (F007 R12, R13).
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { CommandBlock } from './CommandBlock'
import { cx } from './cx'

export type SlotState = 'empty' | 'arriving' | 'arrived' | 'missing'

/** One place on the poster wall. `empty` is the dark the wall fills from. */
export function PosterSlot({
  src,
  title,
  state,
}: {
  src?: string
  title?: string
  state?: SlotState
}) {
  const st = state ?? (src ? 'arrived' : title ? 'missing' : 'empty')
  return (
    <div
      className={cx('pl pl-slot', `pl-slot--${st}`)}
      aria-hidden={st === 'empty' ? 'true' : undefined}
    >
      {(st === 'arrived' || st === 'arriving') && src && (
        <img
          src={src}
          alt={title ?? ''}
          loading="lazy"
          className={st === 'arriving' ? 'pl-slot__arrive' : undefined}
        />
      )}
      {st === 'missing' && <span className="pl-slot__plate">{title}</span>}
    </div>
  )
}

/** The design system's pacing: a new backdrop at most every 8 s, 30 s with reduced motion. */
export const backdropEvery = 8000
export const backdropEveryReduced = 30000

function reducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * The backdrop of the latest matched title, blurred and dimmed, like a
 * screen lighting a dark room. Scan screen only.
 */
export function AmbientBackdrop({
  src,
  theme = 'dark',
}: {
  src?: string
  theme?: 'dark' | 'light'
}) {
  const [layers, setLayers] = useState<string[]>(src ? [src] : [])
  const last = useRef(0)
  useEffect(() => {
    if (!src || src === layers.at(-1)) return
    const every = reducedMotion() ? backdropEveryReduced : backdropEvery
    const wait = Math.max(0, last.current + every - Date.now())
    const timer = setTimeout(() => {
      last.current = Date.now()
      setLayers((l) => [...l.slice(-1), src])
    }, wait)
    return () => clearTimeout(timer)
  }, [src, layers])
  return (
    <div className={cx('pl-ambient', theme === 'light' && 'pl-ambient--light')} aria-hidden="true">
      {layers.map((s, i) => (
        <img
          key={s}
          src={s}
          alt=""
          className={cx(
            'pl-ambient__img',
            i === layers.length - 1 && layers.length > 1 && 'pl-ambient__img--in',
          )}
        />
      ))}
      <div className="pl-ambient__veil" />
    </div>
  )
}

/** One thing the server knows about itself, when it changes something. */
export function ServerFact({
  tone = 'info',
  label,
  command,
  children,
}: {
  tone?: 'hardware' | 'info' | 'warning'
  label?: string
  command?: string
  children: ReactNode
}) {
  return (
    <div className={cx('pl pl-fact', `pl-fact--${tone}`)}>
      {label && <p className="pl-fact__label">{label}</p>}
      <p className="pl-fact__text">
        <span className="pl-fact__mark" aria-hidden="true" />
        <span>{children}</span>
      </p>
      {command && <CommandBlock command={command} />}
    </div>
  )
}
