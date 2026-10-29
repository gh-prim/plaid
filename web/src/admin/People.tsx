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
  Button,
  ChoiceRow,
  DeviceRow,
  Dialog,
  EmptyState,
  Mark,
  PersonRow,
  SegmentedControl,
  TextField,
} from '../ui'
import { Section, useAdmin } from './AdminLayout'

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

/** Everyone with an account, and Add a person (board AP09, F007 R23). */
export function People() {
  const { t } = useTranslation()
  const [users, setUsers] = useState<AdminUser[]>()
  const [adding, setAdding] = useState(false)
  const load = useCallback(() => api.GET('/users').then(({ data }) => setUsers(data ?? [])), [])
  useEffect(() => {
    api.GET('/users').then(({ data }) => setUsers(data ?? []))
  }, [])

  return (
    <Section>
      <div className="adm__head">
        <h2 className="adm__h2">{t('people.title')}</h2>
        <div className="adm__actions">
          <Link to="/pair" className="pl-btn pl-btn--secondary">
            {t('people.pair')}
          </Link>
          <Button onClick={() => setAdding(true)}>{t('people.add')}</Button>
        </div>
      </div>
      {users && users.length <= 1 && !adding && <EmptyState title={t('people.empty')} />}
      {adding && (
        <AddPerson
          onDone={() => {
            setAdding(false)
            void load()
          }}
        />
      )}
      <div className="adm__rows">
        {(users ?? []).map((u) => (
          <PersonRow
            key={u.id}
            to={`/admin/people/${u.id}`}
            name={u.displayName}
            role={u.role}
            meta={u.username}
            disabled={u.disabled}
            mustChangePassword={u.mustChangePassword}
          />
        ))}
      </div>
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
  const local = checked ? checkAccount({ ...form, password, setupCode: '' }) : {}
  const server = fieldCodes(fail)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setChecked(true)
    if (Object.keys(checkAccount({ ...form, password, setupCode: '' })).length) return
    const { error, response } = await api.POST('/users', {
      body: {
        displayName: form.displayName.trim(),
        username: form.username.toLowerCase(),
        temporaryPassword: password,
        role: form.role,
        locale: i18n.resolvedLanguage,
      },
    })
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
        <Button type="submit">{t('people.add')}</Button>
        <Button variant="ghost" onClick={onDone}>
          {t('common.cancel')}
        </Button>
      </div>
    </form>
  )
}

/** One person: devices, password, disable, delete (board AP10, F007 R23). */
export function Person() {
  const { t, i18n } = useTranslation()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { toast } = useAdmin()
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me?.user.id : undefined
  const locale = i18n.resolvedLanguage ?? 'en'
  const [user, setUser] = useState<AdminUser>()
  const [sessions, setSessions] = useState<SessionItem[]>()
  const [fail, setFail] = useState<Failure>()
  const [reset, setReset] = useState<string>()
  const [deleting, setDeleting] = useState(false)

  const load = useCallback(async () => {
    const [u, s] = await Promise.all([
      api.GET('/users'),
      api.GET('/users/{userId}/sessions', { params: { path: { userId: id } } }),
    ])
    setUser(u.data?.find((x) => x.id === id))
    setSessions(s.data ?? [])
  }, [id])
  useEffect(() => {
    Promise.all([
      api.GET('/users'),
      api.GET('/users/{userId}/sessions', { params: { path: { userId: id } } }),
    ]).then(([u, s]) => {
      setUser(u.data?.find((x) => x.id === id))
      setSessions(s.data ?? [])
    })
  }, [id])

  if (!user) return <Section>{null}</Section>

  async function update(body: components['schemas']['UpdateUserRequest']) {
    setFail(undefined)
    const { error, response } = await api.PATCH('/users/{userId}', {
      params: { path: { userId: id } },
      body,
    })
    if (error) return setFail(failure(error, response))
    await load()
  }

  async function resetPassword() {
    const temp = temporaryPassword()
    const { error, response } = await api.POST('/users/{userId}/password', {
      params: { path: { userId: id } },
      body: { temporaryPassword: temp },
    })
    if (error) return setFail(failure(error, response))
    setReset(temp)
    await load()
  }

  return (
    <Section>
      <div className="adm__avatar-head">
        <Avatar name={user.displayName} size={56} />
        <div>
          <h2 className="adm__h2">{user.displayName}</h2>
          <p className="adm__help">
            {[
              user.username,
              user.role === 'admin' ? t('people.role.admin') : t('people.role.user'),
              user.id === me ? t('people.thisIsYou') : undefined,
              user.disabled ? t('people.disabled') : undefined,
            ]
              .filter(Boolean)
              .join(' · ')}
          </p>
        </div>
      </div>

      <section className="adm__section" style={{ gap: 0 }}>
        <h3 className="adm__h3" style={{ marginBottom: 8 }}>
          {t('people.devices.label')}
        </h3>
        {sessions && !sessions.length && <p className="adm__help">{t('people.devices.empty')}</p>}
        <ul className="adm__rows">
          {(sessions ?? []).map((s) => (
            <DeviceRow
              key={s.id}
              name={s.deviceName}
              current={s.current}
              lastSeen={t('people.device.lastSeen', {
                time: formatRelative(locale, new Date(s.lastSeenAt)),
              })}
              onSignOut={() =>
                void api
                  .DELETE('/users/{userId}/sessions/{sessionId}', {
                    params: { path: { userId: id, sessionId: s.id } },
                  })
                  .then(load)
              }
            />
          ))}
        </ul>
      </section>

      <section className="adm__section">
        <h3 className="adm__h3">{t('people.role.label')}</h3>
        <div style={{ maxWidth: 320 }}>
          <SegmentedControl
            label={t('people.role.label')}
            value={user.role}
            onChange={(role) => role !== user.role && void update({ role })}
            options={[
              { value: 'admin', label: t('people.role.admin') },
              { value: 'member', label: t('people.role.user') },
            ]}
          />
        </div>
        {fail && (
          <p role="alert" className="adm__text pl-tone--error" style={{ display: 'flex', gap: 8 }}>
            <Mark tone="error" />
            {errorMessage(t, fail)}
          </p>
        )}
      </section>

      {reset && (
        <div className="pl pl-field">
          <span className="pl-field__label">{t('people.tempPassword.label')}</span>
          <code className="adm__path">{reset}</code>
          <span className="pl-field__help">{t('people.tempPassword.help')}</span>
        </div>
      )}
      <div className="adm__actions">
        <Button variant="secondary" onClick={() => void resetPassword()}>
          {t('people.resetPassword')}
        </Button>
        <Button variant="secondary" onClick={() => void update({ disabled: !user.disabled })}>
          {user.disabled ? t('people.enable') : t('people.disable')}
        </Button>
        {user.id !== me && (
          <Button variant="ghost" className="adm__danger" onClick={() => setDeleting(true)}>
            {t('people.delete.label')}
          </Button>
        )}
      </div>
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
