import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Button, Spinner } from '../ui'
import '../upcoming/upcoming.css'
import './party.css'

type Vote = components['schemas']['PartyVote']
type Item = components['schemas']['PartyItem']
type Summary = components['schemas']['ItemSummary']

/**
 * The movie vote (VoteBig web, VotePh; R31–R34): everyone proposes 2
 * titles, then ranks a top 3 (3, 2, 1 points) with one veto. Votes are
 * anonymous: you see who voted, never for what.
 */
export function VotePage() {
  const { partyId = '' } = useParams()
  const { t, i18n } = useTranslation()
  const [vote, setVote] = useState<Vote | null | undefined>(undefined)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void api
      .GET('/parties/{partyId}/vote', { params: { path: { partyId } } })
      .then(({ data }) => setVote(data ?? null))
      .catch(() => setVote(null))
  }, [partyId])

  const fail = (err: unknown) =>
    setError((err as { error?: { code?: string } })?.error?.code ?? 'generic')

  if (vote === undefined) {
    return (
      <section className="br-page" aria-busy="true">
        <Spinner size={28} />
      </section>
    )
  }
  if (vote === null) {
    return (
      <section className="br-page">
        <p>{t('errors.generic')}</p>
      </section>
    )
  }
  const until = (d: string) =>
    new Intl.DateTimeFormat(i18n.language, {
      weekday: 'long',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(d))
  const titles = new Map(vote.proposals.map((p) => [p.item.id, p.item.title]))
  return (
    <section className="br-page vt-page">
      <Link to="/upcoming" className="pl-link">
        {t('upcoming.title')}
      </Link>
      <h1 className="br-page__title">{t(`party.vote.phase.${vote.phase}`)}</h1>
      {vote.phase === 'proposals' && (
        <Proposals
          vote={vote}
          partyId={partyId}
          until={until(vote.proposalsUntil)}
          onVote={setVote}
          onError={fail}
        />
      )}
      {vote.phase === 'ranking' && (
        <Ranking
          vote={vote}
          partyId={partyId}
          until={until(vote.rankingUntil)}
          onVote={setVote}
          onError={fail}
        />
      )}
      {vote.phase === 'closed' && vote.result && (
        <div className="vt-results">
          {vote.result.tied && vote.result.tied.length > 1 && (
            <p className="pv-party-muted">
              {t('party.vote.tie', { points: vote.result.scores[0]?.points ?? 0 })}
            </p>
          )}
          {vote.result.winner && (
            <p className="pv-party-display">{titles.get(vote.result.winner)}</p>
          )}
          <ol className="vt-scores">
            {vote.result.scores.map((s) => (
              <li key={s.itemId} className={s.vetoed ? 'is-vetoed' : undefined}>
                <span>{titles.get(s.itemId)}</span>
                <span>
                  {s.vetoed ? t('party.vote.vetoed') : t('party.vote.points', { count: s.points })}
                </span>
              </li>
            ))}
          </ol>
          {vote.result.seed && <p className="pv-party-muted">{t('party.vote.drawCheck')}</p>}
        </div>
      )}
      <h2 className="up-h">{t('party.vote.voters')}</h2>
      <ul className="vt-voters">
        {vote.voters.map((v, i) => (
          <li key={i}>
            {v.name} · {v.voted ? t('party.vote.voted') : t('party.vote.notYet')}
          </li>
        ))}
      </ul>
      <p className="pv-party-muted">{t('party.vote.anonymous')}</p>
      {error && (
        <p className="pv-party-error" role="alert">
          {t(`party.error.${error}`, t('party.error.generic'))}
        </p>
      )}
    </section>
  )
}

function Proposals({
  vote,
  partyId,
  until,
  onVote,
  onError,
}: {
  vote: Vote
  partyId: string
  until: string
  onVote: (v: Vote) => void
  onError: (e: unknown) => void
}) {
  const { t } = useTranslation()
  const [q, setQ] = useState('')
  const [found, setFound] = useState<Summary[]>([])
  const [ideas, setIdeas] = useState<Item[]>([])
  const mine = vote.proposals.filter((p) => p.mine).length
  useEffect(() => {
    void api
      .GET('/parties/{partyId}/vote/suggestions', { params: { path: { partyId } } })
      .then(({ data }) => setIdeas(data ?? []))
      .catch(() => {})
  }, [partyId])

  async function search(e: FormEvent) {
    e.preventDefault()
    const { data } = await api
      .GET('/search', { params: { query: { q } } })
      .catch(() => ({ data: undefined }))
    setFound(data?.movies ?? [])
  }
  async function propose(itemId: string) {
    const { data, error } = await api.PUT('/parties/{partyId}/vote/proposals/{itemId}', {
      params: { path: { partyId, itemId } },
    })
    if (data) onVote(data)
    else onError(error)
  }
  async function withdraw(itemId: string) {
    const { data } = await api.DELETE('/parties/{partyId}/vote/proposals/{itemId}', {
      params: { path: { partyId, itemId } },
    })
    if (data) onVote(data)
  }
  return (
    <>
      <p className="pv-party-muted">{t('party.vote.proposeHint', { count: 2 - mine, until })}</p>
      <form className="pv-party-compose" onSubmit={search}>
        <input
          className="pv-party-input"
          value={q}
          placeholder={t('party.vote.search')}
          aria-label={t('party.vote.search')}
          onChange={(e) => setQ(e.target.value)}
        />
        <Button type="submit" variant="secondary">
          {t('nav.search')}
        </Button>
      </form>
      <ul className="vt-list">
        {found.map((m) => (
          <li key={m.id}>
            <span>{m.displayTitle}</span>
            <Button
              variant="cream"
              size="sm"
              disabled={mine >= 2}
              onClick={() => void propose(m.id)}
            >
              {t('party.vote.propose')}
            </Button>
          </li>
        ))}
      </ul>
      {ideas.length > 0 && (
        <>
          <h2 className="up-h">{t('party.vote.suggests')}</h2>
          <ul className="vt-list">
            {ideas.map((m) => (
              <li key={m.id}>
                <span>
                  {m.title} <small className="pv-party-muted">{t('party.vote.nobodySaw')}</small>
                </span>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={mine >= 2}
                  onClick={() => void propose(m.id)}
                >
                  {t('party.vote.propose')}
                </Button>
              </li>
            ))}
          </ul>
        </>
      )}
      <h2 className="up-h">{t('party.vote.proposed', { count: vote.proposals.length })}</h2>
      <ul className="vt-list">
        {vote.proposals.map((p) => (
          <li key={p.item.id}>
            <span>
              {p.item.title}{' '}
              <small className="pv-party-muted">{t('party.vote.by', { name: p.proposedBy })}</small>
            </span>
            {p.mine && (
              <Button size="sm" variant="ghost" onClick={() => void withdraw(p.item.id)}>
                {t('party.vote.withdraw')}
              </Button>
            )}
          </li>
        ))}
      </ul>
    </>
  )
}

function Ranking({
  vote,
  partyId,
  until,
  onVote,
  onError,
}: {
  vote: Vote
  partyId: string
  until: string
  onVote: (v: Vote) => void
  onError: (e: unknown) => void
}) {
  const { t } = useTranslation()
  const [ranking, setRanking] = useState<string[]>(vote.myRanking)
  const [veto, setVeto] = useState<string | null>(vote.myVeto ?? null)
  function pick(id: string) {
    setRanking((r) => (r.includes(id) ? r.filter((x) => x !== id) : r.length < 3 ? [...r, id] : r))
    if (veto === id) setVeto(null)
  }
  async function submit() {
    const { data, error } = await api.PUT('/parties/{partyId}/vote/ballot', {
      params: { path: { partyId } },
      body: { ranking, veto },
    })
    if (data) onVote(data)
    else onError(error)
  }
  return (
    <>
      <p className="pv-party-muted">{t('party.vote.rankHint', { until })}</p>
      <ul className="vt-list">
        {vote.proposals.map((p) => {
          const rank = ranking.indexOf(p.item.id)
          return (
            <li key={p.item.id} className={veto === p.item.id ? 'is-vetoed' : undefined}>
              <button
                type="button"
                className="vt-rank pl-focusable"
                aria-pressed={rank >= 0}
                onClick={() => pick(p.item.id)}
              >
                <span className="vt-rank__n">{rank >= 0 ? rank + 1 : ''}</span>
                {p.item.title}
                <small className="pv-party-muted">
                  {t('party.vote.by', { name: p.proposedBy })}
                </small>
              </button>
              <Button
                size="sm"
                variant={veto === p.item.id ? 'secondary' : 'ghost'}
                aria-pressed={veto === p.item.id}
                onClick={() => {
                  setVeto(veto === p.item.id ? null : p.item.id)
                  setRanking((r) => r.filter((x) => x !== p.item.id))
                }}
              >
                {t('party.vote.veto')}
              </Button>
            </li>
          )
        })}
      </ul>
      <div className="pv-party-stack">
        <Button variant="cream" disabled={ranking.length === 0} onClick={() => void submit()}>
          {t('party.vote.submit')}
        </Button>
        <Button
          variant="ghost"
          onClick={() => {
            setRanking([])
            setVeto(null)
          }}
        >
          {t('party.vote.reset')}
        </Button>
      </div>
    </>
  )
}
