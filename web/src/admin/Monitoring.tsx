import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import {
  formatBytes,
  formatDuration,
  formatNumber,
  formatPercent,
  formatRelative,
  regionName,
} from '../lib/format'
import { Avatar, Spinner } from '../ui'
import { TopBar } from './AdminLayout'
import { Spark } from './Dashboard'
import { WorldMap } from './WorldMap'

type Monitoring = components['schemas']['Monitoring']
type Device = components['schemas']['DeviceHealth']

const periods = [
  { days: 7, key: 'admin.activity.week' },
  { days: 30, key: 'admin.activity.month' },
  { days: 365, key: 'admin.activity.year' },
] as const

/** Below this score a device is one to look at. */
const healthy = 60

/**
 * Monitoring: where remote plays come from on a map, the data sent each
 * day and by whom, and each device's health (direct play, transcodes,
 * codecs it can't decode), worst first.
 */
export function MonitoringPage() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const [days, setDays] = useState<number>(30)
  const { data } = usePolling(
    async (): Promise<Monitoring | null> =>
      (
        await api.GET('/monitoring', {
          params: { query: { days, tzOffsetMinutes: new Date().getTimezoneOffset() } },
        })
      ).data ?? null,
    60_000,
    [days],
  )
  const n = (v: number) => formatNumber(lng, v)
  const total = data?.days.reduce((a, d) => a + d.bytes, 0) ?? 0
  const plays = (data?.local ?? 0) + (data?.remote ?? 0)
  const toCheck = data?.devices.filter((d) => d.score < healthy).length ?? 0
  const most = Math.max(1, ...(data?.days.map((d) => d.bytes) ?? []))
  const date = new Intl.DateTimeFormat(lng, { day: 'numeric', month: 'short' })

  return (
    <>
      <TopBar title={t('admin.nav.monitoring')}>
        <span className="ax-tabs" role="tablist" aria-label={t('admin.activity.period')}>
          {periods.map((p) => (
            <button
              key={p.days}
              type="button"
              role="tab"
              aria-selected={days === p.days}
              className={`ax-tab pl-focusable${days === p.days ? ' is-on' : ''}`}
              onClick={() => setDays(p.days)}
            >
              {t(p.key)}
            </button>
          ))}
        </span>
      </TopBar>
      <div className="ax-body">
        {!data ? (
          <Spinner size={24} />
        ) : (
          <>
            <div className="ax-kpis">
              <section className="ax-card ax-kpi">
                <p>{t('admin.monitoring.sent')}</p>
                <strong>{formatBytes(lng, total)}</strong>
                <Spark values={data.days.map((d) => d.bytes)} />
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.monitoring.remote')}</p>
                <strong>{n(data.remote)}</strong>
                <span>
                  {plays
                    ? t('admin.monitoring.remoteShare', {
                        share: formatPercent(lng, data.remote / plays),
                      })
                    : t('admin.activity.none')}
                </span>
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.monitoring.places')}</p>
                <strong>{n(data.places.length)}</strong>
                <span>
                  {data.places[0]
                    ? (data.places[0].city ?? regionName(lng, data.places[0].country))
                    : t('admin.monitoring.noPlace')}
                </span>
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.monitoring.toCheck')}</p>
                <strong>{n(toCheck)}</strong>
                <span>{t('admin.monitoring.devicesOf', { count: data.devices.length })}</span>
              </section>
            </div>

            <div className="ax-sec has-aside">
              <section className="ax-card">
                <header className="ax-card__head">
                  <h2>{t('admin.monitoring.map')}</h2>
                  <span>{t('admin.monitoring.remotePlays', { count: data.remote })}</span>
                </header>
                <WorldMap
                  label={t('admin.monitoring.map')}
                  points={data.places.map((p) => ({
                    key: `${p.country}-${p.city ?? ''}`,
                    lat: p.lat,
                    lon: p.lon,
                    weight: p.plays,
                    label: `${placeName(lng, p)} · ${t('admin.activity.plays', { count: p.plays })}`,
                  }))}
                />
                <p className="ax-map__foot">
                  {data.geoipReady
                    ? data.unplaced
                      ? t('admin.monitoring.unplaced', { count: data.unplaced })
                      : t('admin.monitoring.localNever')
                    : t('admin.monitoring.downloading')}{' '}
                  <a href="https://db-ip.com" rel="noreferrer" target="_blank">
                    {t('admin.monitoring.credit')}
                  </a>
                </p>
              </section>
              <section className="ax-card">
                <header className="ax-card__head">
                  <h2>{t('admin.monitoring.where')}</h2>
                </header>
                {data.places.length === 0 ? (
                  <p className="ax-none">{t('admin.monitoring.noPlace')}</p>
                ) : (
                  <ul className="adm-group__rows">
                    {data.places.slice(0, 8).map((p) => (
                      <li key={`${p.country}-${p.city ?? ''}`} className="adm-row-item">
                        <div className="adm-row">
                          <span className="adm-row__text">
                            <span className="adm-row__label">{placeName(lng, p)}</span>
                            <span className="adm-row__sub">
                              {t('admin.monitoring.placeSub', {
                                people: t('admin.activity.peopleCount', { count: p.people }),
                                when: formatRelative(lng, new Date(p.lastAt)),
                              })}
                            </span>
                          </span>
                          <span className="adm-row__value">
                            {t('admin.activity.plays', { count: p.plays })}
                          </span>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </div>

            <div className="ax-sec has-aside">
              <section className="ax-card">
                <header className="ax-card__head">
                  <h2>{t('admin.monitoring.perDay')}</h2>
                  <span>{formatBytes(lng, total)}</span>
                </header>
                <ol className={`ax-bars${data.days.length > 31 ? ' ax-bars--dense' : ''}`}>
                  {data.days.map((d, i) => (
                    <li
                      key={d.date}
                      title={`${date.format(new Date(`${d.date}T12:00:00`))} · ${formatBytes(lng, d.bytes)}`}
                    >
                      <span
                        className={`ax-bars__bar${i === data.days.length - 1 ? ' is-today' : ''}`}
                        style={{ height: `${Math.max(2, (d.bytes / most) * 100)}%` }}
                      />
                    </li>
                  ))}
                </ol>
              </section>
              <section className="ax-card">
                <header className="ax-card__head">
                  <h2>{t('admin.monitoring.byPerson')}</h2>
                </header>
                {data.people.length === 0 ? (
                  <p className="ax-none">{t('admin.activity.none')}</p>
                ) : (
                  <table className="ax-table">
                    <tbody>
                      {data.people.map((p) => (
                        <tr key={`${p.userId ?? ''}-${p.displayName}`}>
                          <td>
                            <span className="ax-who">
                              <Avatar name={p.displayName} size={22} />
                              {p.userId ? (
                                <Link
                                  to={`/admin/people/${p.userId}`}
                                  className="ax-strong pl-focusable"
                                >
                                  {p.displayName}
                                </Link>
                              ) : (
                                <b className="ax-strong">{p.displayName}</b>
                              )}
                            </span>
                          </td>
                          <td className="ax-dim">{formatDuration(lng, p.watchedMs)}</td>
                          <td className="ax-strong ax-right">{formatBytes(lng, p.bytes)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </section>
            </div>

            <section className="ax-card">
              <header className="ax-card__head">
                <h2>{t('admin.monitoring.health')}</h2>
                <span>{t('admin.monitoring.healthHelp')}</span>
              </header>
              {data.devices.length === 0 ? (
                <p className="ax-none">{t('admin.activity.none')}</p>
              ) : (
                <table className="ax-table">
                  <thead>
                    <tr>
                      <th>{t('admin.dash.colDevice')}</th>
                      <th>{t('admin.monitoring.score')}</th>
                      <th>{t('admin.activity.directPlay')}</th>
                      <th>{t('admin.monitoring.transcodes')}</th>
                      <th>{t('admin.monitoring.codecs')}</th>
                      <th>{t('admin.monitoring.lastSeen')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.devices.map((d) => (
                      <tr key={`${d.displayName}-${d.deviceName}`}>
                        <td>
                          <span className="ax-who">
                            <span>
                              <b className="ax-strong">{d.deviceName}</b>
                              <small className="ax-dim">
                                {d.displayName}
                                {t('card.separator')}
                                {t('admin.activity.plays', { count: d.plays })}
                              </small>
                            </span>
                          </span>
                        </td>
                        <td>
                          <Score device={d} />
                        </td>
                        <td className="ax-dim">{formatPercent(lng, d.directPlay / d.plays)}</td>
                        <td className={d.transcoded ? 'ax-warn' : 'ax-dim'}>{n(d.transcoded)}</td>
                        <td className={d.codecIssues ? 'ax-warn' : 'ax-dim'}>{n(d.codecIssues)}</td>
                        <td className="ax-dim">{formatRelative(lng, new Date(d.lastAt))}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
          </>
        )}
      </div>
    </>
  )
}

/** A health score as a bar and its number, red, amber or green. */
function Score({ device }: { device: Device }) {
  const { i18n } = useTranslation()
  const tone = device.score < 40 ? 'is-error' : device.score < healthy ? 'is-warning' : 'is-ok'
  return (
    <span className={`ax-score ${tone}`}>
      <span className="ax-bar ax-bar--thin">
        <span style={{ width: `${device.score}%` }} />
      </span>
      <b>{formatNumber(i18n.language, device.score)}</b>
    </span>
  )
}

function placeName(lng: string, p: { city?: string; country: string }): string {
  return p.city ? `${p.city}, ${regionName(lng, p.country)}` : regionName(lng, p.country)
}
