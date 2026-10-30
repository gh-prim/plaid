import type { components } from '../api/schema'

type Stream = components['schemas']['MediaStream']

/** A subtitle choice: a kind of track in a language, or off. */
export type SubtitleChoice = { language: string; forced: boolean; sdh: boolean } | 'off'

export interface TrackChoice {
  audio?: string
  subtitles?: SubtitleChoice
}

// ISO 639-2 has two codes for some languages (fre/fra): one name each.
const bibliographic: Record<string, string> = {
  sqi: 'alb',
  hye: 'arm',
  eus: 'baq',
  mya: 'bur',
  zho: 'chi',
  ces: 'cze',
  nld: 'dut',
  fra: 'fre',
  kat: 'geo',
  deu: 'ger',
  ell: 'gre',
  isl: 'ice',
  mkd: 'mac',
  mri: 'mao',
  msa: 'may',
  fas: 'per',
  ron: 'rum',
  slk: 'slo',
  bod: 'tib',
  cym: 'wel',
}

export const sameLanguage = (a: string, b: string) =>
  (bibliographic[a.toLowerCase()] ?? a.toLowerCase()) ===
  (bibliographic[b.toLowerCase()] ?? b.toLowerCase())

const read = <T>(key: string): T | undefined => {
  try {
    const v = localStorage.getItem(key)
    return v ? (JSON.parse(v) as T) : undefined
  } catch {
    return undefined
  }
}
const write = (key: string, value: unknown) => {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Full or blocked storage: the choice isn't remembered.
  }
}

const tracksKey = (profileId: string) => `plaid.tracks.${profileId}`

/** The last audio and subtitle choice of a profile (F008-R38). */
export const loadChoice = (profileId: string): TrackChoice => read(tracksKey(profileId)) ?? {}

export function rememberAudio(profileId: string, language: string) {
  write(tracksKey(profileId), { ...loadChoice(profileId), audio: language })
}

export function rememberSubtitles(profileId: string, subtitles: SubtitleChoice) {
  write(tracksKey(profileId), { ...loadChoice(profileId), subtitles })
}

/** Volume and mute, per browser. */
export const loadVolume = () => read<{ volume: number; muted: boolean }>('plaid.volume')
export const rememberVolume = (volume: number, muted: boolean) =>
  write('plaid.volume', { volume, muted })

/** "Play next episode automatically", per profile, on by default. */
export const loadAutoplay = (profileId: string) =>
  read<boolean>(`plaid.autoplay.${profileId}`) ?? true
export const rememberAutoplay = (profileId: string, on: boolean) =>
  write(`plaid.autoplay.${profileId}`, on)

/** The subtitle track a choice picks in a file, if one is in its language. */
export function matchSubtitle(
  choice: Exclude<SubtitleChoice, 'off'>,
  subs: Pick<Stream, 'index' | 'language' | 'forced' | 'sdh'>[],
): number | undefined {
  const same = subs.filter((s) => sameLanguage(s.language, choice.language))
  const best =
    same.find((s) => s.forced === choice.forced && s.sdh === choice.sdh) ??
    same.find((s) => s.forced === choice.forced) ??
    same[0]
  return best?.index
}

/**
 * The tracks to ask for when starting a file: the remembered audio
 * language and subtitles when the file has them; absent fields leave the
 * server's defaults (F006-R4).
 */
export function pickTracks(
  choice: TrackChoice,
  streams: Stream[],
): { audioStreamIndex?: number; subtitleStreamIndex?: number | null } {
  const out: { audioStreamIndex?: number; subtitleStreamIndex?: number | null } = {}
  if (choice.audio) {
    const audio = streams.filter(
      (s) => s.type === 'audio' && sameLanguage(s.language, choice.audio!),
    )
    const pick = audio.find((s) => s.default) ?? audio[0]
    if (pick) out.audioStreamIndex = pick.index
  }
  if (choice.subtitles === 'off') out.subtitleStreamIndex = null
  else if (choice.subtitles) {
    const index = matchSubtitle(
      choice.subtitles,
      streams.filter((s) => s.type === 'subtitle'),
    )
    if (index !== undefined) out.subtitleStreamIndex = index
  }
  return out
}

/**
 * C in the player (F008-R42): off → the remembered language (or the first
 * track) → off.
 */
export function cycleSubtitle(
  current: number | null,
  subs: Pick<Stream, 'index' | 'language' | 'forced' | 'sdh'>[],
  remembered?: SubtitleChoice,
): number | null {
  if (current !== null || subs.length === 0) return null
  if (remembered && remembered !== 'off') {
    const index = matchSubtitle(remembered, subs)
    if (index !== undefined) return index
  }
  return (subs.find((s) => !s.forced) ?? subs[0])!.index
}
