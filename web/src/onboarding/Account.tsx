import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { Button, cx, CommandBlock, TextField } from '../ui'
import { Page, StepHead, useServer } from './Page'
import { StatusScreen } from './StatusScreen'

/** The commands that find the setup code; the server logs `first_run_setup_code=…` (F007 R5). */
export const setupCodeCommands = {
  deb: 'sudo journalctl -u plaid | grep first_run_setup_code',
  docker: 'docker logs plaid 2>&1 | grep first_run_setup_code',
}

type Form = { displayName: string; username: string; password: string; setupCode: string }

/** The server's rules (F003-R4, R5), checked before submit so most errors show at once. */
export function checkAccount(f: Form): Partial<Record<keyof Form, string>> {
  const out: Partial<Record<keyof Form, string>> = {}
  const name = f.displayName.trim()
  if (!name) out.displayName = 'required'
  else if (name.length > 64) out.displayName = 'too_long'
  if (!f.username) out.username = 'required'
  else if (f.username.length < 3) out.username = 'too_short'
  else if (f.username.length > 32) out.username = 'too_long'
  else if (!/^[a-z0-9._-]+$/i.test(f.username)) out.username = 'invalid'
  if (!f.password) out.password = 'required'
  else if (f.password.length < 8) out.password = 'too_short'
  else if (f.password.length > 128) out.password = 'too_long'
  return out
}

/**
 * Steps 2: the admin account, with the setup code when the request comes
 * from outside the local network (F007 R4, R5, R6).
 */
export function Account() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { refresh, device } = useAuth()
  const { serverName } = useServer()
  const [form, setForm] = useState<Form>({
    displayName: '',
    username: '',
    password: '',
    setupCode: '',
  })
  const [requiresCode, setRequiresCode] = useState(false)
  const [checked, setChecked] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  // Seconds the server asked us to wait after too many attempts.
  const [lockedFor, setLockedFor] = useState(0)
  const [doneElsewhere, setDoneElsewhere] = useState(false)

  useEffect(() => {
    void api.GET('/setup').then(({ data }) => {
      if (data?.setupComplete) setDoneElsewhere(true)
      else if (data?.requiresSetupCode) setRequiresCode(true)
    })
  }, [])

  useEffect(() => {
    if (!lockedFor) return
    const timer = setTimeout(() => setLockedFor(0), lockedFor * 1000)
    return () => clearTimeout(timer)
  }, [lockedFor])

  if (doneElsewhere) return <StatusScreen kind="done" />

  const set = (key: keyof Form) => (value: string) => setForm((f) => ({ ...f, [key]: value }))
  const local = checked ? checkAccount(form) : {}
  const server = fieldCodes(fail)
  const code = fail?.error?.code
  const locked = lockedFor > 0
  const field = (key: keyof Form) => fieldMessage(t, local[key] ?? server[key])
  const usernameError =
    code === 'username_taken'
      ? t('errors.username_taken')
      : (local.username ?? server.username) === 'invalid'
        ? t('onboarding.account.username.invalid')
        : field('username')
  const passwordError = code === 'weak_password' ? t('errors.weak_password') : field('password')
  const codeError =
    code === 'setup_code_required'
      ? t('errors.setup_code_required')
      : code === 'too_many_attempts'
        ? errorMessage(t, fail!)
        : undefined
  const formError =
    fail &&
    ![
      'username_taken',
      'weak_password',
      'setup_code_required',
      'too_many_attempts',
      'validation_failed',
    ].includes(code ?? '')
      ? errorMessage(t, fail)
      : undefined

  async function submit(e: FormEvent) {
    e.preventDefault()
    setChecked(true)
    if (Object.keys(checkAccount(form)).length) return
    setBusy(true)
    setFail(undefined)
    const { error, response } = await api.POST('/setup/admin', {
      body: {
        displayName: form.displayName.trim(),
        username: form.username.toLowerCase(),
        password: form.password,
        locale: i18n.resolvedLanguage,
        setupCode: requiresCode
          ? form.setupCode.replaceAll('-', '').trim().toUpperCase()
          : undefined,
        device,
      },
    })
    setBusy(false)
    if (error) {
      const f = failure(error, response)
      if (f.error?.code === 'setup_complete') return setDoneElsewhere(true)
      if (f.error?.code === 'setup_code_required') setRequiresCode(true)
      if (f.error?.code === 'too_many_attempts') setLockedFor(f.retryAfterSeconds ?? 60)
      setFail(f)
      return
    }
    await refresh()
    await navigate('/setup/libraries')
  }

  return (
    <Page>
      <StepHead
        step={2}
        title={t('onboarding.account.title')}
        body={t('onboarding.account.body', { serverName })}
      />
      <div className={cx('ob__split', requiresCode && 'ob__split--code')}>
        {requiresCode && (
          <>
            <section className="ob__card">
              <p className="ob__card-title">{t('onboarding.code.title')}</p>
              <p className="ob__body" style={{ fontSize: 15, lineHeight: '22px' }}>
                {t('onboarding.code.body')}
              </p>
              <TextField
                kind="code"
                label={t('onboarding.code.label')}
                value={form.setupCode}
                onValue={set('setupCode')}
                disabled={locked}
                error={codeError}
                form="account"
              />
            </section>
            <section className="ob__where" style={{ gridRow: 'span 2' }}>
              <h2>{t('onboarding.code.where')}</h2>
              <p className="ob__label">{t('onboarding.code.deb')}</p>
              <CommandBlock command={setupCodeCommands.deb} />
              <p className="ob__label">{t('onboarding.code.docker')}</p>
              <CommandBlock command={setupCodeCommands.docker} />
              <p className="ob__body" style={{ fontSize: 15, lineHeight: '22px' }}>
                {t('onboarding.code.other')}
              </p>
            </section>
          </>
        )}
        <form id="account" className="ob__form" onSubmit={(e) => void submit(e)} noValidate>
          {formError && (
            <p role="alert" className="ob__folder-error">
              {formError}
            </p>
          )}
          <TextField
            label={t('onboarding.account.displayName.label')}
            help={t('onboarding.account.displayName.help')}
            value={form.displayName}
            onValue={set('displayName')}
            error={field('displayName')}
            autoComplete="name"
          />
          <TextField
            label={t('onboarding.account.username.label')}
            help={t('onboarding.account.username.help')}
            value={form.username}
            onValue={set('username')}
            error={usernameError}
            autoComplete="username"
            autoCapitalize="none"
          />
          <TextField
            kind="password"
            label={t('onboarding.account.password.label')}
            help={t('onboarding.account.password.help')}
            value={form.password}
            onValue={set('password')}
            error={passwordError}
            autoComplete="new-password"
          />
          <div className="ob__actions">
            <Button type="submit" loading={busy} disabled={busy || locked}>
              {busy ? t('onboarding.account.submitting') : t('onboarding.account.submit')}
            </Button>
          </div>
        </form>
      </div>
    </Page>
  )
}
