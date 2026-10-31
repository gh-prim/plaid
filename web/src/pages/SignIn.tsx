import { useEffect, useId, useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { LanguageSwitcher } from '../components/LanguageSwitcher'
import { FieldGroup, FieldNote, InsetField } from '../onboarding/Fields'
import { Card, Page, useServer } from '../onboarding/Page'
import { Button } from '../ui'

/**
 * Sign in (V2, boards TL01–TL03): the app icon and a glass card. A wrong
 * username or password rings the fields red; too many attempts disables
 * Sign in for the time the server asks (F003 rate limit).
 */
export function SignIn() {
  const { t } = useTranslation()
  const { refresh, device } = useAuth()
  const { serverName } = useServer()
  const noteId = useId()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  // Seconds the server asked us to wait after too many attempts.
  const [lockedFor, setLockedFor] = useState(0)

  useEffect(() => {
    if (!lockedFor) return
    const timer = setTimeout(() => setLockedFor(0), lockedFor * 1000)
    return () => clearTimeout(timer)
  }, [lockedFor])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setFail(undefined)
    const { error, response } = await api.POST('/auth/login', {
      body: { username, password, device },
    })
    setBusy(false)
    if (error) {
      const f = failure(error, response)
      if (f.error?.code === 'too_many_attempts') setLockedFor(f.retryAfterSeconds ?? 60)
      setFail(f)
      return
    }
    await refresh()
  }

  const locked = lockedFor > 0
  const wrong = fail?.error?.code === 'invalid_credentials'
  return (
    <Page className="ob--signin">
      <Card id="signin" icon title={t('signIn.title', { serverName })}>
        <form className="ob__form" onSubmit={(e) => void submit(e)}>
          <FieldGroup
            invalid={wrong}
            notes={
              fail && (
                <FieldNote id={noteId} error alert>
                  {errorMessage(t, fail)}
                </FieldNote>
              )
            }
          >
            <InsetField
              label={t('auth.username')}
              value={username}
              onValue={setUsername}
              invalid={wrong}
              describedBy={fail ? noteId : undefined}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              required
              autoFocus
            />
            <InsetField
              kind="password"
              label={t('auth.password')}
              value={password}
              onValue={setPassword}
              invalid={wrong}
              describedBy={fail ? noteId : undefined}
              autoComplete="current-password"
              required
            />
          </FieldGroup>
          <Button type="submit" block className="ob__cta" loading={busy} disabled={busy || locked}>
            {t('auth.signIn')}
          </Button>
          <p className="ob__foot">{t('signIn.forgot')}</p>
        </form>
      </Card>
      <div className="ob__language ob__language--page">
        <LanguageSwitcher />
      </div>
    </Page>
  )
}
