import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada')
const movie = (id: string, title: string) => ({
  id,
  kind: 'movie',
  displayTitle: title,
  matchStatus: 'matched',
  genres: [],
  fileCount: 1,
  childCount: 0,
  technical: { hdrFormats: [], audioLanguages: [] },
})
const dune = { id: 'c1', name: 'Dune Collection', movieCount: 2, backdrop: { imageId: 'bdc' } }

describe('collections', () => {
  it('goes from the grid to the collections, then to one collection', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': {
        status: 200,
        body: { user: user({ role: 'member' }), profiles: [ada], activeProfile: ada },
      },
      'GET /libraries': { status: 200, body: [] },
      'GET /libraries/lm': { status: 200, body: { id: 'lm', name: 'Movies', type: 'movies' } },
      'GET /libraries/lm/facets': {
        status: 200,
        body: { total: 2, genres: [], decades: [], resolutions: [], hdr: 0, letters: [] },
      },
      'GET /libraries/lm/items': { status: 200, body: { items: [], total: 2 } },
      'GET /collections': { status: 200, body: { collections: [dune] } },
      'GET /collections/c1': {
        status: 200,
        body: {
          ...dune,
          overview: 'The saga of Paul Atreides.',
          items: [movie('m1', 'Dune'), movie('m2', 'Dune: Part Two')],
        },
      },
    })
    await renderApp('/library/lm')
    await userEvent.click(await screen.findByRole('link', { name: 'Collections' }))
    await userEvent.click(await screen.findByRole('link', { name: /Dune Collection/ }))
    expect(await screen.findByRole('heading', { name: 'Dune Collection' })).toBeInTheDocument()
    expect(screen.getByText('The saga of Paul Atreides.')).toBeInTheDocument()
    expect(
      screen.getAllByRole('link', { name: /Dune/ }).map((l) => l.getAttribute('href')),
    ).toEqual(['/item/m1', '/item/m2'])
  })
})
