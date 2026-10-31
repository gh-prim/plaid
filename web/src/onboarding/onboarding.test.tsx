import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiError, fakeApi, profile, server, user } from '../test/fakeApi'
import { FakeEventSource, stubEventSource } from '../test/fakeEventSource'
import { renderApp } from '../test/renderApp'
import { arriveEvery } from './PosterWall'
import { laterKey } from './step'

const signedIn = {
  status: 200,
  body: { user: user(), profiles: [profile('p1', 'Ada')], activeProfile: profile('p1', 'Ada') },
}
const counts = { movies: 0, shows: 0, episodes: 0, files: 0, unrecognized: 0 }
const idle = {
  state: 'idle',
  queued: false,
  full: false,
  counters: {
    found: 0,
    new: 0,
    changed: 0,
    moved: 0,
    missing: 0,
    analyzed: 0,
    failed: 0,
    unrecognized: 0,
  },
}
const library = (extra: Record<string, unknown> = {}) => ({
  id: 'lib1',
  name: 'Movies',
  type: 'movies',
  scanIntervalSeconds: null,
  metadataLanguage: 'en',
  metadataRegion: 'US',
  folders: [{ id: 'f1', path: '/srv/media/movies', watched: true, status: 'ok', fsType: 'ext4' }],
  counts,
  scan: idle,
  ...extra,
})
const capabilities = (hw: Record<string, unknown> = {}) => ({
  status: 200,
  body: {
    ffprobe: { found: true, version: '7.1.1' },
    ffmpeg: { found: true, version: '7.1.1' },
    hwAccel: { configured: 'auto', encoders: [], decoders: [], tonemap: false, ...hw },
    inotifyLimit: 8192,
    inotifyUsed: 10,
    watchLimitReached: [],
  },
})

beforeEach(() => {
  vi.stubGlobal('matchMedia', (q: string) => ({
    matches: q.includes('min-width'),
    media: q,
    addEventListener() {},
    removeEventListener() {},
  }))
})

describe('start', () => {
  it('opens on a fresh server in the browser language, and switches at once', async () => {
    fakeApi({ 'GET /server': server(false) })
    await renderApp('/')
    expect(
      await screen.findByRole('heading', { name: 'Tonight’s movies will come from nas.' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'English' })).toBeChecked()
    await userEvent.click(screen.getByRole('radio', { name: 'Français' }))
    expect(
      await screen.findByRole('heading', { name: 'Ce soir, tes films viendront de nas.' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Configurer nas' })).toBeInTheDocument()
  })

  it('names the server it found, then goes to the account', async () => {
    fakeApi({
      'GET /server': server(false),
      'GET /setup': { status: 200, body: { setupComplete: false, requiresSetupCode: false } },
    })
    await renderApp('/')
    expect(await screen.findByText('PLAID 0.1.0-test')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Set up nas' }))
    expect(
      await screen.findByRole('heading', { name: 'Create the admin account' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Step 1 of 4')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(
      await screen.findByRole('heading', { name: 'Tonight’s movies will come from nas.' }),
    ).toBeInTheDocument()
  })
})

describe('account', () => {
  it('has no setup-code step on the local network, and sends the chosen locale', async () => {
    let done = false
    const calls = fakeApi({
      'GET /server': () => server(done),
      'GET /setup': { status: 200, body: { setupComplete: false, requiresSetupCode: false } },
      'POST /setup/admin': () => {
        done = true
        return { status: 201, body: {} }
      },
      'GET /me': signedIn,
      'GET /libraries': { status: 200, body: [] },
      'GET /fs/browse': { status: 200, body: { path: '/', parent: null, entries: [] } },
    })
    await renderApp('/setup/account', 'fr-FR')
    await screen.findByRole('heading', { name: 'Crée le compte administrateur' })
    expect(screen.queryByLabelText('Code de configuration')).not.toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Nom affiché'), 'Camille')
    await userEvent.type(screen.getByLabelText('Nom d’utilisateur'), 'Camille')
    await userEvent.type(screen.getByLabelText('Mot de passe'), 'plaid and chill forever')
    await userEvent.click(screen.getByRole('button', { name: 'Créer le compte' }))

    expect(
      await screen.findByRole('heading', { name: 'Où sont tes films et séries\u202f?' }),
    ).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/setup/admin')?.body).toMatchObject({
      displayName: 'Camille',
      username: 'camille',
      locale: 'fr',
    })
  })

  it('checks fields before submit and maps server errors to them', async () => {
    const calls = fakeApi({
      'GET /server': server(false),
      'GET /setup': { status: 200, body: { setupComplete: false, requiresSetupCode: false } },
      'POST /setup/admin': apiError(409, 'username_taken'),
    })
    await renderApp('/setup/account')
    await screen.findByRole('heading', { name: 'Create the admin account' })
    await userEvent.type(screen.getByLabelText('Display name'), 'Ada')
    await userEvent.type(screen.getByLabelText('Username'), 'ad')
    await userEvent.type(screen.getByLabelText('Password'), 'plaid and chill')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))
    expect(screen.getByLabelText('Username')).toHaveAttribute('aria-invalid', 'true')
    expect(calls.some((c) => c.path === '/setup/admin')).toBe(false)

    await userEvent.type(screen.getByLabelText('Username'), 'a')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))
    expect(await screen.findByText('This username is already taken.')).toBeInTheDocument()
    expect(screen.getByLabelText('Username')).toHaveAccessibleDescription(
      'This username is already taken.',
    )
  })

  it('asks for the setup code from outside, with the command that finds it', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    fakeApi({
      'GET /server': server(false),
      'GET /setup': { status: 200, body: { setupComplete: false, requiresSetupCode: true } },
      'POST /setup/admin': apiError(403, 'setup_code_required'),
    })
    await renderApp('/setup/account')
    expect(await screen.findByRole('heading', { name: 'Enter the setup code' })).toBeInTheDocument()
    expect(screen.getByText('Only from outside your network')).toBeInTheDocument()
    expect(screen.queryByLabelText('Display name')).not.toBeInTheDocument()
    expect(
      screen.getByText('sudo journalctl -u plaid | grep first_run_setup_code'),
    ).toBeInTheDocument()
    await userEvent.click(screen.getAllByRole('button', { name: 'Copy' })[1]!)
    expect(writeText).toHaveBeenCalledWith('sudo journalctl -u plaid | grep first_run_setup_code')

    // An empty code doesn't go further.
    await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
    expect(screen.getByLabelText('Setup code')).toHaveAttribute('aria-invalid', 'true')

    await userEvent.type(screen.getByLabelText('Setup code'), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
    await userEvent.type(await screen.findByLabelText('Display name'), 'Ada')
    await userEvent.type(screen.getByLabelText('Username'), 'ada')
    await userEvent.type(screen.getByLabelText('Password'), 'plaid and chill')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))
    // The wrong code brings its card back, with the code still typed.
    expect(
      await screen.findByText('This code doesn’t match. Copy the latest one from the log.'),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Setup code')).toHaveValue('wrong')
    expect(screen.getByLabelText('Setup code')).toHaveAccessibleDescription(
      'This code doesn’t match. Copy the latest one from the log.',
    )
  })

  it('locks the code for the time the server asks after too many attempts', async () => {
    const calls = fakeApi({
      'GET /server': server(false),
      'GET /setup': { status: 200, body: { setupComplete: false, requiresSetupCode: true } },
      'POST /setup/admin': apiError(429, 'too_many_attempts', { 'Retry-After': '300' }),
    })
    await renderApp('/setup/account')
    await userEvent.type(await screen.findByLabelText('Setup code'), 'abcd-2345')
    await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
    await userEvent.type(await screen.findByLabelText('Display name'), 'Ada')
    await userEvent.type(screen.getByLabelText('Username'), 'ada')
    await userEvent.type(screen.getByLabelText('Password'), 'plaid and chill')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))
    expect(await screen.findByText('Too many attempts. Try again in 5 min.')).toBeInTheDocument()
    expect(screen.getByLabelText('Setup code')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled()
    expect(calls.find((c) => c.path === '/setup/admin')?.body).toMatchObject({
      setupCode: 'ABCD2345',
    })
  })

  it('says so when someone else finished setup first', async () => {
    fakeApi({
      'GET /server': server(false),
      'GET /setup': { status: 200, body: { setupComplete: false, requiresSetupCode: false } },
      'POST /setup/admin': apiError(409, 'setup_complete'),
    })
    await renderApp('/setup/account')
    await screen.findByLabelText('Display name')
    await userEvent.type(screen.getByLabelText('Display name'), 'Ada')
    await userEvent.type(screen.getByLabelText('Username'), 'ada')
    await userEvent.type(screen.getByLabelText('Password'), 'plaid and chill')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))
    expect(
      await screen.findByRole('heading', { name: 'This server is already set up.' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument()
  })
})

describe('libraries', () => {
  const browse = (
    path: string,
    entries: { name: string; readable: boolean }[],
    parent: string | null = '/',
  ) => ({
    status: 200,
    body: { path, parent, entries: entries.map((e) => ({ ...e, path: `${path}/${e.name}` })) },
  })

  it('picks a folder on the server, explains an unreadable one, and adds the library', async () => {
    let readable = false
    let libs: unknown[] = []
    const calls = fakeApi({
      'GET /server': server(true),
      'GET /me': signedIn,
      'GET /libraries': () => ({ status: 200, body: libs }),
      'GET /fs/browse': (req) => {
        const path = new URL(req.url).searchParams.get('path')
        if (!path) return browse('/srv', [{ name: 'media', readable: true }], null)
        if (path === '/srv/media')
          return browse('/srv/media', [
            { name: "Bob's films", readable },
            { name: 'Kids', readable: true },
          ])
        return browse(path, [])
      },
      'POST /libraries': (_req, body) => {
        libs = [
          library({
            folders: [
              {
                id: 'f1',
                path: (body as { folders: string[] }).folders[0],
                watched: true,
                status: 'ok',
              },
            ],
          }),
        ]
        return { status: 201, body: libs[0] }
      },
    })
    await renderApp('/setup/libraries')
    await screen.findByRole('heading', { name: 'Where are your movies and shows?' })
    await userEvent.click(screen.getByRole('button', { name: 'Add folder' }))
    const picker = await screen.findByRole('dialog', { name: 'Folders on nas' })
    await userEvent.click(await within(picker).findByRole('button', { name: /media/ }))
    await userEvent.click(
      await within(picker).findByRole('button', { name: /Bob's films.*No access/ }),
    )
    expect(
      within(picker).getByText(`sudo setfacl -R -m u:plaid:rX '/srv/media/Bob'\\''s films'`),
    ).toBeInTheDocument()

    readable = true
    await userEvent.click(within(picker).getByRole('button', { name: 'Check again' }))
    expect(await within(picker).findByText('Readable')).toBeInTheDocument()
    await userEvent.click(within(picker).getByRole('button', { name: /Bob's films/ }))
    await userEvent.click(within(picker).getByRole('button', { name: 'Use this folder' }))

    expect(await screen.findByText("/srv/media/Bob's films")).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Add library' }))
    expect(await screen.findByRole('button', { name: 'Follow the scan' })).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'POST' && c.path === '/libraries')?.body).toEqual({
      name: 'Movies',
      type: 'movies',
      folders: ["/srv/media/Bob's films"],
    })
  })

  it('lists the browse roots and filters a long folder', async () => {
    const many = Array.from({ length: 40 }, (_, i) => ({ name: `Folder ${i + 1}`, readable: true }))
    fakeApi({
      'GET /server': server(true),
      'GET /me': signedIn,
      'GET /libraries': { status: 200, body: [] },
      'GET /fs/browse': (req) => {
        const path = new URL(req.url).searchParams.get('path')
        if (!path)
          return {
            status: 200,
            body: {
              path: '',
              parent: null,
              entries: [
                { name: '/srv', path: '/srv', readable: true },
                { name: '/mnt', path: '/mnt', readable: true },
              ],
            },
          }
        return browse(path, many, null)
      },
    })
    await renderApp('/setup/libraries')
    await userEvent.click(await screen.findByRole('button', { name: 'Add folder' }))
    const picker = await screen.findByRole('dialog', { name: 'Folders on nas' })
    expect(await within(picker).findByText(/PLAID_BROWSE_ROOTS/)).toBeInTheDocument()
    expect(within(picker).getByRole('button', { name: 'Parent folder' })).toBeDisabled()
    await userEvent.click(within(picker).getByRole('button', { name: /\/srv/ }))
    const filter = await within(picker).findByRole('textbox', { name: 'Filter 40 folders' })
    await userEvent.type(filter, 'Folder 3')
    expect(within(picker).getAllByRole('button', { name: /Folder 3/ })).toHaveLength(11)
    await userEvent.clear(filter)
    await userEvent.type(filter, 'zzz')
    expect(within(picker).getByText('No folder matches “zzz”.')).toBeInTheDocument()
  })

  it('says which library already holds an overlapping folder', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': signedIn,
      'GET /libraries': { status: 200, body: [library({ name: 'Films' })] },
      'GET /fs/browse': browse('/srv/media/movies', [], '/srv/media'),
      'POST /libraries': apiError(409, 'folder_overlap'),
    })
    await renderApp('/setup/libraries')
    await userEvent.click(await screen.findByRole('button', { name: 'Add another library' }))
    await userEvent.click(screen.getByRole('button', { name: 'Add folder' }))
    const picker = await screen.findByRole('dialog', { name: 'Folders on nas' })
    expect(await within(picker).findByText('No sub-folders.')).toBeInTheDocument()
    await userEvent.click(within(picker).getByRole('button', { name: 'Use this folder' }))
    await userEvent.click(screen.getByRole('button', { name: 'Add library' }))
    expect(await screen.findByText('This folder is already part of Films.')).toBeInTheDocument()
  })

  it('can be skipped, and the home then stays put', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': signedIn,
      'GET /libraries': { status: 200, body: [] },
    })
    await renderApp('/setup/libraries')
    await userEvent.click(await screen.findByRole('button', { name: 'Add libraries later' }))
    expect(localStorage.getItem(laterKey)).toBe('1')
    // The home, in its shell (F008-R1).
    expect(await screen.findAllByRole('button', { name: /^Profile menu for / })).not.toHaveLength(0)
  })
})

describe('resume', () => {
  it('sends an admin back to the libraries step when there is none', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': signedIn,
      'GET /libraries': { status: 200, body: [] },
    })
    await renderApp('/')
    expect(
      await screen.findByRole('heading', { name: 'Where are your movies and shows?' }),
    ).toBeInTheDocument()
  })

  it('never sends a member to the wizard', async () => {
    const member = {
      status: 200,
      body: {
        user: user({ role: 'member' }),
        profiles: [profile('p1', 'Sam')],
        activeProfile: profile('p1', 'Sam'),
      },
    }
    fakeApi({ 'GET /server': server(true), 'GET /me': member })
    await renderApp('/setup/libraries')
    // The home, in its shell (F008-R1).
    expect(await screen.findAllByRole('button', { name: /^Profile menu for / })).not.toHaveLength(0)
  })
})

describe('scan', () => {
  const scanning = (found: number, analyzed: number) => ({
    state: 'analyzing',
    queued: false,
    full: false,
    counters: {
      found,
      new: found,
      changed: 0,
      moved: 0,
      missing: 0,
      analyzed,
      failed: 0,
      unrecognized: 23,
    },
  })
  const routes = (extra: Record<string, unknown> = {}, hw: Record<string, unknown> = {}) => ({
    'GET /server': server(true),
    'GET /me': signedIn,
    'GET /libraries': { status: 200, body: [library({ scan: scanning(1263, 0) })] },
    'GET /server/capabilities': capabilities(hw),
    'GET /settings/playback': { status: 200, body: { maxTranscodes: 4 } },
    'GET /libraries/lib1/items': { status: 200, body: { items: [] } },
    ...extra,
  })

  it('shows the counters, the thread and posters as they arrive', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    stubEventSource()
    fakeApi(routes())
    const { container } = await renderApp('/setup/scan')
    await waitFor(() => expect(FakeEventSource.last).toBeDefined())
    act(() => {
      FakeEventSource.last!.emit('progress', { type: 'progress', progress: scanning(1263, 806) })
      FakeEventSource.last!.emit('metadata', {
        type: 'metadata',
        metadata: { pending: 840, matched: 412, needsReview: 18, unmatched: 3 },
      })
      FakeEventSource.last!.emit('matched', {
        type: 'matched',
        item: {
          id: 'm1',
          kind: 'movie',
          title: 'Dune: Part Two',
          year: 2024,
          poster: { imageId: 'img1' },
        },
      })
    })
    expect(await screen.findByText('806 of 1,263 analyzed')).toBeInTheDocument()
    expect(
      screen.getByText(/412 matched · 18 to review · 3 not found · 23 not recognized/),
    ).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '64')
    await act(() => vi.advanceTimersByTimeAsync(arriveEvery * 2))
    expect(screen.getByRole('img', { name: 'Dune: Part Two' })).toHaveAttribute(
      'src',
      '/api/v1/images/img1?w=342',
    )
    await act(() => vi.advanceTimersByTimeAsync(3000))
    expect(
      screen.getByText('Dune: Part Two just arrived. 412 of your movies are here.'),
    ).toBeInTheDocument()
    // The wall fills the screen behind the words, and stays small in the DOM.
    expect(container.querySelectorAll('.pl-slot').length).toBeLessThanOrEqual(60)
    expect(screen.getByText('Step 3 of 4')).toBeInTheDocument()
    vi.useRealTimers()
  })

  it('tells a Pi with no encoder apart from an NVIDIA box', async () => {
    stubEventSource()
    fakeApi(routes())
    await renderApp('/setup/scan')
    expect(await screen.findByText(/No hardware encoder/)).toBeInTheDocument()
  })

  it('names the GPU when one passed the test', async () => {
    stubEventSource()
    fakeApi(routes({}, { active: 'nvenc' }))
    await renderApp('/setup/scan')
    expect(
      await screen.findByText(
        'Video conversion uses the NVIDIA GPU (NVENC): up to 4 at once when a device can’t play a file as it is.',
      ),
    ).toBeInTheDocument()
  })

  it('waits for matching when files are scanned before the first metadata counter', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    stubEventSource()
    let pending = 30
    fakeApi(
      routes({
        'GET /metadata/status': () => ({
          status: 200,
          body: {
            providers: [],
            libraries: [{ libraryId: 'lib1', pending, matched: 4, needsReview: 0, unmatched: 0 }],
          },
        }),
        'GET /libraries/lib1/items': { status: 200, body: { items: [] } },
      }),
    )
    await renderApp('/setup/scan')
    await waitFor(() => expect(FakeEventSource.last).toBeDefined())
    act(() => FakeEventSource.last!.emit('progress', { type: 'progress', progress: idle }))
    await act(() => vi.advanceTimersByTimeAsync(100))
    expect(screen.queryByRole('button', { name: 'Start watching' })).not.toBeInTheDocument()
    pending = 0
    await act(() => vi.advanceTimersByTimeAsync(2100))
    expect(
      await screen.findByText(/titles? (is|are) home|Nothing to watch yet/),
    ).toBeInTheDocument()
    vi.useRealTimers()
  })

  it('ends on the wall, oldest to newest, with what waits for later', async () => {
    stubEventSource()
    const items = [
      { id: 'a', kind: 'movie', title: 'Metropolis', year: 1927, poster: { imageId: 'p1' } },
      { id: 'b', kind: 'movie', title: 'Sinners', year: 2025, poster: { imageId: 'p2' } },
      { id: 'c', kind: 'movie', title: 'Alien', year: 1979, poster: { imageId: 'p3' } },
    ]
    fakeApi(
      routes({
        'GET /libraries': {
          status: 200,
          body: [
            library({
              counts: { ...counts, movies: 1240, unrecognized: 23 },
              lastScanAt: '2026-10-29T10:00:00Z',
            }),
          ],
        },
        'GET /libraries/lib1/items': { status: 200, body: { items } },
      }),
    )
    await renderApp('/setup/scan')
    await waitFor(() => expect(FakeEventSource.last).toBeDefined())
    act(() => {
      FakeEventSource.last!.emit('progress', { type: 'progress', progress: idle })
      FakeEventSource.last!.emit('metadata', {
        type: 'metadata',
        metadata: { pending: 0, matched: 1199, needsReview: 18, unmatched: 0 },
      })
    })
    expect(
      await screen.findByRole('heading', {
        name: 'From Metropolis to Sinners, 1,240 movies are home.',
      }),
    ).toBeInTheDocument()
    expect(
      screen.getByText(/23 files weren’t recognized and 18 titles need a second look/),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Start watching' })).toBeInTheDocument()
  })
})
