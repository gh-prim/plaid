import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { failure } from '../auth/errors'
import { usePolling } from '../hooks/usePolling'
import { formatDuration, formatNumber, formatRelative } from '../lib/format'
import { methodLabel } from '../player/labels'
import { Avatar, Button, EmptyState, Icon, Spinner } from '../ui'
import { TopBar, useAdmin } from './AdminLayout'
import {
  Faces,
  InvitationsGroup,
  InviteDialog,
  NameDialog,
  useRightsSummary,
  useSharing,
} from './Sharing'

type Activity = components['schemas']['Activity']

/** Groups (step 21 sidebar): every group with who's in it and what it gives (F009). */
export function GroupsPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { data } = useSharing()
  const summary = useRightsSummary()
  const [naming, setNaming] = useState(false)
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
                  <th>{t('people.title')}</th>
                  <th>{t('sharing.libraries.title')}</th>
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
                    <td className="ax-dim">{summary(g, data.libraries)}</td>
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

/** Invitations (step 21 sidebar): the links not used yet, and Invite (F009). */
export function InvitationsPage() {
  const { t } = useTranslation()
  const { refresh } = useAdmin()
  const { data, reload } = useSharing()
  const [inviting, setInviting] = useState(false)
  return (
    <>
      <TopBar title={t('admin.nav.invitations')}>
        <Button size="sm" disabled={!data} onClick={() => setInviting(true)}>
          <Icon name="plus" size={15} />
          {t('sharing.people.invite')}
        </Button>
      </TopBar>
      <div className="ax-body ax-narrow">
        {!data ? (
          <Spinner size={24} />
        ) : data.invitations.length === 0 ? (
          <EmptyState title={t('admin.invitations.none')} />
        ) : (
          <InvitationsGroup
            invitations={data.invitations}
            groups={data.groups}
            onChange={() => void Promise.all([reload(), refresh()])}
          />
        )}
      </div>
      {inviting && data && (
        <InviteDialog
          groups={data.groups}
          onClose={() => {
            setInviting(false)
            void Promise.all([reload(), refresh()])
          }}
        />
      )}
    </>
  )
}

/** Activity (step 21 sidebar): hours watched per day over a month, and the last plays. */
export function ActivityPage() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const [days, setDays] = useState(30)
  const { data } = usePolling(
    async (): Promise<Activity | null> =>
      (
        await api.GET('/activity', {
          params: { query: { days, tzOffsetMinutes: new Date().getTimezoneOffset() } },
        })
      ).data ?? null,
    60_000,
  )
  const n = (v: number) => formatNumber(lng, v)
  const list = data?.days ?? []
  const most = Math.max(1, ...list.map((d) => d.hours))
  const total = list.reduce((a, d) => a + d.hours, 0)
  const date = new Intl.DateTimeFormat(lng, { day: 'numeric', month: 'short' })
  return (
    <>
      <TopBar
        title={t('admin.nav.activity')}
        sub={t('admin.activity.total', { hours: n(Math.round(total)) })}
      >
        <select
          className="ax-filter"
          value={days}
          aria-label={t('admin.activity.period')}
          onChange={(e) => setDays(Number(e.target.value))}
        >
          {[7, 30, 90].map((d) => (
            <option key={d} value={d}>
              {t('admin.activity.days', { count: d })}
            </option>
          ))}
        </select>
      </TopBar>
      <div className="ax-body">
        {!data ? (
          <Spinner size={24} />
        ) : (
          <>
            <section className="ax-card ax-activity">
              <header className="ax-card__head">
                <h2>{t('admin.dash.hoursPerDay')}</h2>
              </header>
              <ol className="ax-bars ax-bars--dense">
                {list.map((d, i) => (
                  <li
                    key={d.date}
                    title={`${date.format(new Date(`${d.date}T12:00:00`))} · ${n(Math.round(d.hours * 10) / 10)} h`}
                  >
                    <span
                      className={`ax-bars__bar${i === list.length - 1 ? ' is-today' : ''}`}
                      style={{ height: `${Math.max(2, (d.hours / most) * 100)}%` }}
                    />
                  </li>
                ))}
              </ol>
            </section>
            <section className="ax-card">
              <header className="ax-card__head">
                <h2>{t('admin.activity.recent')}</h2>
                <span>{n(data.recent.length)}</span>
              </header>
              {data.recent.length === 0 ? (
                <p className="ax-none">{t('admin.activity.none')}</p>
              ) : (
                <table className="ax-table">
                  <thead>
                    <tr>
                      <th>{t('admin.dash.colTitle')}</th>
                      <th>{t('admin.dash.colWho')}</th>
                      <th>{t('admin.dash.colDevice')}</th>
                      <th>{t('admin.activity.how')}</th>
                      <th>{t('admin.activity.watched')}</th>
                      <th>{t('admin.activity.when')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.recent.map((p) => (
                      <tr key={`${p.itemId}-${p.startedAt}`}>
                        <td className="ax-strong">
                          <Link to={`/item/${p.itemId}`} className="pl-focusable">
                            {p.title}
                          </Link>
                        </td>
                        <td>
                          <span className="ax-who">
                            <Avatar name={p.profileName} size={22} />
                            {p.profileName}
                          </span>
                        </td>
                        <td className="ax-dim">
                          {p.deviceName}
                          {t('card.separator')}
                          {p.remote ? t('admin.dash.remote') : t('admin.dash.local')}
                        </td>
                        <td>
                          <span
                            className={`ax-chip ${p.method === 'transcode' ? 'is-warning' : 'is-ok'}`}
                          >
                            {methodLabel(t, p.method)}
                          </span>
                        </td>
                        <td className="ax-dim">{formatDuration(lng, p.watchedMs)}</td>
                        <td className="ax-dim">{formatRelative(lng, new Date(p.startedAt))}</td>
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

/** Unmatched (step 21 sidebar): per library, the files not understood and the titles to check. */
export function UnmatchedPage() {
  const { t, i18n } = useTranslation()
  const { data } = useAdmin()
  const { data: status } = usePolling(
    async () => (await api.GET('/metadata/status')).data ?? null,
    30_000,
  )
  const n = (v: number) => formatNumber(i18n.language, v)
  const libs = data?.libraries ?? []
  return (
    <>
      <TopBar title={t('admin.nav.unmatched')} />
      <div className="ax-body">
        {!data ? (
          <Spinner size={24} />
        ) : !libs.length ? (
          <EmptyState title={t('libraries.empty')} />
        ) : (
          <section className="ax-card">
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
                  const review = status?.libraries.find((x) => x.libraryId === l.id)
                  return (
                    <tr key={l.id}>
                      <td className="ax-strong">
                        <Link to={`/admin/libraries/${l.id}`} className="pl-focusable">
                          {l.name}
                        </Link>
                      </td>
                      <td>
                        {l.counts.unrecognized ? (
                          <Link
                            to={`/admin/libraries/${l.id}/unrecognized`}
                            className="ax-link pl-focusable"
                          >
                            {t('admin.unmatched.filesCount', { count: l.counts.unrecognized })}
                          </Link>
                        ) : (
                          <span className="ax-dim">{t('admin.unmatched.noneShort')}</span>
                        )}
                      </td>
                      <td>
                        {review && review.needsReview + review.unmatched > 0 ? (
                          <Link
                            to={`/admin/libraries/${l.id}/review`}
                            className="ax-link pl-focusable"
                          >
                            {t('admin.unmatched.titlesCount', {
                              count: review.needsReview + review.unmatched,
                            })}
                          </Link>
                        ) : (
                          <span className="ax-dim">
                            {review ? t('admin.unmatched.noneShort') : n(0)}
                          </span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </section>
        )}
      </div>
    </>
  )
}
