import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Playback, type PartyBridge } from '../player/Player'
import '../player/player.css'
import { Button, Spinner } from '../ui'
import { EndSummary } from './EndSummary'
import { HostSheet, LeaveSheet, LimitSheet } from './HostSheets'
import { PartyPanel } from './PartyPanel'
import { FloatingReactions, Notices, NextCard, WaitingCard } from './Notices'
import './party.css'
import { targetAt } from './sync'
import { useParty, type JoinTarget } from './useParty'

type Quality = components['schemas']['Quality']
type Party = components['schemas']['Party']

/** `/party/:partyId` for members (F010, Party13). */
export default function PartyRoute() {
  const { partyId = '' } = useParams()
  const navigate = useNavigate()
  // The launch sheet may send the party to an Apple TV (R22).
  const screen = (useLocation().state as { screen?: string } | null)?.screen
  const target = useMemo(() => ({ partyId, ...(screen ? { screen } : {}) }), [partyId, screen])
  return <PartyPlayer target={target} onLeave={() => void navigate('/', { replace: true })} />
}

/**
 * The party in the browser (Party13): the player with the room over it, the
 * panel with people and the chat beside it, the sync lines on top (Party15).
 */
export function PartyPlayer({ target, onLeave }: { target: JoinTarget; onLeave: () => void }) {
  const { t } = useTranslation()
  const shell = useRef<HTMLDivElement>(null)
  const party = useParty(target)
  const { room, send, attach, report, serverNow, clearLimit } = party
  const [party$, setParty] = useState<Party | null>(null)
  const [sheet, setSheet] = useState<'people' | 'leave' | null>(null)
  const [refused, setRefused] = useState(false)
  const [panel, setPanel] = useState(true)
  const partyId = target.partyId || room.partyId

  // The party's code, title and summary come over REST.
  const partyIdKnown = target.partyId || room.partyId
  useEffect(() => {
    if (!partyIdKnown) return
    void api
      .GET('/parties/{partyId}', { params: { path: { partyId: partyIdKnown } } })
      .then(({ data }) => data && setParty(data))
      .catch(() => {})
  }, [partyIdKnown, room.itemId])

  // A transcode choice of the host (R26): restart with the shared quality,
  // or retry when it's our turn.
  const quality = (room.quality ?? undefined) as Quality | undefined

  const control = (action: 'play' | 'pause' | 'seek', positionMs?: number) => {
    send('party.control', { action, positionMs }).catch(() => {
      setRefused(true)
      setTimeout(() => setRefused(false), 3000)
    })
  }

  if (room.status.k === 'ended' || room.status.k === 'removed') {
    return (
      <div className="pl pv-party-page" data-theme="dark">
        <EndSummary partyId={partyId} removed={room.status.k === 'removed'} onLeave={onLeave} />
      </div>
    )
  }
  if (room.status.k === 'failed') {
    return (
      <div className="pl pv-party-page" data-theme="dark">
        <div className="pv-party-center">
          <p className="pv-party-title">
            {t(`party.error.${room.status.code}`, t('party.error.generic'))}
          </p>
          <Button variant="secondary" onClick={onLeave}>
            {t('party.leave')}
          </Button>
        </div>
      </div>
    )
  }
  if (room.status.k === 'queued') {
    return (
      <div className="pl pv-party-page" data-theme="dark">
        <div className="pv-party-center pv-glass pv-party-card">
          <p className="pv-party-title">{t('party.full.title')}</p>
          <p className="pv-party-muted">{t('party.full.hint', { count: room.max })}</p>
          <p className="pv-party-muted">
            {t('party.full.queue', { position: room.status.position })}
          </p>
          <Button
            variant="secondary"
            onClick={() => {
              void send('party.queue.leave').catch(() => {})
              onLeave()
            }}
          >
            {t('party.full.leave')}
          </Button>
        </div>
      </div>
    )
  }
  if (room.status.k === 'connecting' || !room.itemId) {
    return (
      <div className="pl pv-party-page" data-theme="dark">
        <div className="pv-party-center" role="status" aria-label={t('common.loading')}>
          <Spinner size={34} mono />
        </div>
      </div>
    )
  }

  // The film plays on an Apple TV: this browser is the companion (Party01).
  if (target.screen && target.screen !== 'this') {
    const pb = room.playback
    const at = pb ? targetAt(pb, serverNow()) : 0
    return (
      <div className="pl pv-party-page pv-party-companion" data-theme="dark">
        <div className="pv-party-remote pv-glass">
          <p className="pv-party-kicker">{t('party.companion.onTv')}</p>
          {party$?.item && <p className="pv-party-title">{party$.item.title}</p>}
          <div className="pv-party-remote__buttons">
            <Button
              variant="icon"
              aria-label={t('party.companion.back10')}
              onClick={() => control('seek', Math.max(0, at - 10_000))}
            >
              {t('party.companion.back')}
            </Button>
            <Button
              variant="play"
              icon={pb?.state === 'playing' ? 'pause' : 'play'}
              aria-label={pb?.state === 'playing' ? t('player.pause') : t('player.play')}
              onClick={() => control(pb?.state === 'playing' ? 'pause' : 'play', at)}
            />
            <Button
              variant="icon"
              aria-label={t('party.companion.forward10')}
              onClick={() => control('seek', at + 10_000)}
            >
              {t('party.companion.forward')}
            </Button>
          </div>
        </div>
        <PartyPanel
          party={party}
          code={party$?.code}
          title={party$?.item?.title}
          onPeople={() => setSheet('people')}
          onClose={() => setSheet('leave')}
        />
        {sheet === 'people' && <HostSheet party={party} onClose={() => setSheet(null)} />}
        {sheet === 'leave' && (
          <LeaveSheet
            party={party}
            onLeave={() => {
              void send('party.leave').catch(() => {})
              onLeave()
            }}
            onClose={() => setSheet(null)}
          />
        )}
        {room.limit && room.me === room.host && (
          <LimitSheet party={party} limit={room.limit} onClose={clearLimit} />
        )}
      </div>
    )
  }

  const bridge: PartyBridge = {
    partyId,
    startMs: () => (room.playback ? targetAt(room.playback, serverNow()) : 0),
    control,
    attach,
    blocked: (q) => report('blocked', q ? { quality: q } : {}),
    credits: () => report('credits'),
    quality,
    onBack: () => setSheet('leave'),
    overlay: (
      <>
        <Notices room={room} serverNow={serverNow} refused={refused} />
        <FloatingReactions room={room} />
        <WaitingCard
          room={room}
          serverNow={serverNow}
          onResume={() => void send('party.resume.without').catch(() => {})}
        />
        {room.next && (
          <NextCard
            next={room.next}
            serverNow={serverNow}
            host={room.me === room.host}
            onStop={() => void send('party.next.stop').catch(() => {})}
          />
        )}
        {sheet === 'people' && <HostSheet party={party} onClose={() => setSheet(null)} />}
        {sheet === 'leave' && (
          <LeaveSheet
            party={party}
            onLeave={() => {
              void send('party.leave').catch(() => {})
              onLeave()
            }}
            onClose={() => setSheet(null)}
          />
        )}
        {room.limit && room.me === room.host && (
          <LimitSheet party={party} limit={room.limit} onClose={clearLimit} />
        )}
      </>
    ),
  }

  return (
    <div
      ref={shell}
      className="pl pl-player pv-player pv-party"
      data-theme="dark"
      data-panel={panel || undefined}
    >
      <Playback
        key={`${room.itemId}|${quality ?? ''}|${room.retries}`}
        itemId={room.itemId}
        startMs={null}
        autoplayed={0}
        shell={shell}
        onBack={() => setSheet('leave')}
        party={bridge}
      />
      <button
        type="button"
        className={`pv-party-toggle pv-glass pl-focusable${panel ? ' is-open' : ''}`}
        aria-expanded={panel}
        onClick={() => setPanel((v) => !v)}
      >
        {t('party.pill', { count: room.people.length })}
      </button>
      {panel && (
        <PartyPanel
          party={party}
          code={party$?.code}
          title={party$?.item?.title}
          onPeople={() => setSheet('people')}
          onClose={() => setPanel(false)}
        />
      )}
    </div>
  )
}
