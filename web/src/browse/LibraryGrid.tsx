import { useWindowVirtualizer } from '@tanstack/react-virtual'
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useParams, useSearchParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Button } from '../ui'
import { PosterCard } from './cards/PosterCard'
import { FilterBar } from './FilterBar'
import { activeFilters, clearFilters, gridSearch, parseGrid, type GridState } from './gridQuery'
import { LetterIndex } from './LetterIndex'
import { Unavailable } from './Unavailable'
import { useGridPages } from './useGridPages'

type Library = components['schemas']['Library']
type LibraryFacets = components['schemas']['LibraryFacets']

/** Posters are 120–200 px wide (F008-R10); the column count follows the width. */
const minCard = 120
const maxCard = 200
const gap = 16
const caption = 44

export function columnsFor(width: number): number {
  return Math.max(2, Math.floor((width + gap) / (minCard + gap)))
}

/**
 * A library grid (F008-R10–R15): every top-level item, virtualized over
 * the total so the DOM stays small, sorted and filtered from the URL.
 */
export function LibraryGrid() {
  const { t } = useTranslation()
  const { libraryId = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const location = useLocation()
  const search = params.toString()
  const grid = useMemo(() => parseGrid(new URLSearchParams(search)), [search])
  const [library, setLibrary] = useState<Library | null>()
  const [facets, setFacets] = useState<LibraryFacets>()
  const [hasCollections, setHasCollections] = useState(false)
  const pages = useGridPages(libraryId, grid)

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      api.GET('/libraries/{libraryId}', { params: { path: { libraryId } } }),
      api.GET('/libraries/{libraryId}/facets', { params: { path: { libraryId } } }),
      api.GET('/collections', { params: { query: { libraryId } } }),
    ]).then(([l, f, c]) => {
      if (cancelled) return
      setLibrary(l.data ?? null)
      setFacets(f.data)
      setHasCollections((c.data?.collections.length ?? 0) > 0)
    })
    return () => {
      cancelled = true
    }
  }, [libraryId])

  // Columns from the grid's width.
  const box = useRef<HTMLDivElement>(null)
  const head = useRef<HTMLElement>(null)
  const [width, setWidth] = useState(1024)
  // Where the grid starts on the page, for the window virtualizer.
  const [margin, setMargin] = useState(0)
  useLayoutEffect(() => {
    const el = box.current
    if (!el || typeof ResizeObserver === 'undefined') return
    // The header grows when filter chips appear: that moves the grid too.
    const ro = new ResizeObserver(() => {
      setWidth(el.clientWidth)
      setMargin(el.getBoundingClientRect().top + window.scrollY)
    })
    ro.observe(el)
    if (head.current) ro.observe(head.current)
    return () => ro.disconnect()
  }, [library])
  const cols = columnsFor(width)
  const cardWidth = Math.min(maxCard, (width - gap * (cols - 1)) / cols)
  const rowHeight = Math.round(cardWidth * 1.5 + caption + gap)

  const total = pages.total ?? 0
  const rows = Math.ceil(total / cols)
  const buffer = Math.ceil(
    (2 * (typeof window === 'undefined' ? 800 : window.innerHeight)) / rowHeight,
  )
  const virtualizer = useWindowVirtualizer({
    count: rows,
    estimateSize: () => rowHeight,
    overscan: buffer,
    scrollMargin: margin,
  })
  const range = virtualizer.getVirtualItems()

  // The pages behind the rows in view and their buffer.
  const first = range[0]?.index ?? 0
  const last = range[range.length - 1]?.index ?? 0
  const { ensure } = pages
  useEffect(() => {
    if (total) ensure(first * cols, Math.min(total - 1, (last + 1) * cols - 1))
  }, [first, last, cols, total, ensure])

  // Back comes back to the same place (R14).
  const scrollKey = `plaid.grid.${location.key}`
  const isFirstLoad = location.key === 'default'
  const restored = useRef(false)
  useEffect(() => {
    // A first load has the key "default": nothing of its own to restore.
    if (restored.current || !total || isFirstLoad) return
    restored.current = true
    const y = Number(sessionStorage.getItem(scrollKey))
    if (y > 0) window.scrollTo(0, y)
  }, [total, scrollKey, isFirstLoad])
  useEffect(() => {
    const save = () => sessionStorage.setItem(scrollKey, String(window.scrollY))
    window.addEventListener('scroll', save, { passive: true })
    return () => window.removeEventListener('scroll', save)
  }, [scrollKey])

  const change = (g: GridState) => {
    window.scrollTo(0, 0)
    setParams(gridSearch(g))
  }

  if (library === null) return <Unavailable />
  const shows = library?.type === 'shows'
  const countText =
    pages.total === undefined
      ? undefined
      : shows
        ? t('grid.count.shows', { count: pages.total })
        : t('grid.count.movies', { count: pages.total })

  return (
    <div className="br-grid-page">
      <header ref={head} className="br-grid__head">
        <div>
          <h1 className="br-grid__title">{library?.name}</h1>
          {countText && <p className="br-grid__count">{countText}</p>}
          {hasCollections && (
            <Link to={`/library/${libraryId}/collections`} className="br-row__all pl-focusable">
              {t('grid.collections')}
            </Link>
          )}
        </div>
        <FilterBar grid={grid} facets={facets} total={pages.total} onChange={change} />
      </header>
      {pages.error && (
        <div className="br-error" role="alert">
          <p>{t('errors.generic')}</p>
          <Button variant="secondary" onClick={pages.retry}>
            {t('common.retry')}
          </Button>
        </div>
      )}
      {pages.total === 0 && activeFilters(grid) > 0 && (
        <section className="br-empty br-grid__empty">
          <h2 className="br-empty__title br-empty__title--small">{t('grid.empty.title')}</h2>
          <p className="br-empty__body">{t('grid.empty.body')}</p>
          <Button variant="secondary" onClick={() => change(clearFilters(grid))}>
            {t('grid.empty.action')}
          </Button>
        </section>
      )}
      <div
        ref={box}
        className="br-grid"
        role="grid"
        aria-rowcount={rows}
        aria-label={library?.name}
        aria-busy={pages.total === undefined}
        style={{ height: virtualizer.getTotalSize() }}
      >
        {range.map((row) => (
          <div
            key={row.key}
            role="row"
            aria-rowindex={row.index + 1}
            className="br-grid__row"
            style={{
              transform: `translateY(${row.start - margin}px)`,
              gridTemplateColumns: `repeat(${cols}, ${cardWidth}px)`,
            }}
          >
            {Array.from({ length: cols }, (_, c) => {
              const index = row.index * cols + c
              if (index >= total) return null
              const it = pages.item(index)
              return (
                <div key={c} role="gridcell" className="br-grid__cell">
                  {it ? (
                    <PosterCard item={it} sizes={`${Math.round(cardWidth)}px`} />
                  ) : (
                    <span className="br-card br-card--poster br-card--skeleton" aria-hidden="true">
                      <span className="br-card__art" />
                    </span>
                  )}
                </div>
              )
            })}
          </div>
        ))}
      </div>
      {pages.total !== undefined && pages.total > 0 && pages.total < 60 && (
        <p className="br-grid__end">{t('grid.end', { count: pages.total })}</p>
      )}
      {grid.sort === 'title' &&
        facets &&
        facets.letters.length > 1 &&
        total > 0 &&
        activeFilters(grid) === 0 && (
          <LetterIndex
            letters={facets.letters}
            desc={grid.desc}
            total={total}
            onJump={(i) => virtualizer.scrollToIndex(Math.floor(i / cols), { align: 'start' })}
          />
        )}
    </div>
  )
}
