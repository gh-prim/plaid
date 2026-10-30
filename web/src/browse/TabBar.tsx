import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink } from 'react-router'
import { Icon } from '../ui'
import { ProfileMenu } from './ProfileMenu'
import type { ShellLibrary } from './useShell'

/** Libraries that get their own tab on phones; the rest go under More. */
export const tabLibraries = 3

/** The phone's bottom tab bar (F008-R1): Home, libraries, Search, More. */
export function TabBar({ libraries }: { libraries?: ShellLibrary[] }) {
  const { t } = useTranslation()
  const [more, setMore] = useState(false)
  const shown = libraries?.slice(0, tabLibraries) ?? []
  const rest = libraries?.slice(tabLibraries) ?? []
  return (
    <nav className="br-tabs" aria-label={t('nav.main')}>
      <NavLink to="/" end className="br-tab pl-focusable">
        <Icon name="tv" size={20} />
        {t('nav.home')}
      </NavLink>
      {shown.map((l) => (
        <NavLink key={l.id} to={`/library/${l.id}`} className="br-tab pl-focusable">
          <Icon name="folder" size={20} />
          <span className="br-tab__text">{l.name}</span>
        </NavLink>
      ))}
      <NavLink to="/search" className="br-tab pl-focusable">
        <Icon name="search" size={20} />
        {t('nav.search')}
      </NavLink>
      <button
        type="button"
        className="br-tab pl-focusable"
        aria-haspopup="menu"
        aria-expanded={more}
        onMouseDown={(e) => e.nativeEvent.stopPropagation()}
        onClick={() => setMore((m) => !m)}
      >
        <Icon name="more" size={20} />
        {t('nav.more')}
      </button>
      {more && (
        <ProfileMenu
          className="br-menu--sheet"
          onClose={() => setMore(false)}
          extra={rest.map((l) => (
            <Link
              key={l.id}
              to={`/library/${l.id}`}
              role="menuitem"
              className="pl-menu__item pl-focusable"
              onClick={() => setMore(false)}
            >
              {l.name}
            </Link>
          ))}
        />
      )}
    </nav>
  )
}
