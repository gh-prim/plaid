import { useId } from 'react'

type PinInputProps = {
  label: string
  value: string
  onChange: (pin: string) => void
  error?: string
  autoFocus?: boolean
}

/**
 * A 4-digit PIN as one field, so screen readers announce it once and paste
 * and password managers just work. The digits are spaced out by CSS.
 */
export function PinInput({ label, value, onChange, error, autoFocus }: PinInputProps) {
  const id = useId()
  return (
    <div className="field pin-input">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="password"
        inputMode="numeric"
        autoComplete="off"
        pattern="[0-9]{4}"
        maxLength={4}
        required
        autoFocus={autoFocus}
        value={value}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        onChange={(e) => onChange(e.target.value.replace(/\D/g, '').slice(0, 4))}
      />
      {error && (
        <p id={`${id}-error`} className="field__error">
          {error}
        </p>
      )}
    </div>
  )
}
