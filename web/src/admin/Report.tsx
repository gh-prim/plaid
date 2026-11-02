import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { imageSrc } from '../browse/media'
import { formatBytes, formatNumber, formatRelative } from '../lib/format'
import { Button, EmptyState, Spinner } from '../ui'
import { TopBar, useAdmin } from './AdminLayout'

type Report = components['schemas']['LibraryReport']
type Query = {
  libraryId: string
  show: 'all' | 'unwatched' | 'stale' | 'duplicates'
  quality: '' | 'uhd' | 'fhd' | 'hd' | 'sd'
  genreId: string
  sort: 'size' | 'added' | 'title' | 'plays'
}

const page = 60

/** Picture classes; their names are the same in every language. */
const qualities = [
  ['uhd', '4K'],
  ['fhd', '1080p'],
  ['hd', '720p'],
  ['sd', 'SD'],
] as const

/** The best picture's class as shown on a poster. */
function qualityLabel(height: number): string | undefined {
  if (height <= 0) return undefined
  const i = height >= 1600 ? 0 : height >= 900 ? 1 : height >= 600 ? 2 : 3
  return qualities[i][1]
}

/**
 * The library report: every movie and show as a poster wall, with what it
 * weighs, its best picture and its plays; filtered to what nobody watched,
 * what nobody watches any more, or titles with several files.
 */
export function ReportPage() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const { data: admin } = useAdmin()
  const [q, setQ] = useState<Query>({
    libraryId: '',
    show: 'all',
    quality: '',
    genreId: '',
    sort: 'size',
  })
  const [report, setReport] = useState<{ key: string; data: Report }>()
  const [more, setMore] = useState(false)
  const key = JSON.stringify(q)
  const params = (offset: number) => ({
    query: {
      ...(q.libraryId ? { libraryId: q.libraryId } : {}),
      show: q.show,
      ...(q.quality ? { quality: q.quality } : {}),
      ...(q.genreId ? { genreId: Number(q.genreId) } : {}),
      sort: q.sort,
      offset,
      limit: page,
    },
  })

  useEffect(() => {
    let live = true
    void api.GET('/library-report', { params: params(0) }).then(({ data }) => {
      if (live && data) setReport({ key, data })
    })
    return () => {
      live = false
    }
    // params is built from q, which key follows.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  async function loadMore() {
    if (!report) return
    setMore(true)
    const { data } = await api.GET('/library-report', { params: params(report.data.items.length) })
    setMore(false)
    if (data)
      setReport({ key, data: { ...report.data, items: [...report.data.items, ...data.items] } })
  }

  const set = <K extends keyof Query>(k: K, v: Query[K]) => setQ((x) => ({ ...x, [k]: v }))
  const data = report?.key === key ? report.data : undefined
  const totals = (data ?? report?.data)?.totals
  const n = (v: number) => formatNumber(lng, v)
  const kpi = (show: Query['show'], label: string, count: number, bytes: number) => (
    <button
      type="button"
      className={`ax-card ax-kpi ax-kpi--pick pl-focusable${q.show === show ? ' is-on' : ''}`}
      aria-pressed={q.show === show}
      onClick={() => set('show', q.show === show ? 'all' : show)}
    >
      <p>{label}</p>
      <strong>{n(count)}</strong>
      <span>{formatBytes(lng, bytes)}</span>
    </button>
  )

  return (
    <>
      <TopBar title={t('admin.nav.report')} />
      <div className="ax-body">
        {totals && (
          <div className="ax-kpis">
            {kpi('all', t('admin.report.titles'), totals.titles, totals.bytes)}
            {kpi('unwatched', t('admin.report.unwatched'), totals.unwatched, totals.unwatchedBytes)}
            {kpi('stale', t('admin.report.stale'), totals.stale, totals.staleBytes)}
            {kpi(
              'duplicates',
              t('admin.report.duplicates'),
              totals.duplicates,
              totals.duplicateBytes,
            )}
          </div>
        )}
        <div className="ax-filters">
          <select
            className="ax-filter"
            aria-label={t('admin.libraries.colLibrary')}
            value={q.libraryId}
            onChange={(e) => set('libraryId', e.target.value)}
          >
            <option value="">{t('admin.report.allLibraries')}</option>
            {(admin?.libraries ?? []).map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
          <select
            className="ax-filter"
            aria-label={t('admin.report.quality')}
            value={q.quality}
            onChange={(e) => set('quality', e.target.value as Query['quality'])}
          >
            <option value="">{t('admin.report.anyQuality')}</option>
            {qualities.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <select
            className="ax-filter"
            aria-label={t('admin.report.genre')}
            value={q.genreId}
            onChange={(e) => set('genreId', e.target.value)}
          >
            <option value="">{t('admin.report.anyGenre')}</option>
            {(data ?? report?.data)?.genres.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </select>
          <select
            className="ax-filter"
            aria-label={t('admin.report.sort')}
            value={q.sort}
            onChange={(e) => set('sort', e.target.value as Query['sort'])}
          >
            <option value="size">{t('admin.report.bySize')}</option>
            <option value="added">{t('admin.report.byAdded')}</option>
            <option value="plays">{t('admin.report.byPlays')}</option>
            <option value="title">{t('admin.report.byTitle')}</option>
          </select>
          {data && <span className="ax-dim">{t('admin.report.count', { count: data.total })}</span>}
        </div>
        {!data ? (
          <Spinner size={24} />
        ) : data.items.length === 0 ? (
          <EmptyState title={t('admin.report.none')} />
        ) : (
          <>
            <ul className="ax-wall">
              {data.items.map((r) => {
                const quality = qualityLabel(r.height)
                return (
                  <li key={r.item.id}>
                    <Link to={`/item/${r.item.id}`} className="ax-wall__card pl-focusable">
                      <span className="ax-wall__poster">
                        {r.item.poster ? (
                          <img src={imageSrc(r.item.poster.imageId, 185)} alt="" loading="lazy" />
                        ) : (
                          <i className="ax-poster">{r.item.displayTitle}</i>
                        )}
                        <span className="ax-wall__tags">
                          {quality && <b>{quality}</b>}
                          {r.duplicate && <b className="is-warning">{t('admin.report.dupTag')}</b>}
                        </span>
                      </span>
                      <span className="ax-wall__title">{r.item.displayTitle}</span>
                      <span className="ax-wall__meta">
                        {formatBytes(lng, r.bytes)}
                        {t('card.separator')}
                        {r.plays
                          ? r.lastPlayedAt
                            ? formatRelative(lng, new Date(r.lastPlayedAt))
                            : t('admin.activity.plays', { count: r.plays })
                          : t('admin.report.never')}
                      </span>
                    </Link>
                  </li>
                )
              })}
            </ul>
            {data.items.length < data.total && (
              <div>
                <Button
                  variant="secondary"
                  size="sm"
                  loading={more}
                  onClick={() => void loadMore()}
                >
                  {t('admin.report.more', { count: data.total - data.items.length })}
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}
