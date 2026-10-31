import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { Icon, cx } from '../ui'
import { activeFilters, clearFilters, defaultDesc, type GridState } from './gridQuery'

type LibraryFacets = components['schemas']['LibraryFacets']
type ItemSort = components['schemas']['ItemSort']
type WatchFilter = components['schemas']['WatchFilter']

const sorts: ItemSort[] = ['title', 'added', 'released', 'rating', 'played', 'random']

/**
 * The grid's controls (F008-R12, R13; V2): one segmented control (All,
 * Unwatched or New episodes, In progress) and two quiet menus, Genre and
 * Sort by. Picking applies at once, like a system menu: no panel, no
 * confirm. Filters only a link can set (decade, resolution, several
 * genres) get a "Clear all".
 */
export function FilterBar({
  grid,
  facets,
  shows,
  onChange,
}: {
  grid: GridState
  facets?: LibraryFacets
  /** A shows library: "Unwatched" reads "New episodes". */
  shows?: boolean
  onChange: (g: GridState) => void
}) {
  const { t } = useTranslation()
  const sortLabels: Record<ItemSort, string> = {
    title: t('grid.sort.title'),
    added: t('grid.sort.added'),
    released: t('grid.sort.released'),
    rating: t('grid.sort.rating'),
    played: t('grid.sort.played'),
    random: t('grid.sort.random'),
  }
  const watch: (WatchFilter | 'all')[] = ['all', 'unwatched', 'in_progress']
  const watchLabel = (w: WatchFilter | 'all') =>
    w === 'all'
      ? t('grid.filter.watch.all')
      : w === 'unwatched'
        ? shows
          ? t('grid.filter.watch.newEpisodes')
          : t('grid.filter.watch.unwatched')
        : t('grid.filter.watch.in_progress')
  const genre = grid.genres.length === 1 ? grid.genres[0] : undefined
  const genreName =
    genre !== undefined
      ? (facets?.genres.find((g) => g.id === genre)?.name ?? String(genre))
      : t('grid.filter.allGenres')
  // Set by a link only (a decade chip, a resolution, several genres).
  const hidden =
    activeFilters(grid) - (grid.watch && grid.watch !== 'watched' ? 1 : 0) - (genre ? 1 : 0)

  return (
    <div className="br-filters">
      <div
        className="br-segmented br-glass"
        role="radiogroup"
        aria-label={t('grid.filter.watch.label')}
      >
        {watch.map((w) => {
          const on = (grid.watch ?? 'all') === w
          return (
            <button
              key={w}
              type="button"
              role="radio"
              aria-checked={on}
              className={cx('br-segmented__item pl-focusable', on && 'is-on')}
              onClick={() => onChange({ ...grid, watch: w === 'all' ? undefined : w })}
            >
              {watchLabel(w)}
            </button>
          )
        })}
      </div>
      <span className="br-filters__fill" />
      {hidden > 0 && (
        <button
          type="button"
          className="br-link pl-focusable"
          onClick={() => onChange(clearFilters(grid))}
        >
          {t('grid.filter.clear')}
        </button>
      )}
      {facets && facets.genres.length > 0 && (
        <MenuButton label={t('grid.filter.genre')} value={genreName} active={genre !== undefined}>
          {(close) => (
            <>
              <MenuItem
                checked={grid.genres.length === 0}
                onPick={() => {
                  close()
                  onChange({ ...grid, genres: [] })
                }}
              >
                {t('grid.filter.allGenres')}
              </MenuItem>
              <div className="br-gmenu__sep" aria-hidden="true" />
              {facets.genres.map((g) => (
                <MenuItem
                  key={g.id}
                  checked={genre === g.id}
                  onPick={() => {
                    close()
                    onChange({ ...grid, genres: [g.id] })
                  }}
                >
                  {g.name}
                </MenuItem>
              ))}
            </>
          )}
        </MenuButton>
      )}
      <MenuButton
        label={t('grid.sort.label')}
        prefix={t('grid.sort.by')}
        value={sortLabels[grid.sort]}
      >
        {(close) => (
          <>
            {sorts.map((s) => (
              <MenuItem
                key={s}
                checked={grid.sort === s}
                onPick={() => {
                  close()
                  if (s === grid.sort) return
                  onChange({
                    ...grid,
                    sort: s,
                    desc: defaultDesc(s),
                    seed: s === 'random' ? Math.floor(Math.random() * 1e9) : undefined,
                  })
                }}
              >
                {sortLabels[s]}
              </MenuItem>
            ))}
            {grid.sort !== 'random' && (
              <>
                <div className="br-gmenu__sep" aria-hidden="true" />
                {[false, true].map((desc) => (
                  <MenuItem
                    key={String(desc)}
                    checked={grid.desc === desc}
                    onPick={() => {
                      close()
                      onChange({ ...grid, desc })
                    }}
                  >
                    {desc ? t('grid.order.desc') : t('grid.order.asc')}
                  </MenuItem>
                ))}
              </>
            )}
          </>
        )}
      </MenuButton>
    </div>
  )
}

/** A quiet button that opens a glass menu, closed by a pick, Esc or a click outside. */
function MenuButton({
  label,
  prefix,
  value,
  active,
  children,
}: {
  label: string
  prefix?: string
  value: string
  active?: boolean
  children: (close: () => void) => ReactNode
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    ref.current?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus()
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onDown)
    }
  }, [open])
  return (
    <span ref={ref} className="br-filters__anchor">
      <button
        type="button"
        className={cx('br-quiet pl-focusable', (open || active) && 'is-on')}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`${label}: ${value}`}
        onClick={() => setOpen((o) => !o)}
      >
        {prefix && <span className="br-quiet__prefix">{prefix}</span>}
        {value}
        <Icon name="chevronDown" size={14} />
      </button>
      {open && (
        <div className="br-gmenu" role="menu" aria-label={label}>
          {children(() => setOpen(false))}
        </div>
      )}
    </span>
  )
}

function MenuItem({
  checked,
  onPick,
  children,
}: {
  checked: boolean
  onPick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      role="menuitemradio"
      aria-checked={checked}
      className="br-gmenu__item pl-focusable"
      onClick={onPick}
    >
      <span className="br-gmenu__check" aria-hidden="true">
        {checked && <Icon name="check" size={14} />}
      </span>
      {children}
    </button>
  )
}
