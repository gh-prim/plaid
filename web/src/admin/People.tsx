import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { formatRelative } from '../lib/format'
import { checkAccount } from '../onboarding/Account'
import { Avatar, Button, ChoiceRow, Dialog, EmptyState, Mark, TextField } from '../ui'
import { useAdmin } from './AdminLayout'
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

/** Everyone with an account, and Add a person (board TA05, F007 R23). */
export function People() {
  const { t } = useTranslation()
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me?.user.id : undefined
  const [users, setUsers] = useState<AdminUser[]>()
  const [adding, setAdding] = useState(false)
  const load = useCallback(() => api.GET('/users').then(({ data }) => setUsers(data ?? [])), [])
  useEffect(() => {
    api.GET('/users').then(({ data }) => setUsers(data ?? []))
  }, [])
  const admins = (users ?? []).filter((u) => u.role === 'admin' && !u.disabled).length

  return (
    <Section
      title={t('people.title')}
      glyph="people"
      paneTitle={users ? t('admin.people.count', { count: users.length }) : undefined}
      paneSub={users ? t('admin.people.admins', { count: admins }) : undefined}
    >
      {!users && <Loading />}
      {users && (
        <Group>
          {users.map((u) => (
            <Row
              key={u.id}
              to={`/admin/people/${u.id}`}
              avatar={<Avatar name={u.displayName} size={40} />}
              label={u.displayName}
              value={u.role === 'admin' ? t('people.role.admin') : undefined}
              sub={[
                u.username,
                u.id === me ? t('people.thisIsYou') : undefined,
                u.disabled ? t('people.disabled') : undefined,
                u.mustChangePassword ? t('people.mustChange') : undefined,
              ]
                .filter(Boolean)
                .join(' · ')}
              muted={u.disabled}
            />
          ))}
        </Group>
      )}
      {users && users.length <= 1 && !adding && <EmptyState title={t('people.empty')} />}
      {users && (
        <Group foot={t('admin.people.foot')}>
          {adding ? (
            <Row label={t('people.add')}>
              <AddPerson
                onDone={() => {
                  setAdding(false)
                  void load()
                }}
              />
            </Row>
          ) : (
            <Row label={t('people.add')} onClick={() => setAdding(true)} />
          )}
          <Row to="/pair" label={t('people.pair')} />
        </Group>
      )}
    </Section>
  )
}

function AddPerson({ onDone }: { onDone: () => void }) {
  const { t, i18n } = useTranslation()
  const [form, setForm] = useState({
    displayName: '',
    username: '',
    role: 'member' as 'member' | 'admin',
  })
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
        <span className="adm-sec__initial">
          {user.displayName.trim().charAt(0).toLocaleUpperCase()}
        </span>
      }
      tileBg={`var(--party-${party})`}
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
          label={user.disabled ? t('people.enable') : t('people.disable')}
          sub={user.disabled ? t('people.disabled') : undefined}
          busy={busy === 'disabled'}
          onClick={() => void update('disabled', { disabled: !user.disabled })}
        />
      </Group>
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
