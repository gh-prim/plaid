import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Img } from '../browse/Img'
import { imageSrc } from '../browse/media'
import { formatDuration } from '../lib/format'
import { Icon, cx } from '../ui'
import { PartyAvatar } from './PartyAvatar'

type Vote = components['schemas']['PartyVote']
type Party = components['schemas']['Party']
type PartyItem = components['schemas']['PartyItem']
type Summary = components['schemas']['ItemSummary']
type Filter = 'all' | 'unseen' | 'short' | 'movies' | 'episodes'

/** A title the group could watch, from the suggestions or the search. */
interface Title {
  id: string
  title: string
  posterId?: string
  year?: number
  runtimeMs?: number
  kind: 'movie' | 'episode'
  nobodySaw: boolean
}

const MAX = 2

/** How long until the deadline, in days or else hours (at least 1). */
function timeLeft(until: Date): { days: number } | { hours: number } {
  const ms = until.getTime() - Date.now()
  const days = Math.floor(ms / 86_400_000)
  return days >= 1 ? { days } : { hours: Math.max(1, Math.ceil(ms / 3_600_000)) }
}

function fromParty(it: PartyItem, nobodySaw: boolean): Title {
  return {
    id: it.id,
    title: it.kind === 'episode' && it.showTitle ? `${it.showTitle} · ${it.title}` : it.title,
    posterId: it.posterId,
    year: it.year,
    runtimeMs: it.runtimeMs,
    kind: it.kind,
    nobodySaw,
  }
}

function fromSearch(s: Summary, unseen: Set<string>): Title {
  return {
    id: s.id,
    title:
      s.kind === 'episode' && s.show
        ? `${s.show.displayTitle} · ${s.displayTitle}`
        : s.displayTitle,
    posterId: (s.poster ?? s.still)?.imageId,
    year: s.year,
    runtimeMs: s.runtimeMinutes ? s.runtimeMinutes * 60_000 : undefined,
    kind: s.kind === 'episode' ? 'episode' : 'movie',
    nobodySaw: unseen.has(s.id),
  }
}

/**
 * Everyone proposes their films (VotePropose board, R31, R34): the party and
 * the deadline, your two slots, the library search with its filters, Plaid's
 * suggestions (what nobody in the group has seen), what the group proposed,
 * and what comes next.
 */
export function VotePropose({
  vote,
  party,
  partyId,
  onVote,
  onError,
}: {
  vote: Vote
  party: Party | null
  partyId: string
  onVote: (v: Vote) => void
  onError: (e: unknown) => void
}) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const [ideas, setIdeas] = useState<PartyItem[] | null>(null)
  const [ambient, setAmbient] = useState<string>()
  const [q, setQ] = useState('')
  const [found, setFound] = useState<Summary[] | null>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const [busy, setBusy] = useState<string | null>(null)
  const search = useRef<HTMLInputElement>(null)

  useEffect(() => {
    void api
      .GET('/parties/{partyId}/vote/suggestions', { params: { path: { partyId } } })
      .then(({ data }) => setIdeas(data ?? []))
      .catch(() => setIdeas([]))
    void api
      .GET('/home')
      .then(({ data }) => setAmbient(data?.ambient?.backdrop?.imageId))
      .catch(() => {})
  }, [partyId])

  // The search runs as you type, a beat after the last key.
  useEffect(() => {
    const query = q.trim()
    if (query.length < 2) return
    const timer = setTimeout(() => {
      void api
        .GET('/search', { params: { query: { q: query } } })
        .then(({ data }) => setFound([...(data?.movies ?? []), ...(data?.episodes ?? [])]))
        .catch(() => setFound([]))
    }, 250)
    return () => clearTimeout(timer)
  }, [q])

  // "/" jumps to the search, like the board's key hint says.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName
      if (e.key === '/' && tag !== 'INPUT' && tag !== 'TEXTAREA') {
        e.preventDefault()
        search.current?.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])

  const mine = vote.proposals.filter((p) => p.mine)
  const proposed = new Set(vote.proposals.map((p) => p.item.id))
  const unseen = useMemo(() => new Set((ideas ?? []).map((i) => i.id)), [ideas])
  const searching = q.trim().length >= 2
  const titles = searching
    ? (found ?? []).map((s) => fromSearch(s, unseen))
    : (ideas ?? []).map((i) => fromParty(i, true))
  const shown = titles.filter((x) => {
    switch (filter) {
      case 'unseen':
        return x.nobodySaw
      case 'short':
        return x.runtimeMs !== undefined && x.runtimeMs < 90 * 60_000
      case 'movies':
        return x.kind === 'movie'
      case 'episodes':
        return x.kind === 'episode'
      default:
        return true
    }
  })
  const proposers = [
    ...new Map(vote.proposals.map((p) => [p.proposer.userId, p.proposer])).values(),
  ]
  const backdrop = vote.proposals.find((p) => p.item.backdropId)?.item.backdropId ?? ambient

  async function propose(id: string) {
    setBusy(id)
    const { data, error } = await api.PUT('/parties/{partyId}/vote/proposals/{itemId}', {
      params: { path: { partyId, itemId: id } },
    })
    setBusy(null)
    if (data) onVote(data)
    else onError(error)
  }
  async function withdraw(id: string) {
    setBusy(id)
    const { data, error } = await api.DELETE('/parties/{partyId}/vote/proposals/{itemId}', {
      params: { path: { partyId, itemId: id } },
    })
    setBusy(null)
    if (data) onVote(data)
    else onError(error)
  }

  const until = new Date(vote.proposalsUntil)
  const deadline = new Intl.DateTimeFormat(i18n.language, {
    weekday: 'long',
    hour: '2-digit',
    minute: '2-digit',
  }).format(until)
  const partyWhen = party?.scheduledAt
    ? new Intl.DateTimeFormat(i18n.language, {
        weekday: 'long',
        day: 'numeric',
        month: 'short',
        hour: '2-digit',
        minute: '2-digit',
      }).format(new Date(party.scheduledAt))
    : ''
  const meta = (x: Title) =>
    [x.year, x.runtimeMs ? formatDuration(i18n.language, x.runtimeMs) : undefined]
      .filter(Boolean)
      .join(' · ')
  const left = mine.length < MAX ? MAX - mine.length : 0
  const remaining = timeLeft(until)

  return (
    <div className="vp">
      {backdrop && (
        <div className="vp-light" aria-hidden="true">
          <Img src={imageSrc(backdrop, 1280)} alt="" decoding="async" />
        </div>
      )}
      <Link to="/upcoming" className="vp-back vm-lg pl-focusable">
        <Icon name="chevronLeft" size={16} />
        {t('upcoming.title')}
      </Link>

      <div className="vp-head">
        <div className="vp-head__main">
          {party && (
            <p className="vp-host">
              <PartyAvatar
                name={party.organizer.name}
                color={party.organizer.avatarColor}
                url={party.organizer.avatar.url}
                size={26}
              />
              {party.myAnswer === undefined
                ? t('party.vote.propose.hostMine', { date: partyWhen })
                : t('party.vote.propose.host', { name: party.organizer.name, date: partyWhen })}
            </p>
          )}
          <h1 className="vp-title">
            {left > 0
              ? t('party.vote.propose.title', { count: left })
              : t('party.vote.propose.titleDone')}
          </h1>
          <p className="vp-sub">
            <span>
              <Icon name="clock" size={16} />
              {t('party.vote.propose.until', {
                date: deadline,
                left:
                  'days' in remaining
                    ? t('party.vote.propose.leftDays', { count: remaining.days })
                    : t('party.vote.propose.leftHours', { count: remaining.hours }),
              })}
            </span>
            {proposers.length > 0 && (
              <span>
                <span className="vp-stack">
                  {proposers.map((p) => (
                    <PartyAvatar
                      key={p.userId}
                      name={p.name}
                      color={p.avatarColor}
                      url={p.avatar.url}
                      size={22}
                    />
                  ))}
                </span>
                {t('party.vote.propose.proposers', { count: proposers.length })}
              </span>
            )}
          </p>
        </div>
        <div className="vp-slots">
          {Array.from({ length: MAX }, (_, i) => {
            const p = mine[i]
            return p ? (
              <div key={p.item.id} className="vp-slot">
                <span className="vp-slot__poster">
                  {p.item.posterId && <Img src={imageSrc(p.item.posterId, 120)} alt="" />}
                </span>
                <div className="vp-slot__text">
                  <p>{p.item.title}</p>
                  <p>{t('party.vote.propose.slot', { n: i + 1 })}</p>
                </div>
                <button
                  type="button"
                  className="vp-slot__x pl-focusable"
                  aria-label={t('party.vote.propose.withdraw', { title: p.item.title })}
                  disabled={busy === p.item.id}
                  onClick={() => void withdraw(p.item.id)}
                >
                  <Icon name="x" size={14} />
                </button>
              </div>
            ) : (
              <button
                key={`empty-${i}`}
                type="button"
                className="vp-slot vp-slot--empty pl-focusable"
                onClick={() => search.current?.focus()}
              >
                <Icon name="plus" size={16} />
                {t('party.vote.propose.slotEmpty', { n: i + 1 })}
              </button>
            )
          })}
          <button
            type="button"
            className="vp-done vm-lg pl-focusable"
            onClick={() => void navigate('/upcoming')}
          >
            {t('party.vote.propose.done')}
          </button>
        </div>
      </div>

      <label className="vp-search vm-lg">
        <Icon name="search" size={18} />
        <input
          ref={search}
          type="search"
          value={q}
          placeholder={t('party.vote.propose.search')}
          aria-label={t('party.vote.propose.search')}
          onChange={(e) => {
            setQ(e.target.value)
            if (e.target.value.trim().length < 2) setFound(null)
          }}
        />
        <kbd aria-hidden="true">/</kbd>
      </label>
      <div className="vp-chips" role="radiogroup" aria-label={t('party.vote.propose.filter')}>
        {(['all', 'unseen', 'short', 'movies', 'episodes'] as const).map((f) => (
          <button
            key={f}
            type="button"
            role="radio"
            aria-checked={filter === f}
            className={cx('vp-chip pl-focusable', filter === f && 'is-on')}
            onClick={() => setFilter(f)}
          >
            {t(`party.vote.propose.filters.${f}`)}
          </button>
        ))}
      </div>

      <h2 className="vp-h">
        {searching ? (
          t('party.vote.propose.results')
        ) : (
          <>
            <Icon name="sparkle" size={20} />
            {t('party.vote.suggests')}{' '}
            <span className="vp-h__hint">{t('party.vote.propose.suggestsHint')}</span>
          </>
        )}
      </h2>
      {(searching ? found === null : ideas === null) ? (
        <div className="vp-grid" aria-busy="true">
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="vp-card">
              <span className="vp-card__poster vp-shimmer" />
            </div>
          ))}
        </div>
      ) : shown.length === 0 ? (
        <p className="vp-empty">{t('party.vote.propose.nothing')}</p>
      ) : (
        <div className="vp-grid">
          {shown.map((x) => {
            const on = proposed.has(x.id)
            const own = mine.some((p) => p.item.id === x.id)
            return (
              <div key={x.id} className="vp-card">
                <div className="vp-card__art">
                  <span className={cx('vp-card__poster', on && 'is-on')}>
                    {x.posterId && <Img src={imageSrc(x.posterId, 360)} alt="" />}
                  </span>
                  {on ? (
                    <button
                      type="button"
                      className="vp-card__tag pl-focusable"
                      disabled={!own || busy === x.id}
                      aria-label={
                        own ? t('party.vote.propose.withdraw', { title: x.title }) : undefined
                      }
                      onClick={() => own && void withdraw(x.id)}
                    >
                      <Icon name="check" size={14} />
                      {t('party.vote.propose.proposed')}
                    </button>
                  ) : (
                    <button
                      type="button"
                      className="vp-card__add vm-lg pl-focusable"
                      aria-label={t('party.vote.propose.add', { title: x.title })}
                      disabled={left === 0 || busy === x.id}
                      onClick={() => void propose(x.id)}
                    >
                      <Icon name="plus" size={18} />
                    </button>
                  )}
                </div>
                <p className="vp-card__title">{x.title}</p>
                {meta(x) && <p className="vp-card__meta">{meta(x)}</p>}
                {x.nobodySaw && (
                  <p className="vp-card__why">
                    <Icon name="sparkle" size={12} />
                    {t('party.vote.nobodySaw')}
                  </p>
                )}
              </div>
            )
          })}
        </div>
      )}

      <h2 className="vp-h">
        {t('party.vote.propose.group')} <span className="vp-h__count">{vote.proposals.length}</span>
      </h2>
      <div className="vp-group">
        {vote.proposals.map((p) => (
          <div key={p.item.id} className="vp-mini">
            <div className="vp-mini__art">
              <span className="vp-mini__poster">
                {p.item.posterId && <Img src={imageSrc(p.item.posterId, 240)} alt="" />}
              </span>
              <span className="vp-mini__who">
                <PartyAvatar
                  name={p.proposer.name}
                  color={p.proposer.avatarColor}
                  url={p.proposer.avatar.url}
                  size={26}
                />
              </span>
            </div>
            <p className="vp-mini__title">{p.item.title}</p>
            <p className="vp-mini__by">
              {p.mine ? t('party.vote.propose.yours') : t('party.vote.by', { name: p.proposedBy })}
            </p>
          </div>
        ))}
        <div className="vp-next">
          <p>{t('party.vote.propose.nextTitle')}</p>
          <p>{t('party.vote.propose.nextText', { date: deadline })}</p>
        </div>
      </div>
    </div>
  )
}
