import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { formatRelative } from '../lib/format'
import { checkAccount } from '../onboarding/Account'
import {
  Avatar,
  avatarSrc,
  Button,
  ChoiceRow,
  Dialog,
  EmptyState,
  Icon,
  Mark,
  TextField,
} from '../ui'
import { useAdmin } from './AdminLayout'
import { InvitationsGroup, InviteDialog, NameDialog, PersonSharing, useSharing } from './Sharing'
import { Group, Loading, Row, Section, Switch } from './v2'

type AdminUser = components['schemas']['AdminUser']
type SessionItem = components['schemas']['SessionListItem']

/** A temporary password the admin passes on: 4 words of letters, easy to read out. */
export function temporaryPassword(
  random: (n: number) => number = (n) => crypto.getRandomValues(new Uint32Array(1))[0]! % n,
): string {
  const letters = 'abcdefghijkmnpqrstuvwxyz'
  return Array.from({ length: 4 }, () =>
    Array.from({ length: 4 }, () => letters[random(letters.length)]).join(''),
  ).join('-')
}

/** The same party color Avatar gives a name (F003 profile color index 0–7). */
const partyOf = (name: string) => ((name.codePointAt(0) ?? 0) % 8) + 1

type Filter = 'watching' | 'away' | 'invitations'

/** 30 days, for "Not seen in 30 days". */
const awayMs = 30 * 24 * 3600 * 1000

/**
 * People (canvas step 19, boards Share01 and Share03): the groups as chips
 * over a table of everyone, a search, filters, Invite, and actions on the
 * people selected (F009, F007 R23).
 */
export function People() {
  const { t, i18n } = useTranslation()
  const locale = i18n.resolvedLanguage ?? 'en'
  const navigate = useNavigate()
  const { data: admin, toast } = useAdmin()
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me?.user.id : undefined
  const sharing = useSharing()
  const [group, setGroup] = useState<string>()
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>()
  const [selected, setSelected] = useState<string[]>([])
  const [dialog, setDialog] = useState<'invite' | 'add' | 'group' | 'move' | 'revoke'>()
  const [moveTo, setMoveTo] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  // "Not seen in 30 days" counts from when the page opened.
  const [now] = useState(() => Date.now())

  const data = sharing.data
  if (!data)
    return (
      <div className="adm-sec sh-people">
        <PeopleHead />
        <Loading />
      </div>
    )

  const watching = new Set((admin?.streams ?? []).map((s) => s.userId))
  const groupName = (id: string) => data.groups.find((g) => g.id === id)?.name
  const libraryNames = (u: AdminUser) =>
    u.access.libraryIds == null
      ? t('sharing.people.allLibraries')
      : u.access.libraryIds
          .map((id) => data.libraries.find((l) => l.id === id)?.name)
          .filter(Boolean)
          .join(', ') || t('sharing.group.noLibrary')
  const q = query.trim().toLocaleLowerCase(locale)
  const shown = data.users.filter(
    (u) =>
      (!group || u.groupIds.includes(group)) &&
      (!q ||
        u.displayName.toLocaleLowerCase(locale).includes(q) ||
        u.username.toLocaleLowerCase(locale).includes(q)) &&
      (filter !== 'watching' || watching.has(u.id)) &&
      (filter !== 'away' || !u.lastSeenAt || now - new Date(u.lastSeenAt).getTime() > awayMs),
  )
  const selectable = shown.filter((u) => u.id !== me)
  const picked = data.users.filter((u) => selected.includes(u.id))
  const current = data.groups.find((g) => g.id === group)

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]))
  const close = () => {
    setDialog(undefined)
    setFail(undefined)
  }
  async function each(body: components['schemas']['UpdateUserRequest'], done: string) {
    setBusy(true)
    setFail(undefined)
    for (const u of picked) {
      const { error, response } = await api.PATCH('/users/{userId}', {
        params: { path: { userId: u.id } },
        body,
      })
      if (error) {
        setBusy(false)
        return setFail(failure(error, response))
      }
    }
    setBusy(false)
    close()
    setSelected([])
    toast(done)
    await sharing.reload()
  }

  return (
    <div className="adm-sec sh-people">
      <PeopleHead />

      <div className="sh-chips" role="toolbar" aria-label={t('sharing.groups.title')}>
        <button
          type="button"
          className={`sh-chip pl-focusable${group ? '' : ' is-on'}`}
          aria-pressed={!group}
          onClick={() => setGroup(undefined)}
        >
          {t('sharing.people.everyone')}
          <span>{formatCount(locale, data.users.length)}</span>
        </button>
        {data.groups.map((g) => (
          <button
            key={g.id}
            type="button"
            className={`sh-chip pl-focusable${group === g.id ? ' is-on' : ''}`}
            aria-pressed={group === g.id}
            onClick={() => setGroup(group === g.id ? undefined : g.id)}
          >
            {g.name}
            <span>{formatCount(locale, g.memberIds.length)}</span>
          </button>
        ))}
        <button
          type="button"
          className="sh-chip sh-chip--ghost pl-focusable"
          onClick={() => setDialog('group')}
        >
          <Icon name="plus" size={16} />
          {t('sharing.groups.new')}
        </button>
        {current && (
          <Link
            className="sh-chip sh-chip--ghost pl-focusable"
            to={`/admin/people/groups/${current.id}`}
          >
            {t('sharing.people.groupSettings', { name: current.name })}
            <Icon name="chevronRight" size={14} />
          </Link>
        )}
      </div>

      <div className="sh-toolbar">
        <label className="sh-search">
          <Icon name="search" size={18} />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t('sharing.people.search', { count: data.users.length })}
            aria-label={t('sharing.people.search', { count: data.users.length })}
          />
        </label>
        {(['watching', 'away', 'invitations'] as const).map((f) => (
          <button
            key={f}
            type="button"
            className={`sh-filter pl-focusable${filter === f ? ' is-on' : ''}`}
            aria-pressed={filter === f}
            onClick={() => setFilter(filter === f ? undefined : f)}
          >
            {f === 'watching'
              ? t('sharing.people.watching')
              : f === 'away'
                ? t('sharing.people.away')
                : t('sharing.invitations.title', { count: data.invitations.length })}
          </button>
        ))}
        <Button variant="ghost" onClick={() => setDialog('add')}>
          {t('people.add')}
        </Button>
        <Button onClick={() => setDialog('invite')}>
          <Icon name="plus" size={16} />
          {t('sharing.people.invite')}
        </Button>
      </div>

      {filter === 'invitations' ? (
        <div className="sh-invites">
          <InvitationsGroup
            invitations={data.invitations}
            groups={data.groups}
            onChange={() => void sharing.reload()}
          />
        </div>
      ) : (
        <div className="sh-table" role="table" aria-label={t('people.title')}>
          <div className="sh-tr sh-tr--head" role="row">
            <span className="sh-td sh-td--check" role="columnheader">
              <input
                type="checkbox"
                aria-label={t('sharing.people.selectAll')}
                checked={selectable.length > 0 && selectable.every((u) => selected.includes(u.id))}
                onChange={(e) => setSelected(e.target.checked ? selectable.map((u) => u.id) : [])}
              />
            </span>
            <span className="sh-td sh-td--name" role="columnheader">
              {t('sharing.people.colName')}
            </span>
            <span className="sh-td sh-td--group" role="columnheader">
              {t('sharing.people.colGroup')}
            </span>
            <span className="sh-td sh-td--libs" role="columnheader">
              {t('sharing.people.colLibraries')}
            </span>
            <span className="sh-td sh-td--seen" role="columnheader">
              {t('sharing.people.colSeen')}
            </span>
            <span className="sh-td sh-td--go" aria-hidden="true" />
          </div>
          {shown.map((u) => (
            <div
              key={u.id}
              className={`sh-tr${selected.includes(u.id) ? ' is-selected' : ''}${u.disabled ? ' is-muted' : ''}`}
              role="row"
            >
              <span className="sh-td sh-td--check" role="cell">
                {u.id !== me && (
                  <input
                    type="checkbox"
                    aria-label={t('sharing.people.select', { name: u.displayName })}
                    checked={selected.includes(u.id)}
                    onChange={() => toggle(u.id)}
                  />
                )}
              </span>
              <Link
                className="sh-td sh-td--name pl-focusable"
                role="cell"
                to={`/admin/people/${u.id}`}
              >
                <Avatar
                  name={u.displayName}
                  color={u.avatarColor + 1}
                  picture={u.avatar.url}
                  size={32}
                />
                <span className="sh-who">
                  <b>{u.displayName}</b>
                  {u.id === me && <small>{t('people.thisIsYou')}</small>}
                  {u.disabled && <small>{t('people.disabled')}</small>}
                </span>
              </Link>
              <span className="sh-td sh-td--group" role="cell">
                {u.role === 'admin'
                  ? t('people.role.admin')
                  : u.groupIds.map(groupName).filter(Boolean).join(', ') ||
                    t('sharing.person.noGroups')}
              </span>
              <span className="sh-td sh-td--libs" role="cell">
                {libraryNames(u)}
              </span>
              <span className="sh-td sh-td--seen" role="cell">
                {watching.has(u.id) ? (
                  <span className="sh-live">
                    <Icon name="play" size={12} />
                    {t('sharing.people.watching')}
                  </span>
                ) : u.lastSeenAt ? (
                  formatRelative(locale, new Date(u.lastSeenAt))
                ) : (
                  t('sharing.person.neverSeen')
                )}
              </span>
              <button
                type="button"
                className="sh-td sh-td--go pl-focusable"
                aria-label={t('sharing.people.open', { name: u.displayName })}
                onClick={() => void navigate(`/admin/people/${u.id}`)}
              >
                <Icon name="chevronRight" size={16} />
              </button>
            </div>
          ))}
          {!shown.length && <p className="sh-empty">{t('sharing.people.none')}</p>}
        </div>
      )}

      {selected.length > 0 && (
        <div
          className="sh-bulk br-glass"
          role="region"
          aria-label={t('sharing.people.selected', { count: selected.length })}
        >
          <span>{t('sharing.people.selected', { count: selected.length })}</span>
          <Button
            variant="secondary"
            onClick={() => {
              setMoveTo(data.groups[0]?.id)
              setDialog('move')
            }}
          >
            {t('sharing.people.move')}
          </Button>
          <Button variant="secondary" className="sh-danger" onClick={() => setDialog('revoke')}>
            {t('sharing.person.suspend')}
          </Button>
          <Button variant="ghost" onClick={() => setSelected([])}>
            {t('common.cancel')}
          </Button>
        </div>
      )}

      {dialog === 'invite' && (
        <InviteDialog
          groups={data.groups}
          groupId={group}
          onClose={() => {
            close()
            void sharing.reload()
          }}
        />
      )}
      {dialog === 'add' && (
        <Dialog title={t('people.add')} actions={null}>
          <AddPerson
            groups={data.groups}
            onDone={() => {
              close()
              void sharing.reload()
            }}
          />
        </Dialog>
      )}
      {dialog === 'group' && (
        <NameDialog
          title={t('sharing.groups.new')}
          onClose={close}
          onSave={async (name) => {
            const { data: g, error, response } = await api.POST('/groups', { body: { name } })
            if (error) return failure(error, response)
            close()
            void navigate(`/admin/people/groups/${g.id}`)
          }}
        />
      )}
      {dialog === 'move' && (
        <Dialog
          title={t('sharing.people.moveTitle', { count: picked.length })}
          actions={
            <>
              <Button variant="ghost" onClick={close}>
                {t('common.cancel')}
              </Button>
              <Button
                loading={busy}
                disabled={!moveTo}
                onClick={() =>
                  void each(
                    { groupIds: [moveTo!] },
                    t('sharing.people.moved', { count: picked.length, group: groupName(moveTo!) }),
                  )
                }
              >
                {t('sharing.people.move')}
              </Button>
            </>
          }
        >
          <p className="adm__text">{t('sharing.people.moveHelp')}</p>
          <div className="ob__choices">
            {data.groups.map((g) => (
              <ChoiceRow
                key={g.id}
                name="move"
                label={g.name}
                checked={moveTo === g.id}
                onChange={() => setMoveTo(g.id)}
              />
            ))}
          </div>
          {fail && (
            <p role="alert" className="adm__text pl-tone--error">
              {errorMessage(t, fail)}
            </p>
          )}
        </Dialog>
      )}
      {dialog === 'revoke' && (
        <Dialog
          title={t('sharing.people.revokeTitle', { count: picked.length })}
          actions={
            <>
              <Button variant="ghost" onClick={close}>
                {t('common.cancel')}
              </Button>
              <Button
                loading={busy}
                onClick={() =>
                  void each(
                    { disabled: true },
                    t('sharing.people.revoked', { count: picked.length }),
                  )
                }
              >
                {t('sharing.person.suspend')}
              </Button>
            </>
          }
        >
          <p className="adm__text">{t('sharing.person.suspendHelp')}</p>
          {fail && (
            <p role="alert" className="adm__text pl-tone--error">
              {errorMessage(t, fail)}
            </p>
          )}
        </Dialog>
      )}
    </div>
  )
}

/** The page's head: back to Settings, the title. */
function PeopleHead() {
  const { t } = useTranslation()
  return (
    <header className="adm-sec__head">
      <Link
        to="/admin/settings"
        className="adm-sec__back br-glass pl-focusable"
        aria-label={t('admin.backTo', { name: t('admin.tab.settings') })}
      >
        <Icon name="chevronLeft" size={18} />
        <span>{t('admin.tab.settings')}</span>
      </Link>
      <h1 className="adm-sec__title">{t('people.title')}</h1>
    </header>
  )
}

const formatCount = (locale: string, n: number) => new Intl.NumberFormat(locale).format(n)

function AddPerson({
  groups,
  onDone,
}: {
  groups: components['schemas']['Group'][]
  onDone: () => void
}) {
  const { t, i18n } = useTranslation()
  const [form, setForm] = useState({
    displayName: '',
    username: '',
    role: 'member' as 'member' | 'admin',
  })
  // A new person starts in the household group, like members always did.
  const [groupIds, setGroupIds] = useState<string[]>(() =>
    groups
      .filter((g) => g.household)
      .map((g) => g.id)
      .slice(0, 1),
  )
  const [password] = useState(() => temporaryPassword())
  const [checked, setChecked] = useState(false)
  const [fail, setFail] = useState<Failure>()
  const [saving, setSaving] = useState(false)
  const local = checked ? checkAccount({ ...form, password, setupCode: '' }) : {}
  const server = fieldCodes(fail)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setChecked(true)
    if (Object.keys(checkAccount({ ...form, password, setupCode: '' })).length) return
    setSaving(true)
    const { error, response } = await api.POST('/users', {
      body: {
        displayName: form.displayName.trim(),
        username: form.username.toLowerCase(),
        temporaryPassword: password,
        role: form.role,
        locale: i18n.resolvedLanguage,
        groupIds: form.role === 'member' ? groupIds : [],
      },
    })
    setSaving(false)
    if (error) return setFail(failure(error, response))
    onDone()
  }

  return (
    <form className="adm__form" onSubmit={(e) => void submit(e)} noValidate>
      <TextField
        label={t('onboarding.account.displayName.label')}
        value={form.displayName}
        onValue={(v) => setForm((f) => ({ ...f, displayName: v }))}
        error={fieldMessage(t, local.displayName ?? server.displayName)}
      />
      <TextField
        label={t('onboarding.account.username.label')}
        value={form.username}
        onValue={(v) => setForm((f) => ({ ...f, username: v }))}
        autoCapitalize="none"
        error={
          fail?.error?.code === 'username_taken'
            ? t('errors.username_taken')
            : fieldMessage(t, local.username ?? server.username)
        }
      />
      <div className="pl pl-field">
        <span className="pl-field__label">{t('people.tempPassword.label')}</span>
        <code className="adm__path">{password}</code>
        <span className="pl-field__help">{t('people.tempPassword.help')}</span>
      </div>
      <fieldset className="ob__fieldset">
        <legend className="ob__legend">{t('people.role.label')}</legend>
        <div className="ob__choices">
          <ChoiceRow
            name="role"
            label={t('people.role.user')}
            checked={form.role === 'member'}
            onChange={() => setForm((f) => ({ ...f, role: 'member' }))}
          />
          <ChoiceRow
            name="role"
            label={t('people.role.admin')}
            checked={form.role === 'admin'}
            onChange={() => setForm((f) => ({ ...f, role: 'admin' }))}
          />
        </div>
      </fieldset>
      {form.role === 'member' && groups.length > 0 && (
        <fieldset className="ob__fieldset">
          <legend className="ob__legend">{t('sharing.invite.groups')}</legend>
          <ul className="sh__checks">
            {groups.map((g) => (
              <li key={g.id}>
                <label className="sh__check">
                  <input
                    type="checkbox"
                    checked={groupIds.includes(g.id)}
                    onChange={(e) =>
                      setGroupIds((p) =>
                        e.target.checked ? [...p, g.id] : p.filter((x) => x !== g.id),
                      )
                    }
                  />
                  <span>{g.name}</span>
                </label>
              </li>
            ))}
          </ul>
        </fieldset>
      )}
      {fail && !['username_taken', 'validation_failed'].includes(fail.error?.code ?? '') && (
        <p role="alert" className="ob__folder-error">
          {errorMessage(t, fail)}
        </p>
      )}
      <div className="adm__actions">
        <Button type="submit" loading={saving}>
          {t('people.add')}
        </Button>
        <Button variant="ghost" onClick={onDone}>
          {t('common.cancel')}
        </Button>
      </div>
    </form>
  )
}

/** One person: access, devices, password, delete (board TA06, F007 R23). */
export function Person() {
  const { t, i18n } = useTranslation()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { toast } = useAdmin()
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me?.user.id : undefined
  const locale = i18n.resolvedLanguage ?? 'en'
  const [users, setUsers] = useState<AdminUser[]>()
  const [sessions, setSessions] = useState<SessionItem[]>()
  const [fail, setFail] = useState<Failure>()
  const [reset, setReset] = useState<string>()
  const [deleting, setDeleting] = useState(false)
  const [busy, setBusy] = useState<string>()
  const back = { to: '/admin/people', label: t('people.title') }

  const load = useCallback(async () => {
    const [u, s] = await Promise.all([
      api.GET('/users'),
      api.GET('/users/{userId}/sessions', { params: { path: { userId: id } } }),
    ])
    setUsers(u.data ?? [])
    setSessions(s.data ?? [])
  }, [id])
  useEffect(() => {
    Promise.all([
      api.GET('/users'),
      api.GET('/users/{userId}/sessions', { params: { path: { userId: id } } }),
    ]).then(([u, s]) => {
      setUsers(u.data ?? [])
      setSessions(s.data ?? [])
    })
  }, [id])

  const user = users?.find((x) => x.id === id)
  if (!user)
    return (
      <Section title={t('people.title')} back={back} glyph="people">
        {users ? <EmptyState title={t('people.empty')} /> : <Loading />}
      </Section>
    )

  // The last admin can't stop being one, nor go (the server refuses it too).
  const onlyAdmin =
    user.role === 'admin' &&
    !user.disabled &&
    (users ?? []).filter((u) => u.role === 'admin' && !u.disabled).length === 1

  async function update(key: string, body: components['schemas']['UpdateUserRequest']) {
    setFail(undefined)
    setBusy(key)
    const { error, response } = await api.PATCH('/users/{userId}', {
      params: { path: { userId: id } },
      body,
    })
    setBusy(undefined)
    if (error) return setFail(failure(error, response))
    await load()
  }

  async function resetPassword() {
    const temp = temporaryPassword()
    setBusy('reset')
    const { error, response } = await api.POST('/users/{userId}/password', {
      params: { path: { userId: id } },
      body: { temporaryPassword: temp },
    })
    setBusy(undefined)
    if (error) return setFail(failure(error, response))
    setReset(temp)
    await load()
  }

  const party = partyOf(user.displayName)
  return (
    <Section
      title={user.displayName}
      back={back}
      tile={
        user.avatar.url ? (
          <img className="sh__tile-img" src={avatarSrc(user.avatar.url, 160)} alt="" />
        ) : (
          <span className="adm-sec__initial">
            {user.displayName.trim().charAt(0).toLocaleUpperCase()}
          </span>
        )
      }
      tileBg={`var(--party-${user.avatar.url ? user.avatarColor + 1 : party})`}
      paneTitle={user.displayName}
      paneSub={[
        user.role === 'admin' ? t('people.role.admin') : t('people.role.user'),
        user.username,
        user.id === me ? t('people.thisIsYou') : undefined,
      ]
        .filter(Boolean)
        .join(' · ')}
      status={user.disabled ? { text: t('people.disabled'), tone: 'warning' } : undefined}
    >
      <Group>
        <Row label={t('library.name')} value={user.displayName} />
        <Row label={t('onboarding.account.username.label')} value={user.username} />
      </Group>

      <Group
        title={t('admin.person.access')}
        foot={onlyAdmin ? t('admin.person.onlyAdmin', { name: user.displayName }) : undefined}
      >
        <Row
          label={t('people.role.admin')}
          busy={busy === 'role'}
          control={
            <Switch
              label={t('people.role.admin')}
              on={user.role === 'admin'}
              disabled={onlyAdmin || busy === 'role'}
              onChange={(on) => void update('role', { role: on ? 'admin' : 'member' })}
            />
          }
        />
        <Row
          label={user.disabled ? t('sharing.person.restore') : t('sharing.person.suspend')}
          sub={user.disabled ? t('people.disabled') : t('sharing.person.suspendHelp')}
          danger={!user.disabled}
          busy={busy === 'disabled'}
          onClick={() => void update('disabled', { disabled: !user.disabled })}
        />
      </Group>
      <PersonSharing user={user} onChange={load} />
      {fail && (
        <p role="alert" className="adm__text pl-tone--error adm-sec__alert">
          <Mark tone="error" />
          {errorMessage(t, fail)}
        </p>
      )}

      <Group title={t('people.devices.label')}>
        {!sessions && (
          <li>
            <Loading />
          </li>
        )}
        {sessions && !sessions.length && <Row muted label={t('people.devices.empty')} />}
        {(sessions ?? []).map((s) => (
          <Row
            key={s.id}
            label={s.deviceName}
            sub={
              s.current
                ? t('people.device.current')
                : t('people.device.lastSeen', {
                    time: formatRelative(locale, new Date(s.lastSeenAt)),
                  })
            }
            control={
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  void api
                    .DELETE('/users/{userId}/sessions/{sessionId}', {
                      params: { path: { userId: id, sessionId: s.id } },
                    })
                    .then(load)
                }
              >
                {t('people.device.signOut')}
              </Button>
            }
          />
        ))}
      </Group>

      <Group>
        {reset ? (
          <Row label={t('people.resetPassword')}>
            <div className="pl pl-field">
              <span className="pl-field__label">{t('people.tempPassword.label')}</span>
              <code className="adm__path">{reset}</code>
              <span className="pl-field__help">{t('people.tempPassword.help')}</span>
            </div>
          </Row>
        ) : (
          <Row
            label={t('people.resetPassword')}
            busy={busy === 'reset'}
            onClick={() => void resetPassword()}
          />
        )}
        {user.id !== me &&
          (onlyAdmin ? (
            <Row
              muted
              label={t('people.delete.label')}
              sub={t('admin.person.cantDelete', { name: user.displayName })}
            />
          ) : (
            <Row danger label={t('people.delete.label')} onClick={() => setDeleting(true)} />
          ))}
      </Group>

      {deleting && (
        <Dialog
          title={t('people.delete.confirm', { name: user.displayName })}
          actions={
            <>
              <Button variant="ghost" onClick={() => setDeleting(false)}>
                {t('common.cancel')}
              </Button>
              <Button
                onClick={() =>
                  void (async () => {
                    const { error, response } = await api.DELETE('/users/{userId}', {
                      params: { path: { userId: id } },
                    })
                    setDeleting(false)
                    if (error) return setFail(failure(error, response))
                    toast(t('people.deleted', { name: user.displayName }))
                    await navigate('/admin/people')
                  })()
                }
              >
                {t('people.delete.label')}
              </Button>
            </>
          }
        />
      )}
    </Section>
  )
}
