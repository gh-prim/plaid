import { describe, expect, it } from 'vitest'
import { buildProfile, capabilityProfile, nativeHls, type Probe } from './capabilities'

// What each browser answers, roughly: MSE supports a type when one of the
// patterns is a prefix of it; canPlayType answers for bare containers.
function browser(mse: string[], canPlay: string[], hdrScreen = false): Probe {
  const match = (list: string[], t: string) => list.some((p) => t.startsWith(p))
  return {
    mse: (t) => match(mse, t),
    canPlay: (t) => (canPlay.includes(t) ? 'maybe' : ''),
    hdrScreen,
  }
}

const chrome = browser(
  [
    'video/mp4; codecs="avc1.42',
    'video/mp4; codecs="avc1.4D',
    'video/mp4; codecs="avc1.6400',
    'video/webm; codecs="vp09.00',
    'video/webm; codecs="vp09.02',
    'video/mp4; codecs="av01.0.08M.08"',
    'video/mp4; codecs="av01.0.08M.10"',
    'audio/mp4; codecs="mp4a.40.2"',
    'audio/mpeg',
    'audio/webm; codecs="opus"',
    'audio/mp4; codecs="opus"',
    'audio/mp4; codecs="flac"',
  ],
  ['video/mp4', 'video/webm', 'audio/mpeg'],
)

const firefox = browser(
  [
    'video/mp4; codecs="avc1.42',
    'video/mp4; codecs="avc1.4D',
    'video/mp4; codecs="avc1.640028"',
    'video/mp4; codecs="avc1.640033"',
    'video/webm; codecs="vp09.00',
    'video/mp4; codecs="av01.0.08M.08"',
    'audio/mp4; codecs="mp4a.40.2"',
    'audio/webm; codecs="opus"',
    'audio/webm; codecs="vorbis"',
  ],
  ['video/mp4', 'video/webm'],
)

const safari = browser(
  [
    'video/mp4; codecs="avc1.',
    'video/mp4; codecs="hvc1.1.6.L1',
    'video/mp4; codecs="hvc1.2.4.L1',
    'audio/mp4; codecs="mp4a.40.2"',
    'audio/mp4; codecs="ac-3"',
    'audio/mp4; codecs="ec-3"',
    'audio/mp4; codecs="flac"',
  ],
  ['video/mp4', 'video/quicktime', 'application/vnd.apple.mpegurl'],
  true,
)

const codecs = (p: ReturnType<typeof buildProfile>) => ({
  video: p.video?.map((v) => v.codec),
  audio: p.audio?.map((a) => a.codec),
})

describe('capabilities', () => {
  it('Chrome: H.264, VP9, AV1, AAC and Opus, no MKV, no HEVC', () => {
    const p = buildProfile(chrome)
    expect(codecs(p)).toEqual({
      video: ['h264', 'vp9', 'av1'],
      audio: ['aac', 'mp3', 'opus', 'flac'],
    })
    expect(p.containers).toEqual(['mp4', 'webm'])
    expect(p.video?.[0]).toMatchObject({ maxLevel: 52, maxBitDepth: 8 })
    expect(p.subtitles).toEqual({ text: ['vtt'], separate: ['pgs'] })
    expect(p.protocols).toEqual(['http', 'hls'])
    expect(p.canSwitchAudioTrack).toBe(false)
    expect(p.video?.some((v) => v.hdr)).toBe(false)
    expect(nativeHls(chrome)).toBe(false)
  })

  it('Firefox: no HEVC, H.264 up to level 5.1', () => {
    const p = buildProfile(firefox)
    expect(codecs(p).video).toEqual(['h264', 'vp9', 'av1'])
    expect(p.video?.[0]?.maxLevel).toBe(51)
    expect(p.video?.find((v) => v.codec === 'vp9')?.profiles).toEqual(['profile 0'])
  })

  it('Safari: HEVC Main 10 with HDR on an HDR screen, E-AC-3, native HLS', () => {
    const p = buildProfile(safari)
    const hevc = p.video?.find((v) => v.codec === 'hevc')
    expect(hevc).toMatchObject({ profiles: ['main', 'main 10'], maxBitDepth: 10, maxLevel: 156 })
    expect(hevc?.hdr).toEqual(['hdr10', 'hlg'])
    expect(codecs(p).audio).toEqual(['aac', 'flac', 'ac3', 'eac3'])
    expect(p.containers).toEqual(['mp4', 'mov'])
    expect(nativeHls(safari)).toBe(true)
  })

  it('shows HDR only on an HDR screen', () => {
    const sdr = buildProfile({ ...safari, hdrScreen: false })
    expect(sdr.video?.find((v) => v.codec === 'hevc')?.hdr).toBeUndefined()
  })

  it('is built once per tab session', () => {
    let asked = 0
    const counting: Probe = { ...chrome, mse: (t) => (asked++, chrome.mse(t)) }
    const first = capabilityProfile(counting)
    const n = asked
    expect(capabilityProfile(counting)).toEqual(first)
    expect(asked).toBe(n)
    expect(JSON.parse(sessionStorage.getItem('plaid.capabilities')!)).toEqual(first)
  })
})
