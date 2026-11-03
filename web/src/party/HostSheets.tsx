import { useTranslation } from 'react-i18next'
import { GlassSheet } from '../browse/GlassSheet'
import { Button, SegmentedControl } from '../ui'
import { PartyAvatar } from './PartyAvatar'
import type { PartyApi, TranscodeLimit } from './useParty'

/** The host manages the people (Party17, R14, R16). */
export function HostSheet({ party, onClose }: { party: PartyApi; onClose: () => void }) {
  const { t } = useTranslation()
  const { room, send } = party
  const act = (type: string, data: unknown) => void send(type, data).catch(() => {})
  return (
    <GlassSheet
      title={t('party.host.title')}
      subtitle={t('party.host.subtitle', { count: room.people.length })}
      onClose={onClose}
    >
      <SegmentedControl
        label={t('party.start.controlTitle')}
        value={room.mode}
        options={[
          { value: 'host', label: t('party.control.host') },
          { value: 'everyone', label: t('party.control.everyone') },
        ]}
        onChange={(mode) => act('party.mode.set', { mode })}
      />
      <ul className="pv-party-manage">
        {room.people
          .filter((p) => p.id !== room.me)
          .map((p) => (
            <li key={p.id}>
              <PartyAvatar name={p.name} color={p.avatarColor} url={p.avatarUrl} />
              <span className="pv-party-people__name">
                {p.name}
                <small>
                  {[
                    p.guest && t('party.people.guest'),
                    p.screen && t(`party.screen.${p.screen}`, p.screen),
                    p.quality,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </small>
              </span>
              <span className="pv-party-manage__acts">
                {!p.guest && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => act('party.host.set', { to: p.id })}
                  >
                    {t('party.host.handTo', { name: p.name })}
                  </Button>
                )}
                {room.mode === 'host' && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => act('party.control.grant', { to: p.id, on: !p.canControl })}
                  >
                    {p.canControl ? t('party.host.takeControl') : t('party.host.giveControl')}
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => act('party.remove', { who: p.id })}
                >
                  {t('party.host.remove')}
                </Button>
              </span>
            </li>
          ))}
      </ul>
    </GlassSheet>
  )
}

/** Leaving (Party18, R17): the host picks who takes over or ends it. */
export function LeaveSheet({
  party,
  onLeave,
  onClose,
}: {
  party: PartyApi
  onLeave: () => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { room, send } = party
  const isHost = room.me === room.host
  const next = room.people.find((p) => p.id !== room.me && !p.guest && p.state !== 'away')
  if (!isHost) {
    return (
      <GlassSheet title={t('party.leaveTitle')} onClose={onClose}>
        <div className="pv-party-stack">
          <Button onClick={onLeave}>{t('party.leave')}</Button>
          <Button variant="secondary" onClick={onClose}>
            {t('party.stay')}
          </Button>
        </div>
      </GlassSheet>
    )
  }
  return (
    <GlassSheet
      title={t('party.host.leaveTitle')}
      subtitle={t('party.host.leaveHint')}
      onClose={onClose}
    >
      <div className="pv-party-stack">
        {next && (
          <Button
            onClick={() => void send('party.host.set', { to: next.id }).then(onLeave, onLeave)}
          >
            {t('party.host.handTo', { name: next.name })}
          </Button>
        )}
        <Button variant="secondary" onClick={onClose}>
          {t('party.host.pickOther')}
        </Button>
        <Button variant="ghost" onClick={() => void send('party.end').then(onLeave, onLeave)}>
          {t('party.host.endAll')}
        </Button>
      </div>
    </GlassSheet>
  )
}

/** The server can't convert for everyone (Party19, R26). */
export function LimitSheet({
  party,
  limit,
  onClose,
}: {
  party: PartyApi
  limit: TranscodeLimit
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { send } = party
  const names = new Intl.ListFormat(undefined, { type: 'conjunction' }).format(
    limit.waiting.map((w) => w.name),
  )
  const [first, second] = limit.waiting
  const choose = (data: unknown) => void send('party.transcode.choose', data).finally(onClose)
  return (
    <GlassSheet
      title={t('party.limit.title')}
      subtitle={t('party.limit.hint', { names, count: 1 })}
      onClose={onClose}
    >
      <div className="pv-party-stack">
        <Button onClick={() => choose({ choice: 'shared', quality: limit.sharedQuality })}>
          {t('party.limit.shared', { quality: limit.sharedQuality, count: limit.waiting.length })}
        </Button>
        {first && second && (
          <Button variant="secondary" onClick={() => choose({ choice: 'order', first: first.id })}>
            {t('party.limit.order', { name: first.name, other: second.name })}
          </Button>
        )}
        <Button variant="ghost" onClick={() => choose({ choice: 'cancel' })}>
          {t('common.cancel')}
        </Button>
      </div>
    </GlassSheet>
  )
}
