import { useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { formatDuration } from '../lib/format'
import { minVotes, resolutionName } from './itemFacts'
import { imageSrc, imageSrcSet } from './media'
import { Img } from './Img'
import { useParallax } from './useParallax'
import { PageLight } from './PageLight'
import { HeroPreview } from './HeroPreview'

type ItemDetail = components['schemas']['ItemDetail']

/** The year from the name, else from TMDB's release or first air date. */
export function firstYear(item: { year?: number; releaseDate?: string }): number | undefined {
  return item.year ?? (item.releaseDate ? Number(item.releaseDate.slice(0, 4)) : undefined)
}

export const hdrNames: Record<string, string> = {
  hdr10: 'HDR10',
  hdr10plus: 'HDR10+',
  dolby_vision: 'Dolby Vision',
  hlg: 'HLG',
}

/**
 * The detail header (F008-R17, R21; V2): the page lit by the backdrop,
 * then the backdrop in a frame with the kicker, title, facts, tech tags,
 * three lines of synopsis, the actions (`actions`) and the credits on the
 * right. Fields that fell back to English say so with lang="en".
 */
export function MovieHeader({
  item,
  facts,
  kicker,
  actions,
  notice,
  credits: creditsLine,
  tall,
}: {
  item: ItemDetail
  /** Extra facts after the year (a show's status and seasons). */
  facts?: string[]
  /** Above the title (an episode's show and season). */
  kicker?: ReactNode
  actions: ReactNode
  notice?: ReactNode
  /** Replaces the cast and crew line (an episode's "Next"). */
  credits?: string
  /** The episode page: the hero is the whole page. */
  tall?: boolean
}) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const hero = useRef<HTMLElement>(null)
  useParallax(hero)
  const m = item.metadata
  const fallback = (field: string) =>
    m?.fieldLanguages[field] ? m.fieldLanguages[field] : undefined
  const backdrop = item.backdrop ?? m?.backdrops[0] ?? item.still
  const title = item.displayTitle || item.title || ''

  const meta: string[] = []
  const year = firstYear(item)
  if (year) meta.push(String(year))
  meta.push(...(facts ?? []))
  const runtime = m?.runtimeMinutes ?? (item.kind === 'episode' ? item.runtimeMinutes : undefined)
  if (runtime && item.kind !== 'show') meta.push(formatDuration(lng, runtime * 60000))
  if (item.certification) meta.push(item.certification)
  if (item.rating && (m?.ratingCount ?? 0) >= minVotes)
    meta.push(t('item.rating', { rating: item.rating, count: m?.ratingCount ?? 0 }))

  const tags: string[] = []
  const res = resolutionName(item.technical.resolution?.width, item.technical.resolution?.height)
  if (res) tags.push(res)
  for (const h of item.technical.hdrFormats) tags.push(hdrNames[h] ?? h)

  const list = (names: string[]) => new Intl.ListFormat(lng, { type: 'conjunction' }).format(names)
  const names = (role: string) => m?.credits.filter((c) => c.role === role).map((c) => c.name) ?? []
  const cast = names('cast').slice(0, 3)
  const directors = names('director')
  const writers = names('writer')
  const creators = names('creator')
  const credits =
    creditsLine ??
    [
      cast.length ? t('home.hero.with', { names: list(cast) }) : '',
      creators.length ? t('item.createdBy', { names: list(creators) }) : '',
      directors.length ? t('item.directedBy', { names: list(directors) }) : '',
      writers.length ? t('item.writtenBy', { names: list(writers) }) : '',
    ]
      .filter(Boolean)
      .join(t('card.separator'))
  const glow = backdrop?.colors?.dominant

  return (
    <>
      <PageLight art={backdrop} />
      <section
        ref={hero}
        className={`br-detail${tall ? ' br-detail--tall' : ''}`}
        style={glow ? ({ '--br-glow': glow } as React.CSSProperties) : undefined}
      >
        {/* The picture is clipped by its own frame (it drifts with the
            parallax); the hero itself doesn't clip, so the Edit menu hangs out.
            A movie or show plays a few of its scenes over the backdrop. */}
        <div className="br-detail__frame">
          {backdrop && (
            <Img
              className="br-detail__art"
              src={imageSrc(backdrop.imageId, 1280)}
              srcSet={imageSrcSet(backdrop.imageId, [780, 1280, 1920])}
              sizes="100vw"
              alt=""
            />
          )}
          {(item.kind === 'movie' || item.kind === 'show') && <HeroPreview itemId={item.id} />}
          <div className="br-hero__shade" aria-hidden="true" />
        </div>
        <div className="br-detail__body">
          {kicker && <p className="br-detail__kicker">{kicker}</p>}
          <h1 className="br-detail__title" lang={fallback('title')}>
            {title}
          </h1>
          {m?.originalTitle && m.originalTitle !== title && (
            <p className="br-detail__orig">{t('item.originalTitle', { title: m.originalTitle })}</p>
          )}
          {meta.length > 0 && <p className="br-detail__meta">{meta.join(t('card.separator'))}</p>}
          {m && m.genres.length > 0 && (
            <p className="br-detail__genres">
              {m.genres.map((g, i) => (
                <span key={`${g.id}-${g.name}`}>
                  {i > 0 && ', '}
                  {g.id > 0 && item.libraryId ? (
                    <Link to={`/library/${item.libraryId}?genre=${g.id}`}>{g.name}</Link>
                  ) : (
                    g.name
                  )}
                </span>
              ))}
            </p>
          )}
          {tags.length > 0 && (
            <div className="br-detail__tags">
              {tags.map((x) => (
                <span key={x} className="br-tag">
                  {x}
                </span>
              ))}
            </div>
          )}
          {(m?.overview || item.overview) && (
            <p className="br-detail__overview" lang={fallback('overview')}>
              {m?.overview || item.overview}
            </p>
          )}
          <div className="br-detail__actions">{actions}</div>
          {notice}
        </div>
        {credits && <p className="br-hero__credits">{credits}</p>}
      </section>
    </>
  )
}
