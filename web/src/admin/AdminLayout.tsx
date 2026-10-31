import { createContext, useCallback, useContext, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet, useLocation } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import { Wordmark } from '../browse/Wordmark'
import { Button, Toast } from '../ui'
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
  const [playback, libraries, capabilities, problems] = await Promise.all([
    api.GET('/playback'),
    api.GET('/libraries'),
    api.GET('/server/capabilities'),
    api.GET('/problems'),
  ])
  return {
    streams: playback.data?.sessions ?? [],
    libraries: libraries.data ?? [],
    capabilities: capabilities.data,
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

/**
 * The admin area (V2): a header with Dashboard and Settings as glass tabs
 * and Back to watching, the page lit by what's playing, then the open
 * screen. Settings and its subpages share the Settings tab.
 */
export function AdminLayout() {
  const { t } = useTranslation()
  const location = useLocation()
  const { data, refresh } = usePolling(loadSummary, summaryEvery)
  const [message, setMessage] = useState<string>()
  const toast = useCallback((m: string) => {
    setMessage(m)
    setTimeout(() => setMessage(undefined), 4000)
  }, [])
  const path = location.pathname.replace(/\/$/, '')
  const onDashboard = path === '/admin'
  const onSection = depth(path) > 1
  // The previous page, to know which way this one travels (TM25).
  const [seen, setSeen] = useState({ path, motion: {} as ReturnType<typeof travel> })
  if (seen.path !== path) setSeen({ path, motion: travel(seen.path, path) })
  const motion = seen.path === path ? seen.motion : travel(seen.path, path)
  const light = data?.streams[0] ? streamArt(data.streams[0], 'backdrop', 780) : undefined

  return (
    <AdminContext.Provider value={{ data, refresh, toast }}>
      <div className={`pl adm adm--v2${onSection ? ' adm--section' : ''}`}>
        {light && (
          <div className="br-light" aria-hidden="true">
            <img key={light} src={light} alt="" />
          </div>
        )}
        <header className="adm-top">
          <Link to="/" className="br-top__brand pl-focusable" aria-label={t('admin.back')}>
            <Wordmark size={28} />
          </Link>
          <span className="adm-top__sep" aria-hidden="true" />
          <span className="adm-top__label">{t('nav.admin')}</span>
          <nav className="br-nav br-glass adm-top__tabs" aria-label={t('admin.sections')}>
            <Link
              to="/admin"
              className={`br-nav__tab pl-focusable${onDashboard ? ' active is-on' : ''}`}
              aria-current={onDashboard ? 'page' : undefined}
            >
              {t('admin.tab.dashboard')}
            </Link>
            <Link
              to="/admin/settings"
              className={`br-nav__tab pl-focusable${onDashboard ? '' : ' active is-on'}`}
              aria-current={onDashboard ? undefined : 'page'}
            >
              {t('admin.tab.settings')}
            </Link>
          </nav>
          <span className="br-top__fill" />
          <Link to="/" className="br-sheet__pill br-glass pl-focusable adm-top__back">
            {t('admin.backToWatching')}
          </Link>
          <Link to="/" className="adm-top__done pl-focusable">
            {t('admin.done')}
          </Link>
        </header>
        <main
          key={location.pathname}
          className="adm__main pl-page-enter"
          data-dir={motion.dir}
          data-nav={motion.nav}
        >
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

/** A small "Retry" button for a failed load. */
export function Retry({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <Button variant="secondary" size="sm" onClick={onRetry}>
      {t('common.retry')}
    </Button>
  )
}
