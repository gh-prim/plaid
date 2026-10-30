import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { Button, Chip, Icon, SegmentedControl } from '../ui'
import {
  activeFilters,
  clearFilters,
  defaultDesc,
  type GridState,
  type ResolutionChoice,
} from './gridQuery'

type LibraryFacets = components['schemas']['LibraryFacets']
type ItemSort = components['schemas']['ItemSort']
type WatchFilter = components['schemas']['WatchFilter']

const sorts: ItemSort[] = ['title', 'added', 'released', 'rating', 'played', 'random']

/**
 * Sort, order, the filter panel and the active filters as removable chips
 * (F008-R12, R13). Every change goes to `onChange`, which writes the URL.
 */
export function FilterBar({
  grid,
  facets,
  total,
  onChange,
}: {
  grid: GridState
  facets?: LibraryFacets
  total?: number
  onChange: (g: GridState) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const panel = useRef<HTMLDivElement>(null)
  const count = activeFilters(grid)

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    const onDown = (e: MouseEvent) => {
      if (panel.current && !panel.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onDown)
    }
  }, [open])

  const genreName = (id: number) => facets?.genres.find((g) => g.id === id)?.name ?? String(id)
  const toggle = <T,>(list: T[], v: T) =>
    list.includes(v) ? list.filter((x) => x !== v) : [...list, v]
  const resolutionLabel = (r: ResolutionChoice) =>
    r === 'sd'
      ? t('grid.filter.resolution.sd')
      : r === '2160p'
        ? t('grid.filter.resolution.uhd')
        : r
  const decadeLabel = (from: number) => t('grid.filter.decade', { decade: from })
  const watchLabels: Record<WatchFilter, string> = {
    unwatched: t('grid.filter.watch.unwatched'),
    in_progress: t('grid.filter.watch.in_progress'),
    watched: t('grid.filter.watch.watched'),
  }
  const watchLabel = (w: WatchFilter) => watchLabels[w]
  const sortLabels: Record<ItemSort, string> = {
    title: t('grid.sort.title'),
    added: t('grid.sort.added'),
    released: t('grid.sort.released'),
    rating: t('grid.sort.rating'),
    played: t('grid.sort.played'),
    random: t('grid.sort.random'),
  }

  // The active filters, each removable (R13).
  const chips: { key: string; label: string; remove: () => GridState }[] = []
  if (grid.watch)
    chips.push({
      key: 'w',
      label: watchLabel(grid.watch),
      remove: () => ({ ...grid, watch: undefined }),
    })
  for (const g of grid.genres)
    chips.push({
      key: `g${g}`,
      label: genreName(g),
      remove: () => ({ ...grid, genres: grid.genres.filter((x) => x !== g) }),
    })
  if (grid.decade !== undefined)
    chips.push({
      key: 'd',
      label: decadeLabel(grid.decade),
      remove: () => ({ ...grid, decade: undefined }),
    })
  for (const r of grid.resolutions)
    chips.push({
      key: `r${r}`,
      label: resolutionLabel(r),
      remove: () => ({ ...grid, resolutions: grid.resolutions.filter((x) => x !== r) }),
    })
  if (grid.hdr !== undefined)
    chips.push({
      key: 'h',
      label: t('grid.filter.hdrOnly'),
      remove: () => ({ ...grid, hdr: undefined }),
    })

  return (
    <div className="br-filters">
      <div className="br-filters__bar">
        <label className="br-select">
          <span className="br-visually-hidden">{t('grid.sort.label')}</span>
          <select
            value={grid.sort}
            onChange={(e) => {
              const sort = e.target.value as ItemSort
              onChange({
                ...grid,
                sort,
                desc: defaultDesc(sort),
                seed: sort === 'random' ? Math.floor(Math.random() * 1e9) : undefined,
              })
            }}
          >
            {sorts.map((s) => (
              <option key={s} value={s}>
                {sortLabels[s]}
              </option>
            ))}
          </select>
        </label>
        {grid.sort !== 'random' && (
          <button
            type="button"
            className="br-iconbtn pl-focusable"
            aria-label={grid.desc ? t('grid.order.desc') : t('grid.order.asc')}
            title={grid.desc ? t('grid.order.desc') : t('grid.order.asc')}
            onClick={() => onChange({ ...grid, desc: !grid.desc })}
          >
            <span aria-hidden="true">{grid.desc ? '↓' : '↑'}</span>
          </button>
        )}
        <div className="br-filters__anchor" ref={panel}>
          <Button
            variant="secondary"
            size="sm"
            aria-expanded={open}
            aria-haspopup="dialog"
            onClick={() => setOpen((o) => !o)}
          >
            {count > 0 ? t('grid.filters.count', { count }) : t('grid.filters.label')}
          </Button>
          {open && (
            <div className="pl pl-menu br-panel" role="dialog" aria-label={t('grid.filters.label')}>
              <fieldset className="br-panel__group">
                <legend>{t('grid.filter.watch.label')}</legend>
                <SegmentedControl<WatchFilter | 'all'>
                  label={t('grid.filter.watch.label')}
                  value={grid.watch ?? 'all'}
                  onChange={(v) => onChange({ ...grid, watch: v === 'all' ? undefined : v })}
                  options={[
                    { value: 'all', label: t('grid.filter.watch.all') },
                    ...(['unwatched', 'in_progress', 'watched'] as const).map((w) => ({
                      value: w,
                      label: watchLabel(w),
                    })),
                  ]}
                />
              </fieldset>
              {facets && facets.genres.length > 0 && (
                <fieldset className="br-panel__group">
                  <legend>{t('grid.filter.genre')}</legend>
                  <div className="br-panel__chips">
                    {facets.genres.map((g) => (
                      <Chip
                        key={g.id}
                        selected={grid.genres.includes(g.id)}
                        onClick={() => onChange({ ...grid, genres: toggle(grid.genres, g.id) })}
                      >
                        {g.name}
                      </Chip>
                    ))}
                  </div>
                </fieldset>
              )}
              {facets && facets.decades.length > 0 && (
                <fieldset className="br-panel__group">
                  <legend>{t('grid.filter.years')}</legend>
                  <div className="br-panel__chips">
                    {facets.decades.map((d) => (
                      <Chip
                        key={d.from}
                        selected={grid.decade === d.from}
                        onClick={() =>
                          onChange({ ...grid, decade: grid.decade === d.from ? undefined : d.from })
                        }
                      >
                        {decadeLabel(d.from)}
                      </Chip>
                    ))}
                  </div>
                </fieldset>
              )}
              <fieldset className="br-panel__group">
                <legend>{t('grid.filter.resolution.label')}</legend>
                <div className="br-panel__chips">
                  {(['2160p', '1080p', 'sd'] as const).map((r) => (
                    <Chip
                      key={r}
                      selected={grid.resolutions.includes(r)}
                      onClick={() =>
                        onChange({ ...grid, resolutions: toggle(grid.resolutions, r) })
                      }
                    >
                      {resolutionLabel(r)}
                    </Chip>
                  ))}
                  <Chip
                    selected={grid.hdr === true}
                    onClick={() => onChange({ ...grid, hdr: grid.hdr ? undefined : true })}
                  >
                    {t('grid.filter.hdrOnly')}
                  </Chip>
                </div>
              </fieldset>
              <Button size="sm" onClick={() => setOpen(false)}>
                {total === undefined
                  ? t('common.close')
                  : t('grid.filters.apply', { count: total })}
              </Button>
            </div>
          )}
        </div>
      </div>
      {chips.length > 0 && (
        <ul className="br-filters__chips">
          {chips.map((c) => (
            <li key={c.key}>
              <button
                type="button"
                className="pl-chip is-selected pl-focusable"
                aria-label={t('grid.filter.remove', { name: c.label })}
                onClick={() => onChange(c.remove())}
              >
                {c.label}
                <Icon name="x" size={14} />
              </button>
            </li>
          ))}
          <li>
            <button
              type="button"
              className="br-link pl-focusable"
              onClick={() => onChange(clearFilters(grid))}
            >
              {t('grid.filter.clear')}
            </button>
          </li>
        </ul>
      )}
    </div>
  )
}
