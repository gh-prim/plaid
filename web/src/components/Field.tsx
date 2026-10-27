import { useId } from 'react'
import type { InputHTMLAttributes } from 'react'

type FieldProps = {
  label: string
  /** Error under the field, linked with aria-describedby. */
  error?: string
  /** Help shown under the field when there's no error. */
  hint?: string
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'id'>

export function Field({ label, error, hint, ...input }: FieldProps) {
  const id = useId()
  const noteId = `${id}-note`
  const note = error ?? hint
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={note ? noteId : undefined}
        {...input}
      />
      {note && (
        <p id={noteId} className={error ? 'field__error' : 'field__hint'}>
          {note}
        </p>
      )}
    </div>
  )
}

/** The one-line server error above a form. */
export function FormError({ message }: { message?: string }) {
  if (!message) return null
  return (
    <p role="alert" className="form-error">
      {message}
    </p>
  )
}
