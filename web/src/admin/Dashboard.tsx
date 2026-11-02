import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { imageSrc } from '../browse/media'
import { usePolling } from '../hooks/usePolling'
import {
  formatBytes,
  formatDuration,
  formatMbps,
  formatNumber,
  formatRelative,
  formatTime,
} from '../lib/format'
import { methodLabel } from '../player/labels'
import { Avatar, Button, Icon, Spinner } from '../ui'
import { streamPoster, summaryEvery, TopBar, useAdmin } from './AdminLayout'
import { needs, slow } from './summary'

type ActivePlayback = components['schemas']['ActivePlayback']
type Activity = components['schemas']['Activity']
type Sample = components['schemas']['StatsSample']

/** A drive under this much free space needs the admin (step 21). */
const lowSpace = 0.1

/** A small trend line through values, scaled to the box. */
export function Spark({ values, area }: { values: number[]; area?: boolean }) {
  if (values.length < 2) return <svg className="ax-spark" viewBox="0 0 160 36" aria-hidden="true" />
  const top = Math.max(1, ...values)
  const pts = values.map((v, i) => [(i * 160) / (values.length - 1), 34 - (v / top) * 30] as const)
  const d = pts.map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`).join(' ')
  return (
    <svg
      className={`ax-spark${area ? ' ax-spark--area' : ''}`}
      viewBox="0 0 160 36"
      aria-hidden="true"
    >
      {area && <path className="ax-spark__fill" d={`${d} L160 36 L0 36 Z`} />}
      <path d={d} />
    </svg>
  )
}

/** Keeps every nth sample, so a day of minutes draws as about 60 points. */
const thin = (samples: Sample[]) =>
  samples.filter((_, i) => i % Math.max(1, Math.floor(samples.length / 60)) === 0)

/**
 * The dashboard (canvas step 21, board AdmEN01): four numbers with their
 * trend, what's playing right now, the server's health, what needs you,
 * the hours watched these 7 days and what was added last.
 */
export function Dashboard() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const { data, refresh, toast } = useAdmin()
  const [stopping, setStopping] = useState<string>()
  const [scanning, setScanning] = useState(false)

  async function scanAll() {
    const libs = data?.libraries ?? []
    setScanning(true)
    await Promise.all(
      libs.map((l) =>
        api
          .POST('/libraries/{libraryId}/scan', {
            params: { path: { libraryId: l.id } },
            body: { full: false },
          })
          .catch(() => undefined),
      ),
    )
    setScanning(false)
    toast(t('admin.libraries.scanStarted', { count: libs.length }))
    await refresh()
  }
  const { data: stats } = usePolling(
    async () => (await api.GET('/server/stats')).data ?? null,
    summaryEvery,
  )
  const { data: history } = usePolling(
    async () => (await api.GET('/server/stats/history')).data?.samples ?? [],
    60_000,
  )
  const { data: activity } = usePolling(
    async (): Promise<Activity | null> =>
      (
        await api.GET('/activity', {
          params: { query: { days: 7, tzOffsetMinutes: new Date().getTimezoneOffset() } },
        })
      ).data ?? null,
    60_000,
  )
  const { data: people } = usePolling(async () => (await api.GET('/users')).data ?? [], 60_000)
  const today = new Intl.DateTimeFormat(lng, {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
  }).format(new Date())

  if (!data)
    return (
      <>
        <TopBar title={t('admin.tab.dashboard')} sub={today} />
        <div className="ax-body" role="status" aria-busy="true">
          <Spinner size={28} />
        </div>
      </>
    )

  const n = (v: number) => formatNumber(lng, v)
  const streams = data.streams
  const transcodes = streams.filter((s) => s.session.method === 'transcode').length
  const samples = thin(history ?? [])
  const drives = stats?.drives.filter((d) => d.available && d.totalBytes) ?? []
  const total = drives.reduce((a, d) => a + (d.totalBytes ?? 0), 0)
  const free = drives.reduce((a, d) => a + (d.freeBytes ?? 0), 0)
  const used = total - free
  const peak = (history ?? []).reduce<Sample | undefined>(
    (p, s) => (!p || s.uploadBps > p.uploadBps ? s : p),
    undefined,
  )

  const list = needs(t, data)
  const unmatched = data.libraries.reduce((a, l) => a + l.counts.unrecognized, 0)
  if (unmatched)
    list.push({
      key: 'unmatched',
      title: t('admin.dash.unmatched', { count: unmatched }),
      to: '/admin/unmatched',
      tone: 'warning',
    })
  for (const d of drives)
    if (d.freeBytes !== undefined && d.totalBytes && d.freeBytes / d.totalBytes < lowSpace)
      list.push({
        key: `disk-${d.path}`,
        title: t('admin.dash.diskFull'),
        body: t('admin.dash.left', { size: formatBytes(lng, d.freeBytes), path: d.path }),
        to: '/admin/libraries',
        tone: 'warning',
      })

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

  const days = activity?.days ?? []
  const most = Math.max(1, ...days.map((d) => d.hours))
  const dayLetter = new Intl.DateTimeFormat(lng, { weekday: 'narrow' })

  return (
    <>
      <TopBar title={t('admin.tab.dashboard')} sub={today}>
        <Button
          variant="secondary"
          size="sm"
          loading={scanning}
          disabled={!data.libraries.length}
          onClick={() => void scanAll()}
        >
          <Icon name="refresh" size={14} />
          {t('admin.libraries.scanAll')}
        </Button>
      </TopBar>
      <div className="ax-body ax-dash">
        <section className="ax-kpis" aria-label={t('admin.dash.health')}>
          <div className="ax-card ax-kpi">
            <p>{t('admin.nav.playing')}</p>
            <strong>{n(streams.length)}</strong>
            <span>
              {t('admin.dash.howPlaying', { transcodes, direct: streams.length - transcodes })}
            </span>
            <Spark values={samples.map((s) => s.streams)} />
          </div>
          <div className="ax-card ax-kpi">
            <p>{t('admin.dash.activePeople')}</p>
            <strong>{activity ? n(activity.activePeople) : '—'}</strong>
            <span>{t('admin.dash.ofPeople', { count: people?.length ?? 0 })}</span>
            <Spark values={days.map((d) => d.plays)} />
          </div>
          <div className="ax-card ax-kpi">
            <p>{t('admin.dash.storage')}</p>
            <strong>{total ? formatBytes(lng, used) : '—'}</strong>
            <span>
              {total
                ? t('admin.dash.ofStorage', {
                    total: formatBytes(lng, total),
                    percent: Math.round((used / total) * 100),
                  })
                : t('admin.dash.unknown')}
            </span>
            <span className="ax-bar" aria-hidden="true">
              <span style={{ width: `${total ? Math.round((used / total) * 100) : 0}%` }} />
            </span>
          </div>
          <div className="ax-card ax-kpi">
            <p>{t('admin.dash.upload')}</p>
            <strong>{stats ? formatMbps(lng, stats.uploadBps) : '—'}</strong>
            <span>
              {peak && peak.uploadBps > 0
                ? t('admin.dash.peak', {
                    rate: formatMbps(lng, peak.uploadBps),
                    time: formatTime(lng, new Date(peak.at)),
                  })
                : t('admin.dash.noPeak')}
            </span>
            <Spark area values={samples.map((s) => s.uploadBps)} />
          </div>
        </section>

        <section className="ax-card ax-now" aria-labelledby="ax-now">
          <header className="ax-card__head">
            <h2 id="ax-now">{t('admin.dash.rightNow')}</h2>
            <span>{t('admin.dash.streams', { count: streams.length })}</span>
          </header>
          {streams.length === 0 ? (
            <p className="ax-none">{t('admin.dash.nobody')}</p>
          ) : (
            <table className="ax-table">
              <thead>
                <tr>
                  <th aria-label={t('admin.dash.poster')} />
                  <th>{t('admin.dash.colTitle')}</th>
                  <th>{t('admin.dash.colWho')}</th>
                  <th>{t('admin.dash.colDevice')}</th>
                  <th>{t('admin.dash.colQuality')}</th>
                  <th>{t('admin.dash.colProgress')}</th>
                  <th>{t('admin.dash.colMbps')}</th>
                  <th aria-label={t('admin.dash.stopShort')} />
                </tr>
              </thead>
              <tbody>
                {streams.map((s) => {
                  const p =
                    s.session.durationMs > 0 ? Math.min(1, s.positionMs / s.session.durationMs) : 0
                  const transcoding = s.session.method === 'transcode'
                  const poster = streamPoster(s, 92)
                  return (
                    <tr key={s.session.id}>
                      <td>
                        {poster ? (
                          <img className="ax-thumb" src={poster} alt="" />
                        ) : (
                          <span className="ax-thumb" />
                        )}
                      </td>
                      <td className="ax-strong">{s.itemTitle}</td>
                      <td>
                        <span className="ax-who">
                          <Avatar name={s.profileName} size={22} />
                          {s.profileName}
                        </span>
                      </td>
                      <td className="ax-dim">
                        {s.deviceName}
                        {t('card.separator')}
                        {s.remote ? t('admin.dash.remote') : t('admin.dash.local')}
                      </td>
                      <td>
                        {s.session.quality === 'original'
                          ? t('admin.dash.original')
                          : s.session.quality}{' '}
                        <span
                          className={`ax-chip ${slow(s) ? 'is-error' : transcoding ? 'is-warning' : 'is-ok'}`}
                        >
                          {methodLabel(t, s.session.method)}
                        </span>
                      </td>
                      <td>
                        <span
                          className="ax-bar ax-bar--thin"
                          aria-label={`${Math.round(p * 100)} %`}
                        >
                          <span style={{ width: `${Math.round(p * 100)}%` }} />
                        </span>
                      </td>
                      <td className="ax-dim">
                        {s.remote && s.bitrateBps
                          ? n(Math.round(s.bitrateBps / 1e5) / 10)
                          : t('admin.dash.local')}
                      </td>
                      <td>
                        <button
                          type="button"
                          className="ax-icon-btn pl-focusable"
                          aria-label={t('admin.dash.stop', { name: s.profileName })}
                          disabled={stopping === s.session.id}
                          onClick={() => void stop(s)}
                        >
                          {stopping === s.session.id ? (
                            <Spinner size={12} mono />
                          ) : (
                            <span className="ax-stop" />
                          )}
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </section>

        <section className="ax-card ax-server" aria-labelledby="ax-server">
          <header className="ax-card__head">
            <h2 id="ax-server">{t('admin.dash.server')}</h2>
            {stats && (
              <span>
                {t('admin.dash.up', { duration: formatDuration(lng, stats.uptimeSeconds * 1000) })}
              </span>
            )}
          </header>
          {!stats ? (
            <Spinner size={20} />
          ) : (
            <dl className="ax-meters">
              {[
                {
                  k: t('admin.dash.cpu'),
                  v: stats.cpuPercent !== undefined ? `${n(Math.round(stats.cpuPercent))} %` : '—',
                  p: stats.cpuPercent ?? 0,
                },
                {
                  k: t('admin.dash.memory'),
                  v:
                    stats.memoryUsedBytes !== undefined
                      ? formatBytes(lng, stats.memoryUsedBytes)
                      : '—',
                  p: stats.memoryTotalBytes
                    ? ((stats.memoryUsedBytes ?? 0) / stats.memoryTotalBytes) * 100
                    : 0,
                },
                {
                  k: t('admin.dash.disk'),
                  v: total ? `${n(Math.round((used / total) * 100))} %` : '—',
                  p: total ? (used / total) * 100 : 0,
                  warm: true,
                },
                {
                  k: t('admin.dash.transcode'),
                  v: `${n(stats.transcodes)} / ${n(stats.maxTranscodes)}`,
                  p: stats.maxTranscodes ? (stats.transcodes / stats.maxTranscodes) * 100 : 0,
                  warm: true,
                },
              ].map((m) => (
                <div key={m.k} className="ax-meter">
                  <dt>{m.k}</dt>
                  <dd>
                    <span
                      className={`ax-bar ax-bar--thin${m.warm ? ' ax-bar--warm' : ''}`}
                      aria-hidden="true"
                    >
                      <span style={{ width: `${Math.round(Math.min(100, m.p))}%` }} />
                    </span>
                    <b>{m.v}</b>
                  </dd>
                </div>
              ))}
            </dl>
          )}
        </section>

        <section className="ax-card ax-needs" aria-labelledby="ax-needs">
          <header className="ax-card__head">
            <h2 id="ax-needs">{t('admin.dash.needsAttention')}</h2>
            <span>{n(list.length)}</span>
          </header>
          {list.length === 0 ? (
            <p className="ax-none">{t('admin.home.nothingNeedsYou')}</p>
          ) : (
            <ul className="ax-list">
              {list.map((x) => (
                <li key={x.key}>
                  <Icon name={x.tone === 'error' ? 'error' : 'alert'} size={15} />
                  <span>
                    <b>{x.title}</b>
                    {x.body && <small>{x.body}</small>}
                  </span>
                  <Link to={x.to} className="ax-link pl-focusable">
                    {t('admin.dash.view')}
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="ax-card ax-activity" aria-labelledby="ax-activity">
          <header className="ax-card__head">
            <h2 id="ax-activity">{t('admin.dash.activity')}</h2>
            <span>{t('admin.dash.hoursPerDay')}</span>
          </header>
          {!activity ? (
            <Spinner size={20} />
          ) : (
            <ol className="ax-bars">
              {days.map((d, i) => (
                <li
                  key={d.date}
                  aria-label={t('admin.dash.dayHours', {
                    day: d.date,
                    hours: n(Math.round(d.hours * 10) / 10),
                  })}
                >
                  <span className="ax-bars__v">{n(Math.round(d.hours * 10) / 10)}</span>
                  <span
                    className={`ax-bars__bar${i === days.length - 1 ? ' is-today' : ''}`}
                    style={{ height: `${Math.max(2, (d.hours / most) * 100)}%` }}
                  />
                  <span className="ax-bars__d">
                    {dayLetter.format(new Date(`${d.date}T12:00:00`))}
                  </span>
                </li>
              ))}
            </ol>
          )}
        </section>

        <section className="ax-card ax-added" aria-labelledby="ax-added">
          <header className="ax-card__head">
            <h2 id="ax-added">{t('admin.dash.added')}</h2>
          </header>
          {!activity ? (
            <Spinner size={20} />
          ) : activity.recentlyAdded.length === 0 ? (
            <p className="ax-none">{t('admin.dash.addedNone')}</p>
          ) : (
            <ul className="ax-list">
              {activity.recentlyAdded.slice(0, 5).map((i) => (
                <li key={i.id}>
                  {i.poster ? (
                    <img
                      className="ax-poster"
                      src={imageSrc(i.poster.imageId, 92)}
                      alt=""
                      loading="lazy"
                    />
                  ) : (
                    <i className="ax-poster" aria-hidden="true" />
                  )}
                  <Link to={`/item/${i.id}`} className="pl-focusable">
                    <b>{i.displayTitle}</b>
                    <small>
                      {[
                        data.libraries.find((l) => l.id === i.libraryId)?.name,
                        i.addedAt ? formatRelative(lng, new Date(i.addedAt)) : undefined,
                      ]
                        .filter(Boolean)
                        .join(t('card.separator'))}
                    </small>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </>
  )
}
