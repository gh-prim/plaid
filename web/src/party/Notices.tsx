import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../ui'
import { formatPosition } from './format'
import type { NextUp, Room } from './useParty'

function useNow(serverNow: () => number, every = 500) {
  const [now, setNow] = useState(serverNow)
  useEffect(() => {
    const id = setInterval(() => setNow(serverNow()), every)
    return () => clearInterval(id)
  }, [serverNow, every])
  return now
}

/** The sync lines on top of the picture (Party15): who paused or went
 * back, the gentle catch-up, a pause coming in 5 minutes. */
export function Notices({
  room,
  serverNow,
  refused,
}: {
  room: Room
  serverNow: () => number
  refused: boolean
}) {
  const { t } = useTranslation()
  const now = useNow(serverNow, 1000)
  return (
    <div className="pv-party-notices" aria-live="polite">
      {refused && <p className="pv-glass">{t('party.control.refused')}</p>}
      {room.pauseAtServerMs && room.pauseAtServerMs > now && (
        <p className="pv-glass">
          {t('party.pause.countdown', { time: formatPosition(room.pauseAtServerMs - now) })}
        </p>
      )}
      {room.notices.map((n) => (
        <p key={n.key} className="pv-glass">
          {n.kind === 'caughtUp'
            ? t('party.sync.caughtUp', { seconds: n.seconds })
            : n.kind === 'pauseScheduled'
              ? t('party.pause.scheduled')
              : n.kind === 'seeked'
                ? t('party.sync.seeked', { name: n.name, position: formatPosition(n.positionMs) })
                : t(`party.sync.${n.kind}`, { name: n.name })}
        </p>
      ))}
    </div>
  )
}

/** "Waiting for Sam" (Party15, R11). */
export function WaitingCard({
  room,
  serverNow,
  onResume,
}: {
  room: Room
  serverNow: () => number
  onResume: () => void
}) {
  const { t } = useTranslation()
  const now = useNow(serverNow)
  const w = room.waitingFor
  if (!w || room.playback?.state !== 'waiting') return null
  const names = new Intl.ListFormat(undefined, { type: 'conjunction' }).format(w.names)
  const position = formatPosition(room.playback.positionMs)
  return (
    <div className="pv-party-wait pv-glass" role="status">
      <p className="pv-party-title">{t('party.sync.waiting', { name: names })}</p>
      <p className="pv-party-muted">{t('party.sync.waitingHint', { position })}</p>
      {now >= w.resumeWithoutAtMs && (
        <Button variant="secondary" size="sm" onClick={onResume}>
          {t('party.sync.resumeWithout', { name: names })}
        </Button>
      )}
    </div>
  )
}

/** Next episode together (Party22, R30). */
export function NextCard({
  next,
  serverNow,
  host,
  onStop,
}: {
  next: NextUp
  serverNow: () => number
  host: boolean
  onStop: () => void
}) {
  const { t } = useTranslation()
  const now = useNow(serverNow, 250)
  const seconds = Math.max(0, Math.ceil((next.startsAtServerMs - now) / 1000))
  return (
    <div className="pv-party-next pv-glass" role="status">
      <p className="pv-party-kicker">{t('party.next.title')}</p>
      <p className="pv-party-title">{next.title}</p>
      <p className="pv-party-muted">
        {t('party.next.episode', { season: next.season, episode: next.episode })}
      </p>
      <p className="pv-party-muted">
        {t('party.next.ready', { ready: next.ready, count: next.of })}
      </p>
      <p>{t('party.next.startsIn', { count: seconds })}</p>
      {host && (
        <Button variant="secondary" size="sm" onClick={onStop}>
          {t('party.next.stop')}
        </Button>
      )}
    </div>
  )
}
