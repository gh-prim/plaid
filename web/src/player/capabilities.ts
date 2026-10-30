import type { components } from '../api/schema'

export type CapabilityProfile = components['schemas']['CapabilityProfile']
type Video = components['schemas']['VideoCapability']
type Audio = components['schemas']['AudioCapability']

/** What the browser answers about media types; stubbed in tests. */
export interface Probe {
  /** MediaSource (or ManagedMediaSource) isTypeSupported. */
  mse(type: string): boolean
  /** HTMLVideoElement.canPlayType. */
  canPlay(type: string): '' | 'maybe' | 'probably'
  /** matchMedia('(dynamic-range: high)'). */
  hdrScreen: boolean
}

const mp4 = (codec: string) => `video/mp4; codecs="${codec}"`
const webm = (codec: string) => `video/webm; codecs="${codec}"`

// Levels are tried highest first; the codec strings carry the level the way
// ffprobe reports it (H.264 0x33 = 51 = 5.1, HEVC L153 = 5.1).
const h264Levels = [52, 51, 50, 42, 41, 40, 31, 30]
const hevcLevels = [156, 153, 150, 123, 120, 93]

/**
 * The capability profile of this browser (F008-R34), from what it says it
 * plays. Direct Play goes through <video src> and HLS through MSE, so a
 * codec counts when either answers yes.
 */
export function buildProfile(p: Probe): CapabilityProfile {
  const yes = (type: string) => p.mse(type) || p.canPlay(type) !== ''
  const video: Video[] = []

  const h264 = ['baseline', 'constrained baseline', 'main', 'high'].filter((_, i) =>
    yes(mp4(['avc1.42E01E', 'avc1.42E01E', 'avc1.4D401F', 'avc1.640028'][i]!)),
  )
  if (h264.length) {
    const level = h264Levels.find((l) => yes(mp4(`avc1.6400${l.toString(16).toUpperCase()}`)))
    const high10 = yes(mp4('avc1.6E0028'))
    video.push({
      codec: 'h264',
      profiles: high10 ? [...h264, 'high 10'] : h264,
      maxLevel: level ?? 40,
      maxBitDepth: high10 ? 10 : 8,
    })
  }

  const hevcMain = yes(mp4('hvc1.1.6.L120.90')) || yes(mp4('hev1.1.6.L120.90'))
  const hevcMain10 = yes(mp4('hvc1.2.4.L120.90')) || yes(mp4('hev1.2.4.L120.90'))
  if (hevcMain || hevcMain10) {
    const tag = hevcMain10 ? 'hvc1.2.4' : 'hvc1.1.6'
    const level = hevcLevels.find((l) => yes(mp4(`${tag}.L${l}.90`)))
    video.push({
      codec: 'hevc',
      profiles: [hevcMain && 'main', hevcMain10 && 'main 10'].filter((x): x is string => !!x),
      maxLevel: level ?? 120,
      maxBitDepth: hevcMain10 ? 10 : 8,
      // HEVC strings can't carry the transfer: an HDR screen and Main 10 it is.
      ...(p.hdrScreen && hevcMain10 ? { hdr: ['hdr10', 'hlg'] as Video['hdr'] } : {}),
    })
  }

  const vp9 = yes(webm('vp09.00.40.08')) || yes(mp4('vp09.00.40.08'))
  if (vp9) {
    const p2 = yes(webm('vp09.02.40.10')) || yes(mp4('vp09.02.40.10'))
    video.push({
      codec: 'vp9',
      profiles: p2 ? ['profile 0', 'profile 2'] : ['profile 0'],
      maxBitDepth: p2 ? 10 : 8,
      ...hdrOf(p, (transfer) => webm(`vp09.02.40.10.01.09.${transfer}.09.01`)),
    })
  }

  const av1 = yes(mp4('av01.0.08M.08'))
  if (av1) {
    const ten = yes(mp4('av01.0.08M.10'))
    video.push({
      codec: 'av1',
      profiles: ['main'],
      maxBitDepth: ten ? 10 : 8,
      ...hdrOf(p, (transfer) => mp4(`av01.0.08M.10.0.110.09.${transfer}.09.0`)),
    })
  }

  const audio: Audio[] = []
  const audioType = (container: string, codec: string) => `audio/${container}; codecs="${codec}"`
  if (yes(audioType('mp4', 'mp4a.40.2'))) audio.push({ codec: 'aac', maxChannels: 6 })
  if (yes('audio/mpeg') || yes(audioType('mp4', 'mp4a.69')))
    audio.push({ codec: 'mp3', maxChannels: 2 })
  if (yes(audioType('mp4', 'opus')) || yes(audioType('webm', 'opus')))
    audio.push({ codec: 'opus', maxChannels: 6 })
  if (yes(audioType('webm', 'vorbis'))) audio.push({ codec: 'vorbis', maxChannels: 6 })
  if (yes(audioType('mp4', 'flac'))) audio.push({ codec: 'flac', maxChannels: 6 })
  if (yes(audioType('mp4', 'ac-3'))) audio.push({ codec: 'ac3', maxChannels: 6 })
  if (yes(audioType('mp4', 'ec-3'))) audio.push({ codec: 'eac3', maxChannels: 6 })

  const containers = ['mp4']
  if (p.canPlay('video/webm') !== '') containers.push('webm')
  if (p.canPlay('video/x-matroska') !== '') containers.push('mkv')
  if (p.canPlay('video/quicktime') !== '') containers.push('mov')

  return {
    protocols: ['http', 'hls'],
    containers,
    video,
    audio,
    subtitles: { text: ['vtt'] },
    canSwitchAudioTrack: false,
  }
}

// HDR only with an HDR screen and the codec string with the PQ (16) or HLG
// (18) transfer.
function hdrOf(p: Probe, type: (transfer: string) => string): Pick<Video, 'hdr'> {
  if (!p.hdrScreen) return {}
  const hdr: NonNullable<Video['hdr']> = []
  if (p.mse(type('16'))) hdr.push('hdr10')
  if (p.mse(type('18'))) hdr.push('hlg')
  return hdr.length ? { hdr } : {}
}

export function browserProbe(): Probe {
  const w = window as Window & {
    ManagedMediaSource?: { isTypeSupported(t: string): boolean }
  }
  const ms = w.ManagedMediaSource ?? (typeof MediaSource === 'undefined' ? undefined : MediaSource)
  const video = document.createElement('video')
  return {
    mse: (t) => {
      try {
        return !!ms?.isTypeSupported(t)
      } catch {
        return false
      }
    },
    canPlay: (t) => video.canPlayType(t) as '' | 'maybe' | 'probably',
    hdrScreen: !!window.matchMedia?.('(dynamic-range: high)').matches,
  }
}

/** Plays HLS without hls.js (Safari, iOS). */
export function nativeHls(probe: Probe = browserProbe()): boolean {
  return probe.canPlay('application/vnd.apple.mpegurl') !== ''
}

const storageKey = 'plaid.capabilities'

/** The profile, built once per tab session (sessionStorage). */
export function capabilityProfile(probe?: Probe): CapabilityProfile {
  try {
    const cached = sessionStorage.getItem(storageKey)
    if (cached) return JSON.parse(cached) as CapabilityProfile
  } catch {
    // Unreadable storage: build it again.
  }
  const profile = buildProfile(probe ?? browserProbe())
  try {
    sessionStorage.setItem(storageKey, JSON.stringify(profile))
  } catch {
    // Private mode: fine, it's rebuilt next time.
  }
  return profile
}
