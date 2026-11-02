import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { serverUrl } from '../api/server'
import { useNotify } from './notify'
import './party.css'

type Party = components['schemas']['Party']

/** The home's "Plaid Parties" row (Party21, R40): live parties to join and
 * the next scheduled ones; nothing when there are none. */
export function PartiesRow() {
  const { t, i18n } = useTranslation()
  const [parties, setParties] = useState<Party[]>([])
  const tick = useNotify()
  useEffect(() => {
    void api
      .GET('/parties')
      .then(({ data }) =>
        setParties((data ?? []).filter((p) => p.status !== 'voting').slice(0, 10)),
      )
      .catch(() => {})
  }, [tick])
  if (parties.length === 0) return null
  const day = new Intl.DateTimeFormat(i18n.language, {
    weekday: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
  return (
    <section className="br-row pv-parties-row" aria-label={t('party.row.title')}>
      <h2 className="br-row__title">{t('party.row.title')}</h2>
      <div className="br-row__list">
        {parties.map((p) => (
          <Link
            key={p.id}
            to={p.status === 'live' ? `/party/${p.id}` : `/upcoming?party=${p.id}`}
            className="pv-party-tile pl-focusable"
          >
            {p.item && (
              <img
                src={serverUrl(
                  `/api/v1/items/${p.item.showId ?? p.item.id}/artwork/backdrop?w=640`,
                )}
                alt=""
                loading="lazy"
              />
            )}
            <span className="pv-party-tile__tag">
              {p.status === 'live'
                ? t('party.row.live')
                : p.scheduledAt
                  ? day.format(new Date(p.scheduledAt))
                  : ''}
            </span>
            <span className="pv-party-tile__title">{p.item?.title ?? t('party.vote.pending')}</span>
            <span className="pv-party-tile__line">
              {p.status === 'live'
                ? t('party.row.join', {
                    name: p.organizer.name,
                    count: Math.max(0, (p.live?.count ?? 1) - 1),
                  })
                : t('party.row.by', { name: p.organizer.name })}
            </span>
          </Link>
        ))}
      </div>
    </section>
  )
}
