import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiError, fakeApi } from '../test/fakeApi'
import { fakeVideo } from '../test/fakeVideo'
import { Progress, startSession, updateSession } from './session'

const flush = () => vi.advanceTimersByTimeAsync(0)

describe('session', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('starts a session and reads errors with their Retry-After', async () => {
    fakeApi({
      'POST /items/m1/playback': apiError(503, 'transcode_limit_reached', { 'Retry-After': '30' }),
      'PATCH /playback/s1': { status: 200, body: { id: 's1', url: '/new' } },
    })
    expect(
      await startSession('m1', {
        profile: { canSwitchAudioTrack: false },
        startPositionMs: 0,
        burnSubtitles: false,
      }),
    ).toEqual({
      failure: { code: 'transcode_limit_reached', retryAfter: 30 },
    })
    expect(await updateSession('s1', { positionMs: 5 })).toMatchObject({
      session: { id: 's1', url: '/new' },
    })
  })

  it('reports every 10 s while playing and at once on pause, seek and buffering', async () => {
    const calls = fakeApi({
      'POST /playback/s1/progress': { status: 204 },
      'DELETE /playback/s1': { status: 204 },
    })
    const fv = fakeVideo()
    const video = document.createElement('video')
    const progress = new Progress('s1', vi.fn())
    progress.follow(video)
    fv.loaded(video, 600)
    await video.play()
    await flush()
    fv.advance(video, 10)
    await vi.advanceTimersByTimeAsync(10_000)
    video.pause()
    await flush()
    await vi.advanceTimersByTimeAsync(30_000) // paused: nothing more
    video.currentTime = 42
    await flush()
    await video.play()
    fv.waiting(video)
    await flush()
    progress.stop(43_500)
    await flush()
    await vi.advanceTimersByTimeAsync(30_000)

    const sent = calls.map((c) => [c.method, c.body])
    expect(sent).toEqual([
      ['POST', { positionMs: 0, state: 'playing' }],
      ['POST', { positionMs: 10_000, state: 'playing' }],
      ['POST', { positionMs: 10_000, state: 'paused' }],
      ['POST', { positionMs: 42_000, state: 'paused' }],
      ['POST', { positionMs: 42_000, state: 'playing' }],
      ['POST', { positionMs: 42_000, state: 'buffering' }],
      ['DELETE', { positionMs: 43_500 }],
    ])
  })

  it('stops with keepalive when the page goes away', async () => {
    const calls = fakeApi({ 'DELETE /playback/s1': { status: 204 } })
    const progress = new Progress('s1', vi.fn())
    progress.stop(1000, true)
    progress.stop(2000) // once only
    await flush()
    expect(calls).toHaveLength(1)
    const req = vi.mocked(fetch).mock.calls[0]![0] as Request
    expect(req.keepalive).toBe(true)
  })

  it('goes to the stopped state on a 404 and sends nothing more', async () => {
    const calls = fakeApi({
      'POST /playback/s1/progress': apiError(404, 'playback_session_not_found'),
    })
    const fv = fakeVideo()
    const video = document.createElement('video')
    const gone = vi.fn()
    const progress = new Progress('s1', gone)
    progress.follow(video)
    fv.loaded(video, 600)
    await video.play()
    await flush()
    expect(gone).toHaveBeenCalledOnce()
    await vi.advanceTimersByTimeAsync(30_000)
    progress.stop()
    await flush()
    expect(calls).toHaveLength(1)
    expect(progress.stopped).toBe(true)
  })

  it('doesn’t retry a failed report', async () => {
    const calls = fakeApi({ 'POST /playback/s1/progress': { status: 500 } })
    const progress = new Progress('s1', vi.fn())
    await progress.report(1000)
    await vi.advanceTimersByTimeAsync(30_000)
    expect(calls).toHaveLength(1)
  })
})
