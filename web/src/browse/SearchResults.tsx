import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { firstYear } from './MovieHeader'
import { initials } from './CastRow'
import { imageSrc } from './media'

type SearchResults = components['schemas']['SearchResults']
type ItemSummary = components['schemas']['ItemSummary']

export type Hit = { id: string; href: string; group: 'movies' | 'shows' | 'episodes' | 'people' }

/** The results in the order the keyboard walks them. */
export function flatHits(r?: SearchResults): Hit[] {
  if (!r) return []
  return [
    ...r.movies.map((i) => ({ id: i.id, href: `/item/${i.id}`, group: 'movies' as const })),
    ...r.shows.map((i) => ({ id: i.id, href: `/item/${i.id}`, group: 'shows' as const })),
    ...r.episodes.map((i) => ({ id: i.id, href: `/item/${i.id}`, group: 'episodes' as const })),
    ...r.people.map((p) => ({ id: p.id, href: `/person/${p.id}`, group: 'people' as const })),
  ]
}

/**
 * Search results grouped Movies, Shows, Episodes, People (F008-R25), as a
 * listbox: `active` is the keyboard's option, `onPick` follows a link.
 */
export function SearchResultList({
  id,
  query,
  results,
  active,
  onPick,
}: {
  id: string
  query: string
  results?: SearchResults
  active?: number
  onPick?: () => void
}) {
  const { t } = useTranslation()
  if (!results) return null
  const hits = flatHits(results)
  if (hits.length === 0) {
    return (
      <p className="br-search__empty" role="status">
        {t('search.empty', { query })}
      </p>
    )
  }
  let n = -1
  const option = (key: string, href: string, body: React.ReactNode) => {
    n++
    return (
      <li key={key} role="option" id={`${id}-${n}`} aria-selected={n === active}>
        <Link to={href} className="br-search__hit" tabIndex={-1} onClick={onPick}>
          {body}
        </Link>
      </li>
    )
  }
  const item = (i: ItemSummary, sub?: string) =>
    option(
      i.id,
      `/item/${i.id}`,
      <>
        {i.poster || i.still || i.backdrop ? (
          <img src={imageSrc((i.poster ?? i.still ?? i.backdrop)!.imageId, 92)} alt="" />
        ) : (
          <span className="br-search__thumb" aria-hidden="true" />
        )}
        <span>
          <strong>{i.displayTitle || i.title}</strong>
          {sub && <span>{sub}</span>}
        </span>
      </>,
    )
  const groups: { key: keyof SearchResults; label: string }[] = [
    { key: 'movies', label: t('search.group.movies') },
    { key: 'shows', label: t('search.group.shows') },
    { key: 'episodes', label: t('search.group.episodes') },
    { key: 'people', label: t('search.group.people') },
  ]
  return (
    <ul className="br-search__results" role="listbox" id={id} aria-label={t('search.label')}>
      {groups.map((g) => {
        if (results[g.key].length === 0) return null
        return (
          <li key={g.key} role="presentation">
            <span className="br-search__group" id={`${id}-${g.key}`}>
              {g.label}
            </span>
            <ul role="group" aria-labelledby={`${id}-${g.key}`}>
              {g.key === 'people'
                ? results.people.map((p) =>
                    option(
                      p.id,
                      `/person/${p.id}`,
                      <>
                        {p.portrait ? (
                          <img
                            className="br-search__round"
                            src={imageSrc(p.portrait.imageId, 92)}
                            alt=""
                          />
                        ) : (
                          <span className="br-search__thumb br-search__round" aria-hidden="true">
                            {initials(p.name)}
                          </span>
                        )}
                        <span>
                          <strong>{p.name}</strong>
                        </span>
                      </>,
                    ),
                  )
                : g.key === 'episodes'
                  ? results.episodes.map((e) =>
                      item(
                        e,
                        e.show && e.seasonNumber !== undefined && e.episodeNumber !== undefined
                          ? t('search.episodeOf', {
                              show: e.show.displayTitle,
                              season: e.seasonNumber,
                              episode: e.episodeNumber,
                            })
                          : e.show?.displayTitle,
                      ),
                    )
                  : results[g.key].map((i) => item(i, firstYear(i)?.toString()))}
            </ul>
          </li>
        )
      })}
    </ul>
  )
}
