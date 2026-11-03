// Plaid Party sync rules (F010-R8–R10), the same as the server's and the
// Apple app's (api/events.md): the room holds the reference playback and
// every screen corrects its own drift.

export const driftIgnoreMs = 150
export const driftSettledMs = 50
export const driftSeekMs = 3000
export const rateFast = 1.05
export const rateSlow = 0.95

export interface RoomPlayback {
  state: 'playing' | 'paused' | 'waiting'
  positionMs: number
  atServerMs: number
  rate: number
}

/** Where the room is at server time `serverNow`. */
export function targetAt(pb: RoomPlayback, serverNow: number): number {
  if (pb.state !== 'playing') return pb.positionMs
  return pb.positionMs + (serverNow - pb.atServerMs) * pb.rate
}

export interface Correction {
  rate: number
  seekMs?: number
}

/**
 * What a screen does about its drift: nothing under 150 ms, 1.05 or 0.95
 * until within 50 ms, a seek beyond 3 s. `canRate` false (a player that
 * ignores small rate changes) seeks past 1 s instead.
 */
export function correct(
  localMs: number,
  targetMs: number,
  rate: number,
  canRate = true,
): Correction {
  const drift = targetMs - localMs
  const abs = Math.abs(drift)
  if (abs > driftSeekMs || (!canRate && abs > 1000)) return { rate: 1, seekMs: targetMs }
  if (!canRate) return { rate: 1 }
  if (rate !== 1 && abs <= driftSettledMs) return { rate: 1 }
  if (rate === 1 && abs < driftIgnoreMs) return { rate: 1 }
  return { rate: drift > 0 ? rateFast : rateSlow }
}

export interface ClockSample {
  sentMs: number
  serverMs: number
  receivedMs: number
}

/** The server's clock from sync.ping round trips (R9): the offset of the
 * shortest trip among the last 8. */
export class Clock {
  private samples: ClockSample[] = []

  add(s: ClockSample) {
    this.samples.push(s)
    if (this.samples.length > 8) this.samples.shift()
  }

  get offset(): number {
    if (this.samples.length === 0) return 0
    const best = this.samples.reduce((a, b) =>
      b.receivedMs - b.sentMs < a.receivedMs - a.sentMs ? b : a,
    )
    return best.serverMs - (best.sentMs + best.receivedMs) / 2
  }

  /** Server time now. */
  now(local = Date.now()): number {
    return local + this.offset
  }
}

/** The screen states a player reports (party.report). */
export type ScreenState =
  'loading' | 'ready' | 'playing' | 'paused' | 'buffering' | 'ended' | 'blocked' | 'credits'

export interface SyncHooks {
  /** Server time now. */
  serverNow(): number
  report(state: ScreenState, positionMs: number): void
  /** A drift larger than 1 s was corrected (Party15 toast). */
  onCaughtUp?(behindMs: number): void
}

/**
 * Drives a <video> from the room: applies play, pause and seeks, corrects
 * drift every 250 ms and reports the screen's state. The video's own
 * controls never act alone in a party: the player sends party.control and
 * waits for the room.
 */
export class VideoSync {
  private pb: RoomPlayback | null = null
  private timer: ReturnType<typeof setInterval> | undefined
  private lastState: ScreenState = 'loading'
  private canRate = true
  private applying = false
  private hadPicture = false
  /** Until then, a short wait is our own catch-up seek, not buffering. */
  private ownSeekUntil = 0
  private listeners = new AbortController()

  constructor(
    private video: HTMLVideoElement,
    private hooks: SyncHooks,
  ) {
    const on = (e: string, fn: () => void) =>
      video.addEventListener(e, fn, { signal: this.listeners.signal })
    // The state is worked out from the video on every tick; events only
    // make the report come sooner. (`stalled` isn't buffering: it fires when
    // the network pauses, often with seconds of video ahead.)
    for (const e of ['waiting', 'canplay', 'playing', 'seeked', 'ended']) on(e, () => this.report())
    on('ratechange', () => {
      // Safari's native HLS may refuse a rate change: seek instead.
      if (this.applying && Math.abs(video.playbackRate - this.wanted) > 0.001) this.canRate = false
    })
    this.timer = setInterval(() => this.tick(), 250)
  }

  private wanted = 1

  private pos(): number {
    return this.video.currentTime * 1000
  }

  /**
   * What the screen shows (R11). Without enough data to play it's buffering,
   * unless it never had a picture (loading) or the wait is our own catch-up
   * seek. With data: playing, or ready while the room is paused or waits,
   * so a screen the room waited for always says when it can go again.
   */
  private screenState(): ScreenState {
    const v = this.video
    if (v.ended) return 'ended'
    if (v.readyState < HTMLMediaElement.HAVE_FUTURE_DATA || v.seeking) {
      if (!this.hadPicture) return 'loading'
      if (Date.now() < this.ownSeekUntil && this.lastState !== 'buffering') return this.lastState
      return 'buffering'
    }
    this.hadPicture = true
    return this.pb?.state === 'playing' && !v.paused ? 'playing' : 'ready'
  }

  private report() {
    if (!this.pb) return
    const s = this.screenState()
    if (s === this.lastState) return
    this.lastState = s
    this.hooks.report(s, Math.round(this.pos()))
  }

  /** A new reference state from the room. */
  apply(pb: RoomPlayback) {
    this.pb = pb
    this.tick()
  }

  /** The current reference state. */
  playback(): RoomPlayback | null {
    return this.pb
  }

  private tick() {
    const pb = this.pb
    const v = this.video
    if (!pb || v.readyState === 0) return
    const target = targetAt(pb, this.hooks.serverNow())
    if (pb.state !== 'playing') {
      if (!v.paused) v.pause()
      this.setRate(1)
      if (!v.seeking && Math.abs(this.pos() - target) > driftIgnoreMs) v.currentTime = target / 1000
      this.report()
      return
    }
    if (v.paused) void v.play().catch(() => {})
    const c = correct(this.pos(), target, v.playbackRate, this.canRate)
    if (c.seekMs !== undefined && !v.seeking) {
      const behind = target - this.pos()
      this.ownSeekUntil = Date.now() + 3000
      v.currentTime = c.seekMs / 1000
      if (Math.abs(behind) > 1000) this.hooks.onCaughtUp?.(behind)
    } else if (c.rate !== 1 && v.playbackRate === 1) {
      const behind = target - this.pos()
      if (Math.abs(behind) > 1000) this.hooks.onCaughtUp?.(behind)
    }
    this.setRate(c.rate)
    this.report()
  }

  private setRate(r: number) {
    if (this.video.playbackRate === r) return
    this.wanted = r
    this.applying = true
    this.video.playbackRate = r
    this.applying = false
  }

  destroy() {
    clearInterval(this.timer)
    this.listeners.abort()
    this.video.playbackRate = 1
  }
}
