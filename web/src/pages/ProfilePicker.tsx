import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api, type Profile } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { FormError } from '../components/Field'
import { PinInput } from '../components/PinInput'

/** "Who's watching?": picks the active profile of this session (F003-R17). */
export function ProfilePicker() {
  const { t } = useTranslation()
  const { state, refresh, signOut } = useAuth()
  const navigate = useNavigate()
  const [asking, setAsking] = useState<Profile>()
  const [pin, setPin] = useState('')
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()

  const profiles = state.status === 'ready' ? (state.me?.profiles ?? []) : []

  async function select(profile: Profile, withPin?: string) {
    setBusy(true)
    setFail(undefined)
    const { error, response } = await api.POST('/session/profile', {
      body: { profileId: profile.id, pin: withPin },
    })
    setBusy(false)
    if (error) {
      setFail(failure(error, response))
      setPin('')
      return
    }
    await refresh()
    navigate('/', { replace: true })
  }

  function pick(profile: Profile) {
    setFail(undefined)
    if (profile.hasPin) {
      setAsking(profile)
      setPin('')
    } else {
      void select(profile)
    }
  }

  function submitPin(e: FormEvent) {
    e.preventDefault()
    if (asking && pin.length === 4) void select(asking, pin)
  }

  return (
    <main className="page">
      <h1>{t('profiles.whoIsWatching')}</h1>
      {!asking && <FormError message={fail && errorMessage(t, fail)} />}
      <ul className="profile-tiles">
        {profiles.map((p) => (
          <li key={p.id}>
            <button
              type="button"
              className={`profile-tile profile-tile--${p.avatarColor}`}
              onClick={() => pick(p)}
              disabled={busy}
              aria-pressed={asking?.id === p.id}
            >
              <span className="profile-tile__avatar" aria-hidden="true">
                {p.name.slice(0, 1).toUpperCase()}
              </span>
              <span>{p.name}</span>
            </button>
          </li>
        ))}
      </ul>
      {asking && (
        <form onSubmit={submitPin} className="pin-form">
          <PinInput
            label={t('profiles.enterPin', { name: asking.name })}
            value={pin}
            onChange={setPin}
            error={fail && errorMessage(t, fail)}
            autoFocus
          />
          <button type="submit" disabled={busy || pin.length !== 4} aria-busy={busy}>
            {t('profiles.open')}
          </button>
          <button type="button" className="link-button" onClick={() => setAsking(undefined)}>
            {t('common.cancel')}
          </button>
        </form>
      )}
      <button type="button" className="link-button" onClick={() => void signOut()}>
        {t('auth.signOut')}
      </button>
    </main>
  )
}
