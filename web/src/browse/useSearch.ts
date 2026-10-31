import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { components } from '../api/schema'

type SearchResults = components['schemas']['SearchResults']

/** Keystrokes closer than this make one request (F008-R25). */
export const searchDebounceMs = 150

/**
 * Searches as the query changes: debounced, and a newer query aborts the
 * request of the previous one so an old answer never replaces a new one.
 * `results` is for `query` (undefined while the first answer comes).
 */
export function useSearch(q: string): {
  query: string
  results?: SearchResults
  stale?: boolean
} {
  const query = q.trim()
  const [state, setState] = useState<{ query: string; results?: SearchResults }>({ query: '' })

  useEffect(() => {
    if (!query) return
    const ctrl = new AbortController()
    const timer = setTimeout(() => {
      void api
        .GET('/search', { params: { query: { q: query } }, signal: ctrl.signal })
        .then(({ data }) => {
          if (!ctrl.signal.aborted && data) setState({ query, results: data })
        })
        .catch(() => undefined)
    }, searchDebounceMs)
    return () => {
      clearTimeout(timer)
      ctrl.abort()
    }
  }, [query])

  // While a newer query is on its way, the last results stay (marked
  // stale) so the page crossfades in place instead of blanking (V2).
  if (!query) return { query }
  if (state.query === query) return state
  return state.results ? { query, results: state.results, stale: true } : { query }
}
