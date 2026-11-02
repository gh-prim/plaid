import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { Button, CandidateCard, Dialog, EmptyState, Icon, Spinner, TextField } from '../ui'
import { candidatePoster, parseQuery } from '../browse/matchQuery'
import { useAdmin } from './AdminLayout'
import { MatchSheet } from '../browse/FixMatchSheet'
import { formatBytes, formatNumber } from '../lib/format'
import { Group, Loading, Row, Section } from './v2'
import '../browse/browse.css'

type Candidate = components['schemas']['MatchCandidate']
type ReviewItem = components['schemas']['ReviewItem']
type UnrecognizedFile = components['schemas']['UnrecognizedFile']

export { candidatePoster, parseQuery } from '../browse/matchQuery'

/** A TMDB search; picking a result calls onPick with its id. */
function TmdbSearch({
  kind,
  busy,
  onPick,
}: {
  kind: 'movie' | 'show'
  /** The pick is being applied. */
  busy?: boolean
  onPick: (tmdbId: number) => void
}) {
  const { t } = useTranslation()
  const [q, setQ] = useState('')
  const [results, setResults] = useState<Candidate[]>()
  const [chosen, setChosen] = useState<number>()
  const [loading, setLoading] = useState(false)

  async function search(e: FormEvent) {
    e.preventDefault()
    const p = parseQuery(q)
    if (p.tmdbId) return onPick(p.tmdbId)
    setLoading(true)
    const { data } = await api
      .GET('/metadata/search', { params: { query: { kind, query: p.query, year: p.year } } })
      .catch(() => ({ data: undefined }))
      .finally(() => setLoading(false))
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
      {loading && <Spinner size={28} />}
      {results && !loading && (
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
              <Button size="sm" disabled={busy} loading={busy} onClick={() => onPick(chosen)}>
                {t('review.choose')}
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

/**
 * The season and episode a file's path suggests: "Saison.1/01 - L'Hiver
 * vient.mkv" gives season 1, episode 1; "S02E05E06" gives 2 and 5–6.
 */
export function guessEpisode(path: string): { season: string; episodes: string } {
  const name = path.slice(path.lastIndexOf('/') + 1)
  const se = /s(\d{1,2})[ ._-]*e(\d{1,3})(?:[ ._-]*e(\d{1,3}))?/i.exec(name)
  if (se)
    return {
      season: String(Number(se[1])),
      episodes: [se[2], se[3]].filter(Boolean).map(Number).join('-'),
    }
  const x = /\b(\d{1,2})x(\d{1,3})\b/i.exec(name)
  if (x) return { season: String(Number(x[1])), episodes: String(Number(x[2])) }
  const season = /(?:saison|season|staffel|temporada|stagione|s)[ ._-]*(\d{1,2})(?!\d)/i.exec(path)
  const episode = /^(?:e|ep|episode|épisode)?[ ._-]*(\d{1,3})(?!\d)/i.exec(name)
  return {
    season: season ? String(Number(season[1])) : '',
    episodes: episode ? String(Number(episode[1])) : '',
  }
}

/** "3", "3-4" or "3,4" as episode numbers; undefined when it isn't that. */
function episodeList(text: string): number[] | undefined {
  const range = /^\s*(\d{1,3})\s*-\s*(\d{1,3})\s*$/.exec(text)
  if (range) {
    const [a, b] = [Number(range[1]), Number(range[2])]
    if (b < a || b - a > 9) return undefined
    return Array.from({ length: b - a + 1 }, (_, i) => a + i)
  }
  const list = text.split(',').map((x) => x.trim())
  if (!list.length || list.length > 10 || list.some((x) => !/^\d{1,3}$/.test(x))) return undefined
  return list.map(Number)
}

/** A show was picked for a file: which season and episode it is. */
function EpisodeDialog({
  show,
  path,
  onBack,
  onUse,
}: {
  show: string
  path: string
  onBack: () => void
  onUse: (season: number, episodes: number[]) => Promise<string | undefined>
}) {
  const { t } = useTranslation()
  const guess = guessEpisode(path)
  const [season, setSeason] = useState(guess.season)
  const [episodes, setEpisodes] = useState(guess.episodes)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const seasonOk = /^\d{1,2}$/.test(season.trim())
  const list = episodeList(episodes)
  return (
    <Dialog
      title={t('matchFile.episode.title', { show })}
      actions={
        <>
          <Button variant="ghost" onClick={onBack}>
            {t('matchFile.episode.back')}
          </Button>
          <Button
            loading={busy}
            disabled={!seasonOk || !list}
            onClick={() =>
              void (async () => {
                setBusy(true)
                setError(await onUse(Number(season), list!))
                setBusy(false)
              })()
            }
          >
            {t('matchFile.episode.use')}
          </Button>
        </>
      }
    >
      <p className="adm__text adm__mono">{path.slice(path.lastIndexOf('/') + 1)}</p>
      <TextField
        label={t('matchFile.episode.season')}
        inputMode="numeric"
        value={season}
        onValue={setSeason}
        maxLength={2}
      />
      <TextField
        label={t('matchFile.episode.episodes')}
        help={t('matchFile.episode.help')}
        value={episodes}
        onValue={setEpisodes}
        maxLength={20}
        error={episodes && !list ? t('matchFile.episode.invalid') : error}
      />
    </Dialog>
  )
}

/** Files whose name couldn't be understood (board AP07, F007 R21). */
export function Unrecognized() {
  const { t, i18n } = useTranslation()
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

  // A show picked for a file waits for its season and episode.
  const [episodeFor, setEpisodeFor] = useState<{
    file: UnrecognizedFile
    tmdbId: number
    title: string
  }>()

  async function identify(
    fileId: string,
    body: components['schemas']['IdentifyRequest'],
  ): Promise<Failure | undefined> {
    const { error, response } = await api.POST('/media-files/{fileId}/identify', {
      params: { path: { fileId } },
      body,
    })
    if (error) return failure(error, response)
    setDone((d) => new Set(d).add(fileId))
    void refresh()
    return undefined
  }

  const remaining = (files ?? []).filter((f) => !done.has(f.id))
  const sheet = files?.find((f) => f.id === open)
  const slash = sheet ? sheet.path.lastIndexOf('/') : -1
  return (
    <Section
      title={t('admin.settings.unrecognized')}
      back={{ to: `/admin/libraries/${id}`, label: lib?.name ?? t('libraries.title') }}
      glyph="unmatched"
      paneTitle={files ? t('admin.unrecognized.files', { count: remaining.length }) : undefined}
      paneSub={t('admin.unrecognized.sub')}
    >
      {!files && <Loading />}
      {files && !files.length && <EmptyState title={t('review.empty')} />}
      {files && files.length > 0 && (
        <Group
          title={t('admin.unrecognized.group', {
            name: lib?.name ?? '',
            n: formatNumber(i18n.language, remaining.length),
          })}
          foot={t('review.unrecognized.body')}
        >
          {files.map((f) => {
            const cut = f.path.lastIndexOf('/')
            return (
              <Row
                key={f.id}
                mono
                label={f.path.slice(cut + 1)}
                sub={[f.path.slice(0, cut), formatBytes(i18n.language, f.size)].join(
                  t('card.separator'),
                )}
                control={
                  done.has(f.id) ? (
                    <span className="adm-row__done is-ok">
                      <Icon name="check" size={16} />
                      {t('review.identified')}
                    </span>
                  ) : (
                    <Button variant="secondary" size="sm" onClick={() => setOpen(f.id)}>
                      {t('review.identify')}
                    </Button>
                  )
                }
              />
            )
          })}
        </Group>
      )}
      {sheet && (
        <MatchSheet
          title={t('matchFile.title')}
          kind={lib?.type === 'shows' ? 'show' : 'movie'}
          note={t('matchFile.note')}
          intro={
            <section className="br-file" aria-label={t('matchFile.file')}>
              <p className="br-file__name">{sheet.path.slice(slash + 1)}</p>
              <p className="br-file__facts">
                {[
                  formatBytes(i18n.language, sheet.size),
                  t('matchFile.in', { folder: sheet.path.slice(0, slash) }),
                ].join(t('card.separator'))}
              </p>
            </section>
          }
          empty={
            <>
              <strong>{t('matchFile.empty.title')}</strong>
              <span>{t('matchFile.empty.body')}</span>
            </>
          }
          onClose={() => setOpen(undefined)}
          onUse={async (c) => {
            if (lib?.type === 'shows') {
              setEpisodeFor({ file: sheet, tmdbId: c.tmdbId, title: c.title })
              setOpen(undefined)
              return true
            }
            const fail = await identify(sheet.id, { tmdbId: c.tmdbId })
            if (!fail) setOpen(undefined)
            return !fail
          }}
        />
      )}
      {episodeFor && (
        <EpisodeDialog
          show={episodeFor.title}
          path={episodeFor.file.path}
          onBack={() => {
            setOpen(episodeFor.file.id)
            setEpisodeFor(undefined)
          }}
          onUse={async (seasonNumber, episodeNumbers) => {
            const fail = await identify(episodeFor.file.id, {
              tmdbId: episodeFor.tmdbId,
              seasonNumber,
              episodeNumbers,
            })
            if (!fail) setEpisodeFor(undefined)
            return fail && errorMessage(t, fail)
          }}
        />
      )}
    </Section>
  )
}

/**
 * The item an Edit menu's "Fix the match" points at (?item=), first: its
 * queue entry when it has one, else the item with fresh TMDB candidates.
 */
async function target(itemId: string, queue: ReviewItem[]): Promise<ReviewItem[]> {
  const own = queue.find((r) => r.item.id === itemId)
  if (own) return [own, ...queue.filter((r) => r !== own)]
  const { data: item } = await api.GET('/items/{itemId}', { params: { path: { itemId } } })
  if (!item) return queue
  const { data } = await api.GET('/metadata/search', {
    params: {
      query: { kind: item.kind === 'show' ? 'show' : 'movie', query: item.title ?? '', itemId },
    },
  })
  return [{ item, reason: 'needs_review', candidates: data?.candidates ?? [] }, ...queue]
}

/** Items that need a second look, with their TMDB candidates (board AP08, F007 R22). */
export function Review() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const [params] = useSearchParams()
  const itemId = params.get('item')
  const navigate = useNavigate()
  const { data: admin, toast, refresh } = useAdmin()
  const lib = admin?.libraries.find((l) => l.id === id)
  const [items, setItems] = useState<ReviewItem[]>()
  const [chosen, setChosen] = useState<number>()
  const [searching, setSearching] = useState(false)
  // Matching a show fetches all its seasons first: seconds, not a blink.
  const [matching, setMatching] = useState(false)

  useEffect(() => {
    api
      .GET('/libraries/{libraryId}/review', { params: { path: { libraryId: id } } })
      .then(async ({ data }) => {
        const queue = data?.items ?? []
        setItems(itemId ? await target(itemId, queue) : queue)
      })
      .catch(() => setItems([]))
  }, [id, itemId])

  // One item at a time, like the board: the next one comes after a choice.
  const next = () => {
    setItems((list) => list?.slice(1))
    setChosen(undefined)
    setSearching(false)
  }

  async function match(item: ReviewItem, tmdbId: number) {
    if (matching) return
    setMatching(true)
    const { error } = await api
      .POST('/items/{itemId}/match', {
        params: { path: { itemId: item.item.id } },
        body: { tmdbId },
      })
      .catch((err: unknown) => ({ error: err }))
      .finally(() => setMatching(false))
    if (error) return
    toast(t('review.matched', { title: item.item.displayTitle ?? item.item.title }))
    void refresh()
    // Back to the page the fix started from.
    if (item.item.id === itemId) return void navigate(`/item/${itemId}`)
    next()
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
    <Section
      title={t('review.needs.short')}
      back={{ to: `/admin/libraries/${id}`, label: lib?.name ?? t('libraries.title') }}
      glyph="review"
      paneTitle={items ? t('review.needs.title', { count: items.length }) : undefined}
      paneSub={lib?.name}
    >
      {it && (
        <div>
          <h2 className="adm__h2">{t('review.pick')}</h2>
          <p className="adm__lead">
            {t('review.parsed', {
              parsed: [it.item.title, it.item.year].filter(Boolean).join(' '),
            })}
            . {reason(it.reason)}.
          </p>
        </div>
      )}
      {!items && <Loading label={t('review.loading')} />}
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
            <Button
              disabled={!pick || matching}
              loading={matching}
              onClick={() => pick && void match(it, pick.tmdbId)}
            >
              {pick
                ? t('review.use', { title: pick.title, year: pick.year ?? '' })
                : t('review.choose')}
            </Button>
            <Button
              variant="secondary"
              disabled={matching}
              onClick={() => setSearching(!searching)}
            >
              {t('review.search.label')}
            </Button>
            <Button variant="ghost" disabled={matching} onClick={next}>
              {t('review.none')}
            </Button>
          </div>
          {searching && (
            <TmdbSearch
              kind={it.item.kind === 'show' ? 'show' : 'movie'}
              busy={matching}
              onPick={(tmdbId) => void match(it, tmdbId)}
            />
          )}
        </>
      )}
    </Section>
  )
}
