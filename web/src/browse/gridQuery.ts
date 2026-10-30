import type { components, operations } from '../api/schema'

type ItemSort = components['schemas']['ItemSort']
type WatchFilter = components['schemas']['WatchFilter']
export type ResolutionChoice = '2160p' | '1080p' | 'sd'
type ItemsQuery = NonNullable<operations['listLibraryItems']['parameters']['query']>

/**
 * A library grid's sort and filters (F008-R12–R14). They live in the URL
 * (R14) so links share, Back restores and reload keeps them.
 */
export type GridState = {
  sort: ItemSort
  desc: boolean
  /** The random order of this visit. */
  seed?: number
  watch?: WatchFilter
  genres: number[]
  /** A decade chip: its first year. */
  decade?: number
  resolutions: ResolutionChoice[]
  hdr?: boolean
}

const sorts: ItemSort[] = ['title', 'added', 'released', 'rating', 'played', 'random']
const watches: WatchFilter[] = ['unwatched', 'in_progress', 'watched']
const resolutionChoices: ResolutionChoice[] = ['2160p', '1080p', 'sd']

/** Title reads A→Z by default, every other sort newest or best first. */
export const defaultDesc = (sort: ItemSort) => sort !== 'title'

export const emptyGrid: GridState = { sort: 'title', desc: false, genres: [], resolutions: [] }

const int = (v: string | null) => {
  const n = v === null ? NaN : Number(v)
  return Number.isInteger(n) ? n : undefined
}

/** Reads the grid from the URL; anything unknown falls back to the default. */
export function parseGrid(q: URLSearchParams): GridState {
  const sort = sorts.find((s) => s === q.get('sort')) ?? 'title'
  const order = q.get('order')
  const watch = watches.find((w) => w === q.get('watch'))
  const hdr = q.get('hdr')
  return {
    sort,
    desc: order === 'desc' ? true : order === 'asc' ? false : defaultDesc(sort),
    seed: sort === 'random' ? int(q.get('seed')) : undefined,
    watch,
    genres: q
      .getAll('genre')
      .map(int)
      .filter((n): n is number => n !== undefined),
    decade: int(q.get('decade')),
    resolutions: resolutionChoices.filter((r) => q.getAll('resolution').includes(r)),
    hdr: hdr === 'true' ? true : hdr === 'false' ? false : undefined,
  }
}

/** Writes the grid to the URL, leaving out the defaults. */
export function gridSearch(g: GridState): URLSearchParams {
  const q = new URLSearchParams()
  if (g.sort !== 'title') q.set('sort', g.sort)
  if (g.sort !== 'random' && g.desc !== defaultDesc(g.sort)) q.set('order', g.desc ? 'desc' : 'asc')
  if (g.sort === 'random' && g.seed !== undefined) q.set('seed', String(g.seed))
  if (g.watch) q.set('watch', g.watch)
  for (const id of g.genres) q.append('genre', String(id))
  if (g.decade !== undefined) q.set('decade', String(g.decade))
  for (const r of g.resolutions) q.append('resolution', r)
  if (g.hdr !== undefined) q.set('hdr', String(g.hdr))
  return q
}

/** The API query for one page of the grid, always with its total (R10). */
export function gridApiQuery(g: GridState, offset: number, limit: number): ItemsQuery {
  const q: ItemsQuery = { sort: g.sort, order: g.desc ? 'desc' : 'asc', offset, limit, total: true }
  if (g.sort === 'random') q.seed = g.seed ?? 0
  if (g.watch) q.watch = g.watch
  if (g.genres.length) q.genre = g.genres
  if (g.decade !== undefined) {
    q.yearFrom = g.decade
    q.yearTo = g.decade + 9
  }
  // "720p and lower" is the server's 720p and sd.
  if (g.resolutions.length)
    q.resolution = g.resolutions.flatMap((r) => (r === 'sd' ? (['720p', 'sd'] as const) : [r]))
  if (g.hdr !== undefined) q.hdr = g.hdr
  return q
}

/** How many filters are on, for the Filters button and Clear all. */
export function activeFilters(g: GridState): number {
  return (
    (g.watch ? 1 : 0) +
    g.genres.length +
    (g.decade !== undefined ? 1 : 0) +
    g.resolutions.length +
    (g.hdr !== undefined ? 1 : 0)
  )
}

export function clearFilters(g: GridState): GridState {
  return { sort: g.sort, desc: g.desc, seed: g.seed, genres: [], resolutions: [] }
}
