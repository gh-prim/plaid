import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { Avatar, Icon } from '../ui'
import { ProfileMenu } from './ProfileMenu'
import { ServerPill } from './ServerCard'
import { Wordmark } from './Wordmark'

type Profile = components['schemas']['Profile']
type ActivePlayback = components['schemas']['ActivePlayback']

/**
 * The top bar: the search field on desktop; the wordmark, the admin's
 * server pill and the profile button on phones (F008-R1).
 */
export function TopBar({
  profile,
  admin,
  serverName,
  streams,
}: {
  profile?: Profile
  admin: boolean
  serverName: string
  streams?: ActivePlayback[]
}) {
  const { t } = useTranslation()
  const [menu, setMenu] = useState(false)
  return (
    <header className="br-top">
      <Link to="/" className="br-top__brand pl-focusable">
        <Wordmark size={30} />
      </Link>
      <Link to="/search" className="br-search pl-focusable" aria-label={t('nav.search')}>
        <Icon name="search" size={18} />
        <span>{t('search.placeholder')}</span>
      </Link>
      <span className="br-top__fill" />
      {admin && (
        <span className="br-top__phone">
          <ServerPill name={serverName} streams={streams} />
        </span>
      )}
      {profile && (
        <span className="br-top__phone br-top__profile">
          <button
            type="button"
            className="br-top__me pl-focusable"
            aria-label={t('nav.profileMenu', { name: profile.name })}
            aria-haspopup="menu"
            aria-expanded={menu}
            // The menu closes on outside presses; this one toggles it instead.
            onMouseDown={(e) => e.nativeEvent.stopPropagation()}
            onClick={() => setMenu((m) => !m)}
          >
            <Avatar name={profile.name} color={profile.avatarColor + 1} size={36} />
          </button>
          {menu && <ProfileMenu className="br-menu--down" onClose={() => setMenu(false)} />}
        </span>
      )}
    </header>
  )
}
