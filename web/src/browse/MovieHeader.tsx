import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { formatDuration, languageName } from '../lib/format'
import { Tag } from '../ui'
import { minVotes, resolutionName } from './itemFacts'
import { imageSrc, imageSrcSet } from './media'

type ItemDetail = components['schemas']['ItemDetail']

/** The year from the name, else from TMDB's release or first air date. */
export function firstYear(item: { year?: number; releaseDate?: string }): number | undefined {
  return item.year ?? (item.releaseDate ? Number(item.releaseDate.slice(0, 4)) : undefined)
}

const hdrNames: Record<string, string> = {
  hdr10: 'HDR10',
  hdr10plus: 'HDR10+',
  dolby_vision: 'Dolby Vision',
  hlg: 'HLG',
}

/**
 * The detail header (F008-R17, R21): the backdrop fading into the page,
 * the poster, the logo or title, the facts, the actions (`actions`), then
 * the overview and credits. Fields that fell back to English say so with
 * lang="en".
 */
export function MovieHeader({
  item,
  facts,
  actions,
  notice,
}: {
  item: ItemDetail
  /** Extra facts after the year (a show's status and seasons). */
  facts?: string[]
  actions: ReactNode
  notice?: ReactNode
}) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const m = item.metadata
  const fallback = (field: string) =>
    m?.fieldLanguages[field] ? m.fieldLanguages[field] : undefined
  const backdrop = item.backdrop ?? m?.backdrops[0]
  const title = item.displayTitle || item.title || ''

  const meta: string[] = []
  const year = firstYear(item)
  if (year) meta.push(String(year))
  meta.push(...(facts ?? []))
  if (m?.runtimeMinutes && item.kind === 'movie')
    meta.push(formatDuration(lng, m.runtimeMinutes * 60000))
  if (item.certification) meta.push(item.certification)
  if (item.rating && (m?.ratingCount ?? 0) >= minVotes)
    meta.push(t('item.rating', { rating: item.rating, count: m?.ratingCount ?? 0 }))

  const tags: string[] = []
  const res = resolutionName(item.technical.resolution?.width, item.technical.resolution?.height)
  if (res) tags.push(res)
  for (const h of item.technical.hdrFormats) tags.push(hdrNames[h] ?? h)
  const audio = item.technical.audioLanguages.map((l) => languageName(lng, l))

  const directors = m?.credits.filter((c) => c.role === 'director').map((c) => c.name) ?? []
  const writers = m?.credits.filter((c) => c.role === 'writer').map((c) => c.name) ?? []
  const creators = m?.credits.filter((c) => c.role === 'creator').map((c) => c.name) ?? []
  const list = (names: string[]) => new Intl.ListFormat(lng, { type: 'conjunction' }).format(names)

  return (
    <section className="br-detail">
      {backdrop && (
        <div className="br-detail__backdrop" aria-hidden="true">
          <img
            src={imageSrc(backdrop.imageId, 1280)}
            srcSet={imageSrcSet(backdrop.imageId, [780, 1280, 1920])}
            sizes="100vw"
            alt=""
          />
          <div
            className="br-detail__scrim"
            style={
              backdrop.colors
                ? ({ '--br-dark': backdrop.colors.dark } as React.CSSProperties)
                : undefined
            }
          />
        </div>
      )}
      <div className="br-detail__body">
        <div className="br-detail__poster">
          {item.poster ? (
            <img
              src={imageSrc(item.poster.imageId, 342)}
              srcSet={imageSrcSet(item.poster.imageId, [342, 500, 780])}
              sizes="240px"
              alt=""
            />
          ) : (
            <span className="br-card__plate">{title}</span>
          )}
        </div>
        <div className="br-detail__info">
          {m?.logo ? (
            <h1 className="br-detail__logo">
              <img src={imageSrc(m.logo.imageId, 500)} alt={title} />
            </h1>
          ) : (
            <h1 className="br-detail__title" lang={fallback('title')}>
              {title}
            </h1>
          )}
          {m?.originalTitle && m.originalTitle !== title && (
            <p className="br-detail__orig">{t('item.originalTitle', { title: m.originalTitle })}</p>
          )}
          {meta.length > 0 && <p className="br-detail__meta">{meta.join(t('card.separator'))}</p>}
          {m && m.genres.length > 0 && (
            <p className="br-detail__genres">
              {m.genres.map((g, i) => (
                <span key={`${g.id}-${g.name}`}>
                  {i > 0 && t('card.separator')}
                  {g.id > 0 && item.libraryId ? (
                    <Link to={`/library/${item.libraryId}?genre=${g.id}`}>{g.name}</Link>
                  ) : (
                    g.name
                  )}
                </span>
              ))}
            </p>
          )}
          {(tags.length > 0 || audio.length > 0) && (
            <div className="br-detail__tags">
              {tags.map((x) => (
                <Tag key={x}>{x}</Tag>
              ))}
              {audio.length > 0 && <span>{t('item.tech.audio', { languages: list(audio) })}</span>}
            </div>
          )}
          {m?.tagline && (
            <p className="br-detail__tagline" lang={fallback('tagline')}>
              {m.tagline}
            </p>
          )}
          <div className="br-detail__actions">{actions}</div>
          {notice}
          {m?.overview && (
            <p className="br-detail__overview" lang={fallback('overview')}>
              {m.overview}
            </p>
          )}
          {(directors.length > 0 || writers.length > 0 || creators.length > 0) && (
            <p className="br-detail__credits">
              {[
                creators.length ? t('item.createdBy', { names: list(creators) }) : '',
                directors.length ? t('item.directedBy', { names: list(directors) }) : '',
                writers.length ? t('item.writtenBy', { names: list(writers) }) : '',
              ]
                .filter(Boolean)
                .join(t('card.separator'))}
            </p>
          )}
          {m && m.studios.length > 0 && <p className="br-detail__studios">{list(m.studios)}</p>}
        </div>
      </div>
    </section>
  )
}
