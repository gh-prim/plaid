import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { Field, FormError } from '../components/Field'

/** Shown after signing in with a temporary password (F003-R15). */
export function ChangePassword() {
  const { t } = useTranslation()
  const { refresh, signOut } = useAuth()
  const [form, setForm] = useState({ current: '', next: '', confirm: '' })
  const [mismatch, setMismatch] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()

  const set = (key: keyof typeof form) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))

  async function submit(e: FormEvent) {
    e.preventDefault()
    setMismatch(form.next !== form.confirm)
    if (form.next !== form.confirm) return
    setBusy(true)
    setFail(undefined)
    const { error, response } = await api.POST('/me/password', {
      body: { currentPassword: form.current, newPassword: form.next },
    })
    setBusy(false)
    if (error) {
      setFail(failure(error, response))
      return
    }
    await refresh()
  }

  const fields = fieldCodes(fail)
  return (
    <main className="page">
      <h1>{t('auth.changePassword.title')}</h1>
      <form onSubmit={(e) => void submit(e)} noValidate>
        <FormError message={fail && errorMessage(t, fail)} />
        <Field
          label={t('auth.currentPassword')}
          type="password"
          value={form.current}
          onChange={set('current')}
          autoComplete="current-password"
          required
          autoFocus
        />
        <Field
          label={t('auth.newPassword')}
          type="password"
          value={form.next}
          onChange={set('next')}
          autoComplete="new-password"
          required
          hint={t('auth.passwordHint')}
          error={fieldMessage(t, fields.newPassword)}
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
        <button type="submit" disabled={busy} aria-busy={busy}>
          {t('auth.changePassword.submit')}
        </button>
      </form>
      <button type="button" className="link-button" onClick={() => void signOut()}>
        {t('auth.signOut')}
      </button>
    </main>
  )
}
