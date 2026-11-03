import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { GlassSheet } from '../browse/GlassSheet'
import { Img } from '../browse/Img'
import { imageSrc } from '../browse/media'
import { formatDuration } from '../lib/format'
import { Button, Icon, Spinner, cx } from '../ui'
import type { IconName } from '../ui/Icon'
import '../upcoming/upcoming.css'
import { InviteSheet } from './LaunchSheet'
import { useNotify } from './notify'
import { PartyAvatar } from './PartyAvatar'
import './party.css'

type Party = components['schemas']['Party']
type Vote = components['schemas']['PartyVote']
type Invite = components['schemas']['PartyInvite']
type Person = components['schemas']['PartyPerson']

const HOUR = 3_600_000
const DAY = 24 * HOUR

/** "dans 5 jours", or "dans 2 h 14" on the day. */
function untilText(lang: string, at: Date, from: Date, inDuration: (d: string) => string): string {
  const ms = at.getTime() - from.getTime()
  if (ms >= DAY) {
    return new Intl.RelativeTimeFormat(lang, { numeric: 'auto' }).format(
      Math.round(ms / DAY),
      'day',
    )
  }
  return inDuration(formatDuration(lang, Math.max(ms, 60_000)))
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

/** What the time of day says right now; kept out of render for purity. */
function now(): Date {
  return new Date()
}

/**
 * The host's party page (PartyHost board): when, where the vote stands,
 * the settings and who comes, with a nudge for those who haven't answered,
 * proposed or voted.
 */
export function PartyHostPage() {
  const { partyId = '' } = useParams()
  const { t } = useTranslation()
  const tick = useNotify()
  const [party, setParty] = useState<Party | null | undefined>(undefined)
  const [vote, setVote] = useState<Vote | null>(null)
  const [ambient, setAmbient] = useState<string>()

  useEffect(() => {
    let alive = true
    void api
      .GET('/parties/{partyId}', { params: { path: { partyId } } })
      .then(({ data }) => {
        if (!alive) return
        setParty(data ?? null)
        if (data?.vote) {
          void api
            .GET('/parties/{partyId}/vote', { params: { path: { partyId } } })
            .then(({ data: v }) => alive && setVote(v ?? null))
            .catch(() => {})
        }
      })
      .catch(() => alive && setParty(null))
    return () => {
      alive = false
    }
  }, [partyId, tick])
  useEffect(() => {
    void api
      .GET('/home')
      .then(({ data }) => setAmbient(data?.ambient?.backdrop?.imageId))
      .catch(() => {})
  }, [])

  if (party === undefined) {
    return (
      <section className="br-page ph-page" aria-busy="true">
        <Spinner size={28} />
      </section>
    )
  }
  if (party === null) {
    return (
      <section className="br-page ph-page">
        <p>{t('party.error.party_not_found')}</p>
      </section>
    )
  }
  return (
    <HostView party={party} vote={vote} ambient={ambient} onParty={setParty} onVote={setVote} />
  )
}

function HostView({
  party,
  vote,
  ambient,
  onParty,
  onVote,
}: {
  party: Party
  vote: Vote | null
  ambient?: string
  onParty: (p: Party) => void
  onVote: (v: Vote) => void
}) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const [sheet, setSheet] = useState<'share' | 'edit' | 'invite' | null>(null)
  const [menu, setMenu] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const at = party.scheduledAt ? new Date(party.scheduledAt) : null
  const item = party.item
  const lang = i18n.language

  const fail = (err: unknown) =>
    setError((err as { error?: { code?: string } })?.error?.code ?? 'generic')

  async function patch(body: components['schemas']['PartyUpdate']) {
    const { data, error: err } = await api.PATCH('/parties/{partyId}', {
      params: { path: { partyId: party.id } },
      body,
    })
    if (data) onParty(data)
    else fail(err)
  }
  async function cancel() {
    await api.DELETE('/parties/{partyId}', { params: { path: { partyId: party.id } } })
    void navigate('/upcoming')
  }

  const date = at
    ? capitalize(
        new Intl.DateTimeFormat(lang, {
          weekday: 'long',
          day: 'numeric',
          month: 'long',
          hour: '2-digit',
          minute: '2-digit',
        }).format(at),
      )
    : ''
  const until = at
    ? untilText(lang, at, now(), (d) => t('party.host.inDuration', { duration: d }))
    : ''
  const sub = item
    ? [date, until, item.runtimeMs ? formatDuration(lang, item.runtimeMs) : undefined]
        .filter(Boolean)
        .join(' · ')
    : [capitalize(until), t('party.host.groupPicks')].filter(Boolean).join(' · ')

  // The lit band: the title's backdrop once chosen, else the proposals'
  // posters, else the home's.
  const posters = (vote?.proposals ?? []).map((p) => p.item.posterId).filter(Boolean) as string[]
  const tiles = posters.length
    ? Array.from({ length: 6 }, (_, i) => posters[i % posters.length]!)
    : []

  return (
    <section className="br-page ph-page">
      {item?.backdropId ? (
        <div className="ph-hero" aria-hidden="true">
          <Img src={imageSrc(item.backdropId, 1920)} alt="" decoding="async" />
        </div>
      ) : tiles.length ? (
        <div className="ph-light ph-light--grid" aria-hidden="true">
          {tiles.map((id, i) => (
            <Img key={i} src={imageSrc(id, 342)} alt="" decoding="async" />
          ))}
        </div>
      ) : (
        ambient && (
          <div className="ph-light" aria-hidden="true">
            <Img src={imageSrc(ambient, 1280)} alt="" decoding="async" />
          </div>
        )
      )}

      <Link to="/upcoming" className="vp-back vm-lg pl-focusable">
        <Icon name="chevronLeft" size={16} />
        {t('upcoming.title')}
      </Link>

      <p className="ph-kicker">
        <Icon name="users" size={17} />
        {t('party.host.kicker')}
      </p>
      <h1 className={cx('ph-title', item && 'ph-title--item')}>{item ? item.title : date}</h1>
      <p className="ph-sub">{sub}</p>

      <div className="ph-acts">
        {item && party.status !== 'voting' && (
          <button
            type="button"
            className="ph-btn ph-btn--lit vm-lg pl-focusable"
            onClick={() => void navigate(`/party/${party.id}`)}
          >
            <Icon name="play" size={15} />
            {t('party.host.openLobby')}
          </button>
        )}
        <button
          type="button"
          className="ph-btn vm-lg pl-focusable"
          onClick={() => setSheet('share')}
        >
          <Icon name="link" size={17} />
          {t('party.host.share')}
        </button>
        {!item && (
          <button
            type="button"
            className="ph-btn vm-lg pl-focusable"
            onClick={() => setSheet('edit')}
          >
            <Icon name="pencil" size={16} />
            {t('party.host.edit')}
          </button>
        )}
        <span className="ph-more">
          <button
            type="button"
            className="ph-btn ph-btn--icon vm-lg pl-focusable"
            aria-label={t('party.host.more')}
            aria-expanded={menu}
            onClick={() => setMenu((m) => !m)}
          >
            <Icon name="more" size={20} />
          </button>
          {menu && (
            <span className="ph-menu vm-lg" role="menu">
              {item && (
                <button
                  type="button"
                  role="menuitem"
                  className="pl-focusable"
                  onClick={() => {
                    setMenu(false)
                    setSheet('edit')
                  }}
                >
                  <Icon name="pencil" size={15} />
                  {t('party.host.edit')}
                </button>
              )}
              <button
                type="button"
                role="menuitem"
                className="pl-focusable"
                onClick={() => void cancel()}
              >
                <Icon name="trash" size={15} />
                {t('upcoming.party.cancel')}
              </button>
            </span>
          )}
        </span>
      </div>

      {party.vote && <Timeline party={party} vote={vote} />}

      {error && (
        <p className="pv-party-error" role="alert">
          {t(`party.error.${error}`, t('party.error.generic'))}
        </p>
      )}

      <div className="ph-grid">
        <div className="ph-col">
          {vote && <VoteCard party={party} vote={vote} onVote={onVote} onError={fail} />}
          <div className="ph-card">
            <h3 className="ph-card__title">{t('party.host.settings')}</h3>
            <SettingRow
              icon="remote"
              label={t('party.host.remote')}
              value={t(`party.control.${party.controlMode}`)}
              onClick={() =>
                void patch({ controlMode: party.controlMode === 'host' ? 'everyone' : 'host' })
              }
            />
            {party.reminder !== undefined && (
              <SettingRow
                icon="bell"
                label={t('party.host.reminder')}
                value={party.reminder ? t('party.host.reminderOn') : t('party.host.reminderOff')}
                onClick={() => void patch({ reminder: !party.reminder })}
              />
            )}
            {party.vote && (
              <SettingRow
                icon="vote"
                label={t('party.host.vote')}
                value={t('party.host.voteRules')}
                onClick={() => void navigate(`/party/${party.id}/vote`)}
              />
            )}
            <button type="button" className="ph-cancel pl-focusable" onClick={() => void cancel()}>
              <Icon name="trash" size={16} />
              {t('upcoming.party.cancel')}
              <span>{t('party.host.cancelHint')}</span>
            </button>
          </div>
        </div>
        <Guests party={party} vote={vote} onInvite={() => setSheet('invite')} onError={fail} />
      </div>

      {sheet === 'share' && (
        <InviteSheet party={party} onClose={() => setSheet(null)} onGo={() => setSheet(null)} />
      )}
      {sheet === 'edit' && (
        <EditSheet
          party={party}
          onClose={() => setSheet(null)}
          onSaved={(p) => {
            onParty(p)
            setSheet(null)
          }}
        />
      )}
      {sheet === 'invite' && (
        <InviteMore
          party={party}
          onClose={() => setSheet(null)}
          onDone={() => {
            setSheet(null)
            void api
              .GET('/parties/{partyId}', { params: { path: { partyId: party.id } } })
              .then(({ data }) => data && onParty(data))
          }}
        />
      )}
    </section>
  )
}

/** Proposals → ranking → the party, the current step lit. */
function Timeline({ party, vote }: { party: Party; vote: Vote | null }) {
  const { t, i18n } = useTranslation()
  const short = (d: string) =>
    new Intl.DateTimeFormat(i18n.language, {
      weekday: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(d))
  const phase = vote?.phase ?? (party.vote?.closed ? 'closed' : 'proposals')
  const step = phase === 'proposals' ? 0 : phase === 'ranking' ? 1 : 2
  const v = party.vote!
  const today =
    party.scheduledAt && new Date(party.scheduledAt).toDateString() === now().toDateString()
  const steps = [
    {
      name: step === 0 ? t('party.host.step.proposalsNow') : t('party.host.step.proposals'),
      when:
        step > 0
          ? t('party.host.step.done')
          : t('party.host.step.until', { date: short(v.proposalsUntil) }),
    },
    {
      name: step === 1 ? t('party.host.step.rankingNow') : t('party.host.step.ranking'),
      when:
        step > 1
          ? t('party.host.step.done')
          : t('party.host.step.until', { date: short(v.rankingUntil) }),
    },
    {
      name: today ? t('party.host.step.partyToday') : t('party.title'),
      when: party.scheduledAt ? short(party.scheduledAt) : '',
    },
  ]
  return (
    <ol className="ph-steps">
      <span className="ph-steps__track" aria-hidden="true" />
      <span className="ph-steps__fill" style={{ width: `${step * 33}%` }} aria-hidden="true" />
      {steps.map((s, i) => (
        <li
          key={i}
          className={cx('ph-step', i < step && 'is-done', i === step && 'is-now')}
          aria-current={i === step ? 'step' : undefined}
        >
          <span className="ph-step__dot">{i < step && <Icon name="check" size={10} />}</span>
          <p className="ph-step__name">{s.name}</p>
          <p className="ph-step__when">{s.when}</p>
        </li>
      ))}
    </ol>
  )
}

function VoteCard({
  party,
  vote,
  onVote,
  onError,
}: {
  party: Party
  vote: Vote
  onVote: (v: Vote) => void
  onError: (err: unknown) => void
}) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)
  const votePage = `/party/${party.id}/vote`
  const voted = vote.voters.filter((v) => v.voted).length

  async function rankNow() {
    setBusy(true)
    const { data, error } = await api.POST('/parties/{partyId}/vote/ranking', {
      params: { path: { partyId: party.id } },
    })
    setBusy(false)
    if (data) onVote(data)
    else onError(error)
  }

  if (vote.phase === 'closed') {
    const scores = (vote.result?.scores ?? []).filter((s) => !s.vetoed).slice(0, 4)
    const top = Math.max(1, ...scores.map((s) => s.points))
    const items = new Map(vote.proposals.map((p) => [p.item.id, p.item]))
    const closedAt = new Intl.DateTimeFormat(i18n.language, {
      weekday: 'long',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(vote.rankingUntil))
    return (
      <div className="ph-card">
        <div className="ph-card__head">
          <div>
            <h3 className="ph-card__title">{t('party.host.chosen')}</h3>
            <p className="ph-card__hint">
              {t('party.host.closedHint', {
                date: closedAt,
                count: voted,
                total: vote.voters.length,
              })}
            </p>
          </div>
          <Link to={votePage} className="ph-btn ph-btn--sm vm-lg pl-focusable">
            {t('party.host.details')}
          </Link>
        </div>
        <ul className="ph-scores">
          {scores.map((s, i) => {
            const it = items.get(s.itemId)
            return (
              <li key={s.itemId}>
                <span className="ph-poster ph-poster--xs">
                  {it?.posterId && <Img src={imageSrc(it.posterId, 92)} alt="" decoding="async" />}
                </span>
                <div>
                  <p>
                    <span>{it?.title}</span>
                    <span>{t('party.vote.points', { count: s.points })}</span>
                  </p>
                  <span className="ph-bar">
                    <span
                      className={cx(i === 0 && 'is-top')}
                      style={{ width: `${(s.points / top) * 100}%` }}
                    />
                  </span>
                </div>
              </li>
            )
          })}
        </ul>
      </div>
    )
  }

  const proposing = vote.phase === 'proposals'
  const left = new Date(vote.proposalsUntil).getTime() - now().getTime()
  const leftText =
    left >= DAY
      ? t('party.vote.propose.leftDays', { count: Math.floor(left / DAY) })
      : t('party.vote.propose.leftHours', { count: Math.max(1, Math.floor(left / HOUR)) })
  const rankingUntil = new Intl.DateTimeFormat(i18n.language, {
    weekday: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(vote.rankingUntil))
  return (
    <div className="ph-card">
      <div className="ph-card__head">
        <div>
          <h3 className="ph-card__title">
            {proposing
              ? t('party.host.proposed', { count: vote.proposals.length })
              : t('party.host.ranking')}
          </h3>
          <p className="ph-card__hint">
            {proposing
              ? t('party.host.proposingHint', { left: leftText })
              : t('party.host.rankingHint', {
                  count: voted,
                  total: vote.voters.length,
                  date: rankingUntil,
                })}
          </p>
        </div>
        <div className="ph-card__acts">
          <button
            type="button"
            className="ph-btn ph-btn--sm vm-lg pl-focusable"
            onClick={() => void navigate(votePage)}
          >
            {proposing ? t('party.host.propose') : t('party.host.rank')}
          </button>
          {proposing && (
            <button
              type="button"
              className="ph-btn ph-btn--sm ph-btn--lit vm-lg pl-focusable"
              disabled={busy || vote.proposals.length < 2}
              onClick={() => void rankNow()}
            >
              {t('party.host.rankNow')}
            </button>
          )}
        </div>
      </div>
      {vote.proposals.length > 0 && (
        <ul className="ph-posters">
          {vote.proposals.map((p) => (
            <li key={p.item.id}>
              <span className="ph-poster">
                {p.item.posterId && (
                  <Img src={imageSrc(p.item.posterId, 220)} alt="" decoding="async" />
                )}
                <PartyAvatar
                  name={p.proposer.name}
                  color={p.proposer.avatarColor}
                  url={p.proposer.avatar.url}
                  size={22}
                />
              </span>
              <p>{p.item.title}</p>
            </li>
          ))}
        </ul>
      )}
      {proposing && <p className="ph-card__note">{t('party.host.rankNowHint')}</p>}
    </div>
  )
}

function SettingRow({
  icon,
  label,
  value,
  onClick,
}: {
  icon: IconName
  label: string
  value: string
  onClick: () => void
}) {
  return (
    <button type="button" className="ph-setting pl-focusable" onClick={onClick}>
      <Icon name={icon} size={18} />
      <span className="ph-setting__label">{label}</span>
      <span className="ph-setting__value">
        {value}
        <Icon name="chevronRight" size={14} />
      </span>
    </button>
  )
}

/** Who still has something to do: answer, propose or vote. */
function behind(i: Invite, vote: Vote | null, proposers: Set<string>): boolean {
  if (i.answer === 'no') return false
  if (i.answer === 'pending') return true
  if (vote?.phase === 'proposals') return !proposers.has(i.person.userId)
  if (vote?.phase === 'ranking')
    return !vote.voters.some((v) => v.userId === i.person.userId && v.voted)
  return false
}

function Guests({
  party,
  vote,
  onInvite,
  onError,
}: {
  party: Party
  vote: Vote | null
  onInvite: () => void
  onError: (err: unknown) => void
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const [nudged, setNudged] = useState(false)
  const coming = party.invites.filter((i) => i.answer === 'yes').length
  const counts = new Map<string, number>()
  for (const p of vote?.proposals ?? [])
    counts.set(p.proposer.userId, (counts.get(p.proposer.userId) ?? 0) + 1)
  const proposers = new Set(counts.keys())
  const invites = [...party.invites].sort(
    (a, b) => Number(a.answer === 'no') - Number(b.answer === 'no'),
  )
  const late = invites.filter((i) => behind(i, vote, proposers))
  const names = new Intl.ListFormat(undefined, { type: 'conjunction' }).format(
    late.map((i) => i.person.name),
  )

  function doing(i: Invite): string {
    if (i.answer === 'no' || !vote) return ''
    if (vote.phase === 'proposals') {
      const n = counts.get(i.person.userId) ?? 0
      return n ? t('party.host.didPropose', { count: n }) : t('party.host.notProposed')
    }
    return vote.voters.some((v) => v.userId === i.person.userId && v.voted)
      ? t('party.host.didVote')
      : t('party.host.notVoted')
  }

  async function copy() {
    await navigator.clipboard?.writeText(party.link).catch(() => {})
    setCopied(true)
  }
  async function nudge() {
    const { error } = await api.POST('/parties/{partyId}/nudge', {
      params: { path: { partyId: party.id } },
      body: { userIds: late.map((i) => i.person.userId) },
    })
    if (error) onError(error)
    else setNudged(true)
  }

  return (
    <div className="ph-card">
      <div className="ph-card__head">
        <h3 className="ph-card__title">
          {t('party.host.guests')}{' '}
          <span>
            {t('party.host.coming', { count: coming })} ·{' '}
            {t('party.host.invited', { count: party.invites.length })}
          </span>
        </h3>
        <button type="button" className="ph-btn ph-btn--sm vm-lg pl-focusable" onClick={onInvite}>
          <Icon name="userPlus" size={16} />
          {t('party.invite.people')}
        </button>
      </div>
      <button type="button" className="ph-code pl-focusable" onClick={() => void copy()}>
        <Icon name="link" size={17} />
        <span className="ph-code__code">{party.code.replace('-', ' ')}</span>
        <span>{copied ? t('party.invite.copied') : t('party.host.copyLink')}</span>
      </button>
      <ul className="ph-guests">
        {invites.map((i) => (
          <li key={i.person.userId} className={cx(i.answer === 'no' && 'is-out')}>
            <PartyAvatar
              name={i.person.name}
              color={i.person.avatarColor}
              url={i.person.avatar.url}
              size={34}
            />
            <div>
              <p>{i.person.name}</p>
              <p>{doing(i)}</p>
            </div>
            <span className={cx('ph-chip', i.answer === 'yes' && 'is-yes')}>
              {t(`party.host.answer.${i.answer}`)}
            </span>
          </li>
        ))}
      </ul>
      {late.length > 0 && (
        <button
          type="button"
          className="ph-btn ph-btn--sm ph-btn--block vm-lg pl-focusable"
          disabled={nudged}
          onClick={() => void nudge()}
        >
          <Icon name="bell" size={15} />
          {nudged ? t('party.host.nudged') : t('party.host.nudge', { names })}
        </button>
      )}
    </div>
  )
}

/** "Modifier": move the party; a vote's ranking must end before it. */
function EditSheet({
  party,
  onClose,
  onSaved,
}: {
  party: Party
  onClose: () => void
  onSaved: (p: Party) => void
}) {
  const { t } = useTranslation()
  const local = (iso?: string) => {
    if (!iso) return ''
    const d = new Date(iso)
    d.setMinutes(d.getMinutes() - d.getTimezoneOffset())
    return d.toISOString().slice(0, 16)
  }
  const [when, setWhen] = useState(local(party.scheduledAt))
  const [error, setError] = useState<string | null>(null)
  async function save() {
    const { data, error: err } = await api.PATCH('/parties/{partyId}', {
      params: { path: { partyId: party.id } },
      body: { scheduledAt: new Date(when).toISOString() },
    })
    if (data) onSaved(data)
    else
      setError(
        (err as { details?: { code?: string }[] })?.details?.[0]?.code === 'before_vote'
          ? 'voteDeadlines'
          : 'generic',
      )
  }
  return (
    <GlassSheet
      title={t('party.host.editTitle')}
      onClose={onClose}
      footer={
        <Button variant="cream" block disabled={!when} onClick={() => void save()}>
          {t('party.host.save')}
        </Button>
      }
    >
      <label className="ph-field">
        <span>{t('party.host.when')}</span>
        <input
          type="datetime-local"
          value={when}
          onChange={(e) => setWhen(e.target.value)}
          className="pl-focusable"
        />
      </label>
      {error && (
        <p className="pv-party-error" role="alert">
          {t(`party.error.${error}`, t('party.error.generic'))}
        </p>
      )}
    </GlassSheet>
  )
}

/** "Inviter": people on the server not invited yet. */
function InviteMore({
  party,
  onClose,
  onDone,
}: {
  party: Party
  onClose: () => void
  onDone: () => void
}) {
  const { t } = useTranslation()
  const [people, setPeople] = useState<Person[] | null>(null)
  const [picked, setPicked] = useState<Set<string>>(new Set())
  useEffect(() => {
    const already = new Set(party.invites.map((i) => i.person.userId))
    void api
      .GET('/parties/people')
      .then(({ data }) => setPeople((data ?? []).filter((p) => !already.has(p.userId))))
      .catch(() => setPeople([]))
  }, [party.invites])
  async function send() {
    await api.POST('/parties/{partyId}/invites', {
      params: { path: { partyId: party.id } },
      body: { userIds: [...picked] },
    })
    onDone()
  }
  return (
    <GlassSheet
      title={t('party.invite.people')}
      onClose={onClose}
      footer={
        <Button variant="cream" block disabled={picked.size === 0} onClick={() => void send()}>
          {t('party.host.inviteSend', { count: picked.size })}
        </Button>
      }
    >
      {people === null ? (
        <Spinner size={24} />
      ) : people.length === 0 ? (
        <p className="ph-card__hint">{t('party.invite.nobody')}</p>
      ) : (
        <ul className="ph-pick">
          {people.map((p) => {
            const on = picked.has(p.userId)
            return (
              <li key={p.userId}>
                <button
                  type="button"
                  aria-pressed={on}
                  className={cx('pl-focusable', on && 'is-on')}
                  onClick={() => {
                    const next = new Set(picked)
                    if (on) next.delete(p.userId)
                    else next.add(p.userId)
                    setPicked(next)
                  }}
                >
                  <PartyAvatar name={p.name} color={p.avatarColor} url={p.avatar.url} size={46} />
                  <span>{p.name}</span>
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </GlassSheet>
  )
}
