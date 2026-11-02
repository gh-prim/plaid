import { serverUrl } from '../api/server'
import { cx } from '../ui'

/** A person's picture or initial on their party color (F009 avatars). */
export function PartyAvatar({
  name,
  color,
  url,
  host,
  size = 36,
}: {
  name: string
  color: number
  url?: string
  host?: boolean
  size?: number
}) {
  return (
    <span
      className={cx('pv-party-avatar', host && 'is-host')}
      style={{ width: size, height: size, background: `var(--party-${(color % 8) + 1})` }}
      aria-hidden="true"
    >
      {url ? <img src={serverUrl(url)} alt="" /> : (name.trim()[0] ?? '?').toUpperCase()}
    </span>
  )
}
