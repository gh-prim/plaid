import { useId, useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { FieldGroup, FieldNote, InsetField } from '../onboarding/Fields'
import { Card, Page } from '../onboarding/Page'
import { Button } from '../ui'

/**
 * Shown after signing in with a temporary password (F003-R15; V2, board
 * TL04). The API needs the temporary password too, so it stays a field.
 */
export function ChangePassword() {
  const { t } = useTranslation()
  const { refresh, signOut } = useAuth()
  const ids = useId()
  const [form, setForm] = useState({ current: '', next: '', confirm: '' })
  const [mismatch, setMismatch] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()

  const set = (key: keyof typeof form) => (value: string) =>
    setForm((f) => ({ ...f, [key]: value }))

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
  const nextError = fieldMessage(t, fields.newPassword)
  const formError = fail && !nextError ? errorMessage(t, fail) : undefined
  const confirmError = mismatch ? t('errors.password_mismatch') : undefined
  const empty = !form.current || !form.next || !form.confirm
  return (
    <Page>
      <Card
        id="change-password"
        title={t('auth.changePassword.title')}
        body={t('auth.changePassword.body')}
      >
        <form className="ob__form" onSubmit={(e) => void submit(e)} noValidate>
          <FieldGroup
            invalid={!!formError}
            notes={
              <>
                {formError && (
                  <FieldNote error alert>
                    {formError}
                  </FieldNote>
                )}
                <FieldNote id={`${ids}-next`} error={!!nextError}>
                  {nextError ?? t('auth.passwordHint')}
                </FieldNote>
                {confirmError && (
                  <FieldNote id={`${ids}-confirm`} error alert>
                    {confirmError}
                  </FieldNote>
                )}
              </>
            }
          >
            <InsetField
              kind="password"
              label={t('auth.currentPassword')}
              value={form.current}
              onValue={set('current')}
              autoComplete="current-password"
              required
              autoFocus
            />
            <InsetField
              kind="password"
              label={t('auth.newPassword')}
              value={form.next}
              onValue={set('next')}
              invalid={!!nextError}
              describedBy={`${ids}-next`}
              autoComplete="new-password"
              required
            />
            <InsetField
              kind="password"
              label={t('auth.confirmPassword')}
              value={form.confirm}
              onValue={set('confirm')}
              invalid={!!confirmError}
              describedBy={confirmError ? `${ids}-confirm` : undefined}
              autoComplete="new-password"
              required
            />
          </FieldGroup>
          <Button type="submit" block className="ob__cta" loading={busy} disabled={busy || empty}>
            {t('auth.changePassword.submit')}
          </Button>
          <button type="button" className="ob__link" onClick={() => void signOut()}>
            {t('auth.signOut')}
          </button>
        </form>
      </Card>
    </Page>
  )
}
