// Ported from the PLAID design system: Chip, SegmentedControl, ChoiceRow.
import type { ReactNode } from 'react'
import { cx } from './cx'
import { Icon } from './Icon'

/** A removable or toggleable value, such as a folder of a library. */
export function Chip({
  selected,
  onClick,
  children,
}: {
  selected?: boolean
  onClick?: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      className={cx('pl-chip pl-focusable', selected && 'is-selected')}
      onClick={onClick}
    >
      {selected && <Icon name="check" size={16} stroke={2.25} />}
      {children}
    </button>
  )
}

export function SegmentedControl<V extends string>({
  options,
  value,
  label,
  onChange,
}: {
  options: { value: V; label: string; disabled?: boolean }[]
  value: V
  label: string
  onChange: (v: V) => void
}) {
  return (
    <div className="pl pl-seg" role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={o.value === value}
          className={cx('pl-seg__item pl-focusable', o.value === value && 'is-selected')}
          disabled={o.disabled}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

/** A large radio row (language, library type). */
export function ChoiceRow({
  label,
  name,
  checked,
  lang,
  onChange,
}: {
  label: string
  name: string
  checked: boolean
  lang?: string
  onChange: () => void
}) {
  return (
    <label className={cx('pl pl-choice pl-focusable', checked && 'is-checked')} lang={lang}>
      <input type="radio" name={name} checked={checked} onChange={onChange} />
      <span className="pl-choice__dot" />
      <span className="pl-choice__label">{label}</span>
      {checked && <Icon name="check" size={20} className="pl-choice__check" />}
    </label>
  )
}
