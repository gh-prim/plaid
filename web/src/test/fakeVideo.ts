import { vi } from 'vitest'

interface State {
  time: number
  duration: number
  paused: boolean
  ended: boolean
  volume: number
  muted: boolean
  readyState: number
  buffered: number
  error: { code: number } | null
}

/**
 * A minimal HTMLMediaElement for jsdom, which can't play anything: play,
 * pause, currentTime, volume and their events, plus helpers that load,
 * advance, end or fail a video. Restored by vi.restoreAllMocks.
 */
export function fakeVideo() {
  const states = new WeakMap<HTMLMediaElement, State>()
  const state = (el: HTMLMediaElement) => {
    let s = states.get(el)
    if (!s) {
      s = {
        time: 0,
        duration: NaN,
        paused: true,
        ended: false,
        volume: 1,
        muted: false,
        readyState: 0,
        buffered: 0,
        error: null,
      }
      states.set(el, s)
    }
    return s
  }
  const fire = (el: HTMLMediaElement, ...names: string[]) =>
    names.forEach((n) => el.dispatchEvent(new Event(n)))
  const proto = HTMLMediaElement.prototype

  vi.spyOn(proto, 'play').mockImplementation(function (this: HTMLMediaElement) {
    const s = state(this)
    if (s.paused) {
      s.paused = false
      s.ended = false
      fire(this, 'play', 'playing')
    }
    return Promise.resolve()
  })
  vi.spyOn(proto, 'pause').mockImplementation(function (this: HTMLMediaElement) {
    const s = state(this)
    if (!s.paused) {
      s.paused = true
      fire(this, 'pause')
    }
  })
  vi.spyOn(proto, 'load').mockImplementation(function (this: HTMLMediaElement) {
    const s = state(this)
    Object.assign(s, { time: 0, paused: true, readyState: 0, error: null })
    fire(this, 'emptied')
  })
  const prop = <K extends keyof State>(name: string, key: K, event?: string) => {
    // jsdom leaves a few out (error): give them an accessor to spy on.
    if (!Object.getOwnPropertyDescriptor(proto, name))
      Object.defineProperty(proto, name, { configurable: true, get() {}, set() {} })
    vi.spyOn(proto, name as 'currentTime', 'get').mockImplementation(function (
      this: HTMLMediaElement,
    ) {
      return state(this)[key] as never
    })
    if (event)
      vi.spyOn(proto, name as 'currentTime', 'set').mockImplementation(function (
        this: HTMLMediaElement,
        v: unknown,
      ) {
        state(this)[key] = v as State[K]
        if (name === 'currentTime') fire(this, 'seeking', 'seeked', 'timeupdate')
        else fire(this, event)
      })
  }
  prop('currentTime', 'time', 'seeked')
  prop('duration', 'duration')
  prop('paused', 'paused')
  prop('ended', 'ended')
  prop('volume', 'volume', 'volumechange')
  prop('muted', 'muted', 'volumechange')
  prop('readyState', 'readyState')
  prop('error', 'error')
  vi.spyOn(proto, 'buffered', 'get').mockImplementation(function (this: HTMLMediaElement) {
    const end = state(this).buffered
    return {
      length: end > 0 ? 1 : 0,
      start: () => 0,
      end: () => end,
    } as unknown as TimeRanges
  })

  return {
    state,
    /** The metadata arrives: duration in seconds. */
    loaded(el: HTMLMediaElement, duration: number) {
      Object.assign(state(el), { duration, readyState: 4 })
      fire(el, 'loadedmetadata', 'durationchange', 'canplay')
    },
    /** Plays on by `seconds`. */
    advance(el: HTMLMediaElement, seconds: number) {
      const s = state(el)
      s.time = Math.min(s.time + seconds, Number.isNaN(s.duration) ? Infinity : s.duration)
      s.buffered = Math.max(s.buffered, s.time + 30)
      fire(el, 'timeupdate')
    },
    waiting(el: HTMLMediaElement) {
      fire(el, 'waiting')
    },
    end(el: HTMLMediaElement) {
      const s = state(el)
      Object.assign(s, { time: s.duration, paused: true, ended: true })
      fire(el, 'timeupdate', 'pause', 'ended')
    },
    fail(el: HTMLMediaElement, code: number) {
      state(el).error = { code }
      fire(el, 'error')
    },
  }
}
