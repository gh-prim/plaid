import { useSearchParams } from 'react-router'
import type { components } from '../api/schema'

type SearchResults = components['schemas']['SearchResults']

/** The search field's id, which the "/" shortcut focuses. */
export const searchFieldId = 'br-search-field'

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
 * The search query, kept in the URL (?q=): the field in the tab bar and
 * the search page share it, and Back comes back to the same results.
 */
export function useSearchQuery(): [string, (q: string) => void] {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  return [q, (next: string) => setParams(next ? { q: next } : {}, { replace: true })]
}
