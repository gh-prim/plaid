import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { formatBytes, formatDuration } from '../lib/format'
import { channelsLabel, languageLabel } from '../player/labels'
import { presentFiles, resolutionName } from './itemFacts'
import { firstYear, hdrNames } from './MovieHeader'

type ItemDetail = components['schemas']['ItemDetail']

/** Distinct, in order. */
const unique = (xs: string[]) => [...new Set(xs.filter(Boolean))]

/**
 * About (V2), at the foot of a detail page: a glass panel with the title,
 * genres and the whole synopsis, then Information, Languages and
 * Accessibility. Each column shows only what the server knows.
 */
export function AboutSection({ item, facts }: { item: ItemDetail; facts?: [string, string][] }) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const m = item.metadata
  const list = (xs: string[]) => new Intl.ListFormat(lng, { type: 'conjunction' }).format(xs)
  const file = presentFiles(item.files)[0]
  const streams = file?.streams ?? []
  const date = (d?: string) =>
    d ? new Intl.DateTimeFormat(lng, { dateStyle: 'medium' }).format(new Date(`${d}T12:00:00`)) : ''

  const info: [string, string][] = []
  const year = firstYear(item)
  if (year)
    info.push([
      item.kind === 'show' ? t('item.about.firstAired') : t('item.about.released'),
      String(year),
    ])
  if (m?.runtimeMinutes && item.kind === 'movie')
    info.push([t('item.about.runtime'), formatDuration(lng, m.runtimeMinutes * 60000)])
  if (item.certification) info.push([t('item.about.rated'), item.certification])
  if (file)
    info.push([
      t('item.about.file'),
      [
        file.edition,
        resolutionName(file.width, file.height),
        file.hdrFormat ? (hdrNames[file.hdrFormat] ?? file.hdrFormat) : '',
        formatBytes(lng, file.size),
      ]
        .filter(Boolean)
        .join(t('card.separator')),
    ])
  if (item.addedAt) info.push([t('item.about.added'), date(item.addedAt.slice(0, 10))])
  if (m && m.studios.length > 0) info.push([t('item.about.studios'), list(m.studios)])

  const languages: [string, string][] = []
  const audio = streams.filter((s) => s.type === 'audio')
  const audioNames = audio.length
    ? unique(
        audio.map((a) =>
          a.channels
            ? t('player.audio.track', {
                language: languageLabel(t, lng, a.language),
                channels: channelsLabel(t, a.channels),
              })
            : languageLabel(t, lng, a.language),
        ),
      )
    : unique(item.technical.audioLanguages.map((l) => languageLabel(t, lng, l)))
  if (audioNames.length) languages.push([t('item.about.audio'), list(audioNames)])
  const subs = streams.filter((s) => s.type === 'subtitle')
  const subNames = unique(
    subs.map((s) => {
      const language = languageLabel(t, lng, s.language)
      if (s.forced) return t('player.subtitles.forced', { language })
      if (s.sdh) return t('player.subtitles.sdhName', { language })
      return language
    }),
  )
  if (subNames.length) languages.push([t('item.about.subtitles'), list(subNames)])
  const sdh = subs.some((s) => s.sdh)

  const columns = [
    { title: t('item.about.information'), rows: [...info, ...(facts ?? [])] },
    { title: t('item.about.languages'), rows: languages },
    {
      title: t('item.about.accessibility'),
      rows: sdh ? ([['SDH', t('item.about.sdh')]] as [string, string][]) : [],
    },
  ].filter((c) => c.rows.length > 0)

  const genres = m?.genres.map((g) => g.name) ?? item.genres
  return (
    <section className="br-about" aria-label={t('item.about.title')}>
      <div className="br-about__panel br-glass">
        <h2 className="br-about__label">{t('item.about.title')}</h2>
        <strong className="br-about__name">{item.displayTitle || item.title}</strong>
        {genres.length > 0 && <span className="br-about__genres">{genres.join(', ')}</span>}
        {m?.tagline && <p className="br-about__tagline">{m.tagline}</p>}
        {m?.overview && <p className="br-about__overview">{m.overview}</p>}
      </div>
      {columns.map((c) => (
        <div key={c.title} className="br-about__col">
          <h3>{c.title}</h3>
          <dl>
            {c.rows.map(([k, v]) => (
              <div key={k}>
                <dt>{k}</dt>
                <dd>{v}</dd>
              </div>
            ))}
          </dl>
        </div>
      ))}
    </section>
  )
}
