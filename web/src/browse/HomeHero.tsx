import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { formatDuration } from '../lib/format'
import { Icon } from '../ui'
import { firstYear } from './MovieHeader'
import { imageSrc, imageSrcSet, progress, watchLink } from './media'
import { Img } from './Img'
import { useParallax } from './useParallax'

type ItemSummary = components['schemas']['ItemSummary']
type ItemDetail = components['schemas']['ItemDetail']

/**
 * The home's framed hero (V2): the ambient title's backdrop in a frame,
 * its kicker, title, facts, two lines of synopsis, the one amber button
 * and the credits. `resume` is the Continue watching item it stands for
 * (an episode stands for its show), so the button resumes it.
 */
export function HomeHero({
  item,
  resume,
  libraryName,
}: {
  item: ItemSummary
  resume?: ItemSummary
  libraryName?: string
}) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const [detail, setDetail] = useState<ItemDetail | null>()
  const hero = useRef<HTMLDivElement>(null)
  useParallax(hero)

  useEffect(() => {
    let cancelled = false
    api
      .GET('/items/{itemId}', { params: { path: { itemId: item.id } } })
      .then(({ data }) => !cancelled && setDetail(data ?? null))
      .catch(() => !cancelled && setDetail(null))
    return () => {
      cancelled = true
    }
  }, [item.id])

  const backdrop = item.backdrop
  const m = detail?.metadata
  const title = item.displayTitle || item.title || ''

  const meta: string[] = []
  const year = firstYear(item)
  if (year) meta.push(String(year))
  const runtime = m?.runtimeMinutes ?? item.runtimeMinutes
  if (runtime && item.kind === 'movie') meta.push(formatDuration(lng, runtime * 60000))
  if (item.certification) meta.push(item.certification)
  if (item.genres.length) meta.push(item.genres.slice(0, 2).join(', '))

  const list = (names: string[]) => new Intl.ListFormat(lng, { type: 'conjunction' }).format(names)
  const cast = m?.credits.filter((c) => c.role === 'cast').slice(0, 3) ?? []
  const leads = m?.credits.filter((c) => c.role === 'director' || c.role === 'creator') ?? []
  const credits = [
    cast.length ? t('home.hero.with', { names: list(cast.map((c) => c.name)) }) : '',
    leads.length
      ? leads[0]?.role === 'creator'
        ? t('item.createdBy', { names: list(leads.map((c) => c.name)) })
        : t('item.directedBy', { names: list(leads.map((c) => c.name)) })
      : '',
  ].filter(Boolean)

  const p = resume ? progress(resume) : undefined
  const episode = resume?.kind === 'episode'
  const kicker = resume
    ? t('home.row.continueWatching')
    : libraryName
      ? t('home.hero.newIn', { library: libraryName })
      : undefined
  const cta = resume
    ? episode && resume.seasonNumber !== undefined && resume.episodeNumber !== undefined
      ? t('item.resumeEpisode', { season: resume.seasonNumber, episode: resume.episodeNumber })
      : t('home.hero.resume')
    : t('item.play')
  const playable = resume ?? (item.kind === 'movie' ? item : undefined)
  const glow = backdrop?.colors?.dominant

  return (
    <div
      ref={hero}
      className="br-hero"
      style={glow ? ({ '--br-glow': glow } as React.CSSProperties) : undefined}
    >
      {backdrop && (
        <Img
          className="br-hero__art"
          src={imageSrc(backdrop.imageId, 1280)}
          srcSet={imageSrcSet(backdrop.imageId, [780, 1280, 1920])}
          sizes="100vw"
          alt=""
        />
      )}
      <div className="br-hero__shade" aria-hidden="true" />
      <div className="br-hero__body">
        {kicker && <p className="br-hero__kicker">{kicker}</p>}
        <h1 className="br-hero__title">{title}</h1>
        {meta.length > 0 && <p className="br-hero__meta">{meta.join(t('card.separator'))}</p>}
        {detail === undefined ? (
          // The synopsis is on its way: two quiet lines hold its place.
          <span className="br-hero__wait" aria-hidden="true">
            <span className="br-skeleton" />
            <span className="br-skeleton" />
          </span>
        ) : (
          m?.overview && <p className="br-hero__synopsis">{m.overview}</p>
        )}
        <div className="br-hero__actions">
          {playable ? (
            <Link to={watchLink(playable)} className="br-hero__btn br-hero__btn--play pl-focusable">
              <Icon name="play" size={18} />
              {cta}
            </Link>
          ) : null}
          <Link
            to={`/item/${item.id}`}
            className={`br-hero__btn pl-focusable ${playable ? 'br-glass' : 'br-hero__btn--play'}`}
          >
            {t('card.moreInfo')}
          </Link>
          {p?.leftMs !== undefined && (
            <span className="br-hero__progress">
              <span className="br-hero__track" aria-hidden="true">
                <span style={{ width: `${Math.round(p.fraction * 100)}%` }} />
              </span>
              {t('card.timeLeft', { duration: formatDuration(lng, p.leftMs) })}
            </span>
          )}
        </div>
      </div>
      {credits.length > 0 && (
        <p className="br-hero__credits">{credits.join(t('card.separator'))}</p>
      )}
    </div>
  )
}
