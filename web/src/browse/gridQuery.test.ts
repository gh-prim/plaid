import { describe, expect, it } from 'vitest'
import {
  activeFilters,
  clearFilters,
  emptyGrid,
  gridApiQuery,
  gridSearch,
  parseGrid,
} from './gridQuery'

const q = (s: string) => new URLSearchParams(s)

describe('gridQuery', () => {
  it('defaults to title A→Z with no filter, and writes nothing for it', () => {
    expect(parseGrid(q(''))).toEqual(emptyGrid)
    expect(gridSearch(emptyGrid).toString()).toBe('')
  })

  it('reads the spec’s example URL and writes it back', () => {
    const g = parseGrid(q('sort=added&order=desc&genre=53&watch=unwatched'))
    expect(g).toMatchObject({ sort: 'added', desc: true, genres: [53], watch: 'unwatched' })
    // desc is the default for added, so order isn't written.
    expect(gridSearch(g).toString()).toBe('sort=added&watch=unwatched&genre=53')
    expect(parseGrid(gridSearch(g))).toEqual(g)
  })

  it('round-trips every filter', () => {
    const g = {
      ...emptyGrid,
      sort: 'rating' as const,
      desc: false,
      watch: 'in_progress' as const,
      genres: [878, 27],
      decade: 1990,
      resolutions: ['2160p' as const, 'sd' as const],
      hdr: true,
    }
    expect(parseGrid(gridSearch(g))).toEqual(g)
    expect(activeFilters(g)).toBe(7)
    expect(clearFilters(g)).toEqual({ ...emptyGrid, sort: 'rating', desc: false })
  })

  it('keeps the random seed of the visit', () => {
    const g = parseGrid(q('sort=random&seed=42&order=asc'))
    expect(g.seed).toBe(42)
    expect(gridSearch(g).toString()).toBe('sort=random&seed=42')
  })

  it('ignores what it doesn’t know', () => {
    expect(parseGrid(q('sort=popular&watch=half&genre=x&resolution=8k&hdr=maybe'))).toEqual(
      emptyGrid,
    )
  })

  it('builds the API query', () => {
    const g = parseGrid(q('sort=released&genre=18&genre=35&decade=1990&resolution=sd&hdr=false'))
    expect(gridApiQuery(g, 400, 200)).toEqual({
      sort: 'released',
      order: 'desc',
      offset: 400,
      limit: 200,
      total: true,
      genre: [18, 35],
      yearFrom: 1990,
      yearTo: 1999,
      resolution: ['720p', 'sd'],
      hdr: false,
    })
    expect(gridApiQuery(parseGrid(q('sort=random')), 0, 200)).toMatchObject({
      sort: 'random',
      seed: 0,
    })
  })
})
