import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada')
const signedIn = (role: 'admin' | 'member') => ({
  status: 200,
  body: { user: user({ role }), profiles: [ada], activeProfile: ada },
})
const library = (counts = { movies: 0, shows: 0 }) => ({
  id: 'lm',
  name: 'Movies',
  type: 'movies',
  metadataLanguage: 'en',
  metadataRegion: 'US',
  lastScanAt: '2026-10-30T12:00:00Z',
  folders: [],
  counts: { episodes: 0, files: 0, unrecognized: 0, ...counts },
  scan: { state: 'idle', queued: false, full: false, counters: {} },
})
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

function routes(role: 'admin' | 'member', home: unknown, libraries: unknown[]) {
  return {
    'GET /server': server(true),
    'GET /me': signedIn(role),
    'GET /home': { status: 200, body: home },
    'GET /libraries': { status: 200, body: libraries },
    'GET /libraries/lm/facets': {
      status: 200,
      body: { total: 2, genres: [], decades: [], resolutions: [], hdr: 0, letters: [] },
    },
    'GET /playback': { status: 200, body: { sessions: [] } },
  }
}

describe('home', () => {
  it('shows the server’s rows in order, titled from their type, with See all', async () => {
    const dune = summary('m1', 'Dune: Part Two', {
      backdrop: { imageId: 'bd1' },
      userState: { positionMs: 3_734_000, durationMs: 9_960_000, watched: false, playCount: 0 },
    })
    fakeApi(
      routes(
        'member',
        {
          ambient: dune,
          rows: [
            { id: 'continue_watching', type: 'continue_watching', items: [dune] },
            {
              id: 'recently_added.lm',
              type: 'recently_added',
              libraryId: 'lm',
              libraryName: 'Movies',
              items: [summary('m2', 'Barbie')],
              seeAll: { libraryId: 'lm', sort: 'added', order: 'desc' },
            },
            {
              id: 'genre.878',
              type: 'genre',
              genre: { id: 878, name: 'Science Fiction' },
              items: [summary('m3', 'Alien')],
              seeAll: { libraryId: 'lm', sort: 'title', order: 'asc', genre: 878 },
            },
          ],
        },
        [library({ movies: 3, shows: 0 })],
      ),
    )
    await renderApp('/')
    const rows = await screen.findAllByRole('region')
    expect(rows.map((r) => within(r).getByRole('heading').textContent)).toEqual([
      'Continue watching',
      'Recently added in Movies',
      'Science Fiction',
    ])
    expect(within(rows[0]!).getByRole('link', { name: /Dune: Part Two/ })).toHaveAttribute(
      'href',
      '/watch/m1?t=3734',
    )
    expect(within(rows[0]!).getByText('1h 44m left')).toBeInTheDocument()
    expect(within(rows[1]!).getByRole('link', { name: 'See all' })).toHaveAttribute(
      'href',
      '/library/lm?sort=added&order=desc',
    )
    expect(within(rows[2]!).getByRole('link', { name: 'See all' })).toHaveAttribute(
      'href',
      '/library/lm?sort=title&order=asc&genre=878',
    )
    expect(document.querySelector('.br-ambient img')?.getAttribute('src')).toContain('/images/bd1')
  })

  it('tells a member with no library to ask the admin', async () => {
    fakeApi(routes('member', { rows: [] }, []))
    await renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Nothing here yet.' })).toBeInTheDocument()
    expect(screen.getByText('Ask your server’s admin to add a library.')).toBeInTheDocument()
  })

  it('offers an admin with no library to add one', async () => {
    localStorage.setItem('plaid.onboarding.later', '1')
    fakeApi(routes('admin', { rows: [] }, []))
    await renderApp('/')
    expect(await screen.findByRole('link', { name: 'Add a library' })).toHaveAttribute(
      'href',
      '/admin/libraries',
    )
  })

  it('says the first scan is on its way', async () => {
    fakeApi(routes('member', { rows: [] }, [library({ movies: 212, shows: 0 })]))
    await renderApp('/')
    expect(
      await screen.findByRole('heading', { name: 'Your movies are on their way.' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/212 found so far/)).toBeInTheDocument()
  })
})
