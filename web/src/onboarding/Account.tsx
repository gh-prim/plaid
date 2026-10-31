import { useEffect, useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { Button, CommandBlock } from '../ui'
import { FieldGroup, FieldNote, InsetField } from './Fields'
import { Card, LoadingCard, Page, useServer, useStepLabel } from './Page'
import { StatusScreen } from './StatusScreen'
import { slide, useStepNav } from './transition'

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

/** The code as the server wants it: no dashes, no spaces, upper case. */
function cleanCode(code: string): string {
  return code.replaceAll('-', '').trim().toUpperCase()
}

/**
 * The admin account (board TO02), after the setup code when the request
 * comes from outside the local network (board TO03; F007 R2, R4, R5, R6).
 * The code is only checked by `POST /setup/admin`: a wrong one brings the
 * code card back with its error.
 */
export function Account() {
  const { t, i18n } = useTranslation()
  const go = useStepNav()
  const stepLabel = useStepLabel()
  const { refresh, device } = useAuth()
  const { serverName } = useServer()
  const ids = useId()
  const [form, setForm] = useState<Form>({
    displayName: '',
    username: '',
    password: '',
    setupCode: '',
  })
  // Unknown until GET /setup answers.
  const [requiresCode, setRequiresCode] = useState<boolean>()
  const [phase, setPhase] = useState<'code' | 'account'>('account')
  const [checked, setChecked] = useState(false)
  const [codeChecked, setCodeChecked] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  // Seconds the server asked us to wait after too many attempts.
  const [lockedFor, setLockedFor] = useState(0)
  const [doneElsewhere, setDoneElsewhere] = useState(false)

  useEffect(() => {
    api
      .GET('/setup')
      .then(({ data }) => {
        if (data?.setupComplete) setDoneElsewhere(true)
        setRequiresCode(!!data?.requiresSetupCode)
        if (data?.requiresSetupCode) setPhase('code')
      })
      // Unknown: the server says so on submit (setup_code_required).
      .catch(() => setRequiresCode(false))
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
  // Notes sit under the whole group: a generic one names its field.
  const labels = {
    username: t('onboarding.account.username.label'),
    displayName: t('onboarding.account.displayName.label'),
    password: t('onboarding.account.password.label'),
  }
  const field = (key: keyof typeof labels) => {
    const message = fieldMessage(t, local[key] ?? server[key])
    return message && t('onboarding.field.error', { field: labels[key], error: message })
  }
  const usernameError =
    code === 'username_taken'
      ? t('errors.username_taken')
      : (local.username ?? server.username) === 'invalid'
        ? t('onboarding.account.username.invalid')
        : field('username')
  const displayNameError = field('displayName')
  const passwordError = code === 'weak_password' ? t('errors.weak_password') : field('password')
  const codeError =
    code === 'setup_code_required'
      ? t('errors.setup_code_required')
      : code === 'too_many_attempts'
        ? errorMessage(t, fail!)
        : codeChecked && !cleanCode(form.setupCode)
          ? t('fieldErrors.required')
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

  const toAccount = () => {
    setCodeChecked(true)
    if (!cleanCode(form.setupCode) || locked) return
    slide(() => setPhase('account'))
  }

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
        setupCode: requiresCode ? cleanCode(form.setupCode) : undefined,
        device,
      },
    })
    setBusy(false)
    if (error) {
      const f = failure(error, response)
      if (f.error?.code === 'setup_complete') return setDoneElsewhere(true)
      if (f.error?.code === 'too_many_attempts') setLockedFor(f.retryAfterSeconds ?? 60)
      setFail(f)
      // The code was wrong, or it's now needed: back to its card.
      if (f.error?.code === 'setup_code_required' || f.error?.code === 'too_many_attempts') {
        setRequiresCode(true)
        slide(() => setPhase('code'), 'back')
      }
      return
    }
    await refresh()
    go('/setup/libraries')
  }

  if (requiresCode === undefined) {
    return (
      <Page step={1} onBack={() => go('/setup', 'back')}>
        <LoadingCard />
      </Page>
    )
  }

  if (phase === 'code') {
    return (
      <Page step={1} onBack={() => go('/setup', 'back')}>
        <Card
          id="code"
          kicker={t('onboarding.code.kicker')}
          title={t('onboarding.code.title')}
          body={t('onboarding.code.body', { serverName })}
        >
          <form
            className="ob__form"
            noValidate
            onSubmit={(e) => {
              e.preventDefault()
              toAccount()
            }}
          >
            <FieldGroup
              invalid={!!codeError}
              notes={
                codeError && (
                  <FieldNote id={`${ids}-code`} error alert>
                    {codeError}
                  </FieldNote>
                )
              }
            >
              <InsetField
                kind="code"
                label={t('onboarding.code.label')}
                value={form.setupCode}
                onValue={set('setupCode')}
                disabled={locked}
                invalid={!!codeError}
                describedBy={codeError ? `${ids}-code` : undefined}
                autoFocus
              />
            </FieldGroup>
            <section className="ob__where">
              <h2>{t('onboarding.code.where')}</h2>
              <div className="ob-fields__box">
                <div className="ob__cmd">
                  <span className="ob__cmd-label">{t('onboarding.code.docker')}</span>
                  <CommandBlock command={setupCodeCommands.docker} />
                </div>
                <div className="ob__cmd">
                  <span className="ob__cmd-label">{t('onboarding.code.deb')}</span>
                  <CommandBlock command={setupCodeCommands.deb} />
                </div>
              </div>
              <p className="ob__foot">{t('onboarding.code.other')}</p>
            </section>
            <Button type="submit" block className="ob__cta" disabled={locked}>
              {t('onboarding.code.continue')}
            </Button>
          </form>
        </Card>
      </Page>
    )
  }

  const back = requiresCode
    ? () => slide(() => setPhase('code'), 'back')
    : () => go('/setup', 'back')
  const note = (key: string, error: string | undefined) => (error ? `${ids}-${key}` : undefined)
  return (
    <Page step={1} onBack={back}>
      <Card
        id="account"
        kicker={stepLabel(1)}
        title={t('onboarding.account.title')}
        body={t('onboarding.account.body', { serverName })}
      >
        <form className="ob__form" onSubmit={(e) => void submit(e)} noValidate>
          <FieldGroup
            notes={
              <>
                {formError && (
                  <FieldNote error alert>
                    {formError}
                  </FieldNote>
                )}
                {usernameError && (
                  <FieldNote id={`${ids}-username`} error>
                    {usernameError}
                  </FieldNote>
                )}
                {displayNameError && (
                  <FieldNote id={`${ids}-displayName`} error>
                    {displayNameError}
                  </FieldNote>
                )}
                <FieldNote id={`${ids}-password`} error={!!passwordError}>
                  {passwordError ?? t('onboarding.account.password.help')}
                </FieldNote>
              </>
            }
          >
            <InsetField
              label={labels.username}
              value={form.username}
              onValue={set('username')}
              invalid={!!usernameError}
              describedBy={note('username', usernameError)}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              autoFocus
            />
            <InsetField
              label={labels.displayName}
              value={form.displayName}
              onValue={set('displayName')}
              invalid={!!displayNameError}
              describedBy={note('displayName', displayNameError)}
              autoComplete="name"
            />
            <InsetField
              kind="password"
              label={labels.password}
              value={form.password}
              onValue={set('password')}
              invalid={!!passwordError}
              describedBy={`${ids}-password`}
              autoComplete="new-password"
            />
          </FieldGroup>
          <Button type="submit" block className="ob__cta" loading={busy} disabled={busy || locked}>
            {busy ? t('onboarding.account.submitting') : t('onboarding.account.submit')}
          </Button>
          <p className="ob__foot">{t('onboarding.account.others')}</p>
        </form>
      </Card>
    </Page>
  )
}
