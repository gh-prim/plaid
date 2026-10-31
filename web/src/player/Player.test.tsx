import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiError, fakeApi, profile, server, user } from '../test/fakeApi'
import { fakeVideo } from '../test/fakeVideo'
import { renderApp } from '../test/renderApp'
import { rememberSubtitles } from './trackMemory'

const ada = profile('p1', 'Ada')

const stream = (index: number, type: string, language: string, extra = {}) => ({
  index,
  type,
  language,
  default: false,
  forced: false,
  sdh: false,
  imageBased: false,
  external: false,
  ...extra,
})

const file = {
  id: 'f1',
  path: '/m/dune.mkv',
  size: 1e9,
  modifiedAt: '2026-01-01T00:00:00Z',
  parseStatus: 'ok',
  probeStatus: 'ok',
  width: 1920,
  height: 1080,
  bitrate: 8e6,
  streams: [
    stream(1, 'audio', 'eng', { default: true, channels: 6 }),
    stream(2, 'audio', 'fre', { channels: 2 }),
    stream(3, 'subtitle', 'eng'),
    stream(4, 'subtitle', 'fre'),
    stream(5, 'subtitle', 'eng', { sdh: true }),
    stream(6, 'subtitle', 'kor', { imageBased: true }),
  ],
}

const movie = (extra = {}) => ({
  id: 'm1',
  kind: 'movie',
  displayTitle: 'Dune: Part Two',
  year: 2024,
  runtimeMinutes: 166,
  matchStatus: 'matched',
  genres: [],
  fileCount: 1,
  childCount: 0,
  technical: { hdrFormats: [], audioLanguages: [] },
  files: [file],
  ...extra,
})

const session = (extra = {}) => ({
  id: 's1',
  itemId: 'm1',
  fileId: 'f1',
  method: 'direct_play',
  reasons: [],
  url: '/api/v1/stream/tok1/file',
  expiresAfterIdleSeconds: 60,
  durationMs: 600_000,
  startPositionMs: 0,
  quality: 'original',
  qualities: ['original', '720p'],
  warnings: [],
  audioTracks: [
    { index: 1, codec: 'eac3', channels: 6, language: 'eng', default: true, selected: true },
    { index: 2, codec: 'aac', channels: 2, language: 'fre', default: false, selected: false },
  ],
  subtitles: [
    {
      index: 3,
      codec: 'subrip',
      language: 'eng',
      forced: false,
      selected: false,
      delivery: 'external',
      url: '/api/v1/stream/tok1/subtitles/3.vtt',
    },
    {
      index: 4,
      codec: 'subrip',
      language: 'fre',
      forced: false,
      selected: false,
      delivery: 'external',
      url: '/api/v1/stream/tok1/subtitles/4.vtt',
    },
    {
      index: 5,
      codec: 'subrip',
      language: 'eng',
      forced: false,
      selected: false,
      delivery: 'external',
      url: '/api/v1/stream/tok1/subtitles/5.vtt',
    },
    {
      index: 6,
      codec: 'hdmv_pgs_subtitle',
      language: 'kor',
      forced: false,
      selected: false,
      delivery: 'burn_in',
    },
  ],
  ...extra,
})

function setup(routes: Parameters<typeof fakeApi>[0] = {}, item: unknown = movie()) {
  return fakeApi({
    'GET /server': server(true),
    'GET /me': {
      status: 200,
      body: { user: user({ role: 'member' }), profiles: [ada], activeProfile: ada },
    },
    'GET /items/m1': { status: 200, body: item },
    'POST /items/m1/playback': { status: 201, body: session() },
    'POST /playback/s1/progress': { status: 204 },
    'DELETE /playback/s1': { status: 204 },
    ...routes,
  })
}

const video = () => document.querySelector('video')!
const started = (calls: ReturnType<typeof fakeApi>) =>
  calls.filter((c) => c.method === 'POST' && c.path === '/items/m1/playback')

describe('player', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('starts a session with the browser’s profile and the remembered tracks, then plays', async () => {
    fakeVideo()
    rememberSubtitles('p1', { language: 'eng', forced: false, sdh: true })
    const calls = setup()
    await renderApp('/watch/m1?t=90')
    expect(await screen.findByText('Dune: Part Two')).toBeInTheDocument()
    expect(screen.getByText('2024 · 2h 46m')).toBeInTheDocument()
    const body = started(calls)[0]!.body as Record<string, unknown>
    expect(body).toMatchObject({ startPositionMs: 90_000, subtitleStreamIndex: 5 })
    expect(body.profile).toMatchObject({ protocols: ['http', 'hls'], subtitles: { text: ['vtt'] } })
    await waitFor(() => expect(video().getAttribute('src')).toBe('/api/v1/stream/tok1/file'))
    expect(video().paused).toBe(false)
    expect(document.querySelectorAll('video track')).toHaveLength(3)
  })

  it('asks before resuming from a plain Play; ?t= doesn’t', async () => {
    fakeVideo()
    const calls = setup(
      {},
      movie({
        userState: { positionMs: 3_733_000, durationMs: 9_960_000, watched: false, playCount: 0 },
      }),
    )
    await renderApp('/watch/m1')
    expect(
      await screen.findByRole('dialog', { name: 'Pick up where you left off?' }),
    ).toBeInTheDocument()
    expect(screen.getByText('You stopped at 1:02:13 in Dune: Part Two.')).toBeInTheDocument()
    expect(started(calls)).toHaveLength(0)
    await userEvent.click(screen.getByRole('button', { name: 'Resume from 1:02:13' }))
    await waitFor(() => expect(started(calls)).toHaveLength(1))
    expect(started(calls)[0]!.body).toMatchObject({ startPositionMs: 3_733_000 })
  })

  it('switches text subtitles in the page and reloads for audio at the same moment', async () => {
    const fv = fakeVideo()
    const calls = setup({
      'PATCH /playback/s1': (_req, body) => ({
        status: 200,
        body: session({
          url: '/api/v1/stream/tok2/master.m3u8',
          method: 'direct_play',
          audioTracks: session().audioTracks.map((a) => ({
            ...a,
            selected: a.index === (body as { audioStreamIndex: number }).audioStreamIndex,
          })),
        }),
      }),
    })
    await renderApp('/watch/m1?t=0')
    await screen.findByText('Dune: Part Two')
    fv.loaded(video(), 600)
    await userEvent.click(await screen.findByRole('button', { name: 'Subtitles: Off' }))
    const menu = screen.getByRole('menu', { name: 'Subtitles' })
    expect(
      within(menu)
        .getAllByRole('menuitemradio')
        .map((r) => r.textContent),
    ).toEqual([
      'Off',
      'English',
      'French',
      'English SDH',
      'KoreanImage subtitles: the video reloads',
    ])
    await userEvent.click(within(menu).getByRole('menuitemradio', { name: 'French' }))
    expect(screen.getByRole('button', { name: 'Subtitles: French' })).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'PATCH')).toBe(false)

    video().currentTime = 120
    await userEvent.click(screen.getByRole('button', { name: 'Audio: English · 5.1' }))
    await userEvent.click(screen.getByRole('menuitemradio', { name: 'French · Stereo' }))
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({
        positionMs: 120_000,
        audioStreamIndex: 2,
      }),
    )
    await waitFor(() => expect(video().getAttribute('src')).toBe('/api/v1/stream/tok2/master.m3u8'))
    expect(screen.getByRole('button', { name: 'Audio: French · Stereo' })).toBeInTheDocument()
  })

  it('keeps the stream and says why when a quality change hits the conversion limit', async () => {
    fakeVideo()
    setup({
      'PATCH /playback/s1': apiError(503, 'transcode_limit_reached', { 'Retry-After': '30' }),
    })
    await renderApp('/watch/m1?t=0')
    await userEvent.click(await screen.findByRole('button', { name: 'Quality: Original' }))
    await userEvent.click(screen.getByRole('menuitemradio', { name: /720p/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('The server is busy converting')
    expect(video().getAttribute('src')).toBe('/api/v1/stream/tok1/file')
  })

  it('shows "Playback stopped." when the server ended the session, and starts again', async () => {
    const fv = fakeVideo()
    let reports = 0
    const calls = setup({
      // The first report goes through; the session is gone by the seek.
      'POST /playback/s1/progress': () =>
        reports++ ? apiError(404, 'playback_session_not_found') : { status: 204 },
    })
    await renderApp('/watch/m1?t=0')
    await screen.findByText('Dune: Part Two')
    fv.loaded(video(), 600)
    video().currentTime = 75
    expect(await screen.findByRole('heading', { name: 'Playback stopped.' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Start again from 1:15' }))
    await waitFor(() => expect(started(calls)).toHaveLength(2))
    expect(started(calls)[1]!.body).toMatchObject({ startPositionMs: 75_000 })
  })

  it('retries a decode error once as a conversion, then offers the VLC link', async () => {
    const fv = fakeVideo()
    const calls = setup({
      'POST /items/m1/playback': (_req, body) => ({
        status: 201,
        body:
          (body as { profile: { preset?: string } }).profile.preset === 'external'
            ? session({ id: 's9', url: '/api/v1/stream/ext/file' })
            : session(),
      }),
    })
    const write = vi.fn(async () => {})
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: write },
      configurable: true,
    })
    await renderApp('/watch/m1?t=0')
    await screen.findByText('Dune: Part Two')
    await waitFor(() => expect(video().getAttribute('src')).toBeTruthy())
    fv.fail(video(), 3)
    await waitFor(() => expect(started(calls)).toHaveLength(2))
    expect(started(calls)[1]!.body).toMatchObject({ forceMethod: 'transcode' })
    await waitFor(() => expect(video().getAttribute('src')).toBeTruthy())
    fv.fail(video(), 4)
    expect(
      await screen.findByRole('heading', { name: 'This video can’t play in this browser.' }),
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Copy link for VLC' }))
    await waitFor(() =>
      expect(write).toHaveBeenCalledWith(`${window.location.origin}/api/v1/stream/ext/file`),
    )
    expect(screen.getByRole('button', { name: 'Link copied' })).toBeInTheDocument()
  })

  it('says what to do when the server is busy, and counts down Try again', async () => {
    fakeVideo()
    setup({
      'POST /items/m1/playback': apiError(503, 'transcode_limit_reached', { 'Retry-After': '30' }),
    })
    await renderApp('/watch/m1?t=0')
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The server is busy converting other videos.',
    )
    expect(screen.getByRole('button', { name: 'Try again in 30 s' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
    expect(screen.getByRole('button', { name: 'Lower quality' })).toBeInTheDocument()
  })

  it('answers the keyboard: Space, M, arrows, digits and ?', async () => {
    const fv = fakeVideo()
    setup()
    await renderApp('/watch/m1?t=0')
    await screen.findByText('Dune: Part Two')
    fv.loaded(video(), 600)
    await waitFor(() => expect(video().paused).toBe(false))
    await userEvent.keyboard(' ')
    expect(video().paused).toBe(true)
    await userEvent.keyboard('m')
    expect(video().muted).toBe(true)
    await userEvent.keyboard('5')
    expect(video().currentTime).toBe(300)
    await userEvent.keyboard('{ArrowRight}')
    expect(video().currentTime).toBe(310)
    await userEvent.keyboard('c')
    expect(screen.getByRole('button', { name: 'Subtitles: English' })).toBeInTheDocument()
    await userEvent.keyboard('?')
    expect(screen.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeInTheDocument()
  })
})

describe('next episode', () => {
  const episode = (id: string, n: number, extra = {}) =>
    movie({
      id,
      kind: 'episode',
      displayTitle: `Episode ${n}`,
      seasonNumber: 1,
      episodeNumber: n,
      show: { id: 'show1', displayTitle: 'Dark' },
      ...extra,
    })

  function setupShow(next: unknown, state?: unknown) {
    const calls = setup({
      'GET /items/m1': { status: 200, body: episode('m1', 1) },
      'GET /items/m1/up-next': next ? { status: 200, body: { item: next } } : { status: 204 },
    })
    return { calls, path: { pathname: '/watch/m1', search: '?t=0', state } }
  }

  it('shows the card in the last 30 s, then counts 10 s down and plays it', async () => {
    const fv = fakeVideo()
    const { path } = setupShow(episode('m2', 2, { runtimeMinutes: 52 }))
    await renderApp(path)
    await screen.findByText(/^Dark · S\d+ E\d+$/)
    fv.loaded(video(), 600)
    fv.advance(video(), 575)
    const card = await screen.findByRole('region', { name: 'Next episode' })
    expect(card).toHaveTextContent('S1 E2 · Episode 2')
    expect(card).toHaveTextContent('Dark · 52m')
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'Date'] })
    act(() => fv.end(video()))
    expect(await screen.findByText('Playing in 10 seconds')).toBeInTheDocument()
    act(() => vi.advanceTimersByTime(10_500))
    vi.useRealTimers()
    await waitFor(() => expect(document.querySelector('video')).toBeTruthy())
    // The next episode's player asks for its own session.
    await waitFor(() =>
      expect(screen.queryByRole('region', { name: 'Next episode' })).not.toBeInTheDocument(),
    )
  })

  it('asks "Still watching?" after 3 automatic episodes in a row', async () => {
    const fv = fakeVideo()
    const { path } = setupShow(episode('m2', 2), { autoplayed: 3 })
    await renderApp(path)
    await screen.findByText(/^Dark · S\d+ E\d+$/)
    fv.loaded(video(), 600)
    await screen.findByRole('button', { name: /Subtitles/ })
    act(() => fv.end(video()))
    expect(await screen.findByRole('dialog', { name: 'Still watching?' })).toBeInTheDocument()
    expect(screen.getByText('Next episode · S1 E2 · Episode 2')).toBeInTheDocument()
  })

  it('ends the show with one warm line', async () => {
    const fv = fakeVideo()
    const { path } = setupShow(null)
    await renderApp(path)
    await screen.findByText(/^Dark · S\d+ E\d+$/)
    fv.loaded(video(), 600)
    await screen.findByRole('button', { name: /Subtitles/ })
    act(() => fv.end(video()))
    expect(
      await screen.findByRole('heading', { name: 'That’s the last episode of Dark for now.' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to Dark' })).toBeInTheDocument()
  })
})
