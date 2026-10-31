import { act, renderHook, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import en from '../locales/en/common.json'
import { apiError, fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'
import { depth, travel } from './AdminLayout'
import { reasonCodes, reasonText } from './reasons'
import { temporaryPassword } from './People'
import { parseQuery } from './Review'
import { useLeaving } from './v2'

const admin = {
  status: 200,
  body: { user: user(), profiles: [profile('p1', 'Ada')], activeProfile: profile('p1', 'Ada') },
}
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
const library = (
  folders: unknown[] = [
    { id: 'f1', path: '/srv/media/movies', watched: true, status: 'ok', fsType: 'ext4' },
  ],
) => ({
  id: 'lib1',
  name: 'Movies',
  type: 'movies',
  scanIntervalSeconds: null,
  metadataLanguage: 'en',
  metadataRegion: 'US',
  lastScanAt: '2026-10-29T10:00:00Z',
  folders,
  counts: { movies: 1240, shows: 0, episodes: 0, files: 1263, unrecognized: 23 },
  scan: idle,
})
const caps = {
  status: 200,
  body: {
    ffprobe: { found: true },
    ffmpeg: { found: true },
    hwAccel: { configured: 'auto', encoders: [], decoders: [], tonemap: false },
    inotifyLimit: 0,
    inotifyUsed: 0,
    watchLimitReached: [],
  },
}
const stream = (
  id: string,
  method: string,
  extra: Record<string, unknown> = {},
  session: Record<string, unknown> = {},
) => ({
  session: {
    id,
    itemId: `i-${id}`,
    fileId: 'f',
    method,
    reasons: [],
    url: '',
    expiresAfterIdleSeconds: 60,
    durationMs: 6_000_000,
    startPositionMs: 0,
    quality: 'original',
    qualities: [],
    warnings: [],
    audioTracks: [],
    subtitles: [],
    ...session,
  },
  userId: 'u2',
  username: 'sam',
  profileId: 'p2',
  profileName: 'Sam',
  deviceName: 'Firefox on Linux',
  itemTitle: `Movie ${id}`,
  positionMs: 1_200_000,
  remote: false,
  startedAt: '2026-10-29T20:00:00Z',
  lastActivityAt: '2026-10-29T20:30:00Z',
  ...extra,
})
const settings = {
  maxTranscodes: 2,
  maxTranscodesDefault: 2,
  remoteMaxBitrate: 8_000_000,
  localNetworks: [],
  transcodeDirMaxBytes: 0,
  transcodeDirMaxBytesDefault: 10e9,
  encoderPreset: 'fast',
  hwAccel: 'auto',
  hardware: { configured: 'auto', encoders: [], decoders: [], tonemap: false },
}

function routes(extra: Record<string, unknown> = {}) {
  return {
    'GET /server': server(true),
    'GET /me': admin,
    'GET /playback': { status: 200, body: { sessions: [] } },
    'GET /libraries': { status: 200, body: [library()] },
    'GET /server/capabilities': caps,
    'GET /problems': { status: 200, body: { problems: [] } },
    'GET /settings/playback': { status: 200, body: settings },
    ...extra,
  }
}

beforeEach(() => {
  vi.stubGlobal('matchMedia', (q: string) => ({
    matches: false,
    media: q,
    addEventListener() {},
    removeEventListener() {},
  }))
})

describe('access', () => {
  it('sends a member home without asking for admin data', async () => {
    const member = {
      status: 200,
      body: {
        user: user({ role: 'member' }),
        profiles: [profile('p1', 'Sam')],
        activeProfile: profile('p1', 'Sam'),
      },
    }
    const calls = fakeApi({ 'GET /server': server(true), 'GET /me': member })
    await renderApp('/admin')
    // The home, in its shell (F008-R1).
    expect(await screen.findAllByRole('button', { name: /^Profile menu for / })).not.toHaveLength(0)
    expect(calls.some((c) => c.path === '/playback')).toBe(false)
  })
})

describe('home', () => {
  it('says all is well on a quiet evening, and Settings lists the sections', async () => {
    fakeApi(routes())
    await renderApp('/admin')
    expect(await screen.findByText('All good. Nothing is playing.')).toBeInTheDocument()
    expect(screen.getByText('Nobody is watching right now')).toBeInTheDocument()
    expect(screen.getByText('Nothing needs you.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: 'Settings' }))
    expect(await screen.findByRole('link', { name: /Libraries/ })).toHaveAttribute(
      'href',
      '/admin/libraries',
    )
  })

  it('lists an unavailable NAS folder under "needs you"', async () => {
    fakeApi(
      routes({
        'GET /libraries': {
          status: 200,
          body: [
            library([
              {
                id: 'f1',
                path: '/mnt/nas/Films',
                watched: false,
                status: 'unavailable',
                fsType: 'nfs',
              },
            ]),
          ],
        },
      }),
    )
    await renderApp('/admin')
    const needs = await screen.findByRole('region', { name: 'Needs you' })
    expect(within(needs).getByText('/mnt/nas/Films can’t be reached.')).toBeInTheDocument()
    expect(screen.getByText('One thing needs you.')).toBeInTheDocument()
  })
})

describe('playing now', () => {
  it('puts the slow software transcode first, with its reasons and the advice', async () => {
    fakeApi(
      routes({
        'GET /playback': {
          status: 200,
          body: {
            sessions: [
              stream('a', 'direct_play'),
              stream('b', 'transcode', { speed: 0.8 }, { reasons: ['video_codec_not_supported'] }),
              stream(
                'c',
                'transcode',
                { speed: 1.4, remote: true },
                { reasons: ['bitrate_too_high'], hardwareAcceleration: 'vaapi' },
              ),
              stream('d', 'direct_stream', {}, { reasons: ['container_not_supported'] }),
            ],
          },
        },
      }),
    )
    await renderApp('/admin/playing')
    const cards = await screen.findAllByRole('article')
    expect(within(cards[0]!).getAllByText('Movie b')[0]).toBeInTheDocument()
    expect(within(cards[0]!).getByText('Can’t keep up (0.8×): it will buffer.')).toBeInTheDocument()
    expect(
      within(cards[0]!).getByText('This device can’t play this video format, so it’s converted.'),
    ).toBeInTheDocument()
    expect(within(cards[0]!).getByText(/Ask Sam to pick a lower quality/)).toBeInTheDocument()
    expect(
      screen.getByText('Watching from outside: lowered to your 8 Mbps limit for remote streams.'),
    ).toBeInTheDocument()
    expect(screen.getByText(/4 streams · 2 of 2 conversions in use/)).toBeInTheDocument()
  })

  it('stops a stream after a confirmation', async () => {
    let sessions = [stream('b', 'transcode', { speed: 1.2 })]
    const calls = fakeApi(
      routes({
        'GET /playback': () => ({ status: 200, body: { sessions } }),
        'DELETE /playback/b': () => {
          sessions = []
          return { status: 204 }
        },
      }),
    )
    await renderApp('/admin/playing')
    await userEvent.click(await screen.findByRole('button', { name: 'Stop' }))
    const dialog = screen.getByRole('dialog', { name: 'Stop Movie b for Sam?' })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Stop stream' }))
    expect(await screen.findByText('Stopped Movie b for Sam.')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/playback/b')).toBe(true)
    expect(await screen.findByText('Nothing is playing.')).toBeInTheDocument()
  })
})

describe('libraries', () => {
  it('shows an unavailable folder with what to check', async () => {
    fakeApi(
      routes({
        'GET /libraries': {
          status: 200,
          body: [
            library([
              {
                id: 'f1',
                path: '/mnt/nas/Films',
                watched: false,
                status: 'unavailable',
                fsType: 'nfs',
              },
            ]),
          ],
        },
      }),
    )
    await renderApp('/admin/libraries/lib1')
    expect(await screen.findByText(/Its movies are hidden until it’s back/)).toBeInTheDocument()
    expect(
      screen.getByText(`findmnt '/mnt/nas/Films' || sudo mount '/mnt/nas/Films'`),
    ).toBeInTheDocument()
  })

  it('matches an item to the chosen TMDB candidate', async () => {
    const calls = fakeApi(
      routes({
        'GET /libraries/lib1/review': {
          status: 200,
          body: {
            items: [
              {
                item: { id: 'it1', kind: 'movie', title: 'Dune', year: 2021 },
                reason: 'needs_review',
                candidates: [
                  { tmdbId: 438631, title: 'Dune', year: 2021, confidence: 0.82 },
                  { tmdbId: 841, title: 'Dune', year: 1984, confidence: 0.61 },
                ],
              },
            ],
          },
        },
        'POST /items/it1/match': { status: 200, body: {} },
      }),
    )
    await renderApp('/admin/libraries/lib1/review')
    expect(await screen.findByText('82% match')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: /Dune.*1984/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Use Dune (1984)' }))
    await waitFor(() =>
      expect(calls.find((c) => c.path === '/items/it1/match')?.body).toEqual({ tmdbId: 841 }),
    )
    expect(await screen.findByText('Dune is matched and locked.')).toBeInTheDocument()
  })

  it('opens on the item a "Fix match" came from, even outside the queue', async () => {
    let office = 0
    const calls = fakeApi(
      routes({
        'GET /libraries/lib1/review': {
          status: 200,
          body: {
            items: [
              {
                item: { id: 'b1', kind: 'show', title: 'Band of Brothers' },
                reason: 'needs_review',
                candidates: [{ tmdbId: 4613, title: 'Band of Brothers', year: 2001 }],
              },
            ],
          },
        },
        // The review reads it once; the item page it returns to, after.
        'GET /items/s1': () =>
          office++
            ? { status: 404 }
            : {
                status: 200,
                body: { id: 's1', kind: 'show', title: 'The Office (US)', libraryId: 'lib1' },
              },
        'GET /metadata/search': {
          status: 200,
          body: {
            candidates: [{ tmdbId: 2316, title: 'The Office', year: 2005, confidence: 0.9 }],
          },
        },
        'POST /items/s1/match': { status: 200, body: {} },
      }),
    )
    await renderApp('/admin/libraries/lib1/review?item=s1')
    expect(await screen.findByRole('radio', { name: /The Office.*2005/ })).toBeInTheDocument()
    expect(screen.queryByText('Band of Brothers')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: /The Office.*2005/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Use The Office (2005)' }))
    await waitFor(() =>
      expect(calls.find((c) => c.path === '/items/s1/match')?.body).toEqual({ tmdbId: 2316 }),
    )
    await waitFor(() => expect(office).toBe(2))
  })

  it('shows the match is on its way and sends it once', async () => {
    let release = () => {}
    const calls = fakeApi(
      routes({
        'GET /libraries/lib1/review': {
          status: 200,
          body: {
            items: [
              {
                item: { id: 'it1', kind: 'show', title: 'The Office' },
                reason: 'needs_review',
                candidates: [{ tmdbId: 2316, title: 'The Office', year: 2005 }],
              },
            ],
          },
        },
        'POST /items/it1/match': () =>
          new Promise((r) => (release = () => r({ status: 200, body: {} }))),
      }),
    )
    await renderApp('/admin/libraries/lib1/review')
    await userEvent.click(await screen.findByRole('radio', { name: /The Office/ }))
    const use = screen.getByRole('button', { name: /Use The Office/ })
    await userEvent.click(use)
    expect(use).toHaveAttribute('aria-busy', 'true')
    expect(use).toBeDisabled()
    await userEvent.click(use)
    expect(calls.filter((c) => c.path === '/items/it1/match')).toHaveLength(1)
    release()
    expect(await screen.findByText('The Office is matched and locked.')).toBeInTheDocument()
  })

  it('identifies an unrecognized file through a TMDB search', async () => {
    const calls = fakeApi(
      routes({
        'GET /libraries/lib1/unrecognized': {
          status: 200,
          body: {
            files: [
              {
                id: 'mf1',
                path: '/srv/media/movies/movie.final.FINAL.v2.mkv',
                size: 1,
                probeStatus: 'ok',
              },
            ],
          },
        },
        'GET /metadata/search': {
          status: 200,
          body: { candidates: [{ tmdbId: 19, title: 'Metropolis', year: 1927 }] },
        },
        'POST /media-files/mf1/identify': { status: 200, body: {} },
      }),
    )
    await renderApp('/admin/libraries/lib1/unrecognized')
    await userEvent.click(await screen.findByRole('button', { name: 'Identify' }))
    const sheet = screen.getByRole('dialog', { name: 'Match this file' })
    expect(within(sheet).getByText('movie.final.FINAL.v2.mkv')).toBeInTheDocument()
    await userEvent.type(
      within(sheet).getByRole('searchbox', { name: 'Search TMDB' }),
      'Metropolis 1927',
    )
    await userEvent.click(
      await within(sheet).findByRole(
        'button',
        { name: 'Use Metropolis (1927)' },
        { timeout: 2000 },
      ),
    )
    expect(await screen.findByText('Identified')).toBeInTheDocument()
    const search = calls.find((c) => c.path === '/metadata/search')
    expect(search).toBeDefined()
    expect(calls.find((c) => c.path === '/media-files/mf1/identify')?.body).toEqual({ tmdbId: 19 })
  })
})

describe('people', () => {
  const users = [
    {
      id: 'u1',
      username: 'ada',
      displayName: 'Ada',
      role: 'admin',
      locale: 'en',
      mustChangePassword: false,
      disabled: false,
      createdAt: '2026-10-01T00:00:00Z',
    },
  ]
  it('explains the last admin', async () => {
    fakeApi(
      routes({
        'GET /users': { status: 200, body: users },
        'GET /users/u1/sessions': { status: 200, body: [] },
        'PATCH /users/u1': apiError(409, 'last_admin'),
      }),
    )
    await renderApp('/admin/people/u1')
    await userEvent.click(await screen.findByRole('button', { name: 'Disable' }))
    expect(
      await screen.findByText('You’re the last admin. Make someone else an admin first.'),
    ).toBeInTheDocument()
  })

  it('signs a device out', async () => {
    const calls = fakeApi(
      routes({
        'GET /users': { status: 200, body: users },
        'GET /users/u1/sessions': {
          status: 200,
          body: [
            {
              id: 's9',
              deviceName: 'Living-room TV',
              deviceType: 'tv',
              createdAt: '2026-10-01T00:00:00Z',
              lastSeenAt: '2026-10-29T10:00:00Z',
              current: false,
            },
          ],
        },
        'DELETE /users/u1/sessions/s9': { status: 204 },
      }),
    )
    await renderApp('/admin/people/u1')
    await userEvent.click(await screen.findByRole('button', { name: 'Sign out' }))
    await waitFor(() =>
      expect(calls.some((c) => c.method === 'DELETE' && c.path === '/users/u1/sessions/s9')).toBe(
        true,
      ),
    )
  })
})

describe('transcoding', () => {
  it('sends only what changed, saved with one button', async () => {
    const calls = fakeApi(
      routes({
        'PATCH /settings/playback': (_r: Request, body: unknown) => ({
          status: 200,
          body: { ...settings, ...(body as object) },
        }),
      }),
    )
    await renderApp('/admin/transcoding')
    const field = await screen.findByLabelText('Conversions at once')
    await userEvent.clear(field)
    await userEvent.type(field, '3')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByText('Saved.')).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ maxTranscodes: 3 })
  })

  it('keeps rare settings behind "Show advanced" and refuses an invalid network', async () => {
    const calls = fakeApi(routes())
    await renderApp('/admin/transcoding')
    await screen.findByLabelText('Hardware acceleration')
    expect(screen.queryByLabelText('Also count as home')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Show advanced' }))
    await userEvent.type(screen.getByLabelText('Also count as home'), '10.8.0.0/33')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(screen.getByText('One network per line, like 10.8.0.0/24.')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'PATCH')).toBe(false)
  })

  it('runs the hardware test again', async () => {
    const calls = fakeApi(
      routes({
        'POST /server/capabilities/hardware-test': {
          status: 200,
          body: {
            configured: 'auto',
            active: 'vaapi',
            device: '/dev/dri/renderD128',
            encoders: ['h264_vaapi'],
            decoders: ['h264'],
            tonemap: true,
          },
        },
      }),
    )
    await renderApp('/admin/transcoding')
    await userEvent.click(await screen.findByRole('button', { name: 'Test the GPU again' }))
    expect(
      await screen.findByText('VAAPI on /dev/dri/renderD128 passed the hardware test.'),
    ).toBeInTheDocument()
    expect(
      calls.some((c) => c.method === 'POST' && c.path === '/server/capabilities/hardware-test'),
    ).toBe(true)
  })
})

describe('metadata', () => {
  it('stores a key and never shows it again', async () => {
    let source = 'builtin'
    const calls = fakeApi(
      routes({
        'GET /metadata/settings': () => ({
          status: 200,
          body: {
            tmdb: { keySource: source },
            fanart: { keySource: 'none', personalKeySet: false },
          },
        }),
        'GET /metadata/status': () => ({
          status: 200,
          body: {
            providers: [{ provider: 'tmdb', state: 'ok', keySource: source }],
            libraries: [],
          },
        }),
        'PATCH /metadata/settings': () => {
          source = 'custom'
          return {
            status: 200,
            body: {
              tmdb: { keySource: 'custom' },
              fanart: { keySource: 'none', personalKeySet: false },
            },
          }
        },
      }),
    )
    await renderApp('/admin/metadata')
    await userEvent.click(await screen.findByRole('button', { name: 'Use my own key' }))
    await userEvent.type(screen.getByLabelText('Use my own key'), 'abc123')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ tmdbApiKey: 'abc123' }),
    )
    expect(screen.queryByDisplayValue('abc123')).not.toBeInTheDocument()
  })
})

describe('problems', () => {
  it('lists what happened and what to do; empty after 7 quiet days', async () => {
    fakeApi(
      routes({
        'GET /problems': {
          status: 200,
          body: {
            problems: [
              {
                id: 'p1',
                code: 'hw_fallback',
                params: { accel: 'vaapi' },
                firstSeen: '2026-10-27T20:00:00Z',
                lastSeen: '2026-10-27T20:00:00Z',
                count: 1,
              },
            ],
          },
        },
      }),
    )
    await renderApp('/admin/problems')
    expect(await screen.findByText(/VAAPI failed 3 times at/)).toBeInTheDocument()
    expect(screen.getByText(/Test the GPU again in Transcoding/)).toBeInTheDocument()
  })

  it('says when there is nothing', async () => {
    fakeApi(routes())
    await renderApp('/admin/problems')
    expect(await screen.findByText('No problems in the last 7 days.')).toBeInTheDocument()
  })
})

describe('section pages (V2)', () => {
  const nas = [
    {
      id: 'f1',
      path: '/mnt/nas/Films',
      watched: false,
      status: 'unavailable',
      fsType: 'nfs',
    },
  ]
  const users = [
    {
      id: 'u1',
      username: 'ada',
      displayName: 'Ada',
      role: 'admin',
      locale: 'en',
      mustChangePassword: false,
      disabled: false,
      createdAt: '2026-10-01T00:00:00Z',
    },
    {
      id: 'u2',
      username: 'sam',
      displayName: 'Sam',
      role: 'member',
      locale: 'en',
      mustChangePassword: false,
      disabled: false,
      createdAt: '2026-10-02T00:00:00Z',
    },
  ]
  const session = (id: string, deviceName: string, deviceType: string, lastSeenAt: string) => ({
    id,
    deviceName,
    deviceType,
    createdAt: '2026-10-01T00:00:00Z',
    lastSeenAt,
    current: false,
  })

  it('puts Needs you first in Settings, then the sections in groups', async () => {
    fakeApi(routes({ 'GET /libraries': { status: 200, body: [library(nas)] } }))
    await renderApp('/admin/settings')
    const needs = await screen.findByRole('link', { name: /Needs you/ })
    expect(needs).toHaveAttribute('href', '/admin')
    expect(within(needs).getByText('1 thing')).toBeInTheDocument()
    const group = screen.getByRole('region', { name: 'Library' })
    expect(within(group).getByRole('link', { name: /Libraries/ })).toHaveTextContent('Unavailable')
    expect(within(group).getByRole('link', { name: /Not recognized/ })).toHaveTextContent('23')
    expect(
      within(screen.getByRole('region', { name: 'People and devices' })).getByRole('link', {
        name: /Devices/,
      }),
    ).toHaveAttribute('href', '/admin/devices')
    // The tile says where you are: the server and its state in words.
    expect(screen.getByText('One thing needs you.')).toBeInTheDocument()
  })

  it('shows the libraries with their trouble, and scans them all', async () => {
    const calls = fakeApi(
      routes({
        'GET /libraries': { status: 200, body: [library(nas)] },
        'POST /libraries/lib1/scan': { status: 202, body: {} },
      }),
    )
    await renderApp('/admin/libraries')
    const movies = await screen.findByRole('link', { name: /Movies/ })
    expect(movies).toHaveAttribute('href', '/admin/libraries/lib1')
    expect(within(movies).getByText('1 of 1 folders unavailable')).toBeInTheDocument()
    expect(screen.getByText('One folder unavailable')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Scan all libraries now' }))
    expect(await screen.findByText('Scanning 1 library.')).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/libraries/lib1/scan')?.body).toEqual({ full: false })
  })

  it('goes back from a library to Libraries', async () => {
    fakeApi(routes())
    await renderApp('/admin/libraries/lib1')
    expect(await screen.findByRole('link', { name: 'Back to Libraries' })).toHaveAttribute(
      'href',
      '/admin/libraries',
    )
    // The back link shows before the library has loaded.
    expect(await screen.findByRole('heading', { level: 1, name: 'Movies' })).toBeInTheDocument()
    expect(screen.getByText('/srv/media/movies')).toBeInTheDocument()
    expect(screen.getByText('Reachable')).toBeInTheDocument()
  })

  it('keeps the only admin an admin, and says why', async () => {
    fakeApi(
      routes({
        'GET /users': { status: 200, body: [users[0]] },
        'GET /users/u1/sessions': { status: 200, body: [] },
      }),
    )
    await renderApp('/admin/people/u1')
    const toggle = await screen.findByRole('switch', { name: 'Admin' })
    expect(toggle).toBeChecked()
    expect(toggle).toBeDisabled()
    expect(
      screen.getByText(
        'Ada is the only admin. Make someone else an admin before turning this off.',
      ),
    ).toBeInTheDocument()
  })

  it('makes a member an admin with the switch', async () => {
    const calls = fakeApi(
      routes({
        'GET /users': { status: 200, body: users },
        'GET /users/u2/sessions': { status: 200, body: [] },
        'PATCH /users/u2': { status: 200, body: { ...users[1], role: 'admin' } },
      }),
    )
    await renderApp('/admin/people/u2')
    const toggle = await screen.findByRole('switch', { name: 'Admin' })
    expect(toggle).not.toBeChecked()
    await userEvent.click(toggle)
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ role: 'admin' }),
    )
    expect(screen.getByRole('button', { name: 'Delete account' })).toBeInTheDocument()
  })

  it('shows a spinner while a person loads, never a blank', async () => {
    fakeApi(
      routes({
        'GET /users': () => new Promise(() => {}),
        'GET /users/u1/sessions': { status: 200, body: [] },
      }),
    )
    await renderApp('/admin/people/u1')
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeInTheDocument()
  })

  it('lists every signed-in device, TVs apart, and signs one out', async () => {
    const calls = fakeApi(
      routes({
        'GET /users': { status: 200, body: users },
        'GET /users/u1/sessions': {
          status: 200,
          body: [session('s1', 'Chrome on MacBook', 'web', '2026-10-29T10:00:00Z')],
        },
        'GET /users/u2/sessions': {
          status: 200,
          body: [session('s2', 'Living room TV', 'tvos', '2026-10-29T11:00:00Z')],
        },
        'DELETE /users/u2/sessions/s2': { status: 204 },
      }),
    )
    await renderApp('/admin/devices')
    const tvs = await screen.findByRole('region', { name: 'TVs' })
    expect(within(tvs).getByText('Living room TV')).toBeInTheDocument()
    expect(within(tvs).getByText('Sam')).toBeInTheDocument()
    const others = screen.getByRole('region', { name: 'Browsers and phones' })
    expect(within(others).getByText('Chrome on MacBook')).toBeInTheDocument()
    expect(screen.getByText('2 devices')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Sign out Living room TV' }))
    await waitFor(() =>
      expect(calls.some((c) => c.method === 'DELETE' && c.path === '/users/u2/sessions/s2')).toBe(
        true,
      ),
    )
    expect(await screen.findByText('Living room TV is signed out.')).toBeInTheDocument()
  })

  it('splits problems into going on and solved', async () => {
    fakeApi(
      routes({
        'GET /problems': {
          status: 200,
          body: {
            problems: [
              {
                id: 'p1',
                code: 'folder_unavailable',
                params: { path: '/mnt/nas/Films' },
                firstSeen: '2026-10-29T20:00:00Z',
                lastSeen: '2026-10-29T21:40:00Z',
                count: 1,
              },
              {
                id: 'p2',
                code: 'hw_fallback',
                params: { accel: 'vaapi' },
                firstSeen: '2026-10-27T20:00:00Z',
                lastSeen: '2026-10-27T20:00:00Z',
                resolvedAt: '2026-10-28T10:00:00Z',
                count: 1,
              },
            ],
          },
        },
      }),
    )
    await renderApp('/admin/problems')
    const open = await screen.findByRole('region', { name: 'Going on' })
    expect(within(open).getByText(/\/mnt\/nas\/Films disappeared at/)).toBeInTheDocument()
    expect(
      within(open).getByText(`findmnt '/mnt/nas/Films' || sudo mount '/mnt/nas/Films'`),
    ).toBeInTheDocument()
    const solved = screen.getByRole('region', { name: 'Solved' })
    expect(within(solved).getByText(/VAAPI failed 3 times at/)).toBeInTheDocument()
    expect(within(solved).queryByText(/Test the GPU again/)).not.toBeInTheDocument()
    expect(screen.getByText('1 going on')).toBeInTheDocument()
  })
})

describe('motion (TM25)', () => {
  it('pushes into a section and pops back; the tabs travel sideways', () => {
    expect(depth('/admin')).toBe(0)
    expect(depth('/admin/settings')).toBe(1)
    expect(depth('/admin/libraries')).toBe(2)
    expect(depth('/admin/libraries/lib1/review')).toBe(4)
    expect(travel('/admin', '/admin/settings')).toEqual({ dir: 'right' })
    expect(travel('/admin/settings', '/admin')).toEqual({ dir: 'left' })
    expect(travel('/admin/settings', '/admin/libraries')).toEqual({ nav: 'push' })
    expect(travel('/admin/libraries/lib1', '/admin/libraries')).toEqual({ nav: 'pop' })
    expect(travel('/admin/people', '/admin/libraries')).toEqual({})
  })

  it('marks the page with the way it travelled', async () => {
    fakeApi(routes())
    await renderApp('/admin/settings')
    await userEvent.click(await screen.findByRole('link', { name: /Libraries/ }))
    await screen.findByRole('link', { name: 'Back to Settings' })
    expect(document.querySelector('.adm__main')).toHaveAttribute('data-nav', 'push')
    await userEvent.click(screen.getByRole('link', { name: 'Back to Settings' }))
    await screen.findByRole('link', { name: /Libraries/ })
    expect(document.querySelector('.adm__main')).toHaveAttribute('data-nav', 'pop')
  })

  it('keeps a stream that ended for a moment, flagged as leaving', () => {
    vi.useFakeTimers()
    try {
      const key = (s: string) => s
      const { result, rerender } = renderHook(({ items }) => useLeaving(items, key, 320), {
        initialProps: { items: ['a', 'b'] },
      })
      rerender({ items: ['b'] })
      expect(result.current).toEqual([
        { item: 'a', leaving: true },
        { item: 'b', leaving: false },
      ])
      act(() => vi.advanceTimersByTime(320))
      expect(result.current).toEqual([{ item: 'b', leaving: false }])
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('helpers', () => {
  it('has a sentence for every reason the server sends', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', resources: { en: { common: en } }, defaultNS: 'common' })
    for (const code of reasonCodes) {
      expect(reasonText(i18n.t, code, {})).not.toBe(en.reason.other)
    }
  })
  it('reads a title, a year or a TMDB link', () => {
    expect(parseQuery('Metropolis 1927')).toEqual({ query: 'Metropolis', year: 1927 })
    expect(parseQuery('https://www.themoviedb.org/movie/19-metropolis')).toMatchObject({
      tmdbId: 19,
    })
  })
  it('makes readable temporary passwords', () => {
    expect(temporaryPassword(() => 0)).toBe('aaaa-aaaa-aaaa-aaaa')
    expect(temporaryPassword()).toMatch(/^[a-z]{4}(-[a-z]{4}){3}$/)
  })
})

describe('remote access', () => {
  const off = {
    enabled: false,
    portMode: 'auto',
    internalPort: 32501,
    state: 'off',
    portMapping: { state: 'off' },
    certificate: { state: 'off' },
    reachability: { state: 'off' },
  }
  const ready = {
    ...off,
    enabled: true,
    state: 'ok',
    externalPort: 32501,
    publicIp: '203.0.113.7',
    hostname: 'living-room-k3x7qa.getplaid.direct',
    url: 'https://living-room-k3x7qa.getplaid.direct:32501',
    shortUrl: 'https://getplaid.direct/living-room-k3x7qa',
    portMapping: { state: 'ok', method: 'upnp' },
    certificate: { state: 'ok', method: 'letsencrypt', at: '2027-01-29T00:00:00Z' },
    reachability: { state: 'ok', at: '2026-10-31T20:00:00Z' },
  }

  it('turns on with one switch and shows the link to share', async () => {
    const calls = fakeApi(
      routes({
        'GET /remote-access': { status: 200, body: off },
        'PATCH /remote-access': { status: 200, body: ready },
      }),
    )
    await renderApp('/admin/remote-access')
    await userEvent.click(await screen.findByRole('switch', { name: 'Remote access' }))
    expect(
      await screen.findByText('https://getplaid.direct/living-room-k3x7qa'),
    ).toBeInTheDocument()
    expect(screen.getByText('Open with UPnP, port 32501')).toBeInTheDocument()
    expect(screen.getAllByText('Reachable from outside').length).toBeGreaterThan(0)
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ enabled: true })
  })

  it('explains a router without UPnP and takes a manual port', async () => {
    const calls = fakeApi(
      routes({
        'GET /remote-access': {
          status: 200,
          body: {
            ...off,
            enabled: true,
            state: 'degraded',
            portMapping: { state: 'failed', error: 'no_gateway' },
            certificate: { state: 'pending' },
            reachability: { state: 'pending' },
          },
        },
        'PATCH /remote-access': (_r: Request, body: unknown) => ({
          status: 200,
          body: { ...ready, ...(body as object), portMapping: { state: 'ok', method: 'manual' } },
        }),
      }),
    )
    await renderApp('/admin/remote-access')
    expect(await screen.findByText(/No router answered UPnP or NAT-PMP/)).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('External port'), '443')
    await userEvent.click(screen.getByRole('button', { name: 'Use this port' }))
    expect(await screen.findByText('Forwarded by hand, port 32501')).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({
      portMode: 'manual',
      manualPort: 443,
    })
  })
})
