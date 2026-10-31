import { useId, useState, type InputHTMLAttributes, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { cx } from '../ui'

/**
 * V2 forms (boards TO02, TO03, TL01–TL04): fields grouped in one glass
 * box, the label on the left of each row, the notes (errors, help) under
 * the box. A field points at its note with `describedBy`.
 */
export function FieldGroup({
  invalid,
  notes,
  children,
}: {
  /** The whole box turns red (wrong username or password, a wrong code). */
  invalid?: boolean
  notes?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="ob-fields">
      <div className={cx('ob-fields__box', invalid && 'is-error')}>{children}</div>
      {notes}
    </div>
  )
}

/** One line under a group: help, or an error. */
export function FieldNote({
  id,
  error,
  alert,
  children,
}: {
  id?: string
  error?: boolean
  /** Announced as it appears (a server error). */
  alert?: boolean
  children: ReactNode
}) {
  return (
    <p
      id={id}
      className={cx('ob-fields__note', error && 'is-error')}
      role={alert ? 'alert' : undefined}
    >
      {children}
    </p>
  )
}

type InsetFieldProps = {
  label: string
  value: string
  onValue: (value: string) => void
  kind?: 'text' | 'password' | 'code'
  invalid?: boolean
  describedBy?: string
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type' | 'id'>

/** One row of a group: its label, the input, Show/Hide for a password. */
export function InsetField({
  label,
  value,
  onValue,
  kind = 'text',
  invalid,
  describedBy,
  ...input
}: InsetFieldProps) {
  const { t } = useTranslation()
  const [reveal, setReveal] = useState(false)
  const id = useId()
  const code = kind === 'code'
  return (
    <div className={cx('ob-field', code && 'ob-field--code')}>
      <label className="ob-field__label" htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        type={kind === 'password' && !reveal ? 'password' : 'text'}
        value={value}
        aria-invalid={invalid ? 'true' : undefined}
        aria-describedby={describedBy}
        autoComplete={code ? 'one-time-code' : undefined}
        autoCapitalize={code ? 'characters' : undefined}
        spellCheck={code ? false : undefined}
        onChange={(e) => onValue(e.target.value)}
        {...input}
      />
      {kind === 'password' && (
        <button
          type="button"
          className="ob-field__show"
          aria-label={reveal ? t('ui.hidePassword') : t('ui.showPassword')}
          onClick={() => setReveal(!reveal)}
        >
          {reveal ? t('onboarding.field.hide') : t('onboarding.field.show')}
        </button>
      )}
    </div>
  )
}
