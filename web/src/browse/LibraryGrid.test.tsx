import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'
import { columnsFor } from './LibraryGrid'

const ada = profile('p1', 'Ada')
const movie = (i: number) => ({
  id: `m${i}`,
  kind: 'movie',
  displayTitle: `Movie ${i}`,
  matchStatus: 'matched',
  genres: [],
  fileCount: 1,
  childCount: 0,
  technical: { hdrFormats: [], audioLanguages: [] },
})

function setup(total: number) {
  const calls = fakeApi({
    'GET /server': server(true),
    'GET /me': {
      status: 200,
      body: { user: user({ role: 'member' }), profiles: [ada], activeProfile: ada },
    },
    'GET /libraries': { status: 200, body: [] },
    'GET /libraries/lm': { status: 200, body: { id: 'lm', name: 'Movies', type: 'movies' } },
    'GET /libraries/lm/facets': {
      status: 200,
      body: {
        total,
        genres: [{ id: 878, name: 'Science Fiction', count: 12 }],
        decades: [{ from: 1990, count: 3 }],
        resolutions: [],
        hdr: 0,
        letters: [],
      },
    },
    'GET /libraries/lm/items': (req) => {
      const q = new URL(req.url).searchParams
      const offset = Number(q.get('offset'))
      const limit = Number(q.get('limit'))
      const n = q.has('genre') ? 0 : total
      const items = Array.from({ length: Math.max(0, Math.min(limit, n - offset)) }, (_, i) =>
        movie(offset + i),
      )
      return { status: 200, body: { items, total: n } }
    },
  })
  return calls
}

describe('library grid', () => {
  it('counts the library and never puts more than 300 cards in the DOM, whatever its size', async () => {
    const calls = setup(5000)
    await renderApp('/library/lm')
    expect(await screen.findByText('5,000 movies')).toBeInTheDocument()
    await screen.findByRole('link', { name: /Movie 0/ })
    const cards = document.querySelectorAll('.br-card--poster')
    expect(cards.length).toBeGreaterThan(0)
    expect(cards.length).toBeLessThanOrEqual(300)
    expect(screen.getByRole('grid')).toHaveAttribute(
      'aria-rowcount',
      String(Math.ceil(5000 / columnsFor(1024))),
    )
    // Only the pages near the view were fetched.
    const offsets = calls.filter((c) => c.path === '/libraries/lm/items').map((c) => c)
    expect(offsets.length).toBeLessThanOrEqual(2)
  })

  it('keeps sort and filters in the URL, shows them as chips, and says when nothing matches', async () => {
    const calls = setup(30)
    await renderApp('/library/lm?sort=added&genre=878')
    expect(
      await screen.findByRole('heading', { name: 'Nothing matches these filters.' }),
    ).toBeInTheDocument()
    const req = calls.find((c) => c.path === '/libraries/lm/items')!
    expect(req).toBeDefined()
    expect(screen.getByRole('combobox', { name: 'Sort' })).toHaveValue('added')
    const chip = screen.getByRole('button', { name: 'Remove filter Science Fiction' })
    await userEvent.click(chip)
    expect(await screen.findByText('30 movies')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Remove filter Science Fiction' }),
    ).not.toBeInTheDocument()
  })

  it('filters from the panel', async () => {
    setup(30)
    await renderApp('/library/lm')
    await screen.findByText('30 movies')
    await userEvent.click(screen.getByRole('button', { name: 'Filters' }))
    const panel = screen.getByRole('dialog', { name: 'Filters' })
    await userEvent.click(within(panel).getByRole('button', { name: '1990s' }))
    expect(await screen.findByRole('button', { name: 'Remove filter 1990s' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Clear all' }))
    expect(screen.queryByRole('button', { name: 'Remove filter 1990s' })).not.toBeInTheDocument()
  })

  it('says a library that doesn’t exist for this profile is unavailable', async () => {
    setup(0)
    await renderApp('/library/nope')
    expect(
      await screen.findByRole('heading', { name: 'This title isn’t available on this profile.' }),
    ).toBeInTheDocument()
  })
})
