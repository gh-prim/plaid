import type { components } from '../api/schema'

type Method = components['schemas']['PlaybackMethod']
type HlsModule = typeof import('hls.js')

/** How a fatal error reached the engine. */
export type EngineFailure =
  | { kind: 'media'; code: number } // MediaError code: 3 decode, 4 not supported
  | { kind: 'network'; status?: number }

export interface EngineOptions {
  video: HTMLVideoElement
  /** PlaybackSession.url, relative to the server. */
  url: string
  method: Method
  startMs: number
  /** Safari: play HLS without hls.js. */
  nativeHls: boolean
  onFatal(failure: EngineFailure): void
  /** Injected in tests; the real one is a lazy chunk. */
  loadHls?: () => Promise<HlsModule>
}

export interface Engine {
  kind: 'file' | 'native-hls' | 'hls.js'
  destroy(): void
}

// A segment still being converted answers 503 segment_not_ready with
// Retry-After: 1 (F006). hls.js doesn't give the headers to the retry
// policy, so it waits the server's second and tries again, for a minute.
const segmentRetry = {
  maxNumRetry: 60,
  retryDelayMs: 1000,
  maxRetryDelayMs: 1000,
  backoff: 'linear' as const,
  shouldRetry: (
    _config: unknown,
    count: number,
    _timeout: boolean,
    response: { code?: number } | undefined,
    retry: boolean,
  ) => (response?.code === 503 ? count < 60 : retry),
}

/**
 * Attaches a session's stream to the <video> (F008-R35): the file for
 * Direct Play, else HLS natively (Safari) or through hls.js. The one place
 * that sets and clears the video's source.
 */
export async function attach(o: EngineOptions): Promise<Engine> {
  const { video } = o
  const onError = () => {
    const code = video.error?.code ?? 0
    if (code === 3 || code === 4) o.onFatal({ kind: 'media', code })
    else o.onFatal({ kind: 'network' })
  }
  const clear = () => {
    video.removeEventListener('error', onError)
    video.removeAttribute('src')
    video.load()
  }

  if (o.method === 'direct_play' || o.nativeHls) {
    video.addEventListener('error', onError)
    seekOnLoad(video, o.startMs)
    video.src = o.url
    return { kind: o.method === 'direct_play' ? 'file' : 'native-hls', destroy: clear }
  }

  const { default: Hls } = await (o.loadHls ?? (() => import('hls.js')))()
  const hls = new Hls({
    startPosition: o.startMs / 1000,
    // Subtitles come as <track> elements, for every engine alike.
    enableWebVTT: false,
    enableCEA708Captions: false,
    fragLoadPolicy: {
      default: {
        maxTimeToFirstByteMs: 30_000,
        maxLoadTimeMs: 60_000,
        timeoutRetry: { maxNumRetry: 2, retryDelayMs: 0, maxRetryDelayMs: 0 },
        errorRetry: segmentRetry,
      },
    },
  })
  let recovered = false
  hls.on(Hls.Events.ERROR, (_event, data) => {
    if (!data.fatal) return
    if (data.type === Hls.ErrorTypes.MEDIA_ERROR && !recovered) {
      recovered = true
      hls.recoverMediaError()
      return
    }
    if (data.type === Hls.ErrorTypes.MEDIA_ERROR) o.onFatal({ kind: 'media', code: 3 })
    else o.onFatal({ kind: 'network', status: data.response?.code })
  })
  hls.loadSource(o.url)
  hls.attachMedia(video)
  return {
    kind: 'hls.js',
    destroy() {
      hls.destroy()
      clear()
    },
  }
}

// The file and native HLS start at 0: seek once the metadata is there.
function seekOnLoad(video: HTMLVideoElement, ms: number) {
  if (ms <= 0) return
  video.addEventListener(
    'loadedmetadata',
    () => {
      video.currentTime = ms / 1000
    },
    { once: true },
  )
}

/**
 * Shows the subtitle track with this stream index, hides every other one,
 * HLS renditions that Safari adds on its own included. `null` turns them off.
 */
export function showSubtitle(video: HTMLVideoElement, index: number | null) {
  const tracks = video.textTracks as TextTrackList | undefined
  if (!tracks) return
  for (let i = 0; i < tracks.length; i++) {
    const track = tracks[i]!
    const ours = track.id.startsWith('sub-')
    track.mode = ours && index !== null && track.id === `sub-${index}` ? 'showing' : 'disabled'
  }
}
