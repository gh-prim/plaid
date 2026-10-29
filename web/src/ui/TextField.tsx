// Ported from the PLAID design system: TextField. Always a visible label;
// `code` is the setup-code field.
import { useId, useState, type InputHTMLAttributes } from 'react'
import { useTranslation } from 'react-i18next'
import { Spinner } from './Button'
import { cx } from './cx'
import { Icon } from './Icon'

type TextFieldProps = {
  kind?: 'text' | 'password' | 'search' | 'code'
  label?: string
  help?: string
  /** Turns the field to the error state, with this message under it. */
  error?: string
  loading?: boolean
  value: string
  onValue: (value: string) => void
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type'>

export function TextField({
  kind = 'text',
  label,
  help,
  error,
  loading,
  value,
  onValue,
  disabled,
  className,
  ...input
}: TextFieldProps) {
  const { t } = useTranslation()
  const [reveal, setReveal] = useState(false)
  const id = useId()
  const helpId = `${id}-help`
  const note = error ?? help
  const type = kind === 'password' && !reveal ? 'password' : kind === 'search' ? 'search' : 'text'
  return (
    <div className={cx('pl pl-field', error && 'is-error', disabled && 'is-disabled', className)}>
      {label && (
        <label className="pl-field__label" htmlFor={id}>
          {label}
        </label>
      )}
      <div
        className={cx(
          'pl-field__box',
          kind === 'search' && 'pl-field__box--search',
          kind === 'code' && 'pl-field__box--code',
        )}
      >
        {kind === 'search' && <Icon name="search" size={20} />}
        <input
          id={id}
          type={type}
          value={value}
          disabled={disabled}
          aria-invalid={error ? 'true' : undefined}
          aria-label={label ? undefined : input.placeholder}
          aria-describedby={note ? helpId : undefined}
          autoComplete={kind === 'code' ? 'one-time-code' : input.autoComplete}
          autoCapitalize={kind === 'code' ? 'characters' : input.autoCapitalize}
          spellCheck={kind === 'code' ? false : input.spellCheck}
          onChange={(e) => onValue(e.target.value)}
          {...input}
        />
        {kind === 'password' && (
          <button
            type="button"
            className="pl-field__btn"
            aria-label={reveal ? t('ui.hidePassword') : t('ui.showPassword')}
            onClick={() => setReveal(!reveal)}
          >
            <Icon name={reveal ? 'eyeOff' : 'eye'} size={20} />
          </button>
        )}
        {kind === 'search' && value && (
          <button
            type="button"
            className="pl-field__btn"
            aria-label={t('ui.clear')}
            onClick={() => onValue('')}
          >
            <Icon name="x" size={18} />
          </button>
        )}
        {loading && <Spinner size={18} mono />}
      </div>
      {note && (
        <span id={helpId} className="pl-field__help">
          {error && <Icon name="error" size={14} />}
          {note}
        </span>
      )}
    </div>
  )
}
