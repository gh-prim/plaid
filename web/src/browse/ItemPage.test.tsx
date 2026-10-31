import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada')
const base = {
  kind: 'movie',
  libraryId: 'lm',
  matchStatus: 'matched',
  genres: ['Science Fiction'],
  fileCount: 1,
  childCount: 0,
  technical: {
    resolution: { width: 3840, height: 1600 },
    hdrFormats: ['dolby_vision'],
    audioLanguages: ['en', 'fr'],
  },
}
const file = (id: string, extra: Record<string, unknown> = {}) => ({
  id,
  path: `/m/${id}.mkv`,
  size: 30e9,
  modifiedAt: '2026-01-01T00:00:00Z',
  parseStatus: 'ok',
  probeStatus: 'ok',
  width: 3840,
  height: 1600,
  streams: [],
  ...extra,
})
const metadata = (extra: Record<string, unknown> = {}) => ({
  fieldLanguages: {},
  lockedFields: [],
  localFields: [],
  studios: ['Legendary'],
  trailers: [
    { site: 'YouTube', key: 'teaser1', language: 'en', type: 'Teaser', official: false },
    { site: 'YouTube', key: 'Way9Dexny3w', language: 'en', type: 'Trailer', official: true },
  ],
  credits: [
    { personId: 'p-tc', name: 'Timothée Chalamet', role: 'cast', character: 'Paul Atreides' },
    { personId: 'p-dv', name: 'Denis Villeneuve', role: 'director' },
  ],
  providerIds: {},
  backdrops: [],
  genres: [{ id: 878, name: 'Science Fiction' }],
  originalTitle: 'Dune : Deuxième partie',
  overview: 'Paul Atreides unites with Chani.',
  runtimeMinutes: 166,
  ratingCount: 6000,
  ...extra,
})

function setup(item: unknown, extra: Record<string, unknown> = {}) {
  return fakeApi({
    'GET /server': server(true),
    'GET /me': {
      status: 200,
      body: { user: user({ role: 'member' }), profiles: [ada], activeProfile: ada },
    },
    'GET /libraries': { status: 200, body: [] },
    'GET /items/m1': { status: 200, body: item },
    ...extra,
  })
}

describe('movie page', () => {
  it('shows the facts, the crew, and Resume with Start over', async () => {
    setup({
      ...base,
      id: 'm1',
      displayTitle: 'Dune: Part Two',
      year: 2024,
      rating: 8.1,
      certification: 'PG-13',
      files: [file('f1')],
      userState: { positionMs: 3_733_000, watched: false, playCount: 0 },
      metadata: metadata(),
    })
    await renderApp('/item/m1')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Dune: Part Two' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Original title: Dune : Deuxième partie')).toBeInTheDocument()
    expect(screen.getByText('2024 · 2h 46m · PG-13 · TMDB 8.1 · 6,000 votes')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Science Fiction' })).toHaveAttribute(
      'href',
      '/library/lm?genre=878',
    )
    expect(screen.getByText('4K')).toBeInTheDocument()
    expect(screen.getByText('Dolby Vision')).toBeInTheDocument()
    const about = screen.getByRole('region', { name: 'About' })
    expect(within(about).getByText('English and French')).toBeInTheDocument()
    expect(screen.getByText(/Directed by Denis Villeneuve/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Resume from 1:02:13' })).toHaveAttribute(
      'href',
      '/watch/m1?t=3733',
    )
    expect(screen.getByRole('link', { name: 'Start over' })).toHaveAttribute(
      'href',
      '/watch/m1?t=0',
    )
    const cast = screen.getByRole('region', { name: 'Cast and crew' })
    expect(within(cast).getByRole('link', { name: /Timothée Chalamet/ })).toHaveAttribute(
      'href',
      '/person/p-tc',
    )
  })

  it('looks again until details still on their way arrive', async () => {
    let gets = 0
    const movie = {
      ...base,
      id: 'm1',
      displayTitle: 'Dune: Part Two',
      files: [file('f1')],
      metadata: metadata(),
    }
    setup(undefined, {
      'GET /items/m1': () => ({
        status: 200,
        body: gets++ ? movie : { ...movie, matchStatus: 'pending', displayTitle: 'Dune Part Two' },
      }),
    })
    await renderApp('/item/m1')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Dune Part Two' }),
    ).toBeInTheDocument()
    await waitFor(
      () =>
        expect(
          screen.getByRole('heading', { level: 1, name: 'Dune: Part Two' }),
        ).toBeInTheDocument(),
      { timeout: 5000 },
    )
  })

  it('loads nothing from YouTube before the click, then the official trailer', async () => {
    setup({
      ...base,
      id: 'm1',
      displayTitle: 'Dune: Part Two',
      files: [file('f1')],
      metadata: metadata(),
    })
    await renderApp('/item/m1')
    await screen.findByRole('heading', { level: 1 })
    expect(document.querySelector('iframe')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Play trailer' }))
    expect(document.querySelector('iframe')?.getAttribute('src')).toBe(
      'https://www.youtube-nocookie.com/embed/Way9Dexny3w?autoplay=1',
    )
  })

  it('chooses a version, and plays that file', async () => {
    setup({
      ...base,
      id: 'm1',
      displayTitle: 'Blade Runner',
      files: [
        file('f1', { edition: 'Final Cut' }),
        file('f2', { edition: 'Theatrical', width: 1920, height: 1080 }),
      ],
      metadata: metadata(),
    })
    await renderApp('/item/m1')
    const version = await screen.findByRole('combobox', { name: 'Version' })
    await userEvent.selectOptions(version, 'f2')
    expect(screen.getByRole('link', { name: 'Play' })).toHaveAttribute(
      'href',
      '/watch/m1?fileId=f2',
    )
  })

  it('says when the files can’t be found, with no Play', async () => {
    setup({
      ...base,
      id: 'm1',
      displayTitle: 'Oppenheimer',
      files: [file('f1', { missingSince: '2026-10-29T00:00:00Z' })],
      metadata: metadata(),
    })
    await renderApp('/item/m1')
    expect(
      await screen.findByText('This title’s files can’t be found right now.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Play' })).not.toBeInTheDocument()
  })

  it('never says why a title is unavailable', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': {
        status: 200,
        body: { user: user({ role: 'member' }), profiles: [ada], activeProfile: ada },
      },
      'GET /libraries': { status: 200, body: [] },
      'GET /items/m9': {
        status: 404,
        body: { error: { code: 'not_found', message: 'no such item' } },
      },
    })
    await renderApp('/item/m9')
    expect(
      await screen.findByRole('heading', { name: 'This title isn’t available on this profile.' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to home' })).toHaveAttribute('href', '/')
  })
})

describe('episode page', () => {
  it('is the episode’s own hero, with its show and season above the title', async () => {
    setup({
      ...base,
      id: 'm1',
      kind: 'episode',
      displayTitle: 'Who Is Alive?',
      seasonNumber: 2,
      episodeNumber: 3,
      files: [file('f1')],
      userState: { positionMs: 60000, watched: false, playCount: 0 },
      show: { id: 'sh', displayTitle: 'Severance' },
      metadata: metadata({ trailers: [] }),
    })
    await renderApp('/item/m1')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Who Is Alive?' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Severance · Season 2' })).toHaveAttribute(
      'href',
      '/item/sh?season=2',
    )
    expect(screen.getByRole('link', { name: 'Resume from 1:00' })).toHaveAttribute(
      'href',
      '/watch/m1?t=60',
    )
    expect(screen.getByRole('link', { name: 'Start over' })).toHaveAttribute(
      'href',
      '/watch/m1?t=0',
    )
  })
})

describe('show page', () => {
  const season = (n: number, id: string) => ({
    ...base,
    id,
    kind: 'season',
    displayTitle: '',
    seasonNumber: n,
    childCount: 2,
    technical: { hdrFormats: [], audioLanguages: [] },
    userState: {
      positionMs: 0,
      watched: false,
      playCount: 0,
      watchedEpisodes: n === 1 ? 2 : 0,
      totalEpisodes: 2,
    },
  })
  const episode = (s: number, n: number, id: string, extra: Record<string, unknown> = {}) => ({
    ...base,
    id,
    kind: 'episode',
    displayTitle: `Episode ${n}`,
    seasonNumber: s,
    episodeNumber: n,
    runtimeMinutes: 52,
    overview: `What happens in ${n}.`,
    technical: { hdrFormats: [], audioLanguages: [] },
    show: { id: 'm1', displayTitle: 'Dark' },
    ...extra,
  })

  it('plays the up-next episode, lists the season’s episodes and switches season', async () => {
    const calls = setup(
      {
        ...base,
        id: 'm1',
        kind: 'show',
        displayTitle: 'Dark',
        year: 2017,
        childCount: 3,
        files: [],
        metadata: metadata({ showStatus: 'ended', endDate: '2020-06-27', trailers: [] }),
      },
      {
        'GET /items/m1/up-next': {
          status: 200,
          body: {
            item: episode(2, 1, 's2e1', {
              userState: { positionMs: 60000, watched: false, playCount: 0 },
            }),
          },
        },
        'GET /libraries/lm/items': (req: Request) => {
          const parent = new URL(req.url).searchParams.get('parentId')
          if (parent === 'm1')
            return {
              status: 200,
              body: { items: [season(0, 's0'), season(2, 's2'), season(1, 's1')] },
            }
          if (parent === 's1')
            return { status: 200, body: { items: [episode(1, 1, 's1e1'), episode(1, 2, 's1e2')] } }
          return { status: 200, body: { items: [episode(2, 1, 's2e1'), episode(2, 2, 's2e2')] } }
        },
        'PUT /items/s2/watched': { status: 204 },
      },
    )
    await renderApp('/item/m1')
    expect(await screen.findByText('2017–2020 · Ended · 3 seasons')).toBeInTheDocument()
    expect(await screen.findByRole('link', { name: 'Resume S2 E1' })).toHaveAttribute(
      'href',
      '/watch/s2e1?t=60',
    )
    // The up-next episode's season opens first; seasons are pills, specials last.
    const tabs = await screen.findByRole('tablist', { name: 'Choose a season' })
    expect(within(tabs).getByRole('tab', { name: 'Season 2' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((b) => b.textContent),
    ).toEqual(['Season 1', 'Season 2', 'Specials'])
    expect(await screen.findByRole('link', { name: /1\. Episode 1/ })).toHaveAttribute(
      'href',
      '/watch/s2e1',
    )
    expect(screen.getByText('What happens in 1.')).toBeInTheDocument()
    await userEvent.click(within(tabs).getByRole('tab', { name: 'Season 1' }))
    expect(await screen.findByRole('link', { name: /2\. Episode 2/ })).toHaveAttribute(
      'href',
      '/watch/s1e2',
    )
    expect(screen.getByText('2 episodes · 2 watched')).toBeInTheDocument()
    await userEvent.click(within(tabs).getByRole('tab', { name: 'Season 2' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Mark season as watched' }))
    expect(calls.some((c) => c.method === 'PUT' && c.path === '/items/s2/watched')).toBe(true)
  })

  it('stops the pills after eight seasons; the last one opens them all', async () => {
    setup(
      {
        ...base,
        id: 'm1',
        kind: 'show',
        displayTitle: 'The Simpsons',
        childCount: 12,
        files: [],
        metadata: metadata({ trailers: [] }),
      },
      {
        'GET /items/m1/up-next': { status: 200, body: {} },
        'GET /libraries/lm/items': (req: Request) => {
          const parent = new URL(req.url).searchParams.get('parentId')
          if (parent === 'm1')
            return {
              status: 200,
              body: {
                items: [0, ...Array.from({ length: 12 }, (_, i) => i + 1)].map((n) =>
                  season(n, `s${n}`),
                ),
              },
            }
          return { status: 200, body: { items: [] } }
        },
      },
    )
    await renderApp('/item/m1')
    const tabs = await screen.findByRole('tablist', { name: 'Choose a season' })
    expect(within(tabs).getAllByRole('tab')).toHaveLength(7)
    await userEvent.click(within(tabs).getByRole('button', { name: 'All 12 seasons and Specials' }))
    const picker = screen.getByRole('dialog', { name: 'Choose a season' })
    await userEvent.click(within(picker).getByRole('button', { name: 'Season 11' }))
    expect(within(tabs).getByRole('tab', { name: 'Season 11' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
  })
})
