import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { Icon, cx } from '../ui'
import { PosterCard } from './cards/PosterCard'
import { initials } from './CastRow'
import { Img } from './Img'
import { imageSrc } from './media'
import { firstYear } from './MovieHeader'
import { PageLight } from './PageLight'
import { flatHits, searchFieldId, useSearchQuery } from './SearchResults'
import { useSearch } from './useSearch'

type ItemSummary = components['schemas']['ItemSummary']
type Artwork = components['schemas']['Artwork']
type Scope = 'all' | 'movies' | 'shows' | 'episodes' | 'people'

const recentKey = 'plaid.search.recent'
const recentMax = 6
/** Genre tiles before typing: the biggest genres of the movies library. */
const genreTiles = 8
const topMax = 12

function loadRecent(): string[] {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(recentKey) ?? '[]')
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
  } catch {
    return []
  }
}

function remember(q: string) {
  const query = q.trim()
  if (!query) return
  try {
    const list = [query, ...loadRecent().filter((r) => r !== query)].slice(0, recentMax)
    localStorage.setItem(recentKey, JSON.stringify(list))
  } catch {
    // Private window: nothing to remember.
  }
}

/** Movies and shows mixed, one of each in turn, as the server ranked them. */
function interleave(a: ItemSummary[], b: ItemSummary[]): ItemSummary[] {
  const out: ItemSummary[] = []
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    if (a[i]) out.push(a[i]!)
    if (b[i]) out.push(b[i]!)
  }
  return out
}

/**
 * Search (F008-R25, V2): the field is the tab bar's magnifier (phones have
 * it here). Before typing, recent searches and genres as tiles; then
 * filters with their counts, Top results as posters, Episodes and People.
 */
export function SearchPage() {
  const { t } = useTranslation()
  const { state } = useAuth()
  const serverName = state.status === 'ready' ? state.server.name : ''
  const navigate = useNavigate()
  const [q, setQ] = useSearchQuery()
  const { query, results, stale } = useSearch(q)
  const [scope, setScope] = useState<Scope>('all')
  const [recent, setRecent] = useState(loadRecent)
  const [genres, setGenres] = useState<{ id: number; name: string; art?: Artwork }[]>()
  const [libraryId, setLibraryId] = useState<string>()

  // Genres as tiles, each lit by one of its best titles.
  useEffect(() => {
    let cancelled = false
    void (async () => {
      const { data } = await api.GET('/libraries')
      const lib = data?.find((l) => l.type === 'movies') ?? data?.[0]
      if (!lib || cancelled) return
      setLibraryId(lib.id)
      const f = await api.GET('/libraries/{libraryId}/facets', {
        params: { path: { libraryId: lib.id } },
      })
      const top = (f.data?.genres ?? []).slice(0, genreTiles)
      if (cancelled) return
      setGenres(top.map((g) => ({ id: g.id, name: g.name })))
      const arts = await Promise.all(
        top.map((g) =>
          api
            .GET('/libraries/{libraryId}/items', {
              params: {
                path: { libraryId: lib.id },
                query: { genre: [g.id], sort: 'rating', order: 'desc', limit: 1 },
              },
            })
            .then(({ data }) => data?.items[0]?.backdrop)
            .catch(() => undefined),
        ),
      )
      if (!cancelled) setGenres(top.map((g, i) => ({ id: g.id, name: g.name, art: arts[i] })))
    })().catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  // Enter opens the first result, from the tab bar's field too.
  const hits = flatHits(results)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement
      if (e.key !== 'Enter' || !el.id?.startsWith(searchFieldId) || !hits[0]) return
      e.preventDefault()
      remember(q)
      void navigate(hits[0].href)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [hits, q, navigate])

  const pick = () => {
    remember(q)
    setRecent(loadRecent())
  }
  const top = results ? interleave(results.movies, results.shows) : []
  const counts = results && {
    all: hits.length,
    movies: results.movies.length,
    shows: results.shows.length,
    episodes: results.episodes.length,
    people: results.people.length,
  }
  const scopes: { key: Scope; label: string }[] = [
    { key: 'all', label: t('search.scope.all') },
    { key: 'movies', label: t('search.group.movies') },
    { key: 'shows', label: t('search.group.shows') },
    { key: 'episodes', label: t('search.group.episodes') },
    { key: 'people', label: t('search.group.people') },
  ]
  const posters =
    scope === 'movies'
      ? (results?.movies ?? [])
      : scope === 'shows'
        ? (results?.shows ?? [])
        : scope === 'all'
          ? top.slice(0, topMax)
          : []
  const show = (s: Scope) => scope === 'all' || scope === s

  return (
    <div className="br-searchpage">
      <PageLight art={top[0]?.backdrop} />
      {/* Phones: the tab bar is at the bottom, so the field is here. */}
      <label className="br-searchpage__field" role="search">
        <Icon name="search" size={18} />
        <input
          id={`${searchFieldId}-phone`}
          type="search"
          autoComplete="off"
          aria-label={t('search.label', { serverName })}
          placeholder={t('search.placeholder')}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => e.key === 'Escape' && setQ('')}
        />
      </label>

      {!query && (
        <div className="br-searchpage__start">
          {recent.length > 0 && (
            <section aria-labelledby="search-recent">
              <h2 id="search-recent" className="br-searchpage__h">
                {t('search.recent')}
              </h2>
              <div className="br-searchpage__chips">
                {recent.map((r) => (
                  <button
                    key={r}
                    type="button"
                    className="br-searchpage__chip br-glass pl-focusable"
                    onClick={() => setQ(r)}
                  >
                    <Icon name="restart" size={16} />
                    {r}
                  </button>
                ))}
              </div>
            </section>
          )}
          {genres && genres.length > 0 && libraryId && (
            <section aria-labelledby="search-genres">
              <h2 id="search-genres" className="br-searchpage__h">
                {t('search.genres')}
              </h2>
              <ul className="br-searchpage__genres">
                {genres.map((g) => (
                  <li key={g.id}>
                    <Link
                      to={`/library/${libraryId}?genre=${g.id}`}
                      className="br-genretile pl-focusable"
                    >
                      {g.art && <img src={imageSrc(g.art.imageId, 500)} alt="" loading="lazy" />}
                      <span>{g.name}</span>
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </div>
      )}

      {query && !results && (
        <div className="br-searchpage__results" aria-busy="true" aria-label={t('common.loading')}>
          <ul className="br-searchpage__posters">
            {Array.from({ length: 6 }, (_, i) => (
              <li key={i} className="br-card br-card--poster br-card--skeleton" aria-hidden="true">
                <span className="br-card__art" />
              </li>
            ))}
          </ul>
        </div>
      )}

      {query && results && hits.length === 0 && (
        <div className="br-searchpage__none" role="status">
          <strong>{t('search.empty', { query })}</strong>
          <span>{t('search.emptyHint', { serverName })}</span>
        </div>
      )}

      {query && results && counts && hits.length > 0 && (
        <div className={cx('br-searchpage__results', stale && 'is-stale')} aria-busy={stale}>
          <div
            className="br-searchpage__scopes"
            role="tablist"
            aria-label={t('search.scope.label')}
          >
            {scopes
              .filter((s) => s.key === 'all' || counts[s.key] > 0)
              .map((s) => (
                <button
                  key={s.key}
                  type="button"
                  role="tab"
                  aria-selected={scope === s.key}
                  className={cx('br-pill pl-focusable', scope === s.key && 'is-on')}
                  onClick={() => setScope(s.key)}
                >
                  {s.label}
                  <span className="br-pill__count">{counts[s.key]}</span>
                </button>
              ))}
          </div>

          {posters.length > 0 && (
            <section aria-labelledby="search-top">
              <h2 id="search-top" className="br-searchpage__h">
                {scope === 'all'
                  ? t('search.top')
                  : scope === 'movies'
                    ? t('search.group.movies')
                    : t('search.group.shows')}
              </h2>
              <ul className="br-searchpage__posters">
                {posters.map((i) => (
                  <li key={i.id} onClickCapture={pick}>
                    <PosterCard item={i} caption={false} sizes="204px" />
                    <span className="br-searchpage__cap">
                      <strong>{i.displayTitle || i.title}</strong>
                      <span>{kindLine(t, i)}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {show('episodes') && results.episodes.length > 0 && (
            <section aria-labelledby="search-episodes">
              <h2 id="search-episodes" className="br-searchpage__h">
                {t('search.group.episodes')}
              </h2>
              <ul className="br-searchpage__episodes">
                {results.episodes.map((e) => (
                  <li key={e.id}>
                    <Link to={`/item/${e.id}`} className="br-episode pl-focusable" onClick={pick}>
                      <span className="br-episode__art">
                        {(e.still ?? e.backdrop) && (
                          <Img
                            src={imageSrc((e.still ?? e.backdrop)!.imageId, 500)}
                            alt=""
                            loading="lazy"
                          />
                        )}
                      </span>
                      <span className="br-episode__text">
                        <span className="br-episode__date">
                          {e.show && e.seasonNumber !== undefined && e.episodeNumber !== undefined
                            ? t('search.episodeOf', {
                                show: e.show.displayTitle,
                                season: e.seasonNumber,
                                episode: e.episodeNumber,
                              })
                            : e.show?.displayTitle}
                        </span>
                        <strong>{e.displayTitle || e.title}</strong>
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {show('people') && results.people.length > 0 && (
            <section aria-labelledby="search-people">
              <h2 id="search-people" className="br-searchpage__h">
                {t('search.group.people')}
              </h2>
              <ul className="br-searchpage__people">
                {results.people.map((p) => (
                  <li key={p.id}>
                    <Link
                      to={`/person/${p.id}`}
                      className="br-searchperson pl-focusable"
                      onClick={pick}
                    >
                      {p.portrait ? (
                        <Img src={imageSrc(p.portrait.imageId, 185)} alt="" />
                      ) : (
                        <span aria-hidden="true">{initials(p.name)}</span>
                      )}
                      <strong>{p.name}</strong>
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </div>
      )}
    </div>
  )
}

/** "Movie · 2024" or "Show · 4 seasons" under a poster. */
function kindLine(t: ReturnType<typeof useTranslation>['t'], i: ItemSummary) {
  if (i.kind === 'show')
    return [t('fixMatch.kind.show'), i.childCount ? t('item.seasons', { count: i.childCount }) : '']
      .filter(Boolean)
      .join(t('card.separator'))
  const year = firstYear(i)
  return [t('fixMatch.kind.movie'), year ? String(year) : '']
    .filter(Boolean)
    .join(t('card.separator'))
}
