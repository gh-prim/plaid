import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Button, Spinner } from '../ui'
import { formatPosition } from './format'

type Party = components['schemas']['Party']

/** The end of the party (Party16, R21), or being removed from it. */
export function EndSummary({
  partyId,
  removed,
  onLeave,
}: {
  partyId: string
  removed: boolean
  onLeave: () => void
}) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const [party, setParty] = useState<Party | null | undefined>(removed ? null : undefined)

  useEffect(() => {
    if (removed || !partyId) return
    let tries = 0
    // The summary is stored as the room closes: ask again shortly if needed.
    const load = () =>
      void api
        .GET('/parties/{partyId}', { params: { path: { partyId } } })
        .then(({ data }) => {
          if (data?.summary || tries++ > 3) setParty(data ?? null)
          else setTimeout(load, 500)
        })
        .catch(() => setParty(null))
    load()
  }, [partyId, removed])

  if (removed) {
    return (
      <div className="pv-party-center pv-glass pv-party-card">
        <p className="pv-party-title">{t('party.removed')}</p>
        <Button variant="secondary" onClick={onLeave}>
          {t('party.leave')}
        </Button>
      </div>
    )
  }
  if (party === undefined) {
    return (
      <div className="pv-party-center" role="status" aria-label={t('common.loading')}>
        <Spinner size={34} mono />
      </div>
    )
  }
  const s = party?.summary
  const together = s ? duration(i18n.language, s.togetherMs) : ''
  return (
    <div className="pv-party-center pv-glass pv-party-card pv-party-end">
      <p className="pv-party-kicker">{t('party.end.title')}</p>
      {party?.item && <p className="pv-party-display">{party.item.title}</p>}
      {s && (
        <>
          <p className="pv-party-muted">
            {t('party.end.with', {
              names: new Intl.ListFormat(i18n.language, { type: 'conjunction' }).format(s.people),
            })}
          </p>
          <dl className="pv-party-stats">
            <div>
              <dt>{together}</dt>
              <dd>{t('party.end.together')}</dd>
            </div>
            <div>
              <dt>{s.reactions}</dt>
              <dd>{t('party.end.reactions', { count: s.reactions })}</dd>
            </div>
            <div>
              <dt>{s.pauseRequests}</dt>
              <dd>{t('party.end.pauses', { count: s.pauseRequests })}</dd>
            </div>
          </dl>
          {s.peakEmoji && s.peakPositionMs !== undefined && (
            <p>
              {t('party.end.peak', { position: formatPosition(s.peakPositionMs) })} {s.peakEmoji}
            </p>
          )}
        </>
      )}
      <div className="pv-party-stack">
        {party?.item && (
          <Button
            variant="cream"
            onClick={() =>
              void navigate(`/item/${party.item!.showId ?? party.item!.id}`, {
                state: { schedulePartyLike: party.id },
              })
            }
          >
            {t('party.end.next')}
          </Button>
        )}
        <Button variant="ghost" onClick={onLeave}>
          {t('party.end.leave')}
        </Button>
      </div>
    </div>
  )
}

/** 1 h 34 in the reader's language. */
function duration(lng: string, ms: number): string {
  const hours = Math.floor(ms / 3_600_000)
  const minutes = Math.floor((ms % 3_600_000) / 60_000)
  const unit = (n: number, u: 'hour' | 'minute') =>
    new Intl.NumberFormat(lng, { style: 'unit', unit: u, unitDisplay: 'narrow' }).format(n)
  return hours > 0 ? `${unit(hours, 'hour')} ${unit(minutes, 'minute')}` : unit(minutes, 'minute')
}
