import { Fragment, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Button, CandidateCard, EmptyState, FileRow, TextField } from '../ui'
import { Section, useAdmin } from './AdminLayout'

type Candidate = components['schemas']['MatchCandidate']
type ReviewItem = components['schemas']['ReviewItem']
type UnrecognizedFile = components['schemas']['UnrecognizedFile']

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

/** A TMDB search; picking a result calls onPick with its id. */
function TmdbSearch({
  kind,
  onPick,
}: {
  kind: 'movie' | 'show'
  onPick: (tmdbId: number) => void
}) {
  const { t } = useTranslation()
  const [q, setQ] = useState('')
  const [results, setResults] = useState<Candidate[]>()
  const [chosen, setChosen] = useState<number>()

  async function search(e: FormEvent) {
    e.preventDefault()
    const p = parseQuery(q)
    if (p.tmdbId) return onPick(p.tmdbId)
    const { data } = await api.GET('/metadata/search', {
      params: { query: { kind, query: p.query, year: p.year } },
    })
    setResults(data?.candidates ?? [])
  }

  return (
    <div className="adm__section">
      <form className="adm__form" onSubmit={(e) => void search(e)}>
        <TextField
          kind="search"
          label={t('review.search.label')}
          help={t('review.search.help')}
          value={q}
          onValue={setQ}
        />
      </form>
      {results && (
        <div className="adm__list" role="radiogroup">
          {results.map((c) => (
            <CandidateCard
              key={c.tmdbId}
              name="search"
              title={c.title}
              meta={c.year ? String(c.year) : undefined}
              poster={candidatePoster(c)}
              selected={chosen === c.tmdbId}
              onSelect={() => setChosen(c.tmdbId)}
            />
          ))}
          {chosen != null && (
            <div>
              <Button size="sm" onClick={() => onPick(chosen)}>
                {t('review.choose')}
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

/** Files whose name couldn't be understood (board AP07, F007 R21). */
export function Unrecognized() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const { data: admin, refresh } = useAdmin()
  const lib = admin?.libraries.find((l) => l.id === id)
  const [files, setFiles] = useState<UnrecognizedFile[]>()
  const [open, setOpen] = useState<string>()
  const [done, setDone] = useState<Set<string>>(new Set())

  useEffect(() => {
    api
      .GET('/libraries/{libraryId}/unrecognized', { params: { path: { libraryId: id } } })
      .then(({ data }) => setFiles(data?.files ?? []))
      .catch(() => setFiles([]))
  }, [id])

  async function identify(fileId: string, tmdbId: number) {
    const { error } = await api.POST('/media-files/{fileId}/identify', {
      params: { path: { fileId } },
      body: { tmdbId },
    })
    if (!error) {
      setDone((d) => new Set(d).add(fileId))
      setOpen(undefined)
      void refresh()
    }
  }

  const remaining = (files ?? []).filter((f) => !done.has(f.id))
  return (
    <Section>
      <div>
        {lib && <p className="adm__kicker">{lib.name}</p>}
        <h2 className="adm__h2">{t('review.unrecognized.title', { count: remaining.length })}</h2>
        <p className="adm__lead">{t('review.unrecognized.body')}</p>
      </div>
      {files && !files.length && <EmptyState title={t('review.empty')} />}
      <ul className="adm__rows">
        {(files ?? []).map((f) => {
          const slash = f.path.lastIndexOf('/')
          return (
            <Fragment key={f.id}>
              <FileRow
                name={f.path.slice(slash + 1)}
                folder={f.path.slice(0, slash)}
                identified={done.has(f.id)}
                onIdentify={() => setOpen(open === f.id ? undefined : f.id)}
              />
              {open === f.id && (
                <li>
                  <TmdbSearch
                    kind={lib?.type === 'shows' ? 'show' : 'movie'}
                    onPick={(tmdbId) => void identify(f.id, tmdbId)}
                  />
                </li>
              )}
            </Fragment>
          )
        })}
      </ul>
    </Section>
  )
}

/** Items that need a second look, with their TMDB candidates (board AP08, F007 R22). */
export function Review() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const { data: admin, toast, refresh } = useAdmin()
  const lib = admin?.libraries.find((l) => l.id === id)
  const [items, setItems] = useState<ReviewItem[]>()
  const [chosen, setChosen] = useState<number>()
  const [searching, setSearching] = useState(false)

  useEffect(() => {
    api
      .GET('/libraries/{libraryId}/review', { params: { path: { libraryId: id } } })
      .then(({ data }) => setItems(data?.items ?? []))
      .catch(() => setItems([]))
  }, [id])

  // One item at a time, like the board: the next one comes after a choice.
  const next = () => {
    setItems((list) => list?.slice(1))
    setChosen(undefined)
    setSearching(false)
  }

  async function match(item: ReviewItem, tmdbId: number) {
    const { error } = await api.POST('/items/{itemId}/match', {
      params: { path: { itemId: item.item.id } },
      body: { tmdbId },
    })
    if (error) return
    toast(t('review.matched', { title: item.item.displayTitle ?? item.item.title }))
    next()
    void refresh()
  }

  const reason = (r: string) =>
    r === 'duplicate'
      ? t('review.reason.duplicate')
      : r === 'unmatched'
        ? t('review.reason.unmatched')
        : t('review.reason.candidates')

  const it = items?.[0]
  const pick = it?.candidates.find((c) => c.tmdbId === chosen)
  return (
    <Section>
      <div>
        <p className="adm__kicker">
          {lib?.name} · {t('review.needs.title', { count: items?.length ?? 0 })}
        </p>
        <h2 className="adm__h2">{it ? t('review.pick') : t('review.needs.short')}</h2>
        {it && (
          <p className="adm__lead">
            {t('review.parsed', {
              parsed: [it.item.title, it.item.year].filter(Boolean).join(' '),
            })}{' '}
            {reason(it.reason)}.
          </p>
        )}
      </div>
      {items && !it && (
        <div className="adm__empty">
          <p className="title">{t('review.empty')}</p>
        </div>
      )}
      {it && (
        <>
          <div className="adm__grid adm__grid--3" role="radiogroup" aria-label={t('review.pick')}>
            {it.candidates.map((c) => (
              <CandidateCard
                key={c.tmdbId}
                name={`cand-${it.item.id}`}
                title={c.title}
                meta={c.year ? String(c.year) : undefined}
                score={c.confidence}
                poster={candidatePoster(c)}
                selected={chosen === c.tmdbId}
                onSelect={() => setChosen(c.tmdbId)}
              />
            ))}
          </div>
          <div className="adm__actions">
            <Button disabled={!pick} onClick={() => pick && void match(it, pick.tmdbId)}>
              {pick
                ? t('review.use', { title: pick.title, year: pick.year ?? '' })
                : t('review.choose')}
            </Button>
            <Button variant="secondary" onClick={() => setSearching(!searching)}>
              {t('review.search.label')}
            </Button>
            <Button variant="ghost" onClick={next}>
              {t('review.none')}
            </Button>
          </div>
          {searching && (
            <TmdbSearch
              kind={it.item.kind === 'show' ? 'show' : 'movie'}
              onPick={(tmdbId) => void match(it, tmdbId)}
            />
          )}
        </>
      )}
    </Section>
  )
}
