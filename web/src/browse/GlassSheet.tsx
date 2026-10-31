import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { Icon, cx } from '../ui'

/**
 * The one glass sheet of the admin's modals (V2): the page stays behind,
 * blurred and dimmed; the sheet is frosted glass with a light top edge,
 * 26 px corners and a round close button. Esc and a click outside close it.
 */
export function GlassSheet({
  title,
  subtitle,
  mono,
  back,
  wide,
  onClose,
  children,
  footer,
}: {
  title: string
  subtitle?: string
  /** The subtitle is a path: monospace, cut with an ellipsis. */
  mono?: boolean
  /** A round back button before the title. */
  back?: { label: string; onClick: () => void }
  wide?: boolean
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
}) {
  const { t } = useTranslation()
  const sheet = useRef<HTMLDivElement>(null)
  // Closing plays the opening backwards, quicker (200 ms), then unmounts.
  const [closing, setClosing] = useState(false)
  const close = useCallback(() => {
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return onClose()
    setClosing(true)
    setTimeout(onClose, 200)
  }, [onClose])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && close()
    document.addEventListener('keydown', onKey)
    // Something inside takes focus unless a field already asked for it.
    if (!sheet.current?.contains(document.activeElement)) sheet.current?.focus()
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = overflow
    }
  }, [close])

  return createPortal(
    <div
      className={cx('pl br-sheet-scrim', closing && 'is-closing')}
      role="presentation"
      onMouseDown={(e) => e.target === e.currentTarget && close()}
    >
      <div
        ref={sheet}
        className={cx('br-sheet', wide && 'br-sheet--wide')}
        role="dialog"
        aria-modal="true"
        aria-labelledby="sheet-title"
        tabIndex={-1}
      >
        <div className="br-sheet__head">
          {back && (
            <button
              type="button"
              className="br-sheet__round pl-focusable"
              aria-label={back.label}
              onClick={back.onClick}
            >
              <Icon name="chevronLeft" size={16} />
            </button>
          )}
          <div className="br-sheet__titles">
            <h2 id="sheet-title" className="br-sheet__title">
              {title}
            </h2>
            {subtitle && (
              <p className={cx('br-sheet__sub', mono && 'br-sheet__sub--mono')} title={subtitle}>
                {subtitle}
              </p>
            )}
          </div>
          <button
            type="button"
            className="br-sheet__round pl-focusable"
            aria-label={t('common.close')}
            onClick={close}
          >
            <Icon name="x" size={14} />
          </button>
        </div>
        <div className="br-sheet__body">{children}</div>
        {footer && <div className="br-sheet__foot">{footer}</div>}
      </div>
    </div>,
    document.body,
  )
}

/** How long a toast stays, unless the pointer rests on it. */
const toastMs = 5000

/** A glass toast at the bottom: what changed, with an action (Undo). It
 * rises in, waits (paused while hovered) and sinks away. */
export function GlassToast({
  children,
  action,
  onDone,
  art,
}: {
  children: ReactNode
  action?: { label: string; onClick: () => void }
  onDone: () => void
  art?: string
}) {
  const [hovered, setHovered] = useState(false)
  const [leaving, setLeaving] = useState(false)
  useEffect(() => {
    if (hovered) return
    const timer = setTimeout(() => setLeaving(true), toastMs)
    return () => clearTimeout(timer)
  }, [hovered])
  useEffect(() => {
    if (!leaving) return
    const timer = setTimeout(onDone, 240)
    return () => clearTimeout(timer)
  }, [leaving, onDone])
  return createPortal(
    <div
      className={cx('pl br-toast', leaving && 'is-leaving')}
      role="status"
      onPointerEnter={() => setHovered(true)}
      onPointerLeave={() => setHovered(false)}
    >
      {art && <img className="br-toast__art" src={art} alt="" />}
      <span className="br-toast__text">{children}</span>
      {action && (
        <button type="button" className="br-sheet__pill pl-focusable" onClick={action.onClick}>
          {action.label}
        </button>
      )}
    </div>,
    document.body,
  )
}
