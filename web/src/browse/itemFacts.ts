import type { components } from '../api/schema'

type MediaFile = components['schemas']['MediaFile']
type Trailer = components['schemas']['Trailer']

/** A resolution as people say it: 4K, 1080p, 720p, or the height. */
export function resolutionName(width?: number, height?: number): string | undefined {
  if (!width || !height) return undefined
  if (width >= 3000 || height >= 2000) return '4K'
  if (width >= 1800 || height >= 1000) return '1080p'
  if (width >= 1200 || height >= 700) return '720p'
  return `${height}p`
}

/** The files still on disk. */
export const presentFiles = (files: MediaFile[]) => files.filter((f) => !f.missingSince)

/**
 * The trailer to offer (F008-R20): an official one in the UI's language,
 * then an official one in English, then any.
 */
export function pickTrailer(trailers: Trailer[], language: string): Trailer | undefined {
  const yt = trailers.filter((t) => t.site === 'YouTube')
  const lang = language.slice(0, 2)
  return (
    yt.find((t) => t.official && t.language === lang) ??
    yt.find((t) => t.official && t.language === 'en') ??
    yt[0]
  )
}

/** TMDB ratings from fewer votes than this aren't shown (canvas rule). */
export const minVotes = 50
