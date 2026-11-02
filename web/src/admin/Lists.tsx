import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { failure } from '../auth/errors'
import { usePolling } from '../hooks/usePolling'
import { formatDuration, formatNumber, formatPercent, formatRelative } from '../lib/format'
import { methodLabel } from '../player/labels'
import { Avatar, Button, EmptyState, Icon, Spinner } from '../ui'
import { TopBar, useAdmin } from './AdminLayout'
import { Spark } from './Dashboard'
import { Faces, InviteDialog, NameDialog, useSharing } from './Sharing'

type Activity = components['schemas']['Activity']
type Invitation = components['schemas']['Invitation']
type SharingGroup = components['schemas']['Group']

/** Groups (board AdmV2 10): who's in each, what it opens, its limits. */
export function GroupsPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { data } = useSharing()
  const [naming, setNaming] = useState(false)
  const libs = (g: SharingGroup) =>
    g.household
      ? t('sharing.group.everything')
      : g.libraryIds
          .map((id) => data?.libraries.find((l) => l.id === id)?.name)
          .filter(Boolean)
          .join(', ') || t('sharing.group.noLibrary')
  return (
    <>
      <TopBar title={t('sharing.groups.title')}>
        <Button size="sm" onClick={() => setNaming(true)}>
          <Icon name="plus" size={15} />
          {t('sharing.groups.new')}
        </Button>
      </TopBar>
      <div className="ax-body">
        <p className="ax-lead">{t('sharing.groups.foot')}</p>
        {!data ? (
          <Spinner size={24} />
        ) : (
          <section className="ax-card">
            <table className="ax-table ax-table--rows">
              <thead>
                <tr>
                  <th>{t('sharing.group.name')}</th>
                  <th>{t('admin.groups.members')}</th>
                  <th>{t('sharing.libraries.title')}</th>
                  <th>{t('admin.groups.limits')}</th>
                  <th>{t('admin.groups.profiles')}</th>
                  <th aria-hidden="true" />
                </tr>
              </thead>
              <tbody>
                {data.groups.map((g) => (
                  <tr key={g.id} onClick={() => void navigate(`/admin/people/groups/${g.id}`)}>
                    <td className="ax-strong">
                      <Link
                        to={`/admin/people/groups/${g.id}`}
                        className="pl-focusable"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {g.name}
                      </Link>
                    </td>
                    <td>
                      <span className="ax-who">
                        <Faces ids={g.memberIds} users={data.users} />
                        {formatNumber(i18n.language, g.memberIds.length)}
                      </span>
                    </td>
                    <td className="ax-dim ax-clip">{libs(g)}</td>
                    <td className="ax-dim">
                      {[
                        t(`sharing.quality.${g.rights.maxQuality}`),
                        g.rights.maxStreams == null
                          ? t('sharing.streams.unlimited')
                          : t('sharing.streams.count', { count: g.rights.maxStreams }),
                      ].join(t('card.separator'))}
                    </td>
                    <td className="ax-dim">
                      {g.rights.canCreateProfiles
                        ? t('admin.groups.canCreate')
                        : t('admin.groups.cannot')}
                    </td>
                    <td>
                      <Icon name="chevronRight" size={14} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        )}
      </div>
      {naming && (
        <NameDialog
          title={t('sharing.groups.new')}
          onClose={() => setNaming(false)}
          onSave={async (name) => {
            const { data: g, error, response } = await api.POST('/groups', { body: { name } })
            if (error) return failure(error, response)
            setNaming(false)
            void navigate(`/admin/people/groups/${g.id}`)
          }}
        />
      )}
    </>
  )
}

/**
 * Invitations (board AdmV2 12): pending links first, with Copy link and
 * Cancel, then the ones accepted or expired lately; Resend makes a new link
 * for the same person and groups.
 */
export function InvitationsPage() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const { refresh, toast } = useAdmin()
  const { data, reload } = useSharing()
  const [inviting, setInviting] = useState(false)
  const [busy, setBusy] = useState<string>()
  const now = useNow()
  const date = new Intl.DateTimeFormat(lng, { day: 'numeric', month: 'short' })
  const list = data?.history ?? []
  const pending = list.filter((i) => i.status === 'pending')
  const reloadAll = () => void Promise.all([reload(), refresh()])
  const copy = (token: string) =>
    void navigator.clipboard
      .writeText(`${window.location.origin}/invite/${token}`)
      .then(() => toast(t('sharing.invite.copied')), undefined)

  async function cancel(inv: Invitation) {
    setBusy(inv.id)
    await api.DELETE('/invitations/{invitationId}', { params: { path: { invitationId: inv.id } } })
    setBusy(undefined)
    reloadAll()
  }

  async function resend(inv: Invitation) {
    setBusy(inv.id)
    const { data: made } = await api.POST('/invitations', {
      body: { name: inv.name, groupIds: inv.groupIds },
    })
    setBusy(undefined)
    if (made) copy(made.token)
    reloadAll()
  }

  const sent = (iso: string) => {
    const d = new Date(iso)
    return now - d.getTime() < 7 * 864e5 ? formatRelative(lng, d) : date.format(d)
  }
  const expires = (inv: Invitation) => {
    if (inv.status === 'accepted')
      return <span className="ax-dim">{t('admin.invitations.accepted')}</span>
    if (inv.status === 'expired')
      return <span className="ax-dim">{t('admin.invitations.expired')}</span>
    const days = Math.max(1, Math.ceil((new Date(inv.expiresAt).getTime() - now) / 864e5))
    return (
      <span className={days <= 2 ? 'ax-warn' : 'ax-dim'}>
        {t('admin.invitations.inDays', { count: days })}
      </span>
    )
  }

  return (
    <>
      <TopBar title={t('admin.nav.invitations')}>
        <Button size="sm" disabled={!data} onClick={() => setInviting(true)}>
          <Icon name="plus" size={15} />
          {t('sharing.people.invite')}
        </Button>
      </TopBar>
      <div className="ax-body">
        <p className="ax-lead">{t('sharing.invitations.foot')}</p>
        {!data ? (
          <Spinner size={24} />
        ) : list.length === 0 ? (
          <EmptyState title={t('admin.invitations.none')} />
        ) : (
          <section className="ax-card">
            <header className="ax-card__head">
              <h2>{t('admin.invitations.pending', { count: pending.length })}</h2>
            </header>
            <table className="ax-table">
              <thead>
                <tr>
                  <th>{t('admin.invitations.for')}</th>
                  <th>{t('admin.groups.group')}</th>
                  <th>{t('admin.invitations.sent')}</th>
                  <th>{t('admin.invitations.expires')}</th>
                  <th aria-hidden="true" />
                </tr>
              </thead>
              <tbody>
                {list.map((inv) => (
                  <tr key={inv.id}>
                    <td className="ax-strong">
                      {inv.userId ? (
                        <Link to={`/admin/people/${inv.userId}`} className="pl-focusable">
                          {inv.name}
                        </Link>
                      ) : (
                        inv.name
                      )}
                    </td>
                    <td className="ax-dim">
                      {inv.groupIds
                        .map((id) => data.groups.find((g) => g.id === id)?.name)
                        .filter(Boolean)
                        .join(', ')}
                    </td>
                    <td className="ax-dim">{sent(inv.createdAt)}</td>
                    <td>{expires(inv)}</td>
                    <td>
                      <span className="ax-actions">
                        {inv.status === 'pending' ? (
                          <>
                            {inv.token && (
                              <Button
                                variant="secondary"
                                size="sm"
                                aria-label={t('admin.invitations.copyFor', { name: inv.name })}
                                onClick={() => copy(inv.token!)}
                              >
                                <Icon name="copy" size={14} />
                                {t('admin.invitations.copy')}
                              </Button>
                            )}
                            <Button
                              variant="secondary"
                              size="sm"
                              loading={busy === inv.id}
                              aria-label={t('admin.invitations.cancelFor', { name: inv.name })}
                              onClick={() => void cancel(inv)}
                            >
                              {t('sharing.invitations.cancel')}
                            </Button>
                          </>
                        ) : inv.status === 'accepted' ? (
                          <span className="ax-chip is-ok">{t('admin.invitations.accepted')}</span>
                        ) : (
                          <>
                            <span className="ax-chip">{t('admin.invitations.expired')}</span>
                            <Button
                              variant="secondary"
                              size="sm"
                              loading={busy === inv.id}
                              aria-label={t('admin.invitations.resendFor', { name: inv.name })}
                              onClick={() => void resend(inv)}
                            >
                              {t('admin.invitations.resend')}
                            </Button>
                          </>
                        )}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        )}
      </div>
      {inviting && data && (
        <InviteDialog
          groups={data.groups}
          onClose={() => {
            setInviting(false)
            reloadAll()
          }}
        />
      )}
    </>
  )
}

/** A clock read once per render pass, for "in 2 d" and "2 h ago". */
function useNow() {
  const [now] = useState(() => Date.now())
  return now
}

const periods = [
  { days: 7, key: 'admin.activity.week' },
  { days: 30, key: 'admin.activity.month' },
  { days: 365, key: 'admin.activity.year' },
] as const

/**
 * Activity (board AdmV2 08): the week's numbers on top (hours with their
 * curve, people, the most watched title, Direct Play's share), then the
 * last plays over a week, a month or a year, for everyone or one person.
 */
export function ActivityPage() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const [days, setDays] = useState<number>(7)
  const [person, setPerson] = useState('')
  const { data: users } = usePolling(async () => (await api.GET('/users')).data ?? null, 300_000)
  const tz = new Date().getTimezoneOffset()
  const query = (d: number) => ({
    days: d,
    tzOffsetMinutes: tz,
    ...(person ? { userId: person } : {}),
  })
  const { data: week } = usePolling(
    async (): Promise<Activity | null> =>
      (await api.GET('/activity', { params: { query: query(7) } })).data ?? null,
    60_000,
    [person],
  )
  const { data } = usePolling(
    async (): Promise<Activity | null> =>
      (await api.GET('/activity', { params: { query: query(days) } })).data ?? null,
    60_000,
    [days, person],
  )
  const n = (v: number) => formatNumber(lng, v)
  const hours = week?.days.reduce((a, d) => a + d.hours, 0) ?? 0
  const tot = week?.totals
  return (
    <>
      <TopBar title={t('admin.nav.activity')}>
        <select
          className="ax-filter"
          value={person}
          aria-label={t('admin.activity.person')}
          onChange={(e) => setPerson(e.target.value)}
        >
          <option value="">{t('admin.activity.everyone')}</option>
          {(users ?? []).map((u) => (
            <option key={u.id} value={u.id}>
              {u.displayName}
            </option>
          ))}
        </select>
      </TopBar>
      <div className="ax-body">
        {!week || !data ? (
          <Spinner size={24} />
        ) : (
          <>
            <div className="ax-kpis">
              <section className="ax-card ax-kpi">
                <p>{t('admin.activity.hoursWeek')}</p>
                <strong>{n(Math.round(hours * 10) / 10)}</strong>
                <Spark values={week.days.map((d) => d.hours)} />
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.activity.people')}</p>
                <strong>{n(tot?.people ?? 0)}</strong>
                <span>{t('admin.activity.plays', { count: tot?.plays ?? 0 })}</span>
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.activity.most')}</p>
                <strong className="ax-kpi__title">{tot?.top?.title ?? '—'}</strong>
                {tot?.top && <span>{t('admin.activity.plays', { count: tot.top.plays })}</span>}
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.activity.directPlay')}</p>
                <strong>{tot?.plays ? formatPercent(lng, tot.directPlay / tot.plays) : '—'}</strong>
                <span>{t('admin.activity.directHelp')}</span>
              </section>
            </div>
            <section className="ax-card">
              <header className="ax-card__head">
                <h2>{t('admin.activity.recent')}</h2>
                <span className="ax-tabs" role="tablist" aria-label={t('admin.activity.period')}>
                  {periods.map((p) => (
                    <button
                      key={p.days}
                      type="button"
                      role="tab"
                      aria-selected={days === p.days}
                      className={`ax-tab pl-focusable${days === p.days ? ' is-on' : ''}`}
                      onClick={() => setDays(p.days)}
                    >
                      {t(p.key)}
                    </button>
                  ))}
                </span>
              </header>
              {data.recent.length === 0 ? (
                <p className="ax-none">{t('admin.activity.none')}</p>
              ) : (
                <table className="ax-table">
                  <thead>
                    <tr>
                      <th>{t('admin.dash.colWho')}</th>
                      <th>{t('admin.activity.watched')}</th>
                      <th>{t('admin.activity.when')}</th>
                      <th>{t('admin.activity.how')}</th>
                      <th>{t('admin.activity.length')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.recent.map((p) => (
                      <tr key={`${p.itemId}-${p.startedAt}`}>
                        <td>
                          <span className="ax-who">
                            <Avatar name={p.profileName} size={22} />
                            <span>
                              <b className="ax-strong">{p.displayName}</b>
                              <small className="ax-dim">
                                {p.deviceName}
                                {t('card.separator')}
                                {p.remote ? t('admin.dash.remote') : t('admin.dash.local')}
                              </small>
                            </span>
                          </span>
                        </td>
                        <td className="ax-strong">
                          <Link to={`/item/${p.itemId}`} className="pl-focusable">
                            {p.title}
                          </Link>
                        </td>
                        <td className="ax-dim">{formatRelative(lng, new Date(p.startedAt))}</td>
                        <td>
                          <span
                            className={`ax-chip ${p.method === 'transcode' ? 'is-warning' : 'is-ok'}`}
                          >
                            {methodLabel(t, p.method)}
                          </span>
                        </td>
                        <td className="ax-dim">{formatDuration(lng, p.watchedMs)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
          </>
        )}
      </div>
    </>
  )
}

/**
 * Unmatched (board AdmV2 09): how much isn't understood or needs a look,
 * the share matched, then each library with its links.
 */
export function UnmatchedPage() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const navigate = useNavigate()
  const { data } = useAdmin()
  const { data: status } = usePolling(
    async () => (await api.GET('/metadata/status')).data ?? null,
    30_000,
  )
  const n = (v: number) => formatNumber(lng, v)
  const libs = data?.libraries ?? []
  const files = libs.reduce((a, l) => a + l.counts.unrecognized, 0)
  const per = status?.libraries ?? []
  const review = per.reduce((a, c) => a + c.needsReview + c.unmatched, 0)
  const matched = per.reduce((a, c) => a + c.matched, 0)
  const total = matched + review
  return (
    <>
      <TopBar title={t('admin.nav.unmatched')} />
      <div className="ax-body">
        {!data ? (
          <Spinner size={24} />
        ) : !libs.length ? (
          <EmptyState title={t('libraries.empty')} />
        ) : (
          <>
            <div className="ax-kpis ax-kpis--3">
              <section className="ax-card ax-kpi">
                <p>{t('admin.unmatched.files')}</p>
                <strong>{n(files)}</strong>
                <span>{t('admin.unmatched.filesHelp')}</span>
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.unmatched.review')}</p>
                <strong>{status ? n(review) : '—'}</strong>
                <span>{t('admin.unmatched.reviewHelp')}</span>
              </section>
              <section className="ax-card ax-kpi">
                <p>{t('admin.unmatched.matched')}</p>
                <strong>{total ? formatPercent(lng, matched / total) : '—'}</strong>
                <span>{t('admin.unmatched.matchedOf', { count: total })}</span>
              </section>
            </div>
            <section className="ax-card">
              <header className="ax-card__head">
                <h2>{t('admin.unmatched.byLibrary')}</h2>
              </header>
              <table className="ax-table ax-table--rows">
                <thead>
                  <tr>
                    <th>{t('admin.libraries.colLibrary')}</th>
                    <th>{t('admin.unmatched.files')}</th>
                    <th>{t('admin.unmatched.review')}</th>
                  </tr>
                </thead>
                <tbody>
                  {libs.map((l) => {
                    const r = per.find((x) => x.libraryId === l.id)
                    const titles = r ? r.needsReview + r.unmatched : 0
                    // The whole row opens what's left to do: the files first, then the titles.
                    const to = l.counts.unrecognized
                      ? `/admin/libraries/${l.id}/unrecognized`
                      : titles
                        ? `/admin/libraries/${l.id}/review`
                        : `/admin/libraries/${l.id}`
                    return (
                      <tr key={l.id} onClick={() => void navigate(to)}>
                        <td className="ax-strong">
                          <Link
                            to={`/admin/libraries/${l.id}`}
                            className="pl-focusable"
                            onClick={(e) => e.stopPropagation()}
                          >
                            {l.name}
                          </Link>
                        </td>
                        <td>
                          {l.counts.unrecognized ? (
                            <Link
                              to={`/admin/libraries/${l.id}/unrecognized`}
                              className="ax-link pl-focusable"
                              onClick={(e) => e.stopPropagation()}
                            >
                              {t('admin.unmatched.filesCount', { count: l.counts.unrecognized })}
                            </Link>
                          ) : (
                            <span className="ax-dim">{t('admin.unmatched.noneShort')}</span>
                          )}
                        </td>
                        <td>
                          {titles > 0 ? (
                            <Link
                              to={`/admin/libraries/${l.id}/review`}
                              className="ax-link pl-focusable"
                              onClick={(e) => e.stopPropagation()}
                            >
                              {t('admin.unmatched.titlesCount', { count: titles })}
                            </Link>
                          ) : (
                            <span className="ax-dim">
                              {r ? t('admin.unmatched.noneShort') : n(0)}
                            </span>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </section>
          </>
        )}
      </div>
    </>
  )
}
