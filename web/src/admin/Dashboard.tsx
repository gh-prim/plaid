import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import { formatBytes, formatDuration, formatMbps, formatNumber } from '../lib/format'
import { useServer } from '../onboarding/Page'
import { Avatar, Icon, Spinner } from '../ui'
import { imageSrc } from '../browse/media'
import { methodLabel } from '../player/labels'
import { streamArt, summaryEvery, useAdmin } from './AdminLayout'
import { needs, slow, statusLine } from './summary'
import { Roll, useLeaving } from './v2'

type ActivePlayback = components['schemas']['ActivePlayback']
type ServerStats = components['schemas']['ServerStats']
type ItemSummary = components['schemas']['ItemSummary']

/** Readings kept for the health tiles' small trend lines. */
const trendLength = 9
const weekMs = 7 * 24 * 3600 * 1000
const noStreams: ActivePlayback[] = []
const streamKey = (s: ActivePlayback) => s.session.id

/**
 * The admin dashboard (V2): the server's name and status in words, what's
 * playing as big cards (each with Stop), four health tiles with a small
 * trend, then Needs you, storage per drive and what was added this week.
 */
export function Dashboard() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const { serverName, version } = useServer()
  const { data, refresh, toast } = useAdmin()
  const [history, setHistory] = useState<ServerStats[]>([])
  const { data: stats } = usePolling(async () => {
    const { data: s } = await api.GET('/server/stats')
    if (s) setHistory((h) => [...h, s].slice(-trendLength))
    return s
  }, summaryEvery)
  const [added, setAdded] = useState<{ items: ItemSummary[]; count: number }>()
  const [stopping, setStopping] = useState<string>()
  // A stream that ends fades before its slot closes (TM25).
  const shown = useLeaving(data?.streams ?? noStreams, streamKey)

  useEffect(() => {
    if (!data?.libraries.length) return
    let cancelled = false
    void Promise.all(
      data.libraries.map((l) =>
        api
          .GET('/libraries/{libraryId}/items', {
            params: {
              path: { libraryId: l.id },
              query: { sort: 'added', order: 'desc', limit: 30 },
            },
          })
          .then(({ data: page }) => page?.items ?? [])
          .catch(() => []),
      ),
    ).then((lists) => {
      if (cancelled) return
      const since = Date.now() - weekMs
      const all = lists
        .flat()
        .filter((i) => i.addedAt && new Date(i.addedAt).getTime() > since)
        .sort((a, b) => (b.addedAt ?? '').localeCompare(a.addedAt ?? ''))
      setAdded({ items: all.filter((i) => i.poster).slice(0, 6), count: all.length })
    })
    return () => {
      cancelled = true
    }
    // The libraries list, not every poll of it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data?.libraries.length])

  if (!data)
    return (
      <div className="adm__empty" role="status" aria-busy="true">
        <Spinner size={28} />
      </div>
    )

  const list = needs(t, data)
  const tone = list.some((n) => n.tone === 'error')
    ? 'error'
    : list.length || data.streams.some(slow)
      ? 'warning'
      : 'ok'
  const n = (v: number) => formatNumber(lng, v)
  const facts = [
    t('welcome.version', { version }),
    stats ? t('admin.dash.up', { duration: formatDuration(lng, stats.uptimeSeconds * 1000) }) : '',
    stats?.model ?? '',
  ].filter(Boolean)

  async function stop(s: ActivePlayback) {
    if (stopping) return
    setStopping(s.session.id)
    await api
      .DELETE('/playback/{sessionId}', { params: { path: { sessionId: s.session.id } } })
      .catch(() => {})
    setStopping(undefined)
    toast(t('playing.stopped', { title: s.itemTitle, user: s.profileName }))
    await refresh()
  }

  const trend = (pick: (s: ServerStats) => number | undefined, top: number) => {
    const values = history.map(pick).filter((v): v is number => v !== undefined)
    if (values.length < 2) return undefined
    return values
      .map((v, i) => {
        const x = (i * 140) / (values.length - 1)
        const y = 24 - (Math.min(v, top) / top) * 22
        return `${i ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`
      })
      .join(' ')
  }
  const hw = data.capabilities?.hwAccel.active
  const tiles = [
    {
      key: 'cpu',
      label: t('admin.dash.cpu'),
      value: stats?.cpuPercent !== undefined ? `${n(Math.round(stats.cpuPercent))} %` : '—',
      sub:
        stats?.cpuPercent === undefined
          ? t('admin.dash.unknown')
          : stats.cpuPercent > 85
            ? t('admin.dash.busy')
            : stats.cpuPercent < 10
              ? t('admin.dash.idle')
              : t('admin.dash.normal'),
      tone: stats?.cpuPercent === undefined ? undefined : stats.cpuPercent > 85 ? 'warning' : 'ok',
      line: trend((s) => s.cpuPercent, 100),
    },
    {
      key: 'transcodes',
      label: t('admin.dash.conversions'),
      value: stats ? t('admin.dash.ofMax', { n: stats.transcodes, max: stats.maxTranscodes }) : '—',
      sub: hw ? t('admin.dash.gpu', { accel: hw.toUpperCase() }) : t('admin.dash.software'),
      line: trend((s) => s.transcodes, Math.max(1, stats?.maxTranscodes ?? 1)),
    },
    {
      key: 'upload',
      label: t('admin.dash.upload'),
      value: stats ? formatMbps(lng, stats.uploadBps) : '—',
      sub: t('admin.dash.streams', { count: data.streams.length }),
      line: trend((s) => s.uploadBps, Math.max(1, ...history.map((s) => s.uploadBps))),
    },
    {
      key: 'temp',
      label: t('admin.dash.temperature'),
      value: stats?.temperatureC !== undefined ? `${n(Math.round(stats.temperatureC))} °C` : '—',
      sub:
        stats?.temperatureC === undefined
          ? t('admin.dash.unknown')
          : stats.temperatureC > 80
            ? t('admin.dash.hot')
            : t('admin.dash.fine'),
      tone:
        stats?.temperatureC === undefined ? undefined : stats.temperatureC > 80 ? 'warning' : 'ok',
      line: trend((s) => s.temperatureC, 100),
    },
  ]

  return (
    <div className="adm-dash">
      <section className="adm-dash__head">
        <h1>{serverName}</h1>
        <span className={`adm-dash__status is-${tone}`}>
          <span aria-hidden="true" />
          {statusLine(t, data, list.length)}
        </span>
        <span className="br-filters__fill" />
        <span className="adm-dash__facts">{facts.join(t('card.separator'))}</span>
      </section>

      <section className="adm-dash__playing" aria-labelledby="dash-playing">
        <h2 id="dash-playing" className="adm-dash__h">
          {t('playing.title')} <span>{n(data.streams.length)}</span>
        </h2>
        {shown.length === 0 ? (
          <div className="adm-dash__quiet br-glass">
            <strong>{t('admin.dash.nobody')}</strong>
          </div>
        ) : (
          <ul className="adm-dash__streams">
            {shown.map(({ item: s, leaving }) => {
              const art = streamArt(s, 'backdrop', 780) ?? streamArt(s, 'still', 780)
              const p =
                s.session.durationMs > 0 ? Math.min(1, s.positionMs / s.session.durationMs) : 0
              const transcoding = s.session.method === 'transcode'
              return (
                <li
                  key={s.session.id}
                  className={`adm-stream${leaving ? ' is-leaving' : ''}`}
                  aria-hidden={leaving || undefined}
                >
                  {art && <img src={art} alt="" />}
                  <span className="adm-stream__shade" aria-hidden="true" />
                  <div className="adm-stream__who">
                    <Avatar name={s.profileName} size={30} />
                    <span>
                      <strong>{s.profileName}</strong>
                      <span>{s.deviceName}</span>
                    </span>
                    <span className="br-filters__fill" />
                    <button
                      type="button"
                      className="br-sheet__pill br-glass pl-focusable"
                      aria-label={t('admin.dash.stop', { name: s.profileName })}
                      disabled={stopping === s.session.id}
                      onClick={() => void stop(s)}
                    >
                      {stopping === s.session.id ? (
                        <Spinner size={12} mono />
                      ) : (
                        <Icon name="pause" size={12} />
                      )}
                      {t('admin.dash.stopShort')}
                    </button>
                  </div>
                  <div className="adm-stream__what">
                    <strong>{s.itemTitle}</strong>
                    <span className="adm-stream__track" aria-hidden="true">
                      <span style={{ width: `${Math.round(p * 100)}%` }} />
                    </span>
                    <span className="adm-stream__how">
                      <span
                        className={`adm-chip ${slow(s) ? 'is-error' : transcoding ? 'is-warning' : 'is-ok'}`}
                      >
                        <span aria-hidden="true" />
                        {methodLabel(t, s.session.method)}
                      </span>
                      <span>{s.session.quality}</span>
                    </span>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </section>

      <aside className="adm-dash__health" aria-label={t('admin.dash.health')}>
        {tiles.map((x) => (
          <div key={x.key} className="adm-tile br-glass">
            <p>{x.label}</p>
            <strong>
              <Roll value={x.value} />
            </strong>
            <span className={x.tone === 'warning' ? 'is-warning' : x.tone === 'ok' ? 'is-ok' : ''}>
              {x.sub}
            </span>
            {x.line && (
              <svg viewBox="0 0 140 26" aria-hidden="true">
                <path d={x.line} />
              </svg>
            )}
          </div>
        ))}
      </aside>

      <section className="adm-dash__needs" aria-labelledby="dash-needs">
        <h2 id="dash-needs" className="adm-dash__h">
          {t('admin.needs.title')}
        </h2>
        {list.length === 0 ? (
          <p className="adm-dash__none">{t('admin.home.nothingNeedsYou')}</p>
        ) : (
          <ul className="adm-needs">
            {list.map((x) => (
              <li key={x.key}>
                <Link to={x.to} className="adm-need br-glass pl-focusable">
                  <span className={`adm-need__dot is-${x.tone}`} aria-hidden="true" />
                  <span className="adm-need__text">
                    <strong>{x.title}</strong>
                    {x.body && <span>{x.body}</span>}
                  </span>
                  <Icon name="chevronRight" size={16} />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="adm-dash__storage" aria-labelledby="dash-storage">
        <h2 id="dash-storage" className="adm-dash__h">
          {t('admin.dash.storage')}
        </h2>
        <div className="adm-storage br-glass">
          {!stats && <Spinner size={20} />}
          {stats?.drives.map((d) => {
            const used =
              d.totalBytes && d.freeBytes !== undefined ? 1 - d.freeBytes / d.totalBytes : 0
            return (
              <div key={d.path} className="adm-drive">
                <div className="adm-drive__line">
                  <strong>{d.libraries.join(', ')}</strong>
                  <code>{d.path}</code>
                  <span className="br-filters__fill" />
                  <span className={d.available ? '' : 'is-error'}>
                    {d.available && d.freeBytes !== undefined
                      ? t('admin.dash.free', { size: formatBytes(lng, d.freeBytes) })
                      : t('admin.dash.notAnswering')}
                  </span>
                </div>
                <span
                  className={`adm-drive__bar${d.available ? '' : ' is-error'}`}
                  aria-hidden="true"
                >
                  <span style={{ width: `${Math.round(used * 100)}%` }} />
                </span>
                {d.available && d.totalBytes !== undefined && d.freeBytes !== undefined && (
                  <span className="adm-drive__used">
                    {t('admin.dash.used', {
                      used: formatBytes(lng, d.totalBytes - d.freeBytes),
                      total: formatBytes(lng, d.totalBytes),
                    })}
                  </span>
                )}
              </div>
            )
          })}
        </div>
      </section>

      <section className="adm-dash__added" aria-labelledby="dash-added">
        <h2 id="dash-added" className="adm-dash__h">
          {t('admin.dash.added')} {added && <span>{n(added.count)}</span>}
        </h2>
        {added && added.items.length === 0 && (
          <p className="adm-dash__none">{t('admin.dash.addedNone')}</p>
        )}
        <ul className="adm-added">
          {added?.items.map((i) => (
            <li key={i.id}>
              <Link to={`/item/${i.id}`} className="pl-focusable" aria-label={i.displayTitle}>
                <img src={imageSrc(i.poster!.imageId, 185)} alt="" loading="lazy" />
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}
