import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import type { ScanState } from '../hooks/useScanEvents'
import { Button } from '../ui'
import { useServer } from './Page'
import { posterSrc } from './PosterWall'
import { doneKey, setFlag } from './step'
import { StatusScreen } from './StatusScreen'

type Library = components['schemas']['Library']
type ItemSummary = components['schemas']['ItemSummary']

/** How many pages of movies the end screen reads to find the oldest and newest (200 each). */
const maxPages = 10

/** The oldest and newest movie by year; with one movie, none (F007 R14). */
export function range(movies: ItemSummary[]): { oldest: string; newest: string } | undefined {
  const dated = movies.filter((m) => m.year).sort((a, b) => (a.year ?? 0) - (b.year ?? 0))
  if (dated.length < 2) return undefined
  const title = (m: ItemSummary) => m.displayTitle ?? m.title
  return { oldest: title(dated[0]!), newest: title(dated[dated.length - 1]!) }
}

/** The end of the first scan: the wall of their posters, one serif line, one amber button (R14, R15). */
export function ScanEnd({
  libraries,
  states,
}: {
  libraries: Library[]
  states: Record<string, ScanState>
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { serverName } = useServer()
  const [movies, setMovies] = useState<ItemSummary[]>()
  const [fresh, setFresh] = useState<Library[]>(libraries)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      const { data } = await api.GET('/libraries').catch(() => ({ data: undefined }))
      const libs = data ?? libraries
      const all: ItemSummary[] = []
      for (const l of libs.filter((x) => x.type === 'movies')) {
        let cursor: string | undefined
        for (let page = 0; page < maxPages; page++) {
          const { data: items } = await api.GET('/libraries/{libraryId}/items', {
            params: { path: { libraryId: l.id }, query: { kind: 'movie', limit: 200, cursor } },
          })
          all.push(...(items?.items ?? []))
          cursor = items?.nextCursor
          if (!cursor) break
        }
      }
      if (!cancelled) {
        setFresh(libs)
        setMovies(all)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [libraries])

  const totals = fresh.reduce(
    (a, l) => ({
      movies: a.movies + l.counts.movies,
      shows: a.shows + l.counts.shows,
      episodes: a.episodes + l.counts.episodes,
      unrecognized: a.unrecognized + l.counts.unrecognized,
    }),
    { movies: 0, shows: 0, episodes: 0, unrecognized: 0 },
  )
  const review = libraries.reduce((n, l) => n + (states[l.id]?.metadata?.needsReview ?? 0), 0)
  if (totals.movies + totals.shows === 0) return <StatusScreen kind="zero" />

  const wall = (movies ?? []).filter((m) => m.poster).slice(0, 24)
  const span = movies && range(movies)
  const start = () => {
    setFlag(doneKey)
    void navigate('/')
  }

  return (
    <div className="pl end">
      <ul className="end__wall" aria-label={t('scan.complete.wall')}>
        {wall.map((m) => (
          <li key={m.id}>
            <img src={posterSrc(m.poster!.imageId)} alt={m.displayTitle ?? m.title} />
          </li>
        ))}
      </ul>
      <div className="end__scrim" aria-hidden="true" />
      <div className="end__text">
        <h1 className="end__title">
          {totals.movies > 0 && span
            ? t('scan.complete.title', {
                oldestTitle: span.oldest,
                newestTitle: span.newest,
                count: totals.movies,
              })
            : t('scan.complete.count', { count: totals.movies || totals.shows })}
        </h1>
        {totals.shows > 0 && totals.movies > 0 && (
          <p>
            {t('scan.complete.shows', {
              shows: t('scan.complete.showCount', { count: totals.shows }),
              episodes: t('scan.complete.episodeCount', { count: totals.episodes }),
              serverName,
            })}
          </p>
        )}
        {(totals.unrecognized > 0 || review > 0) && (
          <p>
            {totals.unrecognized > 0 && review > 0
              ? t('scan.later.both', { unrecognized: totals.unrecognized, review })
              : totals.unrecognized > 0
                ? t('scan.later.unrecognized', { count: totals.unrecognized })
                : t('scan.later.review', { count: review })}{' '}
            <Link to="/admin/libraries" style={{ color: 'var(--text-primary)', fontWeight: 600 }}>
              {t('scan.open', { library: t('libraries.title') })}
            </Link>
          </p>
        )}
        <Button onClick={start}>{t('scan.complete.watch')}</Button>
      </div>
    </div>
  )
}
