import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { InviteDialog } from '../admin/Sharing'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Icon, Spinner, cx } from '../ui'
import { InviteSheet } from './LaunchSheet'
import { PartyAvatar } from './PartyAvatar'

type Party = components['schemas']['Party']
type Person = components['schemas']['PartyPerson']
type Group = components['schemas']['Group']

const HOUR = 3600 * 1000

/** Six days to pick from, starting the day after tomorrow: a vote needs time. */
function pickableDays(): Date[] {
  return Array.from({ length: 6 }, (_, i) => {
    const d = new Date()
    d.setHours(0, 0, 0, 0)
    d.setDate(d.getDate() + 2 + i)
    return d
  })
}

/** Ranking closes 3 hours before the party; proposals a day before that
 * when there's time, else halfway from now, on the hour (R31). */
function defaultDeadlines(at: Date): { proposals: Date; ranking: Date } {
  const ranking = new Date(at.getTime() - 3 * HOUR)
  let proposals = new Date(ranking.getTime() - 24 * HOUR)
  if (proposals.getTime() < Date.now() + HOUR) {
    proposals = new Date((Date.now() + ranking.getTime()) / 2)
    proposals.setMinutes(0, 0, 0)
  }
  return { proposals, ranking }
}

/** What's wrong with the deadlines, if anything (R31). */
function deadlineProblem(proposals: Date, ranking: Date, at: Date): string | null {
  if (ranking.getTime() < Date.now() + HOUR) return 'voteTooSoon'
  if (proposals >= ranking || ranking > at || proposals.getTime() < Date.now()) {
    return 'voteDeadlines'
  }
  return null
}

function localInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/**
 * "The group chooses, not you" (VoteModal board, R31): a scheduled party
 * whose movie the group votes for. A day and a time, the two deadlines on a
 * timeline (each can be changed), the invitees, the remote and the reminder.
 * Alone on the server, the invitees give way to an account invitation link.
 */
export function VoteModal({ onClose }: { onClose: () => void }) {
  const { t, i18n } = useTranslation()
  const [days] = useState(pickableDays)
  const [day, setDay] = useState(3)
  const [time, setTime] = useState('21:00')
  const [edited, setEdited] = useState<{ proposals?: Date; ranking?: Date }>({})
  const [people, setPeople] = useState<Person[] | null>(null)
  const [invite, setInvite] = useState<string[]>([])
  const [groups, setGroups] = useState<Group[] | null>(null)
  const [inviting, setInviting] = useState(false)
  const [mode, setMode] = useState<'host' | 'everyone'>('host')
  const [reminder, setReminder] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [made, setMade] = useState<Party | null>(null)
  const panel = useRef<HTMLDivElement>(null)

  useEffect(() => {
    void api
      .GET('/parties/people')
      .then(({ data }) => {
        setPeople(data ?? [])
        setInvite((data ?? []).map((p) => p.userId))
      })
      .catch(() => setPeople([]))
    // Only admins may invite new accounts: without the groups, no link.
    void api
      .GET('/groups')
      .then(({ data }) => setGroups(data ?? null))
      .catch(() => setGroups(null))
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && !inviting && onClose()
    document.addEventListener('keydown', onKey)
    panel.current?.focus()
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = overflow
    }
  }, [onClose, inviting])

  const [h, m] = time.split(':').map(Number)
  const at = new Date(days[day]!)
  at.setHours(h || 0, m || 0, 0, 0)
  const auto = defaultDeadlines(at)
  const proposals = edited.proposals ?? auto.proposals
  const ranking = edited.ranking ?? auto.ranking

  const pickDay = useCallback((i: number) => {
    setDay(i)
    setEdited({})
  }, [])

  async function create() {
    const problem = deadlineProblem(proposals, ranking, at)
    if (problem) return setError(problem)
    setBusy(true)
    setError(null)
    const { data, error: err } = await api
      .POST('/parties', {
        body: {
          controlMode: mode,
          invite,
          reminder,
          scheduledAt: at.toISOString(),
          vote: { proposalsUntil: proposals.toISOString(), rankingUntil: ranking.toISOString() },
        },
      })
      .catch(() => ({ data: undefined, error: { error: { code: 'network' } } }))
    setBusy(false)
    if (!data) return setError((err as { error?: { code?: string } })?.error?.code ?? 'generic')
    setMade(data)
  }

  if (made) return <InviteSheet party={made} onClose={onClose} onGo={onClose} />

  const weekday = new Intl.DateTimeFormat(i18n.language, { weekday: 'short' })
  const short = (d: Date) =>
    new Intl.DateTimeFormat(i18n.language, {
      weekday: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(d)
  const alone = people !== null && people.length === 0

  return createPortal(
    <div
      className="pl vm-scrim"
      role="presentation"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <div
        ref={panel}
        className="vm"
        role="dialog"
        aria-modal="true"
        aria-labelledby="vm-title"
        tabIndex={-1}
      >
        <div className="vm__head">
          <div className="vm__titles">
            <p className="vm__kicker">
              <Icon name="vote" size={15} />
              {t('party.vote.modal.kicker')}
            </p>
            <h2 id="vm-title" className="vm__title">
              {t('party.vote.modal.title')}
            </h2>
            <p className="vm__hint">{t('party.vote.modal.hint')}</p>
          </div>
          <button
            type="button"
            className="vm__round vm-lg pl-focusable"
            aria-label={t('common.close')}
            onClick={onClose}
          >
            <Icon name="x" size={16} />
          </button>
        </div>

        <p className="vm__label" id="vm-when">
          {t('party.vote.modal.when')}
        </p>
        <div className="vm__days" role="radiogroup" aria-labelledby="vm-when">
          {days.map((d, i) => (
            <button
              key={i}
              type="button"
              role="radio"
              aria-checked={day === i}
              className={cx('vm__day pl-focusable', day === i && 'is-on')}
              onClick={() => pickDay(i)}
            >
              <span className="vm__day-name">{weekday.format(d)}</span>
              <span className="vm__day-n">{d.getDate()}</span>
            </button>
          ))}
          <label className="vm__day vm__time vm-lg">
            <span className="vm__day-name">{t('party.vote.modal.time')}</span>
            <input
              type="time"
              value={time}
              step={900}
              onChange={(e) => {
                setTime(e.target.value || '21:00')
                setEdited({})
              }}
            />
          </label>
        </div>

        <ol className="vm__line">
          <Step
            title={t('party.vote.modal.proposals')}
            when={t('party.vote.modal.until', { date: short(proposals) })}
            value={proposals}
            editLabel={t('party.vote.modal.edit', { step: t('party.vote.modal.proposals') })}
            onEdit={(d) => setEdited((v) => ({ ...v, proposals: d }))}
          />
          <Step
            title={t('party.vote.modal.ranking')}
            when={t('party.vote.modal.until', { date: short(ranking) })}
            value={ranking}
            editLabel={t('party.vote.modal.edit', { step: t('party.vote.modal.ranking') })}
            onEdit={(d) => setEdited((v) => ({ ...v, ranking: d }))}
          />
          <Step title={t('party.title')} when={short(at)} last />
        </ol>

        <p className="vm__label">
          {alone || people === null
            ? t('party.vote.modal.invitees')
            : t('party.vote.modal.inviteesCount', { n: invite.length })}
        </p>
        {people === null ? (
          <Spinner size={18} />
        ) : alone ? (
          <div className="vm__alone">
            <span className="vm__alone-icon vm-lg">
              <Icon name="userPlus" size={20} />
            </span>
            <div className="vm__alone-text">
              <p>{t('party.vote.modal.aloneTitle')}</p>
              <p>{t('party.vote.modal.aloneHint')}</p>
            </div>
            {groups && (
              <button
                type="button"
                className="vm__pill vm-lg pl-focusable"
                onClick={() => setInviting(true)}
              >
                <Icon name="link" size={17} />
                {t('party.vote.modal.createLink')}
              </button>
            )}
          </div>
        ) : (
          <div className="vm__people">
            {people.map((p) => {
              const on = invite.includes(p.userId)
              return (
                <button
                  key={p.userId}
                  type="button"
                  aria-pressed={on}
                  className={cx('vm__person pl-focusable', on && 'is-on')}
                  onClick={() =>
                    setInvite((v) => (on ? v.filter((x) => x !== p.userId) : [...v, p.userId]))
                  }
                >
                  <span className="vm__person-pic">
                    <PartyAvatar name={p.name} color={p.avatarColor} url={p.avatar.url} size={46} />
                    {on && (
                      <span className="vm__person-check">
                        <Icon name="check" size={12} />
                      </span>
                    )}
                  </span>
                  <span className="vm__person-name">{p.name}</span>
                </button>
              )
            })}
            <span className="vm__spacer" />
            {groups && (
              <button
                type="button"
                className="vm__pill vm-lg pl-focusable"
                onClick={() => setInviting(true)}
              >
                <Icon name="link" size={17} />
                {t('party.vote.modal.link')}
              </button>
            )}
          </div>
        )}

        <div className="vm__row">
          <Icon name="remote" size={20} />
          <div className="vm__row-text">
            <p>{t('party.vote.modal.remote')}</p>
            <p>{t('party.start.controlHint')}</p>
          </div>
          <div className="vm__seg" role="radiogroup" aria-label={t('party.vote.modal.remote')}>
            {(['host', 'everyone'] as const).map((v) => (
              <button
                key={v}
                type="button"
                role="radio"
                aria-checked={mode === v}
                className={cx('vm__seg-item pl-focusable', mode === v && 'is-on')}
                onClick={() => setMode(v)}
              >
                {t(`party.control.${v}`)}
              </button>
            ))}
          </div>
        </div>
        <div className="vm__row vm__row--tight">
          <Icon name="bell" size={20} />
          <p className="vm__row-text">{t('party.vote.modal.reminder')}</p>
          <button
            type="button"
            role="switch"
            aria-checked={reminder}
            aria-label={t('party.vote.modal.reminder')}
            className={cx('vm__switch pl-focusable', reminder && 'is-on')}
            onClick={() => setReminder((r) => !r)}
          >
            <span />
          </button>
        </div>

        {error && (
          <p className="pv-party-error" role="alert">
            {t(`party.error.${error}`, t('party.error.generic'))}
          </p>
        )}
        <div className="vm__foot">
          <button type="button" className="vm__btn vm-lg pl-focusable" onClick={onClose}>
            {t('common.cancel')}
          </button>
          <button
            type="button"
            className="vm__btn vm__btn--go vm-lg pl-focusable"
            disabled={busy}
            onClick={() => void create()}
          >
            {busy && <Spinner size={16} mono />}
            {t('party.vote.modal.cta')}
          </button>
        </div>
      </div>
      {inviting && groups && <InviteDialog groups={groups} onClose={() => setInviting(false)} />}
    </div>,
    document.body,
  )
}

/** One stop on the timeline: a dot, a name and a date; the deadlines have a
 * pencil that opens the date picker. */
function Step({
  title,
  when,
  value,
  editLabel,
  onEdit,
  last,
}: {
  title: string
  when: string
  value?: Date
  editLabel?: string
  onEdit?: (d: Date) => void
  last?: boolean
}) {
  const input = useRef<HTMLInputElement>(null)
  return (
    <li className={cx('vm__step', last && 'is-last')}>
      <span className="vm__dot" aria-hidden="true" />
      <p className="vm__step-title">{title}</p>
      {onEdit && value ? (
        <label className="vm__step-when vm__step-edit">
          {when}
          <Icon name="pencil" size={13} />
          <input
            ref={input}
            type="datetime-local"
            aria-label={editLabel}
            value={localInput(value)}
            onClick={() => input.current?.showPicker?.()}
            onChange={(e) => e.target.value && onEdit(new Date(e.target.value))}
          />
        </label>
      ) : (
        <p className="vm__step-when">{when}</p>
      )}
    </li>
  )
}
