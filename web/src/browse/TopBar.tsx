import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, useLocation } from 'react-router'
import type { components } from '../api/schema'
import { Avatar, Icon, cx } from '../ui'
import { ActivityMenu } from './ActivityMenu'
import { ProfileMenu } from './ProfileMenu'
import { ServerPill } from './ServerCard'
import { searchFieldId, useSearchQuery } from './SearchResults'
import type { ShellLibrary } from './useShell'
import { Wordmark } from './Wordmark'

type Profile = components['schemas']['Profile']
type ActivePlayback = components['schemas']['ActivePlayback']

/**
 * The top bar (V2): the wordmark, a floating glass tab bar in the center
 * (Home, each library, Search), then the admin's activity and the profile.
 * Phones keep the wordmark and the profile; their tabs sit at the bottom.
 */
export function TopBar({
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
  const { t } = useTranslation()
  const [menu, setMenu] = useState(false)
  const scrolled = useScrolled()
  const { pathname } = useLocation()
  const onSearch = pathname === '/search'
  const [q, setQ] = useSearchQuery()
  const nav = useRef<HTMLElement>(null)
  const [pill, setPill] = useState<React.CSSProperties>({ opacity: 0 })
  useLayoutEffect(() => {
    const el = nav.current
    if (!el) return
    const place = () => {
      const tab = el.querySelector<HTMLElement>('.br-nav__tab.active')
      setPill(
        tab
          ? { opacity: 1, left: tab.offsetLeft, width: tab.offsetWidth }
          : { opacity: 0, left: 0, width: 0 },
      )
    }
    place()
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(place)
    ro.observe(el)
    return () => ro.disconnect()
  }, [pathname, libraries])
  return (
    <header className={cx('br-top', scrolled && 'is-scrolled')}>
      <Link to="/" className="br-top__brand pl-focusable">
        <Wordmark size={30} />
      </Link>
      <nav ref={nav} className="br-nav br-glass" aria-label={t('nav.main')}>
        {/* The cream pill slides to the tab in use (V2 motion). */}
        <span className="br-nav__pill" style={pill} aria-hidden="true" />
        <NavLink to="/" end className="br-nav__tab pl-focusable">
          {t('nav.home')}
        </NavLink>
        {libraries?.map((l) => (
          <NavLink key={l.id} to={`/library/${l.id}`} className="br-nav__tab pl-focusable">
            {l.name}
          </NavLink>
        ))}
        {onSearch ? (
          // On the search page the magnifier becomes the field (V2).
          <label className="br-nav__search" role="search">
            <Icon name="search" size={18} />
            <input
              id={searchFieldId}
              type="search"
              autoFocus
              autoComplete="off"
              aria-label={t('search.label', { serverName })}
              placeholder={t('search.placeholder')}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              onKeyDown={(e) => e.key === 'Escape' && setQ('')}
            />
            {q && (
              <button
                type="button"
                className="br-nav__clear pl-focusable"
                aria-label={t('fixMatch.clear')}
                onClick={() => setQ('')}
              >
                <Icon name="x" size={10} />
              </button>
            )}
          </label>
        ) : (
          <NavLink
            to="/search"
            className="br-nav__tab br-nav__tab--icon pl-focusable"
            aria-label={t('nav.search')}
          >
            <Icon name="search" size={18} />
          </NavLink>
        )}
      </nav>
      <span className="br-top__fill" />
      {admin && <ActivityMenu streams={streams} />}
      {admin && (
        <span className="br-top__phone">
          <ServerPill name={serverName} streams={streams} />
        </span>
      )}
      {profile && (
        <span className="br-top__profile">
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
            <Avatar name={profile.name} color={profile.avatarColor + 1} size={38} />
          </button>
          {menu && <ProfileMenu className="br-menu--down" onClose={() => setMenu(false)} />}
        </span>
      )}
    </header>
  )
}

/** The page has moved: the bar takes a soft shade so it reads over the rows. */
function useScrolled() {
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    const on = () => setScrolled(window.scrollY > 8)
    on()
    window.addEventListener('scroll', on, { passive: true })
    return () => window.removeEventListener('scroll', on)
  }, [])
  return scrolled
}
