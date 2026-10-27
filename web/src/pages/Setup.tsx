import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { Field, FormError } from '../components/Field'
import { LanguageSwitcher } from '../components/LanguageSwitcher'

export function Setup() {
  const { t, i18n } = useTranslation()
  const { refresh, device } = useAuth()
  const [form, setForm] = useState({
    displayName: '',
    username: '',
    password: '',
    confirm: '',
    setupCode: '',
  })
  const [requiresCode, setRequiresCode] = useState(false)
  const [mismatch, setMismatch] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()

  const set = (key: keyof typeof form) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))

  async function submit(e: FormEvent) {
    e.preventDefault()
    setMismatch(form.password !== form.confirm)
    if (form.password !== form.confirm) return
    setBusy(true)
    setFail(undefined)
    const { error, response } = await api.POST('/setup/admin', {
      body: {
        displayName: form.displayName,
        username: form.username,
        password: form.password,
        locale: i18n.resolvedLanguage,
        setupCode: form.setupCode || undefined,
        device,
      },
    })
    setBusy(false)
    if (error) {
      const f = failure(error, response)
      if (f.error?.code === 'setup_code_required') setRequiresCode(true)
      setFail(f)
      return
    }
    await refresh()
  }

  const fields = fieldCodes(fail)
  return (
    <main className="page">
      <h1>{t('setup.title')}</h1>
      <p>{t('setup.subtitle')}</p>
      <form onSubmit={(e) => void submit(e)} noValidate>
        <FormError message={fail && errorMessage(t, fail)} />
        <Field
          label={t('auth.displayName')}
          value={form.displayName}
          onChange={set('displayName')}
          autoComplete="name"
          required
          error={fieldMessage(t, fields.displayName)}
        />
        <Field
          label={t('auth.username')}
          value={form.username}
          onChange={set('username')}
          autoComplete="username"
          autoCapitalize="none"
          required
          hint={t('auth.usernameHint')}
          error={fieldMessage(t, fields.username)}
        />
        <Field
          label={t('auth.password')}
          type="password"
          value={form.password}
          onChange={set('password')}
          autoComplete="new-password"
          required
          hint={t('auth.passwordHint')}
          error={fieldMessage(t, fields.password)}
        />
        <Field
          label={t('auth.confirmPassword')}
          type="password"
          value={form.confirm}
          onChange={set('confirm')}
          autoComplete="new-password"
          required
          error={mismatch ? t('errors.password_mismatch') : undefined}
        />
        {requiresCode && (
          <Field
            label={t('setup.setupCode')}
            value={form.setupCode}
            onChange={set('setupCode')}
            autoCapitalize="characters"
            autoComplete="off"
            required
            hint={t('setup.setupCodeHint')}
          />
        )}
        <button type="submit" disabled={busy} aria-busy={busy}>
          {t('auth.createAdmin')}
        </button>
      </form>
      <LanguageSwitcher />
    </main>
  )
}
