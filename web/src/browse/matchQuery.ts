import type { components } from '../api/schema'

type Candidate = components['schemas']['MatchCandidate']

export function candidatePoster(c: Candidate): string | undefined {
  return c.posterPath
    ? `/api/v1/metadata/candidate-images/${encodeURIComponent(c.posterPath.replace(/^\//, ''))}`
    : undefined
}

/** "Metropolis 1927" → title and year; a TMDB link → its id (F007 R22). */
export function parseQuery(q: string): { query: string; year?: number; tmdbId?: number } {
  const link = /themoviedb\.org\/(?:movie|tv)\/(\d+)/.exec(q)
  if (link) return { query: q, tmdbId: Number(link[1]) }
  const year = /\b(18|19|20)\d{2}\b/.exec(q)
  return {
    query: year ? q.replace(year[0], '').trim() : q.trim(),
    year: year ? Number(year[0]) : undefined,
  }
}
