// Ported from the PLAID design system: Toast, Dialog, BottomSheet,
// ProgressBar, EmptyState.
import { useId, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from './Button'
import { cx } from './cx'
import { Icon, type IconName } from './Icon'

type Tone = 'neutral' | 'success' | 'warning' | 'error' | 'info'
const TOAST_ICON: Partial<Record<Tone, IconName>> = {
  success: 'success',
  warning: 'alert',
  error: 'error',
  info: 'info',
}

/** A short message that leaves on its own; errors stay until dismissed. */
export function Toast({
  variant = 'neutral',
  action,
  onAction,
  onClose,
  children,
}: {
  variant?: Tone
  action?: string
  onAction?: () => void
  onClose?: () => void
  children: ReactNode
}) {
  const { t } = useTranslation()
  const icon = TOAST_ICON[variant]
  return (
    <div
      className={cx('pl pl-toast', `pl-toast--${variant}`)}
      role={variant === 'error' ? 'alert' : 'status'}
    >
      {icon && <Icon name={icon} size={22} className="pl-toast__icon" />}
      <div className="pl-toast__text">{children}</div>
      {action && (
        <Button variant="ghost" size="sm" onClick={onAction}>
          {action}
        </Button>
      )}
      {onClose && (
        <Button variant="icon" size="sm" icon="x" label={t('ui.dismiss')} onClick={onClose} />
      )}
    </div>
  )
}

/** A modal decision; on phones, use BottomSheet for pickers. */
export function Dialog({
  title,
  actions,
  children,
}: {
  title: string
  actions: ReactNode
  children?: ReactNode
}) {
  const id = useId()
  return (
    <div className="pl pl-scrim">
      <div className="pl-dialog" role="dialog" aria-modal="true" aria-labelledby={id}>
        <h2 id={id} className="pl-dialog__title">
          {title}
        </h2>
        {children && <div className="pl-dialog__body">{children}</div>}
        <div className="pl-dialog__actions">{actions}</div>
      </div>
    </div>
  )
}

export function BottomSheet({ title, children }: { title?: string; children: ReactNode }) {
  const id = useId()
  return (
    <div className="pl" style={{ position: 'relative', height: '100%' }}>
      <div className="pl-scrim" style={{ padding: 0 }} />
      <div
        className="pl-sheet"
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? id : undefined}
      >
        <div className="pl-sheet__grab" />
        {title && (
          <h3 id={id} className="pl-sheet__title">
            {title}
          </h3>
        )}
        {children}
      </div>
    </div>
  )
}

/**
 * Progress for posters and scans. The fill is the woven amber thread, the
 * only place the tartan appears in the UI.
 */
export function ProgressBar({
  variant = 'poster',
  value = 0,
  label,
  valueLabel,
  indeterminate,
  error,
}: {
  variant?: 'poster' | 'scan'
  /** 0 to 100. */
  value?: number
  label?: string
  valueLabel?: string
  indeterminate?: boolean
  error?: boolean
}) {
  const { t } = useTranslation()
  const v = Math.max(0, Math.min(100, value))
  return (
    <div className="pl">
      {label && (
        <div className="pl-progress-label">
          <span>{label}</span>
          <span>{indeterminate ? '' : valueLabel}</span>
        </div>
      )}
      <div
        className={cx(
          'pl-progress',
          variant === 'scan' && 'pl-progress--scan',
          indeterminate && 'pl-progress--indeterminate',
          error && 'pl-progress--error',
        )}
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={indeterminate ? undefined : Math.round(v)}
        aria-valuetext={valueLabel}
        aria-label={label ?? t('ui.progress')}
      >
        <div className="pl-progress__fill" style={{ width: `${v}%` }} />
      </div>
    </div>
  )
}

/** Words and one action. */
export function EmptyState({
  icon,
  title,
  actions,
  children,
}: {
  icon?: IconName
  title: string
  actions?: ReactNode
  children?: ReactNode
}) {
  return (
    <div className="pl pl-empty">
      {icon && <Icon name={icon} size={28} className="pl-empty__icon" />}
      <h3 className="pl-empty__title">{title}</h3>
      {children && <p className="pl-empty__body">{children}</p>}
      {actions && <div className="pl-empty__actions">{actions}</div>}
    </div>
  )
}
