import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { formatDuration } from '../lib/format'
import { Icon } from '../ui'
import { Thread } from './cards/Thread'
import { imageSrc, imageSrcSet, progress } from './media'
import { Img } from './Img'

type ItemSummary = components['schemas']['ItemSummary']
type ItemDetail = components['schemas']['ItemDetail']
type UserState = components['schemas']['UserState']

/** The episode a show's main button plays (F008-R21, R32), again after each change. */
export function useUpNext(showId: string, state?: UserState): ItemSummary | undefined {
  const [ep, setEp] = useState<ItemSummary>()
  useEffect(() => {
    let cancelled = false
    void api
      .GET('/items/{itemId}/up-next', { params: { path: { itemId: showId } } })
      .then(({ data }) => {
        if (!cancelled) setEp(data?.item)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [showId, state])
  return ep
}

/** Seasons in order, specials last (canvas rule). */
function bySeason(a: ItemSummary, b: ItemSummary): number {
  const n = (s: ItemSummary) => (s.seasonNumber ? s.seasonNumber : 1e6)
  return n(a) - n(b)
}

/** An episode's air date: TMDB's, else the one in its file name. */
function aired(e: ItemSummary): Date | undefined {
  const d = e.releaseDate ?? e.airDate
  return d ? new Date(`${d}T12:00:00`) : undefined
}

/** More seasons than this: the pills stop and "All seasons" opens the number grid (V2). */
const pillsUpTo = 8

/**
 * A show's seasons and episodes (F008-R21, V2): seasons as pills (in
 * `?season=`; past eight, the last pill opens the number grid), the
 * season's facts and "Mark season as watched", then its episodes as 16:9
 * cards that play.
 */
export function ShowSeasons({
  show,
  upNextSeason,
  onChange,
}: {
  show: ItemDetail
  upNextSeason?: number
  onChange: () => Promise<void>
}) {
  const { t, i18n } = useTranslation()
  const [params, setParams] = useSearchParams()
  const [seasons, setSeasons] = useState<ItemSummary[]>()
  const [episodes, setEpisodes] = useState<{ season: string; items: ItemSummary[] }>()
  const [picking, setPicking] = useState(false)
  const [version, setVersion] = useState(0)
  const libraryId = show.libraryId ?? ''
  // The season shown before this one, for the direction of travel.
  const [shown, setShown] = useState<{ id?: string; at?: number; dir?: 'left' | 'right' }>({})

  useEffect(() => {
    let cancelled = false
    void api
      .GET('/libraries/{libraryId}/items', {
        params: { path: { libraryId }, query: { parentId: show.id, limit: 200 } },
      })
      .then(({ data }) => !cancelled && setSeasons([...(data?.items ?? [])].sort(bySeason)))
      .catch(() => !cancelled && setSeasons([]))
    return () => {
      cancelled = true
    }
  }, [libraryId, show.id, version])

  const wanted = params.get('season')
  const season =
    seasons?.find((s) => String(s.seasonNumber ?? 0) === wanted) ??
    seasons?.find((s) => s.seasonNumber === upNextSeason) ??
    seasons?.find((s) => s.seasonNumber) ??
    seasons?.[0]

  useEffect(() => {
    if (!season) return
    let cancelled = false
    void api
      .GET('/libraries/{libraryId}/items', {
        params: { path: { libraryId }, query: { parentId: season.id, limit: 200 } },
      })
      .then(
        ({ data }) => !cancelled && setEpisodes({ season: season.id, items: data?.items ?? [] }),
      )
      .catch(() => !cancelled && setEpisodes({ season: season.id, items: [] }))
    return () => {
      cancelled = true
    }
  }, [libraryId, season, version])

  if (!seasons) {
    // Seasons on their way: quiet pills and cards hold their place.
    return (
      <section className="br-seasons" aria-busy="true" aria-label={t('common.loading')}>
        <div className="br-pills">
          {[0, 1, 2].map((i) => (
            <span key={i} className="br-skeleton br-pill-skeleton" />
          ))}
        </div>
      </section>
    )
  }
  if (seasons.length === 0 || !season) return null
  const seasonName = (s: ItemSummary) =>
    s.seasonNumber ? t('item.season', { number: s.seasonNumber }) : t('item.specials')
  const st = season.userState
  const watched = st?.watchedEpisodes ?? 0
  const total = st?.totalEpisodes ?? season.childCount
  const many = seasons.length > pillsUpTo
  // Past eight, the pills show the first ones, and the current one when it is further.
  let pills = many ? seasons.slice(0, pillsUpTo - 1) : seasons
  if (many && !pills.includes(season)) pills = [...pills.slice(0, -1), season]
  const numbered = seasons.filter((s) => s.seasonNumber).length
  const hasSpecials = seasons.length > numbered

  const pick = (s: ItemSummary) => {
    setPicking(false)
    const q = new URLSearchParams(params)
    q.set('season', String(s.seasonNumber ?? 0))
    setParams(q, { replace: true })
  }

  async function markSeason() {
    const params = { params: { path: { itemId: season!.id } } }
    if (st?.watched) await api.DELETE('/items/{itemId}/watched', params)
    else await api.PUT('/items/{itemId}/watched', params)
    setVersion((v) => v + 1)
    await onChange()
  }

  const loaded = episodes?.season === season.id ? episodes.items : undefined
  // Going to a later season slides the episodes in from the right (V2).
  const at = seasons.indexOf(season)
  if (shown.id !== season.id) {
    setShown({
      id: season.id,
      at,
      dir: shown.at === undefined ? undefined : at > shown.at ? 'right' : 'left',
    })
  }
  const dir = shown.dir
  return (
    <section className="br-seasons" aria-label={t('item.seasonPicker.label')}>
      <div className="br-pills" role="tablist" aria-label={t('item.seasonPicker.label')}>
        {pills.map((s) => (
          <button
            key={s.id}
            type="button"
            role="tab"
            aria-selected={s.id === season.id}
            className="br-pill pl-focusable"
            onClick={() => pick(s)}
          >
            {seasonName(s)}
            {s.userState?.watched && <Icon name="check" size={14} />}
          </button>
        ))}
        {many && (
          <span className="br-filters__anchor">
            <button
              type="button"
              className="br-pill pl-focusable"
              aria-haspopup="dialog"
              aria-expanded={picking}
              onClick={() => setPicking((p) => !p)}
            >
              {hasSpecials
                ? t('item.seasonPicker.allWithSpecials', { count: numbered })
                : t('item.seasonPicker.all', { count: numbered })}
              <Icon name="chevronDown" size={16} />
            </button>
            {picking && (
              <SeasonPicker
                seasons={seasons}
                current={season}
                name={seasonName}
                onPick={pick}
                onClose={() => setPicking(false)}
              />
            )}
          </span>
        )}
      </div>
      <div className="br-seasons__head">
        <span className="br-seasons__info">
          {[
            season.year ? String(season.year) : '',
            t('item.episodes', { count: total }),
            watched ? t('item.watchedCount', { count: watched }) : '',
          ]
            .filter(Boolean)
            .join(t('card.separator'))}
        </span>
        <button type="button" className="br-link pl-focusable" onClick={() => void markSeason()}>
          {st?.watched ? t('item.markSeasonUnwatched') : t('item.markSeasonWatched')}
        </button>
      </div>
      <ol key={season.id} className="br-row__list br-episodes" data-dir={dir} aria-busy={!loaded}>
        {!loaded &&
          [0, 1, 2, 3].map((i) => (
            <li key={i} className="br-episode" aria-hidden="true">
              <span className="br-episode__art br-skeleton" />
              <span className="br-skeleton br-skeleton--title" />
            </li>
          ))}
        {loaded?.map((e) => {
          const p = progress(e)
          const title = e.displayTitle || e.title || ''
          const when = aired(e)
          const label =
            e.episodeNumber !== undefined
              ? t('item.episodeTitle', { number: e.episodeNumber, title })
              : title
          const state = e.userState?.watched
            ? t('card.watched')
            : p?.leftMs !== undefined
              ? t('item.episode.inProgress', { duration: formatDuration(i18n.language, p.leftMs) })
              : ''
          return (
            <li key={e.id}>
              <Link to={`/watch/${e.id}`} className="br-episode pl-focusable" aria-label={label}>
                <span className="br-episode__art">
                  {e.still ? (
                    <Img
                      src={imageSrc(e.still.imageId, 500)}
                      srcSet={imageSrcSet(e.still.imageId, [342, 500, 780])}
                      sizes="(max-width: 767px) 260px, 316px"
                      alt=""
                      loading="lazy"
                    />
                  ) : (
                    <span className="br-card__plate" aria-hidden="true">
                      {t('card.episodeShort', {
                        season: season.seasonNumber ?? 0,
                        episode: e.episodeNumber ?? 0,
                      })}
                    </span>
                  )}
                  {e.runtimeMinutes && (
                    <span className="br-episode__runtime">
                      {formatDuration(i18n.language, e.runtimeMinutes * 60000)}
                    </span>
                  )}
                  {p && <Thread fraction={p.fraction} />}
                  <span className="br-episode__play" aria-hidden="true">
                    <Icon name="play" size={22} />
                  </span>
                </span>
                <span className="br-episode__text">
                  <span className="br-episode__kicker">
                    {[
                      e.episodeNumber !== undefined
                        ? t('item.episodeNumber', { number: e.episodeNumber })
                        : '',
                      state,
                    ]
                      .filter(Boolean)
                      .join(t('card.separator'))}
                  </span>
                  <strong>{title}</strong>
                  {e.overview && <span className="br-episode__overview">{e.overview}</span>}
                  {when && (
                    <span className="br-episode__date">
                      {new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' }).format(when)}
                    </span>
                  )}
                </span>
              </Link>
            </li>
          )
        })}
      </ol>
    </section>
  )
}

function SeasonPicker({
  seasons,
  current,
  name,
  onPick,
  onClose,
}: {
  seasons: ItemSummary[]
  current: ItemSummary
  name: (s: ItemSummary) => string
  onPick: (s: ItemSummary) => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const ref = useRef<HTMLDivElement>(null)
  const grid = seasons.length > pillsUpTo
  useEffect(() => {
    let typed = ''
    let timer: ReturnType<typeof setTimeout> | undefined
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      // Typing a number jumps to that season.
      if (/^\d$/.test(e.key)) {
        typed += e.key
        clearTimeout(timer)
        timer = setTimeout(() => (typed = ''), 800)
        const s = seasons.find((x) => String(x.seasonNumber) === typed)
        if (s) ref.current?.querySelector<HTMLButtonElement>(`[data-season="${s.id}"]`)?.focus()
      }
    }
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    ref.current?.querySelector<HTMLButtonElement>('[aria-pressed="true"]')?.focus()
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onDown)
      clearTimeout(timer)
    }
  }, [seasons, onClose])
  return (
    <div
      ref={ref}
      className="pl pl-menu br-panel br-seasonpicker"
      role="dialog"
      aria-label={t('item.seasonPicker.label')}
    >
      <div className="br-seasonpicker__head">
        <strong>
          {seasons.some((s) => !s.seasonNumber)
            ? t('item.seasonPicker.title', { count: seasons.filter((s) => s.seasonNumber).length })
            : t('item.seasons', { count: seasons.length })}
        </strong>
        {grid && <span>{t('item.seasonPicker.jump')}</span>}
      </div>
      <div className={grid ? 'br-seasonpicker__grid' : 'br-seasonpicker__list'}>
        {seasons.map((s) => (
          <button
            key={s.id}
            type="button"
            data-season={s.id}
            aria-pressed={s.id === current.id}
            aria-label={name(s)}
            className="br-seasonpicker__item pl-focusable"
            onClick={() => onPick(s)}
          >
            {grid ? (s.seasonNumber ? s.seasonNumber : name(s)) : name(s)}
            {s.userState?.watched && <Icon name="check" size={14} />}
          </button>
        ))}
      </div>
    </div>
  )
}
