import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { candidatePoster, parseQuery } from './matchQuery'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Icon, Spinner, cx } from '../ui'
import { GlassSheet } from './GlassSheet'

type ItemDetail = components['schemas']['ItemDetail']
type Candidate = components['schemas']['MatchCandidate']

/** Results come while typing, this long after the last key. */
const typingPause = 300

/**
 * Fix match (V2): the file path as the subtitle, one search field
 * prefilled from the current match, a Year field, and results while
 * typing. The current match says so; "Use this" applies it and the sheet
 * closes (`onMatched`, which offers Undo).
 */
export function FixMatchSheet({
  item,
  onClose,
  onMatched,
}: {
  item: ItemDetail
  onClose: () => void
  onMatched: (c: Candidate, previous?: number) => void
}) {
  const { t } = useTranslation()
  const current = item.metadata?.providerIds.tmdb
  return (
    <MatchSheet
      title={t('item.edit.fixMatch')}
      subtitle={item.files[0]?.path}
      kind={item.kind === 'show' ? 'show' : 'movie'}
      itemId={item.id}
      initial={item.displayTitle || item.title || ''}
      current={current}
      note={t('fixMatch.keeps')}
      onClose={onClose}
      onUse={async (c) => {
        const { error } = await api
          .POST('/items/{itemId}/match', {
            params: { path: { itemId: item.id } },
            body: { tmdbId: c.tmdbId },
          })
          .catch((err: unknown) => ({ error: err }))
        if (error) return false
        onMatched(c, current)
        return true
      }}
    />
  )
}

/**
 * A TMDB search in a glass sheet (V2): fix match, and match this file.
 * Results come while typing; a TMDB link is used as it is. `onUse` says
 * whether it worked; when it didn't, the sheet says so and stays.
 */
export function MatchSheet({
  title,
  subtitle,
  kind,
  itemId,
  initial = '',
  current,
  intro,
  empty,
  note,
  onClose,
  onUse,
}: {
  title: string
  /** A path, shown in monospace. */
  subtitle?: string
  kind: 'movie' | 'show'
  itemId?: string
  initial?: string
  /** The TMDB id of the current match. */
  current?: number
  /** Above the search (the file being matched). */
  intro?: ReactNode
  /** Before anything is typed. */
  empty?: ReactNode
  note: string
  onClose: () => void
  onUse: (c: Candidate) => Promise<boolean>
}) {
  const { t } = useTranslation()
  const [q, setQ] = useState(initial)
  // Any year to start with: a wrong match often has the wrong year too.
  const [year, setYear] = useState('')
  const [results, setResults] = useState<{ key: string; list: Candidate[] }>()
  const [using, setUsing] = useState<number>()
  const [failed, setFailed] = useState(false)
  const field = useRef<HTMLInputElement>(null)

  const key = `${q}|${year}`
  useEffect(() => {
    const p = parseQuery(q)
    if (p.tmdbId || !p.query) return
    let cancelled = false
    const timer = setTimeout(() => {
      void api
        .GET('/metadata/search', {
          params: { query: { kind, query: p.query, year: Number(year) || p.year, itemId } },
        })
        .then(({ data }) => !cancelled && setResults({ key, list: data?.candidates ?? [] }))
        .catch(() => !cancelled && setResults({ key, list: [] }))
    }, typingPause)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [q, year, kind, itemId, key])

  async function use(c: Candidate) {
    if (using !== undefined) return
    setUsing(c.tmdbId)
    setFailed(false)
    const ok = await onUse(c)
    setUsing(undefined)
    if (!ok) setFailed(true)
  }

  const link = parseQuery(q).tmdbId
  const searching = !link && q.trim() !== '' && results?.key !== key
  const list = results?.list ?? []
  return (
    <GlassSheet
      title={title}
      subtitle={subtitle}
      mono
      onClose={onClose}
      footer={
        <>
          <span className="br-sheet__note">{note}</span>
          <span className="br-filters__fill" />
          <button
            type="button"
            className="br-link pl-focusable"
            onClick={() => field.current?.focus()}
          >
            {t('fixMatch.byLink')}
          </button>
        </>
      }
    >
      {intro}
      <div className="br-sheet__search" role="search">
        <label className="br-field br-field--grow">
          <Icon name="search" size={17} />
          <input
            ref={field}
            // Focused with the current title selected: type to replace it.
            autoFocus
            onFocus={(e) => e.currentTarget.select()}
            type="search"
            aria-label={t('review.search.label')}
            placeholder={t('fixMatch.placeholder')}
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          {searching && <Spinner size={16} />}
          {q && (
            <button
              type="button"
              className="br-field__clear pl-focusable"
              aria-label={t('fixMatch.clear')}
              onClick={() => {
                setQ('')
                field.current?.focus()
              }}
            >
              <Icon name="x" size={10} />
            </button>
          )}
        </label>
        <label className="br-field br-field--year">
          <span className="br-field__label">{t('fixMatch.year')}</span>
          <input
            inputMode="numeric"
            maxLength={4}
            placeholder={t('fixMatch.anyYear')}
            value={year}
            onChange={(e) => setYear(e.target.value.replace(/\D/g, '').slice(0, 4))}
          />
          {year && (
            <button
              type="button"
              className="br-field__clear pl-focusable"
              aria-label={t('fixMatch.clearYear')}
              onClick={() => setYear('')}
            >
              <Icon name="x" size={10} />
            </button>
          )}
        </label>
      </div>
      {link ? (
        <div className="br-results__none">
          <strong>{t('fixMatch.link', { id: link })}</strong>
          <button
            type="button"
            className="br-sheet__pill br-sheet__pill--cream pl-focusable"
            disabled={using !== undefined}
            aria-busy={using === link}
            onClick={() => void use({ tmdbId: link, title: String(link) })}
          >
            {t('fixMatch.use')}
          </button>
        </div>
      ) : (
        <>
          <p className="br-results__count" role="status">
            {searching
              ? t('fixMatch.searching')
              : results
                ? t('fixMatch.count', { count: list.length })
                : ''}
          </p>
          {failed && (
            <p role="alert" className="br-results__error">
              {t('errors.generic')}
            </p>
          )}
          {!q.trim() && empty ? (
            <div className="br-results__none">{empty}</div>
          ) : results && !searching && list.length === 0 ? (
            <div className="br-results__none">
              <strong>
                {year
                  ? t('fixMatch.noneYear', { query: q, year })
                  : t('fixMatch.none', { query: q })}
              </strong>
              <span>{t('fixMatch.noneHint')}</span>
              {year && (
                <button
                  type="button"
                  className="br-sheet__pill pl-focusable"
                  onClick={() => setYear('')}
                >
                  {t('fixMatch.allYears')}
                </button>
              )}
            </div>
          ) : (
            <ul
              className={cx('br-results', searching && 'is-stale')}
              aria-label={t('fixMatch.results')}
            >
              {list.map((c) => (
                <li key={c.tmdbId} className="br-result">
                  <span className="br-result__poster">
                    {c.posterPath ? (
                      <img src={candidatePoster(c)} alt="" loading="lazy" />
                    ) : (
                      <span>{c.title}</span>
                    )}
                  </span>
                  <span className="br-result__text">
                    <span className="br-result__line">
                      <strong>{c.title}</strong>
                      {c.year && <span>{c.year}</span>}
                      {c.tmdbId === current && (
                        <span className="br-result__current">
                          <Icon name="check" size={12} />
                          {t('fixMatch.current')}
                        </span>
                      )}
                    </span>
                    <span className="br-result__meta">
                      {[
                        kind === 'show' ? t('fixMatch.kind.show') : t('fixMatch.kind.movie'),
                        c.originalTitle && c.originalTitle !== c.title ? c.originalTitle : '',
                      ]
                        .filter(Boolean)
                        .join(t('card.separator'))}
                    </span>
                    {c.overview && <span className="br-result__overview">{c.overview}</span>}
                  </span>
                  <button
                    type="button"
                    className="br-sheet__pill br-sheet__pill--cream br-result__use pl-focusable"
                    disabled={using !== undefined}
                    aria-busy={using === c.tmdbId}
                    aria-label={t('review.use', { title: c.title, year: c.year ?? '' })}
                    onClick={() => void use(c)}
                  >
                    {using === c.tmdbId ? <Spinner size={14} /> : t('fixMatch.use')}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </GlassSheet>
  )
}
