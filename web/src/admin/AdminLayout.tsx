import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet, useLocation, useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import { Wordmark } from '../browse/Wordmark'
import { useAuth } from '../auth/AuthProvider'
import { useServer } from '../onboarding/Page'
import { Avatar, Button, Icon, type IconName, Toast } from '../ui'
import type { AdminData } from './summary'
import './admin.css'

type ActivePlayback = components['schemas']['ActivePlayback']

/** The admin home and its index refresh every 10 s while visible (F007 R18). */
export const summaryEvery = 10000

const AdminContext = createContext<{
  data?: AdminData
  refresh: () => Promise<void>
  toast: (message: string) => void
} | null>(null)

/** The admin summary shared by the index and the sections. */
export function useAdmin() {
  const ctx = useContext(AdminContext)
  if (!ctx) throw new Error('useAdmin must be used inside <AdminLayout>')
  return ctx
}

async function loadSummary(): Promise<AdminData> {
  const [playback, libraries, capabilities, problems, invitations] = await Promise.all([
    api.GET('/playback'),
    api.GET('/libraries'),
    api.GET('/server/capabilities'),
    api.GET('/problems'),
    api.GET('/invitations'),
  ])
  return {
    streams: playback.data?.sessions ?? [],
    libraries: libraries.data ?? [],
    capabilities: capabilities.data,
    invitations: invitations.data?.filter((i) => i.status === 'pending').length ?? 0,
    problems: problems.data?.problems ?? [],
  }
}

/** The playing item's poster, through the stream token (F006 R40). */
export function streamPoster(s: ActivePlayback, width = 92): string | undefined {
  return streamArt(s, 'poster', width)
}

/** The playing item's artwork of a kind, through the stream token (F006 R40). */
export function streamArt(
  s: ActivePlayback,
  kind: 'poster' | 'backdrop' | 'still',
  width: number,
): string | undefined {
  const url = s.session.url
  if (!url) return undefined
  return url.replace(/(file|master\.m3u8)(\?.*)?$/, `artwork/${kind}?w=${width}`)
}

/**
 * How deep a path sits: the dashboard 0, Settings 1, a section 2, a
 * library 3, its review 4. Going deeper pushes, coming back pops (TM25).
 */
export function depth(path: string): number {
  const parts = path.replace(/\/$/, '').split('/').slice(2)
  if (!parts.length) return 0
  if (parts[0] === 'settings') return 1
  return parts.length + 1
}

/** The motion between two admin pages: tabs travel sideways, sections push and pop. */
export function travel(from: string, to: string): { dir?: 'left' | 'right'; nav?: 'push' | 'pop' } {
  const a = depth(from)
  const b = depth(to)
  if (a === b) return {}
  if (a <= 1 && b <= 1) return { dir: b > a ? 'right' : 'left' }
  return { nav: b > a ? 'push' : 'pop' }
}

type NavItem = {
  to: string
  label: string
  icon: IconName
  badge?: number
  tone?: 'error'
  end?: boolean
}

/**
 * The admin area (canvas step 21, boards AdmFR/EN 01–03): a sidebar with
 * the server, the sections in four groups with their counts, and who you
 * are; the open page on the right with its own top bar.
 */
export function AdminLayout() {
  const { t } = useTranslation()
  const location = useLocation()
  const { state } = useAuth()
  const { serverName, version } = useServer()
  const { data, refresh } = usePolling(loadSummary, summaryEvery)
  const [message, setMessage] = useState<string>()
  const [menu, setMenu] = useState(false)
  const toast = useCallback((m: string) => {
    setMessage(m)
    setTimeout(() => setMessage(undefined), 4000)
  }, [])
  const me = state.status === 'ready' ? state.me : null
  const profile = me?.activeProfile ?? me?.profiles[0]
  const open = data?.problems.filter((p) => !p.resolvedAt).length ?? 0
  const groups: { title: string; items: NavItem[] }[] = [
    {
      title: t('admin.nav.overview'),
      items: [
        { to: '/admin', label: t('admin.tab.dashboard'), icon: 'activity', end: true },
        {
          to: '/admin/playing',
          label: t('admin.nav.playing'),
          icon: 'play',
          badge: data?.streams.length,
        },
        { to: '/admin/activity', label: t('admin.nav.activity'), icon: 'restart' },
        { to: '/admin/monitoring', label: t('admin.nav.monitoring'), icon: 'globe' },
      ],
    },
    {
      title: t('admin.nav.content'),
      items: [
        { to: '/admin/libraries', label: t('libraries.title'), icon: 'folder' },
        { to: '/admin/report', label: t('admin.nav.report'), icon: 'quality' },
        { to: '/admin/unmatched', label: t('admin.nav.unmatched'), icon: 'info' },
        { to: '/admin/metadata', label: t('admin.nav.metadata'), icon: 'globe' },
      ],
    },
    {
      title: t('admin.nav.access'),
      items: [
        { to: '/admin/people', label: t('people.title'), icon: 'users' },
        { to: '/admin/groups', label: t('sharing.groups.title'), icon: 'users' },
        {
          to: '/admin/invitations',
          label: t('admin.nav.invitations'),
          icon: 'plus',
          badge: data?.invitations,
        },
        { to: '/admin/devices', label: t('admin.nav.devices'), icon: 'tv' },
      ],
    },
    {
      title: t('admin.nav.server'),
      items: [
        { to: '/admin/transcoding', label: t('admin.nav.transcoding'), icon: 'quality' },
        { to: '/admin/remote-access', label: t('admin.nav.remoteAccess'), icon: 'globe' },
        { to: '/admin/party', label: t('party.title'), icon: 'party' },
        {
          to: '/admin/problems',
          label: t('admin.nav.problems'),
          icon: 'alert',
          badge: open,
          tone: 'error',
        },
        { to: '/admin/settings', label: t('admin.tab.settings'), icon: 'more' },
      ],
    },
  ]
  // A group's page lives under People's path but belongs to Groups.
  const path = location.pathname.replace(/^\/admin\/people\/groups/, '/admin/groups')
  const here = (item: NavItem) =>
    item.end ? path.replace(/\/$/, '') === item.to : path.startsWith(item.to)

  return (
    <AdminContext.Provider value={{ data, refresh, toast }}>
      <div className={`pl ax${menu ? ' ax--menu' : ''}`}>
        <aside className="ax-side" aria-label={t('admin.sections')}>
          <div className="ax-side__top">
            <Link to="/" className="ax-brand pl-focusable" aria-label={t('admin.back')}>
              <Wordmark size={22} />
            </Link>
            <button
              type="button"
              className="ax-menu pl-focusable"
              aria-expanded={menu}
              aria-label={t('admin.sections')}
              onClick={() => setMenu(!menu)}
            >
              <Icon name={menu ? 'x' : 'more'} size={20} />
            </button>
          </div>
          <div className="ax-server">
            <b>{serverName}</b>
            <span>
              <i className="ax-dot" aria-hidden="true" />
              {t('admin.serverOnline', { version })}
            </span>
          </div>
          <nav className="ax-nav" onClick={() => setMenu(false)}>
            {groups.map((g) => (
              <div key={g.title} className="ax-nav__group">
                <p className="ax-nav__title">{g.title}</p>
                {g.items.map((item) => (
                  <Link
                    key={item.to}
                    to={item.to}
                    className={`ax-nav__item pl-focusable${here(item) ? ' is-on' : ''}`}
                    aria-current={here(item) ? 'page' : undefined}
                  >
                    <Icon name={item.icon} size={16} />
                    <span>{item.label}</span>
                    {item.badge ? (
                      <b className={`ax-badge${item.tone ? ` ax-badge--${item.tone}` : ''}`}>
                        {item.badge}
                      </b>
                    ) : null}
                  </Link>
                ))}
              </div>
            ))}
          </nav>
          {profile && (
            <Link to="/" className="ax-me pl-focusable">
              <Avatar
                name={profile.name}
                color={profile.avatarColor + 1}
                picture={profile.avatar.url}
                size={30}
              />
              <span>
                <b>{me?.user.displayName ?? profile.name}</b>
                <small>{t('admin.backToApp')}</small>
              </span>
            </Link>
          )}
        </aside>
        <main key={location.pathname} className="ax-main pl-page-enter">
          <Outlet />
        </main>
      </div>
      {message && (
        <div className="adm__toast">
          <Toast variant="success" onClose={() => setMessage(undefined)}>
            {message}
          </Toast>
        </div>
      )}
    </AdminContext.Provider>
  )
}

/**
 * The top bar of an admin page (step 21): its title and a line beside it,
 * the search (⌘K jumps to a person, a library or a section), and the
 * page's actions.
 */
export function TopBar({
  title,
  sub,
  back,
  children,
}: {
  title: string
  sub?: ReactNode
  /** A page inside a section (a library, a person) links back to its list. */
  back?: { to: string; label: string }
  children?: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <header className="ax-top">
      {back && (
        <Link
          to={back.to}
          className="ax-back pl-focusable"
          aria-label={t('admin.backTo', { name: back.label })}
        >
          <Icon name="chevronLeft" size={16} />
          <span>{back.label}</span>
        </Link>
      )}
      <h1 className="ax-top__title">{title}</h1>
      {sub && <span className="ax-top__sub">{sub}</span>}
      <span className="ax-top__fill" />
      <JumpSearch />
      {children}
    </header>
  )
}

/** ⌘K: jump to a section, a library or a person. */
function JumpSearch() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data } = useAdmin()
  const input = useRef<HTMLInputElement>(null)
  const [q, setQ] = useState('')
  const [people, setPeople] = useState<{ id: string; displayName: string; username: string }[]>()
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        input.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  const needle = q.trim().toLocaleLowerCase()
  const hits = needle
    ? [
        ...(data?.libraries ?? [])
          .filter((l) => l.name.toLocaleLowerCase().includes(needle))
          .map((l) => ({
            to: `/admin/libraries/${l.id}`,
            label: l.name,
            kind: t('libraries.title'),
          })),
        ...(people ?? [])
          .filter(
            (u) =>
              u.displayName.toLocaleLowerCase().includes(needle) || u.username.includes(needle),
          )
          .map((u) => ({
            to: `/admin/people/${u.id}`,
            label: u.displayName,
            kind: t('people.title'),
          })),
      ].slice(0, 8)
    : []
  return (
    <div className="ax-search">
      <Icon name="search" size={15} />
      <input
        ref={input}
        type="search"
        value={q}
        placeholder={t('admin.search')}
        aria-label={t('admin.search')}
        onFocus={() => {
          if (!people) void api.GET('/users').then(({ data: u }) => setPeople(u ?? []))
        }}
        onChange={(e) => setQ(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && hits[0]) {
            setQ('')
            void navigate(hits[0].to)
          }
          if (e.key === 'Escape') setQ('')
        }}
      />
      <kbd>{t('admin.searchKey')}</kbd>
      {hits.length > 0 && (
        <ul className="ax-search__hits" role="listbox">
          {hits.map((h) => (
            <li key={h.to}>
              <Link to={h.to} className="pl-focusable" onClick={() => setQ('')}>
                <span>{h.label}</span>
                <small>{h.kind}</small>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

/** A small "Retry" button for a failed load. */
export function Retry({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <Button variant="secondary" size="sm" onClick={onRetry}>
      {t('common.retry')}
    </Button>
  )
}
