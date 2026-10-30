import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink } from 'react-router'
import type { components } from '../api/schema'
import { formatNumber } from '../lib/format'
import { Avatar, Icon } from '../ui'
import { ProfileMenu } from './ProfileMenu'
import { ServerCard } from './ServerCard'
import type { ShellLibrary } from './useShell'
import { Wordmark } from './Wordmark'

type Profile = components['schemas']['Profile']
type ActivePlayback = components['schemas']['ActivePlayback']

/** The desktop and tablet sidebar (F008-R1, canvas "Home, Plex-like shelf"). */
export function Sidebar({
  libraries,
  profile,
  admin,
  serverName,
  streams,
}: {
  libraries?: ShellLibrary[]
  profile?: Profile
  admin: boolean
  serverName: string
  streams?: ActivePlayback[]
}) {
  const { t, i18n } = useTranslation()
  const [menu, setMenu] = useState(false)
  return (
    <aside className="br-side">
      <Link to="/" className="br-side__brand pl-focusable">
        <Wordmark />
      </Link>
      <nav className="br-side__nav" aria-label={t('nav.libraries')}>
        <NavLink to="/" end className="br-side__item pl-focusable">
          <Icon name="tv" size={18} />
          <span className="br-side__text">{t('nav.home')}</span>
        </NavLink>
        {libraries && libraries.length > 0 && (
          <>
            <span className="br-side__label">{t('nav.libraries')}</span>
            {libraries.map((l) => (
              <NavLink key={l.id} to={`/library/${l.id}`} className="br-side__item pl-focusable">
                <Icon name="folder" size={18} />
                <span className="br-side__text">{l.name}</span>
                {l.total !== undefined && (
                  <span className="br-side__count">{formatNumber(i18n.language, l.total)}</span>
                )}
              </NavLink>
            ))}
          </>
        )}
      </nav>
      {admin && (
        <>
          <span className="br-side__label">{t('nav.server')}</span>
          <ServerCard name={serverName} streams={streams} />
          <NavLink to="/admin" className="br-side__item pl-focusable">
            <Icon name="users" size={18} />
            <span className="br-side__text">{t('nav.admin')}</span>
          </NavLink>
        </>
      )}
      <span className="br-side__fill" />
      <div className="br-side__foot">
        {profile && (
          <div className="br-side__profile">
            <button
              type="button"
              className="br-side__item br-side__me pl-focusable"
              aria-label={t('nav.profileMenu', { name: profile.name })}
              aria-haspopup="menu"
              aria-expanded={menu}
              // The menu closes on outside presses; this one toggles it instead.
              onMouseDown={(e) => e.nativeEvent.stopPropagation()}
              onClick={() => setMenu((m) => !m)}
            >
              <Avatar name={profile.name} color={profile.avatarColor + 1} size={36} />
              <span className="br-side__text">
                <strong>{profile.name}</strong>
                <span>{t('profiles.switch')}</span>
              </span>
            </button>
            {menu && <ProfileMenu className="br-menu--up" onClose={() => setMenu(false)} />}
          </div>
        )}
        <NavLink to="/pair" className="br-side__item br-side__item--small pl-focusable">
          <Icon name="tv" size={18} />
          <span className="br-side__text">{t('pairing.title')}</span>
        </NavLink>
      </div>
    </aside>
  )
}
