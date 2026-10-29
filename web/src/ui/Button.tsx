// Ported from the PLAID design system: Button and Spinner. Only primary
// and play carry the amber, one per view.
import type { ButtonHTMLAttributes, CSSProperties, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { cx } from './cx'
import { Icon, type IconName } from './Icon'

export function Spinner({
  size = 28,
  mono,
  label,
}: {
  size?: number
  mono?: boolean
  label?: string
}) {
  const { t } = useTranslation()
  return (
    <span
      className={cx('pl-spinner', mono && 'pl-spinner--mono')}
      style={{ '--pl-s': `${size}px` } as CSSProperties}
      role="status"
      aria-label={label ?? t('ui.loading')}
    />
  )
}

type ButtonProps = {
  variant?: 'primary' | 'secondary' | 'ghost' | 'icon' | 'play'
  size?: 'sm' | 'md'
  icon?: IconName
  /** Required for `icon` buttons: their accessible name. */
  label?: string
  loading?: boolean
  block?: boolean
  children?: ReactNode
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'>

export function Button({
  variant = 'primary',
  size = 'md',
  icon,
  label,
  loading,
  block,
  className,
  type = 'button',
  children,
  ...rest
}: ButtonProps) {
  const shown = icon ?? (variant === 'play' ? 'play' : undefined)
  const iconSize = variant === 'play' ? 26 : size === 'sm' ? 18 : 20
  const isIcon = variant === 'icon'
  return (
    <button
      type={type}
      className={cx(
        'pl-btn pl-focusable',
        `pl-btn--${variant}`,
        size !== 'md' && `pl-btn--${size}`,
        block && 'pl-btn--block',
        loading && 'is-loading',
        className,
      )}
      aria-label={isIcon ? label : undefined}
      aria-busy={loading ? 'true' : undefined}
      {...rest}
    >
      {shown && <Icon name={shown} size={iconSize} />}
      {!isIcon && children != null && <span>{children}</span>}
      {loading && (
        <span className="pl-btn__spin">
          <Spinner size={20} mono />
        </span>
      )}
    </button>
  )
}
