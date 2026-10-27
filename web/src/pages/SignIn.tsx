import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { Field, FormError } from '../components/Field'
import { LanguageSwitcher } from '../components/LanguageSwitcher'

export function SignIn() {
  const { t } = useTranslation()
  const { refresh, device } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setFail(undefined)
    const { error, response } = await api.POST('/auth/login', {
      body: { username, password, device },
    })
    setBusy(false)
    if (error) {
      setFail(failure(error, response))
      return
    }
    await refresh()
  }

  return (
    <main className="page">
      <h1>{t('welcome.title')}</h1>
      <form onSubmit={(e) => void submit(e)}>
        <FormError message={fail && errorMessage(t, fail)} />
        <Field
          label={t('auth.username')}
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
          autoCapitalize="none"
          required
          autoFocus
        />
        <Field
          label={t('auth.password')}
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
          required
        />
        <button type="submit" disabled={busy} aria-busy={busy}>
          {t('auth.signIn')}
        </button>
      </form>
      <LanguageSwitcher />
    </main>
  )
}
