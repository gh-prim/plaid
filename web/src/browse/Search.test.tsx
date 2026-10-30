import { act, renderHook, screen } from '@testing-library/react'
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
    // A new query hides the old results until its own answer.
    rerender({ q: 'dune pa' })
    expect(result.current.results).toBeUndefined()
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

  it('finds while typing, grouped, and walks the results with the keyboard', async () => {
    setup()
    await renderApp('/')
    await screen.findByRole('heading', { name: 'Nothing here yet.' })
    await userEvent.keyboard('/')
    const field = screen.getByRole('combobox', { name: 'Search nas' })
    expect(field).toHaveFocus()
    await userEvent.keyboard('dun')
    const list = await screen.findByRole('listbox')
    expect(screen.getByRole('group', { name: 'Movies' })).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'People' })).toBeInTheDocument()
    expect(screen.getByText('The Bear · S2 E3')).toBeInTheDocument()
    expect(list).toBeInTheDocument()
    await userEvent.keyboard('{ArrowDown}{ArrowDown}')
    const active = field.getAttribute('aria-activedescendant')!
    expect(document.getElementById(active)).toHaveTextContent('Dune: Part Two')
    await userEvent.keyboard('{Enter}')
    expect(
      await screen.findByRole('heading', { name: 'This title isn’t available on this profile.' }),
    ).toBeInTheDocument()
  })

  it('says when nothing matches, and Esc closes then clears', async () => {
    setup()
    await renderApp('/')
    const field = await screen.findByRole('combobox', { name: 'Search nas' })
    await userEvent.type(field, 'zzz')
    expect(await screen.findByText('Nothing called “zzz” here.')).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByText('Nothing called “zzz” here.')).not.toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(field).toHaveValue('')
  })
})
