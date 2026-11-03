import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Icon } from '../ui'
import { formatPosition } from './format'
import { PartyAvatar } from './PartyAvatar'
import { canControl, type ChatLine, type PartyApi } from './useParty'

const reactions = ['😂', '😱', '❤️', '👏', '🦇', '🍿']

/**
 * The panel beside the picture (Party12, Party13): who's here and how
 * they watch, the pause requests, the chat, the six reactions.
 */
export function PartyPanel({
  party,
  code,
  title,
  onPeople,
  onClose,
}: {
  party: PartyApi
  code?: string
  title?: string
  onPeople: () => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { room, send } = party
  const [text, setText] = useState('')
  const [asked, setAsked] = useState(false)
  const list = useRef<HTMLOListElement>(null)
  const isHost = room.me === room.host
  const control = canControl(room)

  useEffect(() => {
    list.current?.lastElementChild?.scrollIntoView?.({ block: 'nearest' })
  }, [room.chat.length])

  function submit(e: FormEvent) {
    e.preventDefault()
    const v = text.trim()
    if (!v) return
    void send('party.chat', { text: v }).then(
      () => setText(''),
      () => {},
    )
  }

  function askPause() {
    void send('party.pause.request').then(
      () => {
        setAsked(true)
        setTimeout(() => setAsked(false), 60_000)
      },
      () => {},
    )
  }

  return (
    <aside className="pv-party-panel pv-glass" aria-label={t('party.title')}>
      <header className="pv-party-panel__head">
        <div>
          <p className="pv-party-kicker">{t('party.inBrowser')}</p>
          {title && <p className="pv-party-title">{title}</p>}
        </div>
        <Button
          variant="icon"
          icon="x"
          size="sm"
          aria-label={t('common.close')}
          onClick={onClose}
        />
      </header>
      <div className="pv-party-panel__actions">
        {code && (
          <button
            type="button"
            className="pl-chip pl-focusable"
            onClick={() => void navigator.clipboard?.writeText(code).catch(() => {})}
          >
            {t('party.invite.withCode', { code })}
          </button>
        )}
        {!control && (
          <Button variant="secondary" size="sm" disabled={asked} onClick={askPause}>
            {asked ? t('party.pause.asked') : t('party.pause.request')}
          </Button>
        )}
        {isHost && (
          <Button variant="ghost" size="sm" onClick={onPeople}>
            {t('party.host.manage')}
          </Button>
        )}
      </div>

      <ul className="pv-party-people">
        {room.people.map((p) => (
          <li key={p.id} className={p.state === 'away' ? 'is-away' : undefined}>
            <PartyAvatar name={p.name} color={p.avatarColor} url={p.avatarUrl} host={p.host} />
            <span className="pv-party-people__name">
              {p.name}
              <small>
                {p.id === room.me ? t('party.people.you') : p.guest ? t('party.people.guest') : ''}
                {p.host ? ` · ${t('party.people.host')}` : ''}
              </small>
            </span>
            <span className="pv-party-people__how">
              {p.state === 'loading' || p.state === 'waiting'
                ? t('party.people.loading')
                : p.state === 'away'
                  ? t('party.people.away')
                  : [p.screen && t(`party.screen.${p.screen}`, p.screen), p.quality]
                      .filter(Boolean)
                      .join(' · ')}
            </span>
          </li>
        ))}
      </ul>

      {isHost &&
        room.asks.map((a) => (
          <div key={a.requestId} className="pv-party-ask" role="alert">
            <p>
              <strong>{t('party.pause.requested', { name: a.name })}</strong>
              <span>{t('party.pause.hostHint')}</span>
            </p>
            <div>
              <Button
                size="sm"
                onClick={() =>
                  void send('party.pause.answer', { requestId: a.requestId, answer: 'now' }).catch(
                    () => {},
                  )
                }
              >
                {t('party.pause.now')}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() =>
                  void send('party.pause.answer', { requestId: a.requestId, answer: 'in5' }).catch(
                    () => {},
                  )
                }
              >
                {t('party.pause.in5')}
              </Button>
            </div>
          </div>
        ))}

      <h2 className="pv-party-panel__h">{t('party.chat.title')}</h2>
      <ol ref={list} className="pv-party-chat" aria-live="polite">
        {room.chat.map((m) => (
          <Line key={m.id} m={m} me={room.me} />
        ))}
      </ol>
      <div className="pv-party-react" role="group" aria-label={t('party.react.label')}>
        {reactions.map((e) => (
          <button
            key={e}
            type="button"
            className="pl-focusable"
            aria-label={t('party.react.send', { emoji: e })}
            onClick={() => void send('party.react', { emoji: e }).catch(() => {})}
          >
            {e}
          </button>
        ))}
      </div>
      <form className="pv-party-compose" onSubmit={submit}>
        <input
          className="pv-party-input"
          value={text}
          maxLength={500}
          placeholder={t('party.chat.placeholder')}
          aria-label={t('party.chat.placeholder')}
          onChange={(e) => setText(e.target.value)}
        />
        <button
          type="submit"
          className="pv-party-send pl-focusable"
          aria-label={t('party.chat.send')}
          disabled={!text.trim()}
        >
          <Icon name="arrowUp" size={20} />
        </button>
      </form>
    </aside>
  )
}

function Line({ m, me }: { m: ChatLine; me: string }) {
  const { t } = useTranslation()
  if (m.system) {
    const v = m.values ?? {}
    const device = v.device ? t(`party.device.${v.device}`, v.device) : ''
    return (
      <li className="pv-party-chat__system">
        {t(`party.chat.system.${m.system}`, { ...v, device, defaultValue: '' })}
      </li>
    )
  }
  const mine = m.from === me
  return (
    <li className={mine ? 'pv-party-chat__mine' : 'pv-party-chat__theirs'}>
      {!mine && (
        <span className="pv-party-chat__who">
          {m.name} · {formatPosition(m.positionMs)}
        </span>
      )}
      <span className="pv-party-chat__bubble">{m.text}</span>
    </li>
  )
}
