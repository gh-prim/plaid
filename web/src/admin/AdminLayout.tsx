import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet, useLocation } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import { formatRelative } from '../lib/format'
import { useServer } from '../onboarding/Page'
import { posterSrc } from '../onboarding/PosterWall'
import { Button, Mark, NeedsYouItem, Toast } from '../ui'
import { needs, slow, statusLine, type AdminData } from './summary'
import './admin.css'

type ActivePlayback = components['schemas']['ActivePlayback']
type ItemSummary = components['schemas']['ItemSummary']

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
  const url = s.session.url
  if (!url) return undefined
  return url.replace(/(file|master\.m3u8)(\?.*)?$/, `artwork/poster?w=${width}`)
}

/** A section: the phone's back link to the index, then its content. */
export function Section({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { serverName } = useServer()
  return (
    <>
      <Link to="/admin" className="pl-btn pl-btn--ghost pl-btn--sm adm__back">
        {t('admin.backTo', { serverName })}
      </Link>
      {children}
    </>
  )
}

/** The short word on a playing row: can't keep up, keeps up, repackaged, plays as is. */
function shortState(
  t: ReturnType<typeof useTranslation>['t'],
  s: ActivePlayback,
): { word: string; tone?: 'warning' | 'success' } {
  if (slow(s)) return { word: t('playing.short.slow'), tone: 'warning' }
  if (s.session.method === 'transcode') return { word: t('playing.short.ok'), tone: 'success' }
  if (s.session.method === 'direct_stream') return { word: t('playing.short.repackaged') }
  return { word: t('playing.short.asIs') }
}

/** The admin area: the index with live summaries, and the open section (boards AD01–AD17, F007 R17, R18). */
export function AdminLayout() {
  const { t } = useTranslation()
  const { serverName, version } = useServer()
  const location = useLocation()
  const { data, refresh } = usePolling(loadSummary, summaryEvery)
  const [message, setMessage] = useState<string>()
  const toast = useCallback((m: string) => {
    setMessage(m)
    setTimeout(() => setMessage(undefined), 4000)
  }, [])
  const atHome = location.pathname.replace(/\/$/, '') === '/admin'

  const list = data ? needs(t, data) : []
  const scanning = data?.libraries.some((l) => l.scan.state !== 'idle')
  const unrecognized = data?.libraries.reduce((a, l) => a + l.counts.unrecognized, 0) ?? 0
  const openProblems = data?.problems.filter((p) => !p.resolvedAt).length ?? 0
  const folderTrouble = data?.libraries.some((l) => l.folders.some((f) => f.status !== 'ok'))
  const hwOff = data?.capabilities?.hwAccel.disabledReason === 'repeated_failures'
  // Grouped like a server admin: what's happening, what you manage, how it runs.
  type Item = {
    to: string
    label: string
    trail?: string
    tone?: 'warning' | 'error'
    exact?: boolean
  }
  const groups: { label: string; items: Item[] }[] = [
    {
      label: t('admin.group.status'),
      items: [
        { to: '/admin', label: t('admin.nav.home'), exact: true },
        {
          to: '/admin/playing',
          label: t('admin.nav.playing'),
          trail: data?.streams.length ? String(data.streams.length) : undefined,
          tone: data?.streams.some(slow) ? 'warning' : undefined,
        },
        {
          to: '/admin/problems',
          label: t('admin.nav.problems'),
          trail: openProblems ? String(openProblems) : undefined,
          tone: openProblems ? 'warning' : undefined,
        },
      ],
    },
    {
      label: t('admin.group.manage'),
      items: [
        {
          to: '/admin/libraries',
          label: t('admin.nav.libraries'),
          trail: folderTrouble
            ? t('folder.status.unavailable')
            : scanning
              ? t('scan.state.short.scanning')
              : undefined,
          tone: folderTrouble ? 'error' : undefined,
        },
        { to: '/admin/people', label: t('admin.nav.people') },
      ],
    },
    {
      label: t('admin.group.settings'),
      items: [
        {
          to: '/admin/transcoding',
          label: t('admin.nav.transcoding'),
          trail: hwOff ? t('admin.summary.hardwareOff') : undefined,
          tone: hwOff ? 'warning' : undefined,
        },
        {
          to: '/admin/metadata',
          label: t('admin.nav.metadata'),
          trail: unrecognized ? String(unrecognized) : undefined,
        },
      ],
    },
  ]
  const isCurrent = (i: Item) =>
    i.exact ? location.pathname.replace(/\/$/, '') === i.to : location.pathname.startsWith(i.to)

  return (
    <AdminContext.Provider value={{ data, refresh, toast }}>
      <div className="pl adm">
        <aside className="adm__index" hidden={!atHome}>
          <div className="adm__top">
            <Link to="/" className="pl-btn pl-btn--ghost pl-btn--sm">
              {t('admin.back')}
            </Link>
            <span className="adm__version">{t('welcome.version', { version })}</span>
          </div>
          <div className="adm__server">
            <h1 className="adm__name">{serverName}</h1>
            {data && <p className="adm__status">{statusLine(t, data, list.length)}</p>}
          </div>
          {list.length > 0 && (
            <section className="adm__needs-box" aria-label={t('admin.needs.title')}>
              <h2>{t('admin.needs.title')}</h2>
              {list.map((x) => (
                <NeedsYouItem key={x.key} to={x.to} title={x.title} tone={x.tone}>
                  {x.body}
                </NeedsYouItem>
              ))}
            </section>
          )}
          <nav className="adm__nav" aria-label={t('admin.sections')}>
            {groups.map((g) => (
              <div key={g.label} className="adm__group">
                <h2 className="adm__group-label">{g.label}</h2>
                <ul>
                  {g.items.map((i) => (
                    <li key={i.to}>
                      <Link
                        to={i.to}
                        className="adm__nav-item pl-focusable"
                        aria-current={isCurrent(i) ? 'page' : undefined}
                      >
                        <span>{i.label}</span>
                        {i.trail && (
                          <span className={`adm__nav-trail ${i.tone ? `pl-tone--${i.tone}` : ''}`}>
                            {i.tone && <Mark tone={i.tone} />}
                            {i.trail}
                          </span>
                        )}
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </nav>
        </aside>
        <main className="adm__main" hidden={atHome}>
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

/** Desktop, beside the index at /admin: what needs you and what's playing, or a quiet summary (boards AD01, AD02). */
export function AdminHome() {
  const { t, i18n } = useTranslation()
  const { data } = useAdmin()
  const locale = i18n.resolvedLanguage ?? 'en'
  const [posters, setPosters] = useState<ItemSummary[]>([])
  const firstLibrary = data?.libraries[0]?.id

  useEffect(() => {
    if (!firstLibrary) return
    api
      .GET('/libraries/{libraryId}/items', {
        params: { path: { libraryId: firstLibrary }, query: { limit: 24 } },
      })
      .then(({ data: page }) => setPosters((page?.items ?? []).filter((i) => i.poster).slice(0, 6)))
      .catch(() => setPosters([]))
  }, [firstLibrary])

  if (!data) return null
  const list = needs(t, data)
  const last = data.libraries
    .map((l) => l.lastScanAt)
    .filter(Boolean)
    .sort()
    .at(-1)

  if (!list.length && !data.streams.length) {
    return (
      <>
        <section className="adm__section" style={{ gap: 8 }}>
          <h2 className="adm__h3">{t('admin.home.nothingNeedsYou')}</h2>
          <p className="adm__text">
            {t('admin.home.library', {
              movies: data.libraries.reduce((a, l) => a + l.counts.movies, 0),
              shows: data.libraries.reduce((a, l) => a + l.counts.shows, 0),
              time: last ? formatRelative(locale, new Date(last)) : '—',
            })}
          </p>
        </section>
        {posters.length > 0 && (
          <section className="adm__section">
            <div className="adm__head" style={{ alignItems: 'baseline' }}>
              <h2 className="adm__h3">{t('admin.home.fromLibrary')}</h2>
              <Link to="/" className="adm__link">
                {t('admin.home.openLibrary')}
              </Link>
            </div>
            <div className="adm__posters">
              {posters.map((p) => (
                <img
                  key={p.id}
                  src={posterSrc(p.poster!.imageId)}
                  alt={p.displayTitle ?? p.title}
                />
              ))}
            </div>
          </section>
        )}
      </>
    )
  }

  return (
    <>
      {list.length > 0 && (
        <section className="adm__section">
          <h2 className="adm__h3">{t('admin.needs.title')}</h2>
          {list.map((x) => (
            <NeedsYouItem key={x.key} card to={x.to} title={x.title} tone={x.tone}>
              {x.body}
            </NeedsYouItem>
          ))}
        </section>
      )}
      {data.streams.length > 0 && (
        <section className="adm__section" style={{ gap: 8 }}>
          <h2 className="adm__h3">{t('playing.title')}</h2>
          <div className="adm__rows">
            {data.streams.map((s) => {
              const st = shortState(t, s)
              const poster = streamPoster(s)
              return (
                <Link
                  key={s.session.id}
                  to="/admin/playing"
                  className="adm__row adm__row--center"
                  style={{ padding: '10px 0' }}
                >
                  {poster ? (
                    <img className="adm__thumb" src={poster} alt="" />
                  ) : (
                    <span className="adm__thumb" />
                  )}
                  <span className="adm__row-body">
                    <span className="adm__row-title">{s.itemTitle}</span>
                    <span className="adm__help">
                      {t('playing.row', {
                        user: s.username,
                        profile: s.profileName,
                        device: s.deviceName,
                      })}
                    </span>
                  </span>
                  <span className={`adm__word ${st.tone ? `pl-tone--${st.tone}` : ''}`}>
                    {st.tone && <Mark tone={st.tone} />}
                    {st.word}
                  </span>
                </Link>
              )
            })}
          </div>
        </section>
      )}
    </>
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
