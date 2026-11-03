import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { GlassSheet } from '../browse/GlassSheet'
import { Button, ChoiceRow, Icon, SegmentedControl, Spinner } from '../ui'
import { PartySocket } from './socket'

type Party = components['schemas']['Party']
type Person = components['schemas']['PartyPerson']
interface Screen {
  sessionId: string
  name: string
}

/** The Apple TVs of the account with their app open (R22). */
function useScreens(open: boolean) {
  const [screens, setScreens] = useState<Screen[]>([])
  useEffect(() => {
    if (!open) return
    const s = new PartySocket()
    const off = s.on<{ screens: Screen[] }>('party.screens', (d) => setScreens(d.screens))
    s.connect()
    s.send('party.screens')
    return () => {
      off()
      s.close()
    }
  }, [open])
  return screens
}

/** The next quarter hour from now, at least 15 minutes ahead (R28). */
function defaultTime(): Date {
  const d = new Date(Date.now() + 60 * 60 * 1000)
  d.setMinutes(Math.ceil(d.getMinutes() / 15) * 15, 0, 0)
  return d
}

function localInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Tomorrow evening at 21:00: a vote needs a few hours (R31). */
function voteTime(): Date {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  d.setHours(21, 0, 0, 0)
  return d
}

/** Ranking closes 3 hours before the party; proposals a day before that
 * when there's time, else halfway from now, on the hour (R31). */
function voteDeadlines(at: Date): { proposalsUntil: Date; rankingUntil: Date } {
  const hour = 3600 * 1000
  const rankingUntil = new Date(at.getTime() - 3 * hour)
  let proposalsUntil = new Date(rankingUntil.getTime() - 24 * hour)
  if (proposalsUntil.getTime() < Date.now() + hour) {
    proposalsUntil = new Date((Date.now() + rankingUntil.getTime()) / 2)
    proposalsUntil.setMinutes(0, 0, 0)
  }
  return { proposalsUntil, rankingUntil }
}

/**
 * Start or schedule a Plaid Party from a title (Party04, Party05), then
 * share it (Party06). Without a title, the party is scheduled and the
 * group votes for the movie first (VoteBig step 1, R31).
 */
export function LaunchSheet({
  itemId,
  title,
  onClose,
}: {
  itemId?: string
  title?: string
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const vote = !itemId
  const [when, setWhen] = useState<'now' | 'schedule'>(vote ? 'schedule' : 'now')
  const [where, setWhere] = useState('this')
  const [mode, setMode] = useState<'host' | 'everyone'>('host')
  const [at, setAt] = useState(() => localInput(vote ? voteTime() : defaultTime()))
  const [earliest] = useState(() => localInput(new Date(Date.now() + 15 * 60 * 1000)))
  const [people, setPeople] = useState<Person[] | null>(null)
  const [invite, setInvite] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [made, setMade] = useState<Party | null>(null)
  const screens = useScreens(true)

  useEffect(() => {
    void api
      .GET('/parties/people')
      .then(({ data }) => setPeople(data ?? []))
      .catch(() => setPeople([]))
  }, [])

  const deadlines = voteDeadlines(new Date(at))

  async function create() {
    if (vote && deadlines.rankingUntil.getTime() < Date.now() + 3600 * 1000) {
      setError('voteTooSoon')
      return
    }
    setBusy(true)
    setError(null)
    const { data, error: err } = await api
      .POST('/parties', {
        body: {
          itemId,
          controlMode: mode,
          invite,
          ...(when === 'schedule' ? { scheduledAt: new Date(at).toISOString() } : {}),
          ...(vote
            ? {
                vote: {
                  proposalsUntil: deadlines.proposalsUntil.toISOString(),
                  rankingUntil: deadlines.rankingUntil.toISOString(),
                },
              }
            : {}),
        },
      })
      .catch(() => ({ data: undefined, error: { error: { code: 'network' } } }))
    setBusy(false)
    if (!data) {
      setError((err as { error?: { code?: string } })?.error?.code ?? 'generic')
      return
    }
    setMade(data)
  }

  if (made) {
    return (
      <InviteSheet
        party={made}
        screen={where}
        onClose={onClose}
        onGo={() =>
          made.status === 'live'
            ? void navigate(`/party/${made.id}`, { state: { screen: where } })
            : onClose()
        }
      />
    )
  }

  const atLabel = new Intl.DateTimeFormat(i18n.language, {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
  }).format(new Date(at))
  const timeLabel = new Intl.DateTimeFormat(i18n.language, { timeStyle: 'short' }).format(
    new Date(at),
  )
  const deadline = (d: Date) =>
    new Intl.DateTimeFormat(i18n.language, {
      weekday: 'long',
      hour: '2-digit',
      minute: '2-digit',
    }).format(d)
  return (
    <GlassSheet
      title={vote ? t('party.vote.launch.title') : t('party.title')}
      subtitle={vote ? t('party.vote.launch.hint') : title}
      onClose={onClose}
      footer={
        <Button variant="cream" block disabled={busy} onClick={() => void create()}>
          {busy ? <Spinner size={16} /> : <Icon name="play" size={15} />}
          {vote
            ? t('party.vote.launch.cta')
            : when === 'now'
              ? t('party.start.cta')
              : t('party.schedule.cta', { date: atLabel, time: timeLabel })}
        </Button>
      }
    >
      {!vote && (
        <SegmentedControl
          label={t('party.start.whenLabel')}
          value={when}
          options={[
            { value: 'now', label: t('party.start.now') },
            { value: 'schedule', label: t('party.start.schedule') },
          ]}
          onChange={setWhen}
        />
      )}
      {when === 'schedule' && (
        <label className="pl-field">
          <span className="pl-field__label">{t('party.schedule.when')}</span>
          <input
            className="pl-field__input"
            type="datetime-local"
            value={at}
            min={earliest}
            onChange={(e) => setAt(e.target.value)}
          />
        </label>
      )}
      {vote && (
        <ol className="pv-vote-steps">
          <li>
            <span className="pv-party-kicker">{t('party.vote.launch.step1')}</span>
            <strong>{t('party.vote.launch.step1Title')}</strong>
            <span className="pv-party-muted">
              {t('party.vote.launch.until', { until: deadline(deadlines.proposalsUntil) })}
            </span>
          </li>
          <li>
            <span className="pv-party-kicker">{t('party.vote.launch.step2')}</span>
            <strong>{t('party.vote.launch.step2Title')}</strong>
            <span className="pv-party-muted">
              {t('party.vote.launch.until', { until: deadline(deadlines.rankingUntil) })}
            </span>
          </li>
        </ol>
      )}
      {vote && <p className="pv-party-muted">{t('party.vote.launch.fair')}</p>}
      {when === 'now' && (
        <>
          <p className="pv-party-label">{t('party.start.whereTitle')}</p>
          <div role="radiogroup" aria-label={t('party.start.whereTitle')}>
            {screens.map((s) => (
              <ChoiceRow
                key={s.sessionId}
                name="party-screen"
                checked={where === s.sessionId}
                label={t('party.start.tv', { name: s.name })}
                onChange={() => setWhere(s.sessionId)}
              />
            ))}
            <ChoiceRow
              name="party-screen"
              checked={where === 'this'}
              label={t('party.start.thisBrowser')}
              onChange={() => setWhere('this')}
            />
          </div>
          <p className="pv-party-muted">
            {where === 'this' ? t('party.start.thisDevice') : t('party.start.tvHint')}
          </p>
        </>
      )}
      <SegmentedControl
        label={t('party.start.controlTitle')}
        value={mode}
        options={[
          { value: 'host', label: t('party.control.host') },
          { value: 'everyone', label: t('party.control.everyone') },
        ]}
        onChange={setMode}
      />
      <p className="pv-party-muted">{t('party.start.controlHint')}</p>
      <p className="pv-party-label">{t('party.invite.people')}</p>
      {people === null ? (
        <Spinner size={18} />
      ) : people.length === 0 ? (
        <p className="pv-party-muted">{t('party.invite.nobody')}</p>
      ) : (
        <div className="pv-party-invitees">
          {people.map((p) => (
            <label
              key={p.userId}
              className={`pl-check${invite.includes(p.userId) ? ' is-checked' : ''}`}
            >
              <input
                type="checkbox"
                checked={invite.includes(p.userId)}
                onChange={(e) =>
                  setInvite((v) =>
                    e.target.checked ? [...v, p.userId] : v.filter((x) => x !== p.userId),
                  )
                }
              />
              <span className="pl-check__box" aria-hidden="true">
                {invite.includes(p.userId) && <Icon name="check" size={14} />}
              </span>
              {p.name}
            </label>
          ))}
        </div>
      )}
      {when === 'schedule' && <p className="pv-party-muted">{t('party.schedule.reminder')}</p>}
      {error && (
        <p className="pv-party-error" role="alert">
          {t(`party.error.${error}`, t('party.error.generic'))}
        </p>
      )}
    </GlassSheet>
  )
}

/** "Your party is ready." with the code and the link (Party06). */
export function InviteSheet({
  party,
  screen,
  onClose,
  onGo,
}: {
  party: Party
  screen?: string
  onClose: () => void
  onGo: () => void
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  async function share() {
    if (navigator.share) {
      await navigator.share({ title: t('party.title'), url: party.link }).catch(() => {})
      return
    }
    await navigator.clipboard?.writeText(party.link).catch(() => {})
    setCopied(true)
  }
  void screen
  return (
    <GlassSheet
      title={party.status === 'live' ? t('party.invite.ready') : t('party.schedule.done')}
      subtitle={t('party.invite.hint')}
      onClose={onClose}
      footer={
        <Button variant="cream" block onClick={onGo}>
          {party.status === 'live' ? t('party.invite.go') : t('common.close')}
        </Button>
      }
    >
      <p className="pv-party-code" aria-label={t('party.invite.codeLabel')}>
        {party.code}
      </p>
      <div className="pv-party-stack">
        <Button
          variant="secondary"
          onClick={() => void navigator.clipboard?.writeText(party.code).catch(() => {})}
        >
          <Icon name="copy" size={16} />
          {t('party.invite.copy')}
        </Button>
        <Button variant="secondary" onClick={() => void share()}>
          {copied ? t('party.invite.copied') : t('party.invite.share')}
        </Button>
      </div>
      <p className="pv-party-muted">{t('party.invite.valid')}</p>
    </GlassSheet>
  )
}
