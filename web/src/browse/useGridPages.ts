import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { gridApiQuery, type GridState } from './gridQuery'

type ItemSummary = components['schemas']['ItemSummary']

/** Grid pages are 200 items (F008-R11). */
export const gridPageSize = 200

/**
 * A library grid's items by position, fetched a page of 200 at a time as
 * their rows come near the view. A new query starts over. `total` comes
 * with the first page, so the scroll height is right from the start.
 */
export function useGridPages(libraryId: string, grid: GridState) {
  const key = `${libraryId}?${JSON.stringify(grid)}`
  const [state, setState] = useState<{
    key: string
    total?: number
    pages: Map<number, ItemSummary[]>
  }>(() => ({ key, pages: new Map() }))
  const inFlight = useRef(new Set<string>())
  const failed = useRef(false)
  const [error, setError] = useState(false)

  if (state.key !== key) setState({ key, pages: new Map() })

  const fetchPage = useCallback(
    (page: number) => {
      const id = `${key}#${page}`
      if (inFlight.current.has(id)) return
      inFlight.current.add(id)
      void api
        .GET('/libraries/{libraryId}/items', {
          params: {
            path: { libraryId },
            query: gridApiQuery(grid, page * gridPageSize, gridPageSize),
          },
        })
        .then(({ data }) => {
          if (!data) {
            failed.current = true
            setError(true)
            inFlight.current.delete(id)
            return
          }
          setState((s) => {
            if (s.key !== key) return s
            const pages = new Map(s.pages)
            pages.set(page, data.items)
            return { key, total: data.total ?? s.total, pages }
          })
        })
    },
    // grid is part of key.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [key, libraryId],
  )

  // The first page tells the total.
  useEffect(() => fetchPage(0), [fetchPage])

  /** Makes sure the pages holding items first..last are loaded or loading. */
  const ensure = useCallback(
    (first: number, last: number) => {
      for (let p = Math.floor(first / gridPageSize); p <= Math.floor(last / gridPageSize); p++) {
        if (!state.pages.has(p)) fetchPage(p)
      }
    },
    [state.pages, fetchPage],
  )

  const item = useCallback(
    (index: number): ItemSummary | undefined =>
      state.pages.get(Math.floor(index / gridPageSize))?.[index % gridPageSize],
    [state.pages],
  )

  const retry = useCallback(() => {
    inFlight.current.clear()
    setError(false)
    fetchPage(0)
  }, [fetchPage])

  return { total: state.key === key ? state.total : undefined, item, ensure, error, retry }
}
