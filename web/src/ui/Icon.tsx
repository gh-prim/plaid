// Ported from the PLAID design system (components/bundle.js). Rounded line
// icons, 1.75 stroke on a 24 grid.
import { cx } from './cx'

const PATHS = {
  play: 'M8 5.5v13a1 1 0 0 0 1.5.86l10.5-6.5a1 1 0 0 0 0-1.72L9.5 4.64A1 1 0 0 0 8 5.5z',
  plus: 'M12 5v14M5 12h14',
  check: 'M5 12.5l4.5 4.5L19 7.5',
  x: 'M6 6l12 12M18 6L6 18',
  search: 'M11 4a7 7 0 1 0 0 14a7 7 0 0 0 0-14zM20 20l-4-4',
  eye: 'M2.5 12s3.5-6.5 9.5-6.5S21.5 12 21.5 12s-3.5 6.5-9.5 6.5S2.5 12 2.5 12zM12 9.5a2.5 2.5 0 1 0 0 5a2.5 2.5 0 0 0 0-5z',
  eyeOff:
    'M3 3l18 18M10.6 5.6A10 10 0 0 1 12 5.5c6 0 9.5 6.5 9.5 6.5a17 17 0 0 1-2.7 3.4M6.6 6.6C3.9 8.4 2.5 12 2.5 12s3.5 6.5 9.5 6.5a9.6 9.6 0 0 0 4.4-1.1M9.9 9.9a2.5 2.5 0 0 0 3.5 3.5',
  chevronDown: 'M6 9.5l6 6 6-6',
  chevronRight: 'M9.5 6l6 6-6 6',
  chevronLeft: 'M14.5 6l-6 6 6 6',
  info: 'M12 3.5a8.5 8.5 0 1 0 0 17a8.5 8.5 0 0 0 0-17zM12 11v5M12 8h.01',
  alert: 'M12 4L2.8 19.5h18.4L12 4zM12 10v4M12 17h.01',
  error: 'M12 3.5a8.5 8.5 0 1 0 0 17a8.5 8.5 0 0 0 0-17zM9 9l6 6M15 9l-6 6',
  success: 'M12 3.5a8.5 8.5 0 1 0 0 17a8.5 8.5 0 0 0 0-17zM8.5 12.2l2.4 2.4 4.6-4.8',
  users:
    'M9 11a3.5 3.5 0 1 0 0-7a3.5 3.5 0 0 0 0 7zM2.5 20c.6-3.4 3.2-5.5 6.5-5.5s5.9 2.1 6.5 5.5M16 4.3a3.5 3.5 0 0 1 0 6.4M18 14.8c1.9.7 3.2 2.5 3.5 5.2',
  folder: 'M3.5 7.5A2 2 0 0 1 5.5 5.5h4l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2z',
  globe:
    'M12 3.5a8.5 8.5 0 1 0 0 17a8.5 8.5 0 0 0 0-17zM3.5 12h17M12 3.5c2.3 2.4 3.5 5.2 3.5 8.5s-1.2 6.1-3.5 8.5c-2.3-2.4-3.5-5.2-3.5-8.5S9.7 5.9 12 3.5z',
  lock: 'M6.5 10.5h11a1.5 1.5 0 0 1 1.5 1.5v7a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 5 19v-7a1.5 1.5 0 0 1 1.5-1.5zM8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5',
  tv: 'M4.5 5.5h15a1.5 1.5 0 0 1 1.5 1.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 16V7a1.5 1.5 0 0 1 1.5-1.5zM9 20.5h6',
  more: 'M6 12h.01M12 12h.01M18 12h.01',
  arrowLeft: 'M19 12H5M11 6l-6 6 6 6',
  restart: 'M4.5 12a7.5 7.5 0 1 0 2.2-5.3M4.5 4.5V8H8',
} as const

export type IconName = keyof typeof PATHS

type IconProps = {
  name: IconName
  size?: number
  filled?: boolean
  stroke?: number
  /** Gives the icon a name for screen readers; without it, it's decorative. */
  label?: string
  className?: string
}

export function Icon({ name, size = 24, filled, stroke, label, className }: IconProps) {
  const fill = filled ?? name === 'play'
  return (
    <svg
      className={cx('pl-icon', className)}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      aria-hidden={label ? undefined : 'true'}
      role={label ? 'img' : undefined}
      aria-label={label}
      fill={fill ? 'currentColor' : 'none'}
      stroke="currentColor"
      strokeWidth={name === 'more' ? 3 : (stroke ?? 1.75)}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d={PATHS[name]} />
    </svg>
  )
}
