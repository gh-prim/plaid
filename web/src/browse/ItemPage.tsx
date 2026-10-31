import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Navigate, useParams } from 'react-router'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import type { components } from '../api/schema'
import { formatBytes, formatClock, formatDuration } from '../lib/format'
import { Icon } from '../ui'
import { PosterCard } from './cards/PosterCard'
import { CastRow } from './CastRow'
import { EditMenu } from './EditMenu'
import { presentFiles, pickTrailer, resolutionName } from './itemFacts'
import { progress, watchLink } from './media'
import { AboutSection } from './AboutSection'
import { firstYear, MovieHeader } from './MovieHeader'
import { ShowSeasons, useUpNext } from './ShowSeasons'
import { TrailerSlot } from './TrailerSlot'
import { Unavailable } from './Unavailable'

type ItemDetail = components['schemas']['ItemDetail']
type CollectionDetail = components['schemas']['CollectionDetail']

const pendingEvery = 3000

/** A movie or show page (F008-R17–R21, R24). Seasons and episodes open their show. */
export function ItemPage() {
  const { t } = useTranslation()
  const { itemId = '' } = useParams()
  const [item, setItem] = useState<ItemDetail | null>()

  const load = useCallback(async () => {
    const { data } = await api.GET('/items/{itemId}', { params: { path: { itemId } } })
    setItem(data ?? null)
  }, [itemId])
  useEffect(() => {
    let cancelled = false
    api
      .GET('/items/{itemId}', { params: { path: { itemId } } })
      .then(({ data }) => {
        if (!cancelled) setItem(data ?? null)
      })
      .catch(() => !cancelled && setItem(null))
    return () => {
      cancelled = true
    }
  }, [itemId])

  // Details on their way (a fresh match, a scan): look again until they land.
  const pending = item?.matchStatus === 'pending'
  useEffect(() => {
    if (!pending) return
    const timer = setInterval(() => void load().catch(() => {}), pendingEvery)
    return () => clearInterval(timer)
  }, [pending, load])

  if (item === null) return <Unavailable />
  if (!item || item.id !== itemId)
    return <div className="br-page" aria-busy="true" aria-label={t('common.loading')} />
  if (item.kind === 'episode')
    return <EpisodePage key={item.matchStatus} item={item} reload={load} />
  if (item.kind === 'season' && item.show) {
    const q = item.seasonNumber !== undefined ? `?season=${item.seasonNumber}` : ''
    return <Navigate to={`/item/${item.show.id}${q}`} replace />
  }
  return item.kind === 'show' ? (
    // Keyed on the status: seasons and episodes load again once matched.
    <ShowPage key={item.matchStatus} item={item} reload={load} />
  ) : (
    <MoviePage key={item.matchStatus} item={item} reload={load} />
  )
}

function WatchedButton({ item, reload }: { item: ItemDetail; reload: () => Promise<void> }) {
  const { t } = useTranslation()
  const watched = item.userState?.watched ?? false
  async function toggle() {
    const params = { params: { path: { itemId: item.id } } }
    if (watched) await api.DELETE('/items/{itemId}/watched', params)
    else await api.PUT('/items/{itemId}/watched', params)
    await reload()
  }
  return (
    <button
      type="button"
      className="br-hero__btn br-glass pl-focusable"
      onClick={() => void toggle()}
    >
      {watched && <Icon name="check" size={18} />}
      {watched ? t('item.markUnwatched') : t('item.markWatched')}
    </button>
  )
}

/** The Edit menu, for admins only (F008-R23). */
function AdminEdit({ item, reload }: { item: ItemDetail; reload: () => Promise<void> }) {
  const { state } = useAuth()
  if (state.status !== 'ready' || state.me?.user.role !== 'admin') return null
  return <EditMenu item={item} onChange={reload} />
}

function MoviePage({ item, reload }: { item: ItemDetail; reload: () => Promise<void> }) {
  const { t, i18n } = useTranslation()
  const files = presentFiles(item.files)
  const [fileId, setFileId] = useState('')
  const missing = item.files.length > 0 && files.length === 0
  const position = item.userState?.positionMs ?? 0
  const file = fileId ? `fileId=${encodeURIComponent(fileId)}` : ''
  const watchPath = (extra: string) => {
    const q = [file, extra].filter(Boolean).join('&')
    return `/watch/${item.id}${q ? `?${q}` : ''}`
  }
  const trailer = pickTrailer(item.metadata?.trailers ?? [], i18n.language)

  const actions = (
    <>
      {!missing && files.length > 0 && (
        <Link
          to={watchPath(position > 0 ? `t=${Math.floor(position / 1000)}` : '')}
          className="br-hero__btn br-hero__btn--play pl-focusable"
        >
          <Icon name="play" size={18} />
          {position > 0
            ? t('playback.resume', { position: formatClock(position) })
            : t('item.play')}
        </Link>
      )}
      {!missing && position > 0 && (
        <Link to={watchPath('t=0')} className="br-hero__btn br-glass pl-focusable">
          {t('playback.startOver')}
        </Link>
      )}
      {files.length > 1 && (
        <label className="br-select br-version">
          <span>{t('item.version')}</span>
          <select value={fileId} onChange={(e) => setFileId(e.target.value)}>
            <option value="">{t('item.versions.auto')}</option>
            {files.map((f) => (
              <option key={f.id} value={f.id}>
                {[f.edition, resolutionName(f.width, f.height), formatBytes(i18n.language, f.size)]
                  .filter(Boolean)
                  .join(t('card.separator'))}
              </option>
            ))}
          </select>
        </label>
      )}
      <WatchedButton item={item} reload={reload} />
      <AdminEdit item={item} reload={reload} />
      <Left item={item} />
    </>
  )
  const notice = missing ? (
    <p role="status" className="br-notice">
      {t('item.filesMissing')}
    </p>
  ) : undefined

  return (
    <div className="br-item">
      <MovieHeader item={item} actions={actions} notice={notice} />
      {trailer && <TrailerSlot trailer={trailer} backdrop={item.backdrop} />}
      {item.metadata && <CastRow credits={item.metadata.credits} />}
      {item.metadata?.collection && (
        <CollectionRow id={item.metadata.collection.id} current={item.id} />
      )}
      <AboutSection item={item} />
    </div>
  )
}

function CollectionRow({ id, current }: { id: string; current: string }) {
  const { t } = useTranslation()
  const [c, setC] = useState<CollectionDetail>()
  useEffect(() => {
    let cancelled = false
    void api
      .GET('/collections/{collectionId}', { params: { path: { collectionId: id } } })
      .then(({ data }) => {
        if (!cancelled) setC(data)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [id])
  if (!c || c.items.length < 2) return null
  return (
    <section className="br-row" aria-label={c.name}>
      <div className="br-row__head">
        <h2 className="br-row__title">
          <Link
            to={`/collection/${c.id}`}
            className="br-row__all pl-focusable"
            aria-label={`${c.name}, ${t('home.row.seeAll')}`}
          >
            {c.name}
            <Icon name="chevronRight" size={18} />
          </Link>
        </h2>
      </div>
      <ul className="br-row__list">
        {c.items.map((it) => (
          <li key={it.id} aria-current={it.id === current ? 'page' : undefined}>
            <PosterCard item={it} sizes="(max-width: 767px) 120px, 204px" />
            {it.id === current && (
              <span className="br-current">{t('item.collection.thisMovie')}</span>
            )}
          </li>
        ))}
      </ul>
    </section>
  )
}

const statusKeys = {
  ended: 'item.status.ended',
  returning: 'item.status.returning',
  canceled: 'item.status.canceled',
  in_production: 'item.status.in_production',
} as const

function ShowPage({ item, reload }: { item: ItemDetail; reload: () => Promise<void> }) {
  const { t, i18n } = useTranslation()
  const m = item.metadata
  const upNext = useUpNext(item.id, item.userState)
  const facts: string[] = []
  const endYear = m?.endDate?.slice(0, 4)
  const year = firstYear(item)
  if (endYear && year && endYear !== String(year)) facts[0] = `${year}–${endYear}`
  if (m?.showStatus) facts.push(t(statusKeys[m.showStatus]))
  if (item.childCount) facts.push(t('item.seasons', { count: item.childCount }))
  const ep = upNext
  const actions = (
    <>
      {ep && ep.seasonNumber !== undefined && ep.episodeNumber !== undefined && (
        <Link to={watchLink(ep)} className="br-hero__btn br-hero__btn--play pl-focusable">
          <Icon name="play" size={18} />
          {(ep.userState?.positionMs ?? 0) > 0
            ? t('item.resumeEpisode', { season: ep.seasonNumber, episode: ep.episodeNumber })
            : t('item.playEpisode', { season: ep.seasonNumber, episode: ep.episodeNumber })}
        </Link>
      )}
      <WatchedButton item={item} reload={reload} />
      <AdminEdit item={item} reload={reload} />
      {ep && <Left item={ep} />}
    </>
  )
  const trailer = pickTrailer(m?.trailers ?? [], i18n.language)
  return (
    <div className="br-item">
      <MovieHeader
        item={{
          ...item,
          year: facts[0] ? undefined : year,
          releaseDate: facts[0] ? undefined : item.releaseDate,
        }}
        facts={facts}
        actions={actions}
      />
      <ShowSeasons show={item} upNextSeason={ep?.seasonNumber} onChange={reload} />
      {trailer && <TrailerSlot trailer={trailer} backdrop={item.backdrop} />}
      {m && <CastRow credits={m.credits} />}
      <AboutSection
        item={item}
        facts={[
          ...(item.childCount
            ? ([[t('item.about.seasons'), t('item.seasons', { count: item.childCount })]] as [
                string,
                string,
              ][])
            : []),
          ...(m?.showStatus
            ? ([[t('item.about.status'), t(statusKeys[m.showStatus])]] as [string, string][])
            : []),
        ]}
      />
    </div>
  )
}

/** The time left beside the actions: a thin track and words. */
function Left({
  item,
}: {
  item: { userState?: ItemDetail['userState'] } & Pick<ItemDetail, 'id'>
}) {
  const { t, i18n } = useTranslation()
  const p = progress(item as ItemDetail)
  if (p?.leftMs === undefined) return null
  return (
    <span className="br-hero__progress">
      <span className="br-hero__track" aria-hidden="true">
        <span style={{ width: `${Math.round(p.fraction * 100)}%` }} />
      </span>
      {t('card.timeLeft', { duration: formatDuration(i18n.language, p.leftMs) })}
    </span>
  )
}

/**
 * An episode (V2, TD04): the whole page is its hero, lit by its still or
 * its show, with its show and season above the title, Resume and Start over.
 */
function EpisodePage({ item, reload }: { item: ItemDetail; reload: () => Promise<void> }) {
  const { t, i18n } = useTranslation()
  const files = presentFiles(item.files)
  const position = item.userState?.positionMs ?? 0
  const facts: string[] = []
  if (item.seasonNumber !== undefined && item.episodeNumber !== undefined)
    facts.push(t('card.episodeShort', { season: item.seasonNumber, episode: item.episodeNumber }))
  const aired = item.releaseDate ?? item.airDate
  if (aired)
    facts.push(
      new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' }).format(
        new Date(`${aired}T12:00:00`),
      ),
    )
  const show = item.show
  const kicker = show ? (
    <Link
      to={`/item/${show.id}${item.seasonNumber !== undefined ? `?season=${item.seasonNumber}` : ''}`}
    >
      {[
        show.displayTitle,
        item.seasonNumber ? t('item.season', { number: item.seasonNumber }) : t('item.specials'),
      ].join(t('card.separator'))}
    </Link>
  ) : undefined
  const actions = (
    <>
      {files.length > 0 && (
        <Link to={watchLink(item)} className="br-hero__btn br-hero__btn--play pl-focusable">
          <Icon name="play" size={18} />
          {position > 0
            ? t('playback.resume', { position: formatClock(position) })
            : t('item.play')}
        </Link>
      )}
      {files.length > 0 && position > 0 && (
        <Link to={`/watch/${item.id}?t=0`} className="br-hero__btn br-glass pl-focusable">
          {t('playback.startOver')}
        </Link>
      )}
      <WatchedButton item={item} reload={reload} />
      <Left item={item} />
    </>
  )
  return (
    <div className="br-item">
      <MovieHeader
        item={{ ...item, year: undefined, releaseDate: undefined }}
        facts={facts}
        kicker={kicker}
        actions={actions}
        tall
      />
    </div>
  )
}
