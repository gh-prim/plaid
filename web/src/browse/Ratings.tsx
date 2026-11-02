import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { formatPercent } from '../lib/format'
import { minVotes } from './itemFacts'

type ItemDetail = components['schemas']['ItemDetail']

/** Rotten Tomatoes calls 60 and up fresh (critics) or full (audience). */
const fresh = 60

/** TMDB's mark: its name on its blue-to-green gradient. */
function TmdbIcon() {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true">
      <defs>
        <linearGradient id="br-tmdb" x1="0" x2="1">
          <stop offset="0" stopColor="#90cea1" />
          <stop offset="1" stopColor="#01b4e4" />
        </linearGradient>
      </defs>
      <rect x="1" y="7" width="30" height="18" rx="9" fill="url(#br-tmdb)" />
      <text x="16" y="20" textAnchor="middle" fontSize="9" fontWeight="800" fill="#0d253f">
        TMDB
      </text>
    </svg>
  )
}

/** A red tomato with its green leaves, or a green splat when rotten. */
function TomatoIcon({ rotten }: { rotten: boolean }) {
  return rotten ? (
    <svg viewBox="0 0 32 32" aria-hidden="true">
      <path fill="#0fc755" d="M16 4l3 6 6-3-2 7 6 2-6 4 3 6-7-2-3 6-3-6-7 2 3-6-6-4 6-2-2-7 6 3z" />
    </svg>
  ) : (
    <svg viewBox="0 0 32 32" aria-hidden="true">
      <circle cx="16" cy="18" r="12" fill="#fa320a" />
      <path
        fill="#00912d"
        d="M16 9c-3-4-7-4-9-2 3 0 5 1 6 3-3-1-6 0-7 3 3-1 6-1 9 0 2-2 6-3 9-2-1-3-4-4-7-3 1-2 3-3 5-3-2-2-5-1-6 4z"
      />
    </svg>
  )
}

/** A full popcorn bucket, or a tipped-over one when most didn't like it. */
function PopcornIcon({ spilled }: { spilled: boolean }) {
  const bucket = (
    <>
      <path fill="#f4ecdc" d="M8 13h16l-2 16H10z" />
      <path fill="#fa320a" d="M11.2 13h2.4l.6 16H12zM18.4 13h2.4L20 29h-2.2z" />
    </>
  )
  const corn = (
    <g fill="#ffd34d">
      <circle cx="10" cy="11" r="3" />
      <circle cx="14.5" cy="9" r="3.4" />
      <circle cx="19" cy="9.5" r="3.2" />
      <circle cx="22.5" cy="11.5" r="2.6" />
    </g>
  )
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true">
      {spilled ? (
        <>
          <g transform="rotate(-100 16 21)">{bucket}</g>
          <g fill="#ffd34d">
            <circle cx="26" cy="27" r="2.4" />
            <circle cx="29" cy="24.5" r="2" />
          </g>
        </>
      ) : (
        <>
          {corn}
          {bucket}
        </>
      )}
    </svg>
  )
}

/**
 * The scores row of a movie or show: TMDB's users, Rotten Tomatoes'
 * Tomatometer (critics) and Popcornmeter (audience), each with its mark.
 * A score nobody has is left out; with none, nothing is drawn.
 */
export function Ratings({ item }: { item: ItemDetail }) {
  const { t, i18n } = useTranslation()
  const m = item.metadata
  const pct = (v: number) => formatPercent(i18n.language, v / 100)
  const scores: { key: string; icon: React.ReactNode; value: number; label: string }[] = []
  const votes = m?.ratingCount ?? 0
  if (item.rating && votes >= minVotes) {
    const value = Math.round(item.rating * 10)
    scores.push({
      key: 'tmdb',
      icon: <TmdbIcon />,
      value,
      label: t('item.ratings.tmdb', { score: pct(value), count: votes }),
    })
  }
  if (m?.criticScore != null) {
    scores.push({
      key: 'critics',
      icon: <TomatoIcon rotten={m.criticScore < fresh} />,
      value: m.criticScore,
      label: t('item.ratings.critics', { score: pct(m.criticScore) }),
    })
  }
  if (m?.audienceScore != null) {
    scores.push({
      key: 'audience',
      icon: <PopcornIcon spilled={m.audienceScore < fresh} />,
      value: m.audienceScore,
      label: t('item.ratings.audience', { score: pct(m.audienceScore) }),
    })
  }
  if (!scores.length) return null
  return (
    <ul className="br-ratings" aria-label={t('item.ratings.title')}>
      {scores.map((s) => (
        <li key={s.key} className="br-rating" title={s.label}>
          {s.icon}
          <span aria-hidden="true">{pct(s.value)}</span>
          <span className="br-visually-hidden">{s.label}</span>
        </li>
      ))}
    </ul>
  )
}
