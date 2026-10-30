import { describe, expect, it, vi } from 'vitest'
import { fakeVideo } from '../test/fakeVideo'
import { attach, showSubtitle, type EngineOptions } from './engine'

// A stand-in for hls.js: records its config and lets the test fire errors.
function fakeHls() {
  const made: FakeHls[] = []
  class FakeHls {
    static Events = { ERROR: 'hlsError' }
    static ErrorTypes = { MEDIA_ERROR: 'mediaError', NETWORK_ERROR: 'networkError' }
    handlers: ((e: string, d: unknown) => void)[] = []
    source = ''
    media?: HTMLVideoElement
    recover = vi.fn()
    destroyed = false
    constructor(public config: Record<string, unknown>) {
      made.push(this)
    }
    on(_e: string, h: (e: string, d: unknown) => void) {
      this.handlers.push(h)
    }
    loadSource(url: string) {
      this.source = url
    }
    attachMedia(v: HTMLVideoElement) {
      this.media = v
    }
    recoverMediaError() {
      this.recover()
    }
    destroy() {
      this.destroyed = true
    }
    emit(data: unknown) {
      this.handlers.forEach((h) => h('hlsError', data))
    }
  }
  const loadHls = (async () => ({ default: FakeHls })) as unknown as EngineOptions['loadHls']
  return { made, loadHls }
}

const base = (video: HTMLVideoElement, extra: Partial<EngineOptions> = {}): EngineOptions => ({
  video,
  url: '/api/v1/stream/tok/file',
  method: 'direct_play',
  startMs: 0,
  nativeHls: false,
  onFatal: vi.fn(),
  ...extra,
})

describe('engine', () => {
  it('plays a Direct Play file in <video> and seeks to the start position', async () => {
    const fv = fakeVideo()
    const video = document.createElement('video')
    const engine = await attach(base(video, { startMs: 3_733_000 }))
    expect(engine.kind).toBe('file')
    expect(video.getAttribute('src')).toBe('/api/v1/stream/tok/file')
    fv.loaded(video, 7200)
    expect(video.currentTime).toBe(3733)
    engine.destroy()
    expect(video.hasAttribute('src')).toBe(false)
  })

  it('plays HLS natively on Safari, without loading hls.js', async () => {
    fakeVideo()
    const { made, loadHls } = fakeHls()
    const video = document.createElement('video')
    const engine = await attach(
      base(video, {
        url: '/api/v1/stream/tok/master.m3u8',
        method: 'transcode',
        nativeHls: true,
        loadHls,
      }),
    )
    expect(engine.kind).toBe('native-hls')
    expect(video.getAttribute('src')).toBe('/api/v1/stream/tok/master.m3u8')
    expect(made).toHaveLength(0)
  })

  it('uses hls.js elsewhere, starting at the position, and retries 503 segments', async () => {
    fakeVideo()
    const { made, loadHls } = fakeHls()
    const video = document.createElement('video')
    const engine = await attach(
      base(video, {
        url: '/api/v1/stream/tok/master.m3u8',
        method: 'direct_stream',
        startMs: 90_000,
        loadHls,
      }),
    )
    expect(engine.kind).toBe('hls.js')
    const hls = made[0]!
    expect(hls.source).toBe('/api/v1/stream/tok/master.m3u8')
    expect(hls.media).toBe(video)
    expect(hls.config).toMatchObject({ startPosition: 90, enableWebVTT: false })
    type Retry = { shouldRetry: (...a: unknown[]) => boolean; retryDelayMs: number }
    const retry = (hls.config.fragLoadPolicy as { default: { errorRetry: Retry } }).default
      .errorRetry
    expect(retry.retryDelayMs).toBe(1000)
    expect(retry.shouldRetry(null, 5, false, { code: 503 }, false)).toBe(true)
    expect(retry.shouldRetry(null, 60, false, { code: 503 }, false)).toBe(false)
    expect(retry.shouldRetry(null, 0, false, { code: 404 }, false)).toBe(false)
    engine.destroy()
    expect(hls.destroyed).toBe(true)
  })

  it('recovers one hls.js media error, then reports it', async () => {
    fakeVideo()
    const { made, loadHls } = fakeHls()
    const onFatal = vi.fn()
    const video = document.createElement('video')
    await attach(base(video, { method: 'transcode', loadHls, onFatal }))
    const hls = made[0]!
    hls.emit({ fatal: false, type: 'networkError' })
    hls.emit({ fatal: true, type: 'mediaError' })
    expect(hls.recover).toHaveBeenCalledOnce()
    expect(onFatal).not.toHaveBeenCalled()
    hls.emit({ fatal: true, type: 'mediaError' })
    expect(onFatal).toHaveBeenCalledWith({ kind: 'media', code: 3 })
    hls.emit({ fatal: true, type: 'networkError', response: { code: 404 } })
    expect(onFatal).toHaveBeenLastCalledWith({ kind: 'network', status: 404 })
  })

  it('reports a decode error of the file', async () => {
    const fv = fakeVideo()
    const onFatal = vi.fn()
    const video = document.createElement('video')
    await attach(base(video, { onFatal }))
    fv.fail(video, 4)
    expect(onFatal).toHaveBeenCalledWith({ kind: 'media', code: 4 })
  })

  it('shows one subtitle track and hides the others', () => {
    const tracks = [
      { id: 'sub-2', mode: 'disabled' },
      { id: 'sub-5', mode: 'showing' },
      { id: '', mode: 'showing' }, // an HLS rendition Safari added
    ]
    const video = {
      textTracks: Object.assign(tracks, { length: 3 }),
    } as unknown as HTMLVideoElement
    showSubtitle(video, 2)
    expect(tracks.map((t) => t.mode)).toEqual(['showing', 'disabled', 'disabled'])
    showSubtitle(video, null)
    expect(tracks.map((t) => t.mode)).toEqual(['disabled', 'disabled', 'disabled'])
  })
})
