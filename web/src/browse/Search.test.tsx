import { act, renderHook, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'
import { searchDebounceMs, useSearch } from './useSearch'

const ada = profile('p1', 'Ada')
const summary = (id: string, title: string, extra: Record<string, unknown> = {}) => ({
  id,
  kind: 'movie',
  displayTitle: title,
  matchStatus: 'matched',
  genres: [],
  fileCount: 1,
  childCount: 0,
  technical: { hdrFormats: [], audioLanguages: [] },
  ...extra,
})
const results = (q: string) =>
  q.startsWith('dun')
    ? {
        movies: [
          summary('m1', 'Dune', { year: 2021 }),
          summary('m2', 'Dune: Part Two', { year: 2024 }),
        ],
        shows: [],
        episodes: [
          summary('e1', 'Dune Day', {
            kind: 'episode',
            seasonNumber: 2,
            episodeNumber: 3,
            show: { id: 's1', displayTitle: 'The Bear' },
          }),
        ],
        people: [{ id: 'p1', name: 'Denis Villeneuve' }],
      }
    : { movies: [], shows: [], episodes: [], people: [] }

afterEach(() => vi.useRealTimers())

describe('useSearch', () => {
  it('waits for the typing to pause, and drops the answers of older queries', async () => {
    vi.useFakeTimers()
    const calls = fakeApi({
      'GET /search': (req) => ({
        status: 200,
        body: results(new URL(req.url).searchParams.get('q') ?? ''),
      }),
    })
    const { result, rerender } = renderHook(({ q }) => useSearch(q), { initialProps: { q: 'd' } })
    rerender({ q: 'du' })
    rerender({ q: 'dun' })
    await act(() => vi.advanceTimersByTimeAsync(searchDebounceMs - 10))
    expect(calls).toHaveLength(0)
    await act(() => vi.advanceTimersByTimeAsync(20))
    expect(calls.map((c) => new URL(`http://x${c.path}`).pathname)).toEqual(['/search'])
    expect(result.current.results?.movies).toHaveLength(2)
    // A new query keeps the old results, marked stale, until its own answer (V2).
    rerender({ q: 'dune pa' })
    expect(result.current.stale).toBe(true)
    expect(result.current.results?.movies).toHaveLength(2)
  })
})

describe('search box', () => {
  function setup() {
    return fakeApi({
      'GET /server': server(true),
      'GET /me': {
        status: 200,
        body: { user: user({ role: 'member' }), profiles: [ada], activeProfile: ada },
      },
      'GET /libraries': { status: 200, body: [] },
      'GET /home': { status: 200, body: { rows: [] } },
      'GET /search': (req) => ({
        status: 200,
        body: results(new URL(req.url).searchParams.get('q') ?? ''),
      }),
      'GET /items/m2': { status: 404 },
    })
  }

  it('opens search in the tab bar with "/", filters by kind, and Enter opens the first result', async () => {
    setup()
    await renderApp('/')
    await screen.findByRole('heading', { name: 'Nothing here yet.' })
    await userEvent.keyboard('/')
    const field = (await screen.findAllByRole('searchbox', { name: 'Search nas' }))[0]!
    expect(field).toHaveFocus()
    await userEvent.keyboard('dun')
    const top = await screen.findByRole('region', { name: 'Top results' })
    expect(within(top).getByRole('link', { name: 'Dune' })).toHaveAttribute('href', '/item/m1')
    expect(within(top).getByText('Movie · 2024')).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'People' })).toHaveTextContent('Denis Villeneuve')
    expect(screen.getByText('The Bear · S2 E3')).toBeInTheDocument()
    // Filters, with their counts; only kinds with hits.
    const tabs = screen.getByRole('tablist', { name: 'Show' })
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((t) => t.textContent),
    ).toEqual(['All4', 'Movies2', 'Episodes1', 'People1'])
    await userEvent.click(within(tabs).getByRole('tab', { name: /People/ }))
    expect(screen.queryByRole('region', { name: 'Top results' })).not.toBeInTheDocument()
    field.focus()
    await userEvent.keyboard('{Enter}')
    expect(
      await screen.findByRole('heading', { name: 'This title isn’t available on this profile.' }),
    ).toBeInTheDocument()
  })

  it('says when nothing matches, and Esc clears', async () => {
    setup()
    await renderApp('/search')
    const field = (await screen.findAllByRole('searchbox', { name: 'Search nas' }))[0]!
    await userEvent.type(field, 'zzz')
    expect(await screen.findByText('Nothing called “zzz” here.')).toBeInTheDocument()
    await userEvent.keyboard('{Escape}{Escape}')
    expect(field).toHaveValue('')
  })
})
