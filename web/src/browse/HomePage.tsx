import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { adminStep } from '../onboarding/step'
import { Button } from '../ui'
import { AmbientTop } from './AmbientTop'
import { Row } from './Row'

type Home = components['schemas']['Home']
type HomeRow = components['schemas']['HomeRow']
type Library = components['schemas']['Library']

/** The home (F008-R5–R9): the ambient top, then the server's shelves in its order. */
export function HomePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { state } = useAuth()
  const isAdmin = state.status === 'ready' && state.me?.user.role === 'admin'
  const [home, setHome] = useState<Home>()
  const [libraries, setLibraries] = useState<Library[]>()
  const [failed, setFailed] = useState(false)

  const apply = useCallback((r: Awaited<ReturnType<typeof fetchHome>>) => {
    setFailed(!r.home)
    if (r.home) setHome(r.home)
    if (r.libraries) setLibraries(r.libraries)
  }, [])
  const load = useCallback(async () => apply(await fetchHome()), [apply])

  useEffect(() => {
    let cancelled = false
    void fetchHome().then((r) => {
      if (cancelled) return
      apply(r)
      // An admin who hasn't finished the onboarding goes back to it (F007 R1, R10).
      if (!isAdmin || !r.libraries) return
      const step = adminStep(r.libraries, true)
      if (step !== 'done') void navigate(`/setup/${step}`, { replace: true })
    })
    return () => {
      cancelled = true
    }
  }, [apply, isAdmin, navigate])

  if (failed) {
    return (
      <div className="br-page br-error" role="alert">
        <p>{t('errors.generic')}</p>
        <Button variant="secondary" onClick={() => void load()}>
          {t('common.retry')}
        </Button>
      </div>
    )
  }
  if (!home || !libraries) return <HomeSkeleton />

  if (home.rows.length === 0) {
    return <EmptyHome libraries={libraries} admin={isAdmin} />
  }
  return (
    <div className="br-home">
      <AmbientTop item={home.ambient} />
      {home.rows.map((r) => (
        <Row
          key={r.id}
          title={rowTitle(t, r)}
          seeAll={seeAllPath(r)}
          page={{ items: r.items, nextCursor: r.nextCursor }}
          landscape={r.type === 'continue_watching'}
          onChange={() => void load()}
          more={async (cursor) =>
            (
              await api.GET('/home/rows/{rowId}', {
                params: { path: { rowId: r.id }, query: { cursor } },
              })
            ).data
          }
        />
      ))}
    </div>
  )
}

async function fetchHome() {
  const [h, l] = await Promise.all([api.GET('/home'), api.GET('/libraries')])
  return { home: h.data, libraries: l.data }
}

/** A row's title from its type and params; the server never sends text (R31). */
function rowTitle(t: ReturnType<typeof useTranslation>['t'], r: HomeRow): string {
  switch (r.type) {
    case 'continue_watching':
      return t('home.row.continueWatching')
    case 'recently_added':
      return t('home.row.recentlyAdded', { libraryName: r.libraryName ?? '' })
    default:
      return t('home.row.genre', { genre: r.genre?.name ?? '' })
  }
}

/** "See all" opens the library grid with the row's query. */
export function seeAllPath(r: HomeRow): string | undefined {
  const a = r.seeAll
  if (!a) return undefined
  const q = new URLSearchParams({ sort: a.sort, order: a.order })
  if (a.genre !== undefined) q.set('genre', String(a.genre))
  return `/library/${a.libraryId}?${q}`
}

/**
 * Nothing to show (R9): no library (admins get F007's "Add a library",
 * members a word for the admin), or a first scan still matching.
 */
function EmptyHome({ libraries, admin }: { libraries: Library[]; admin: boolean }) {
  const { t } = useTranslation()
  if (libraries.length > 0) {
    const found = libraries.reduce((n, l) => n + l.counts.movies + l.counts.shows, 0)
    return (
      <section className="br-page br-empty">
        <h1 className="br-empty__title">{t('home.empty.scanning.title')}</h1>
        <p className="br-empty__body">{t('home.empty.scanning.body', { count: found })}</p>
        <div
          className="pl-progress pl-progress--indeterminate br-empty__progress"
          aria-hidden="true"
        >
          <span className="pl-progress__fill" />
        </div>
      </section>
    )
  }
  if (admin) {
    return (
      <section className="br-page br-empty">
        <h1 className="br-empty__title">{t('home.empty.title')}</h1>
        <p className="br-empty__body">{t('home.empty.body')}</p>
        <Link
          to="/admin/libraries"
          className="pl-btn pl-btn--primary pl-focusable br-empty__action"
        >
          {t('home.empty.action')}
        </Link>
      </section>
    )
  }
  return (
    <section className="br-page br-empty">
      <h1 className="br-empty__title">{t('home.empty.member.title')}</h1>
      <p className="br-empty__body">{t('home.empty.member.body')}</p>
    </section>
  )
}

/** Loading: grey shelves, no spinner over posters, no shimmer. */
function HomeSkeleton() {
  const { t } = useTranslation()
  return (
    <div className="br-home" aria-busy="true" aria-label={t('common.loading')}>
      {[0, 1, 2].map((r) => (
        <div key={r} className="br-row br-row--skeleton" aria-hidden="true">
          <span className="br-skeleton br-skeleton--title" />
          <div className="br-row__list">
            {Array.from({ length: 8 }, (_, i) => (
              <span key={i} className="br-card br-card--poster">
                <span className="br-card__art" />
              </span>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
