import { api } from '../api/client'
import type { components } from '../api/schema'

export type PlaybackSession = components['schemas']['PlaybackSession']
export type PlaybackRequest = components['schemas']['PlaybackRequest']
export type PlaybackUpdate = components['schemas']['PlaybackUpdate']
type State = components['schemas']['PlaybackState']

/** An API error of the player, with the Retry-After it came with. */
export interface PlayerFailure {
  code: string
  retryAfter?: number
}

type Result = { session: PlaybackSession } | { failure: PlayerFailure }

function failure(error: unknown, response: Response): PlayerFailure {
  const code = (error as { error?: { code?: string } })?.error?.code ?? 'generic'
  const after = Number(response.headers.get('Retry-After'))
  return after > 0 ? { code, retryAfter: after } : { code }
}

/** POST /items/{id}/playback (F008-R33). */
export async function startSession(itemId: string, body: PlaybackRequest): Promise<Result> {
  try {
    const { data, error, response } = await api.POST('/items/{itemId}/playback', {
      params: { path: { itemId } },
      body,
    })
    return data ? { session: data } : { failure: failure(error, response) }
  } catch {
    return { failure: { code: 'network' } }
  }
}

/**
 * PATCH /playback/{id} (F008-R37): the server decides again. A new `url`
 * means reload at the position; 503 transcode_limit_reached leaves the
 * session on its current stream.
 */
export async function updateSession(id: string, body: PlaybackUpdate): Promise<Result> {
  try {
    const { data, error, response } = await api.PATCH('/playback/{sessionId}', {
      params: { path: { sessionId: id } },
      body,
    })
    return data ? { session: data } : { failure: failure(error, response) }
  } catch {
    return { failure: { code: 'network' } }
  }
}

export const progressEvery = 10_000

/**
 * Reports a session's progress (F008-R39): every 10 s while playing, at
 * once on pause, seek, and buffering start and end; stops it on leave.
 * A failed report isn't retried, the next one carries the position. A 404
 * means the session ended on the server (expired, or stopped by an admin).
 */
export class Progress {
  private timer?: ReturnType<typeof setInterval>
  private state: State = 'paused'
  private done = false
  private detach?: () => void

  constructor(
    readonly sessionId: string,
    private readonly onGone: () => void,
  ) {}

  /** Follows the video's events until stop. */
  follow(video: HTMLVideoElement) {
    const pos = () => Math.round(video.currentTime * 1000)
    const on: Record<string, () => void> = {
      playing: () => this.set('playing', pos()),
      pause: () => {
        if (!video.ended) this.set('paused', pos())
      },
      waiting: () => this.set('buffering', pos()),
      seeked: () => void this.report(pos()),
    }
    for (const [e, f] of Object.entries(on)) video.addEventListener(e, f)
    this.position = pos
    this.detach = () => {
      for (const [e, f] of Object.entries(on)) video.removeEventListener(e, f)
    }
  }

  private position: () => number = () => 0

  private set(state: State, positionMs: number) {
    if (state === this.state && state !== 'buffering') return
    this.state = state
    clearInterval(this.timer)
    this.timer = undefined
    if (state !== 'paused')
      this.timer = setInterval(() => void this.report(this.position()), progressEvery)
    void this.report(positionMs)
  }

  async report(positionMs: number) {
    if (this.done) return
    try {
      const { error, response } = await api.POST('/playback/{sessionId}/progress', {
        params: { path: { sessionId: this.sessionId } },
        body: { positionMs: Math.max(0, positionMs), state: this.state },
      })
      if (error && response.status === 404) this.gone()
    } catch {
      // Not retried: the next report carries the position.
    }
  }

  private gone() {
    if (this.done) return
    this.halt()
    this.onGone()
  }

  private halt() {
    this.done = true
    clearInterval(this.timer)
    this.detach?.()
  }

  /**
   * DELETE /playback/{id} with the position. `keepalive` lets it leave
   * with the page (pagehide).
   */
  stop(positionMs = this.position(), keepalive = false) {
    if (this.done) return
    this.halt()
    void api
      .DELETE('/playback/{sessionId}', {
        params: { path: { sessionId: this.sessionId } },
        body: { positionMs: Math.max(0, Math.round(positionMs)) },
        keepalive,
      })
      .catch(() => {})
  }

  get stopped() {
    return this.done
  }
}
