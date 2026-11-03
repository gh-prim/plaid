import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { GlassSheet } from '../browse/GlassSheet'
import { InviteSheet } from '../party/LaunchSheet'
import { JoinCode } from '../party/JoinCode'
import { useNotify } from '../party/notify'
import { PartyAvatar } from '../party/PartyAvatar'
import { VoteModal } from '../party/VoteModal'
import '../party/party.css'
import { Button, Icon, SegmentedControl, Spinner, cx } from '../ui'
import './upcoming.css'

type Upcoming = components['schemas']['Upcoming']
type Party = components['schemas']['Party']
type Episode = components['schemas']['UpcomingEpisode']
type Kind = 'all' | 'episodes' | 'parties'

type Entry = { day: string; episode?: Episode; party?: Party }

function dayOf(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/**
 * The Upcoming tab (PartiesWeb, R37–R39): the invitations to answer on
 * top, then day by day the releases of the shows you follow and your
 * parties, as a list or a month calendar.
 */
export function UpcomingPage() {
  const { t, i18n } = useTranslation()
  const [params, setParams] = useSearchParams()
  const [kind, setKind] = useState<Kind>('all')
  const [view, setView] = useState<'list' | 'calendar'>('list')
  const [data, setData] = useState<Upcoming | null | undefined>(undefined)
  const [join, setJoin] = useState(false)
  const [plan, setPlan] = useState(false)
  // The month shown by the calendar, as an offset from this month.
  const [month, setMonth] = useState(0)
  const tick = useNotify()
  const [reload, setReload] = useState(0)
  const openParty = params.get('party')
  const grid = useMemo(() => monthGrid(month), [month])

  useEffect(() => {
    const from = new Date()
    from.setDate(from.getDate() - 7)
    const range =
      view === 'calendar'
        ? { from: dayOf(grid.days[0]!), to: dayOf(grid.days[grid.days.length - 1]!) }
        : { from: dayOf(from) }
    void api
      .GET('/upcoming', { params: { query: { kind, ...range } } })
      .then(({ data }) => setData(data ?? null))
      .catch(() => setData(null))
  }, [kind, tick, reload, view, grid])

  const entries = useMemo(() => {
    if (!data) return []
    const out: Entry[] = []
    for (const e of data.episodes) out.push({ day: e.airDate, episode: e })
    const parties = view === 'calendar' ? [...data.parties, ...data.invitations] : data.parties
    for (const p of parties) {
      const when = p.scheduledAt ? new Date(p.scheduledAt) : new Date()
      out.push({ day: dayOf(when), party: p })
    }
    return out.sort((a, b) => a.day.localeCompare(b.day))
  }, [data, view])

  const dayLabel = (day: string) => {
    const d = new Date(`${day}T12:00:00`)
    const rel = Math.round(
      (d.getTime() - new Date(`${dayOf(new Date())}T12:00:00`).getTime()) / 86_400_000,
    )
    const name = new Intl.DateTimeFormat(i18n.language, {
      weekday: 'long',
      day: 'numeric',
      month: 'short',
    }).format(d)
    if (Math.abs(rel) <= 1) {
      return `${new Intl.RelativeTimeFormat(i18n.language, { numeric: 'auto' }).format(rel, 'day')} · ${name}`
    }
    return name
  }

  return (
    <section className="br-page up-page">
      <header className="up-head">
        {view === 'calendar' ? (
          <div className="up-cal-title">
            <h1 className="br-page__title">
              {new Intl.DateTimeFormat(i18n.language, { month: 'long', year: 'numeric' }).format(
                grid.first,
              )}
            </h1>
            <button
              type="button"
              className="up-round vm-lg pl-focusable"
              aria-label={t('upcoming.calendar.previous')}
              onClick={() => setMonth((m) => m - 1)}
            >
              <Icon name="chevronLeft" size={17} />
            </button>
            <button
              type="button"
              className="up-round vm-lg pl-focusable"
              aria-label={t('upcoming.calendar.next')}
              onClick={() => setMonth((m) => m + 1)}
            >
              <Icon name="chevronRight" size={17} />
            </button>
          </div>
        ) : (
          <div>
            <h1 className="br-page__title">{t('upcoming.title')}</h1>
            <p className="pv-party-muted">{t('upcoming.hint')}</p>
          </div>
        )}
        <div className="up-tools">
          <SegmentedControl
            label={t('upcoming.filter.label')}
            value={kind}
            options={[
              { value: 'all', label: t('upcoming.filter.all') },
              { value: 'episodes', label: t('upcoming.filter.episodes') },
              { value: 'parties', label: t('upcoming.filter.parties') },
            ]}
            onChange={setKind}
          />
          <SegmentedControl
            label={t('upcoming.view.label')}
            value={view}
            options={[
              { value: 'list', label: t('upcoming.view.list') },
              { value: 'calendar', label: t('upcoming.view.calendar') },
            ]}
            onChange={setView}
          />
          <Button variant="secondary" size="sm" onClick={() => setJoin(true)}>
            {t('party.join.title')}
          </Button>
          <button type="button" className="up-new vm-lg pl-focusable" onClick={() => setPlan(true)}>
            <Icon name="plus" size={16} />
            {t('upcoming.newParty')}
          </button>
        </div>
      </header>

      {data === undefined && view === 'list' && (
        <div className="up-loading" role="status" aria-label={t('common.loading')}>
          <Spinner size={28} />
        </div>
      )}
      {data === null && <p className="pv-party-muted">{t('errors.generic')}</p>}

      {data && view === 'list' && data.invitations.length > 0 && (
        <section
          className="up-invites"
          aria-label={t('upcoming.invitations', { count: data.invitations.length })}
        >
          <h2 className="up-h">{t('upcoming.invitations', { count: data.invitations.length })}</h2>
          {data.invitations.map((p) => (
            <InvitationCard key={p.id} party={p} onAnswered={() => setReload((n) => n + 1)} />
          ))}
        </section>
      )}

      {data && view === 'list' && (
        <ol className="up-days">
          {groupByDay(entries).map(([day, list]) => (
            <li key={day}>
              <h2 className="up-h">{dayLabel(day)}</h2>
              {list.map((e, i) =>
                e.episode ? (
                  <EpisodeRow key={i} e={e.episode} onChange={() => setReload((n) => n + 1)} />
                ) : (
                  <PartyRow key={i} p={e.party!} />
                ),
              )}
            </li>
          ))}
          {entries.length === 0 && <p className="pv-party-muted">{t('upcoming.empty')}</p>}
        </ol>
      )}
      {view === 'calendar' && (
        <Month
          grid={grid}
          entries={entries}
          loading={data === undefined}
          onChange={() => setReload((n) => n + 1)}
          onPlan={() => setPlan(true)}
        />
      )}
      <p className="pv-party-muted up-foot">{t('upcoming.followHint')}</p>

      {join && <JoinCode onClose={() => setJoin(false)} />}
      {plan && (
        <VoteModal
          onClose={() => {
            setPlan(false)
            setReload((n) => n + 1)
          }}
        />
      )}
      {openParty && (
        <PartySheet
          id={openParty}
          onClose={() => {
            params.delete('party')
            setParams(params, { replace: true })
          }}
        />
      )}
    </section>
  )
}

function groupByDay(entries: Entry[]): [string, Entry[]][] {
  const m = new Map<string, Entry[]>()
  for (const e of entries) m.set(e.day, [...(m.get(e.day) ?? []), e])
  return [...m.entries()]
}

function EpisodeRow({ e, onChange }: { e: Episode; onChange: () => void }) {
  const { t } = useTranslation()
  async function alert(on: boolean) {
    const path = { showId: e.showId, season: e.season, episode: e.episode }
    await (
      on
        ? api.PUT('/shows/{showId}/alerts/{season}/{episode}', { params: { path } })
        : api.DELETE('/shows/{showId}/alerts/{season}/{episode}', { params: { path } })
    ).catch(() => {})
    onChange()
  }
  return (
    <div className="up-row">
      <div className="up-row__main">
        <p className="up-row__kicker">
          {t('upcoming.episode.code', { show: e.showTitle, season: e.season, episode: e.episode })}
        </p>
        {e.title && <p className="up-row__title">{e.title}</p>}
        <p className="up-row__status">{t(`upcoming.episode.${e.status}`)}</p>
      </div>
      <div className="up-row__acts">
        {e.status === 'released_on_server' && e.itemId ? (
          <Link
            to={`/item/${e.itemId}`}
            className="pl-btn pl-btn--secondary pl-btn--sm pl-focusable"
          >
            {t('item.play')}
          </Link>
        ) : (
          <Button
            size="sm"
            variant={e.alert ? 'secondary' : 'ghost'}
            aria-pressed={e.alert}
            onClick={() => void alert(!e.alert)}
          >
            {e.alert ? t('upcoming.notified') : t('upcoming.notifyMe')}
          </Button>
        )}
      </div>
    </div>
  )
}

function PartyRow({ p }: { p: Party }) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const time = p.scheduledAt
    ? new Intl.DateTimeFormat(i18n.language, { timeStyle: 'short' }).format(new Date(p.scheduledAt))
    : ''
  const hosting = p.myAnswer === undefined
  return (
    <div className="up-row up-row--party">
      <div className="up-row__main">
        <p className="up-row__kicker">
          {p.status === 'live' ? t('party.row.live') : `${t('party.title')} · ${time}`}
        </p>
        <p className="up-row__title">{p.item?.title ?? t('party.vote.pending')}</p>
        <p className="up-row__status">
          {hosting
            ? t('upcoming.party.hosting')
            : t('upcoming.party.by', { name: p.organizer.name })}
        </p>
      </div>
      <div className="up-row__acts">
        {p.status === 'live' ? (
          <Button variant="cream" size="sm" onClick={() => void navigate(`/party/${p.id}`)}>
            {t('party.join.cta')}
          </Button>
        ) : (
          <Link to={`?party=${p.id}`} className="pl-btn pl-btn--ghost pl-btn--sm pl-focusable">
            {t('upcoming.party.open')}
          </Link>
        )}
      </div>
    </div>
  )
}

/** An invitation to answer (PartiesWeb "Une invitation", R28). */
function InvitationCard({ party, onAnswered }: { party: Party; onAnswered: () => void }) {
  const { t, i18n } = useTranslation()
  const when = party.scheduledAt
    ? new Intl.DateTimeFormat(i18n.language, { dateStyle: 'full', timeStyle: 'short' }).format(
        new Date(party.scheduledAt),
      )
    : ''
  async function answer(a: 'yes' | 'maybe' | 'no') {
    await api
      .PUT('/parties/{partyId}/answer', {
        params: { path: { partyId: party.id } },
        body: { answer: a },
      })
      .catch(() => {})
    onAnswered()
  }
  return (
    <div className="up-invite pv-glass">
      <PartyAvatar name={party.organizer.name} color={party.organizer.avatarColor} />
      <div className="up-row__main">
        <p className="up-row__kicker">
          {t('upcoming.invite.from', { name: party.organizer.name })}
        </p>
        <p className="up-row__title">{party.item?.title ?? t('party.vote.pending')}</p>
        <p className="up-row__status">{when}</p>
        {party.note && <p className="up-row__status">« {party.note} »</p>}
      </div>
      <div className="up-row__acts">
        <Button variant="cream" size="sm" onClick={() => void answer('yes')}>
          {t('party.answer.yes')}
        </Button>
        <Button size="sm" variant="secondary" onClick={() => void answer('maybe')}>
          {t('party.answer.maybe')}
        </Button>
        <Button size="sm" variant="ghost" onClick={() => void answer('no')}>
          {t('party.answer.no')}
        </Button>
      </div>
    </div>
  )
}

/** The month calendar (PartiesWeb "Vue calendrier"). */
/** The six weeks shown for a month, Monday first. */
function monthGrid(offset: number): { first: Date; days: Date[] } {
  const now = new Date()
  const first = new Date(now.getFullYear(), now.getMonth() + offset, 1)
  const start = new Date(first)
  start.setDate(1 - ((first.getDay() + 6) % 7))
  const days = Array.from({ length: 42 }, (_, i) => {
    const d = new Date(start)
    d.setDate(start.getDate() + i)
    return d
  })
  return { first, days }
}

/** How a party sits in the calendar (the board's legend). */
function partyKind(p: Party): 'live' | 'hosting' | 'going' | 'invited' {
  if (p.status === 'live') return 'live'
  if (p.myAnswer === undefined) return 'hosting'
  return p.myAnswer === 'yes' || p.myAnswer === 'maybe' ? 'going' : 'invited'
}

/**
 * The month calendar (PartiesWeb "Vue calendrier"): a legend, six weeks of
 * days with their releases and parties, and the selected day on the side.
 * It shows even when nothing is planned.
 */
function Month({
  grid,
  entries,
  loading,
  onChange,
  onPlan,
}: {
  grid: { first: Date; days: Date[] }
  entries: Entry[]
  loading: boolean
  onChange: () => void
  onPlan: () => void
}) {
  const { t, i18n } = useTranslation()
  const today = dayOf(new Date())
  const [picked, setPicked] = useState(today)
  const byDay = new Map(groupByDay(entries))
  const weekday = new Intl.DateTimeFormat(i18n.language, { weekday: 'short' })
  const time = new Intl.DateTimeFormat(i18n.language, { timeStyle: 'short' })
  const selected = byDay.get(picked) ?? []
  return (
    <>
      <ul className="up-legend" aria-label={t('upcoming.calendar.legend')}>
        <li>
          <Icon name="tv" size={14} />
          {t('upcoming.calendar.episode')}
        </li>
        <li>
          <span className="up-swatch is-hosting" />
          {t('upcoming.party.hosting')}
        </li>
        <li>
          <span className="up-swatch is-going" />
          {t('upcoming.calendar.going')}
        </li>
        <li>
          <span className="up-swatch is-invited" />
          {t('upcoming.calendar.invitation')}
        </li>
      </ul>
      <div className="up-cal">
        <div
          className={cx('up-month', loading && 'is-loading')}
          role="grid"
          aria-busy={loading || undefined}
          aria-label={new Intl.DateTimeFormat(i18n.language, {
            month: 'long',
            year: 'numeric',
          }).format(grid.first)}
        >
          <div role="row" className="up-month__row">
            {grid.days.slice(0, 7).map((d) => (
              <span key={d.getDay()} className="up-month__head" role="columnheader">
                {weekday.format(d)}
              </span>
            ))}
          </div>
          {[0, 1, 2, 3, 4, 5].map((w) => (
            <div key={w} role="row" className="up-month__row">
              {grid.days.slice(w * 7, w * 7 + 7).map((d) => {
                const key = dayOf(d)
                const list = byDay.get(key) ?? []
                const other = d.getMonth() !== grid.first.getMonth()
                return (
                  <button
                    key={key}
                    type="button"
                    role="gridcell"
                    aria-selected={picked === key}
                    className={cx(
                      'up-month__day pl-focusable',
                      other && 'is-other',
                      key === today && 'is-today',
                      picked === key && 'is-picked',
                    )}
                    onClick={() => setPicked(key)}
                  >
                    <span className="up-month__n">{d.getDate()}</span>
                    {list.slice(0, 2).map((e, i) =>
                      e.episode ? (
                        <span key={i} className="up-chip up-chip--episode">
                          <Icon name="tv" size={13} />
                          <span>
                            {t('upcoming.calendar.code', {
                              show: e.episode.showTitle,
                              season: e.episode.season,
                              episode: e.episode.episode,
                            })}
                          </span>
                        </span>
                      ) : (
                        <span key={i} className={`up-chip is-${partyKind(e.party!)}`}>
                          <span className="up-chip__when">
                            {e.party!.status === 'live'
                              ? t('upcoming.calendar.live')
                              : e.party!.scheduledAt
                                ? time.format(new Date(e.party!.scheduledAt))
                                : ''}
                          </span>{' '}
                          {e.party!.item?.title ?? t('party.vote.pending')}
                        </span>
                      ),
                    )}
                    {list.length > 2 && (
                      <span className="up-chip__more">
                        {t('upcoming.calendar.more', { n: list.length - 2 })}
                      </span>
                    )}
                  </button>
                )
              })}
            </div>
          ))}
        </div>
        <aside className="up-side" aria-live="polite">
          <p className="up-side__day">
            {new Intl.DateTimeFormat(i18n.language, {
              weekday: 'long',
              day: 'numeric',
              month: 'long',
            }).format(new Date(`${picked}T12:00:00`))}
          </p>
          {selected.length === 0 && (
            <>
              <p className="up-side__empty">{t('upcoming.calendar.nothing')}</p>
              <button type="button" className="up-side__btn vm-lg pl-focusable" onClick={onPlan}>
                <Icon name="plus" size={16} />
                {t('upcoming.newParty')}
              </button>
            </>
          )}
          {selected.map((e, i) =>
            e.episode ? (
              <SideEpisode key={i} e={e.episode} onChange={onChange} />
            ) : (
              <SideParty key={i} p={e.party!} />
            ),
          )}
        </aside>
      </div>
    </>
  )
}

function SideEpisode({ e, onChange }: { e: Episode; onChange: () => void }) {
  const { t } = useTranslation()
  async function alert(on: boolean) {
    const path = { showId: e.showId, season: e.season, episode: e.episode }
    await (
      on
        ? api.PUT('/shows/{showId}/alerts/{season}/{episode}', { params: { path } })
        : api.DELETE('/shows/{showId}/alerts/{season}/{episode}', { params: { path } })
    ).catch(() => {})
    onChange()
  }
  return (
    <div className="up-side__item">
      <p className="up-side__kicker">
        <Icon name="tv" size={14} />
        {t('upcoming.episode.code', { show: e.showTitle, season: e.season, episode: e.episode })}
      </p>
      {e.title && <p className="up-side__title">{e.title}</p>}
      <span className="up-side__badge">{t(`upcoming.episode.${e.status}`)}</span>
      <div className="up-side__acts">
        {e.status === 'released_on_server' && e.itemId ? (
          <Link to={`/item/${e.itemId}`} className="up-side__btn vm-lg pl-focusable">
            <Icon name="play" size={15} />
            {t('item.play')}
          </Link>
        ) : (
          <button
            type="button"
            className="up-side__btn vm-lg pl-focusable"
            aria-pressed={e.alert}
            onClick={() => void alert(!e.alert)}
          >
            <Icon name="bell" size={16} />
            {e.alert ? t('upcoming.notified') : t('upcoming.notifyMe')}
          </button>
        )}
      </div>
    </div>
  )
}

function SideParty({ p }: { p: Party }) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const kind = partyKind(p)
  const time = p.scheduledAt
    ? new Intl.DateTimeFormat(i18n.language, { timeStyle: 'short' }).format(new Date(p.scheduledAt))
    : ''
  return (
    <div className="up-side__item">
      <p className="up-side__kicker">
        <Icon name="party" size={14} />
        {kind === 'live' ? t('party.row.live') : `${t('party.title')} · ${time}`}
      </p>
      <p className="up-side__title">{p.item?.title ?? t('party.vote.pending')}</p>
      <p className="up-side__meta">
        {kind === 'hosting'
          ? t('upcoming.party.hosting')
          : t('upcoming.party.by', { name: p.organizer.name })}
      </p>
      <div className="up-side__acts">
        {kind === 'live' ? (
          <button
            type="button"
            className="up-side__btn vm-lg pl-focusable"
            onClick={() => void navigate(`/party/${p.id}`)}
          >
            {t('party.join.cta')}
          </button>
        ) : (
          <Link to={`?party=${p.id}`} className="up-side__btn vm-lg pl-focusable">
            {t('upcoming.party.open')}
          </Link>
        )}
      </div>
    </div>
  )
}

/** A scheduled party (Party08): when, who comes, start now or share. */
function PartySheet({ id, onClose }: { id: string; onClose: () => void }) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const [party, setParty] = useState<Party | null>(null)
  const [share, setShare] = useState(false)
  useEffect(() => {
    void api
      .GET('/parties/{partyId}', { params: { path: { partyId: id } } })
      .then(({ data }) => setParty(data ?? null))
      .catch(() => {})
  }, [id])
  if (!party) return null
  if (share) return <InviteSheet party={party} onClose={onClose} onGo={onClose} />
  const when = party.scheduledAt
    ? new Intl.DateTimeFormat(i18n.language, { dateStyle: 'full', timeStyle: 'short' }).format(
        new Date(party.scheduledAt),
      )
    : ''
  const hosting = party.myAnswer === undefined
  return (
    <GlassSheet
      title={t('upcoming.party.scheduled')}
      subtitle={party.item?.title}
      onClose={onClose}
    >
      <p>{when}</p>
      {party.vote && !party.vote.closed && (
        <Link to={`/party/${party.id}/vote`} className="pl-btn pl-btn--secondary pl-focusable">
          {t('party.vote.open')}
        </Link>
      )}
      <ul className="pv-party-people">
        <li>
          <PartyAvatar name={party.organizer.name} color={party.organizer.avatarColor} host />
          <span className="pv-party-people__name">{party.organizer.name}</span>
          <span className="pv-party-people__how">{t('party.people.host')}</span>
        </li>
        {party.invites.map((i) => (
          <li key={i.person.userId}>
            <PartyAvatar name={i.person.name} color={i.person.avatarColor} />
            <span className="pv-party-people__name">{i.person.name}</span>
            <span className="pv-party-people__how">{t(`party.answer.state.${i.answer}`)}</span>
          </li>
        ))}
      </ul>
      <div className="pv-party-stack">
        {(party.status === 'live' || party.status === 'lobby' || hosting) && party.item && (
          <Button variant="cream" onClick={() => void navigate(`/party/${party.id}`)}>
            {hosting && party.status !== 'live' ? t('party.lobby.startNow') : t('party.join.cta')}
          </Button>
        )}
        <Button variant="secondary" onClick={() => setShare(true)}>
          {t('party.invite.share')}
        </Button>
        {hosting && (
          <Button
            variant="ghost"
            onClick={() =>
              void api
                .DELETE('/parties/{partyId}', { params: { path: { partyId: party.id } } })
                .finally(onClose)
            }
          >
            {t('upcoming.party.cancel')}
          </Button>
        )}
      </div>
    </GlassSheet>
  )
}
