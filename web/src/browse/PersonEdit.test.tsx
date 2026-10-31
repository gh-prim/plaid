import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada')
const signedIn = (role: 'admin' | 'member') => ({
  status: 200,
  body: { user: user({ role }), profiles: [ada], activeProfile: ada },
})
const movie = (id: string, title: string) => ({
  id,
  kind: 'movie',
  libraryId: 'lm',
  displayTitle: title,
  matchStatus: 'matched',
  genres: [],
  fileCount: 1,
  childCount: 0,
  technical: { hdrFormats: [], audioLanguages: [] },
})
const detail = {
  ...movie('m1', 'Dune: Part Two'),
  files: [],
  metadata: {
    fieldLanguages: {},
    lockedFields: [],
    localFields: [],
    studios: [],
    trailers: [],
    credits: [],
    providerIds: {},
    backdrops: [],
    genres: [],
  },
}

describe('person page', () => {
  it('lists what they’re in on this server, by role', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': signedIn('member'),
      'GET /libraries': { status: 200, body: [] },
      'GET /people/p-dv': {
        status: 200,
        body: {
          id: 'p-dv',
          name: 'Denis Villeneuve',
          credits: [
            { item: movie('m1', 'Dune: Part Two'), role: 'director' },
            { item: movie('m2', 'Arrival'), role: 'director' },
            { item: movie('m2', 'Arrival'), role: 'writer' },
          ],
        },
      },
    })
    await renderApp('/person/p-dv')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Denis Villeneuve' }),
    ).toBeInTheDocument()
    expect(screen.getByText('On nas · 2 titles')).toBeInTheDocument()
    const directing = screen.getByRole('region', { name: 'Directing' })
    expect(within(directing).getAllByRole('link')).toHaveLength(2)
    expect(screen.getByRole('region', { name: 'Writing' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Acting' })).not.toBeInTheDocument()
  })
})

describe('edit menu', () => {
  const routes = (role: 'admin' | 'member') => ({
    'GET /server': server(true),
    'GET /me': signedIn(role),
    'GET /libraries': { status: 200, body: [] },
    'GET /items/m1': { status: 200, body: detail },
  })

  it('isn’t there for a member', async () => {
    fakeApi(routes('member'))
    await renderApp('/item/m1')
    await screen.findByRole('heading', { level: 1, name: 'Dune: Part Two' })
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument()
  })

  it('lets an admin change the poster, go back to automatic, and refresh', async () => {
    let locked = false
    const calls = fakeApi({
      ...routes('admin'),
      'GET /items/m1/artwork': (req: Request) => ({
        status: 200,
        body: {
          candidates:
            new URL(req.url).searchParams.get('kind') === 'poster'
              ? [
                  {
                    id: 'a1',
                    kind: 'poster',
                    source: 'tmdb',
                    language: 'en',
                    width: 2000,
                    height: 3000,
                    selected: true,
                    locked: false,
                  },
                  {
                    id: 'a2',
                    kind: 'poster',
                    source: 'tmdb',
                    language: 'fr',
                    width: 1000,
                    height: 1500,
                    selected: locked,
                    locked,
                  },
                ]
              : [],
        },
      }),
      'PUT /items/m1/artwork/poster': () => {
        locked = true
        return { status: 204 }
      },
      'DELETE /items/m1/artwork/poster': () => {
        locked = false
        return { status: 204 }
      },
      'POST /items/m1/refresh': { status: 202 },
    })
    await renderApp('/item/m1')
    await userEvent.click(await screen.findByRole('button', { name: 'Edit' }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'Change artwork' }))
    const dialog = await screen.findByRole('dialog', { name: 'Artwork' })
    // Applied at once, no Save.
    await userEvent.click(
      await within(dialog).findByRole('button', { name: 'TMDB, French, 1000 × 1500' }),
    )
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ candidateId: 'a2' })
    await userEvent.click(await within(dialog).findByRole('button', { name: /Back to automatic/ }))
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/items/m1/artwork/poster')).toBe(
      true,
    )
    await userEvent.click(within(dialog).getByRole('button', { name: 'Close' }))
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'Refresh metadata' }))
    expect(await screen.findByText('Refreshing. It can take a minute.')).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/items/m1/refresh')?.body).toEqual({ images: false })
  })

  it('fixes the match from a sheet over the page, with Undo', async () => {
    const calls = fakeApi({
      ...routes('admin'),
      'GET /metadata/search': {
        status: 200,
        body: {
          candidates: [
            { tmdbId: 693134, title: 'Dune: Part Two', year: 2024 },
            { tmdbId: 841, title: 'Dune', year: 1984 },
          ],
        },
      },
      'POST /items/m1/match': { status: 200, body: {} },
    })
    await renderApp('/item/m1')
    await userEvent.click(await screen.findByRole('button', { name: 'Edit' }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'Fix match' }))
    const sheet = await screen.findByRole('dialog', { name: 'Fix match' })
    // Prefilled with the current title; results while typing.
    expect(within(sheet).getByRole('searchbox', { name: 'Search TMDB' })).toHaveValue(
      'Dune: Part Two',
    )
    await userEvent.click(
      await within(sheet).findByRole('button', { name: 'Use Dune (1984)' }, { timeout: 2000 }),
    )
    expect(calls.find((c) => c.path === '/items/m1/match')?.body).toEqual({ tmdbId: 841 })
    expect(await screen.findByRole('status')).toHaveTextContent('Matched to Dune (1984)')
    expect(screen.queryByRole('dialog', { name: 'Fix match' })).not.toBeInTheDocument()
  })
})
