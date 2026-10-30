import { describe, expect, it } from 'vitest'
import { shortcutFor } from './shortcuts'
import {
  cycleSubtitle,
  loadAutoplay,
  loadChoice,
  pickTracks,
  rememberAudio,
  rememberAutoplay,
  rememberSubtitles,
  sameLanguage,
} from './trackMemory'
import { endPhase, type EndInput } from './upNext'

const stream = (index: number, type: 'audio' | 'subtitle', language: string, extra = {}) => ({
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

describe('trackMemory', () => {
  const streams = [
    stream(1, 'audio', 'eng', { default: true }),
    stream(2, 'audio', 'fre'),
    stream(3, 'subtitle', 'eng'),
    stream(4, 'subtitle', 'eng', { sdh: true }),
    stream(5, 'subtitle', 'fre', { forced: true }),
    stream(6, 'subtitle', 'fre'),
  ]

  it('remembers per profile and applies the choice where the file has the language', () => {
    rememberAudio('p1', 'fra')
    rememberSubtitles('p1', { language: 'eng', forced: false, sdh: true })
    expect(loadChoice('p2')).toEqual({})
    expect(pickTracks(loadChoice('p1'), streams)).toEqual({
      audioStreamIndex: 2,
      subtitleStreamIndex: 4,
    })
  })

  it('falls back to the closest track in the language, or the server’s default', () => {
    expect(
      pickTracks({ subtitles: { language: 'fre', forced: false, sdh: true } }, streams),
    ).toEqual({ subtitleStreamIndex: 6 })
    expect(
      pickTracks(
        { audio: 'jpn', subtitles: { language: 'kor', forced: false, sdh: false } },
        streams,
      ),
    ).toEqual({})
    expect(pickTracks({ subtitles: 'off' }, streams)).toEqual({ subtitleStreamIndex: null })
  })

  it('treats both ISO 639-2 codes of a language alike', () => {
    expect(sameLanguage('fre', 'fra')).toBe(true)
    expect(sameLanguage('GER', 'deu')).toBe(true)
    expect(sameLanguage('eng', 'fre')).toBe(false)
  })

  it('cycles subtitles off, the remembered language, off', () => {
    const subs = streams.filter((s) => s.type === 'subtitle')
    const fr = { language: 'fre', forced: false, sdh: false }
    expect(cycleSubtitle(null, subs, fr)).toBe(6)
    expect(cycleSubtitle(6, subs, fr)).toBeNull()
    expect(cycleSubtitle(null, subs)).toBe(3)
    expect(cycleSubtitle(null, [])).toBeNull()
  })

  it('plays the next episode automatically unless the profile turned it off', () => {
    expect(loadAutoplay('p1')).toBe(true)
    rememberAutoplay('p1', false)
    expect(loadAutoplay('p1')).toBe(false)
    expect(loadAutoplay('p2')).toBe(true)
  })
})

describe('shortcuts', () => {
  const key = (k: string, mods: Partial<KeyboardEvent> = {}) =>
    shortcutFor({ key: k, shiftKey: false, ctrlKey: false, metaKey: false, altKey: false, ...mods })

  it('maps every R42 key', () => {
    expect(key(' ')).toEqual({ type: 'toggle' })
    expect(key('K')).toEqual({ type: 'toggle' })
    expect(key('ArrowLeft')).toEqual({ type: 'seek', seconds: -10 })
    expect(key('l')).toEqual({ type: 'seek', seconds: 10 })
    expect(key('ArrowUp')).toEqual({ type: 'volume', by: 0.1 })
    expect(key('m')).toEqual({ type: 'mute' })
    expect(key('f')).toEqual({ type: 'fullscreen' })
    expect(key('c')).toEqual({ type: 'subtitles' })
    expect(key('N', { shiftKey: true })).toEqual({ type: 'next' })
    expect(key('n')).toBeNull()
    expect(key('7')).toEqual({ type: 'jump', fraction: 0.7 })
    expect(key('Home')).toEqual({ type: 'start' })
    expect(key('End')).toEqual({ type: 'end' })
    expect(key('Escape')).toEqual({ type: 'escape' })
    expect(key('?', { shiftKey: true })).toEqual({ type: 'help' })
  })

  it('leaves browser shortcuts alone', () => {
    expect(key('f', { metaKey: true })).toBeNull()
    expect(key('l', { ctrlKey: true })).toBeNull()
  })
})

describe('upNext', () => {
  const at = (i: Partial<EndInput>) =>
    endPhase({
      hasNext: true,
      isEpisode: true,
      remainingMs: 600_000,
      ended: false,
      cancelled: false,
      autoplay: true,
      autoplayed: 0,
      ...i,
    })

  it('shows the card in the last 30 s, then counts down at the end', () => {
    expect(at({})).toBe('none')
    expect(at({ remainingMs: 30_000 })).toBe('card')
    expect(at({ ended: true })).toBe('countdown')
  })

  it('asks "Still watching?" after 3 automatic episodes in a row', () => {
    expect(at({ ended: true, autoplayed: 2 })).toBe('countdown')
    expect(at({ ended: true, autoplayed: 3 })).toBe('stillWatching')
  })

  it('keeps the card without a countdown when autoplay is off', () => {
    expect(at({ ended: true, autoplay: false })).toBe('card')
  })

  it('ends on the details after Cancel, and says when the show is over', () => {
    expect(at({ remainingMs: 10_000, cancelled: true })).toBe('none')
    expect(at({ ended: true, cancelled: true })).toBe('leave')
    expect(at({ ended: true, hasNext: false })).toBe('endOfShow')
    expect(at({ ended: true, hasNext: false, isEpisode: false })).toBe('leave')
    expect(at({ ended: true, hasNext: undefined })).toBe('leave')
  })
})
