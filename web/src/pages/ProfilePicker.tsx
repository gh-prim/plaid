import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'
import { api, type Profile } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { markProfilePicked } from '../auth/profileSession'
import { Wordmark } from '../browse/Wordmark'
import { Avatar } from '../ui'
import '../browse/browse.css'

/** "Who's under the plaid?": picks the active profile of this tab (F003-R17, F008-R2). */
export function ProfilePicker() {
  const { t } = useTranslation()
  const { state, refresh, signOut } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [asking, setAsking] = useState<Profile>()
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  // Each failed try remounts the PIN field: empty, focused.
  const [attempt, setAttempt] = useState(0)

  const profiles = state.status === 'ready' ? (state.me?.profiles ?? []) : []
  const from = (location.state as { from?: string } | null)?.from ?? '/'

  async function select(profile: Profile, pin?: string) {
    setBusy(true)
    setFail(undefined)
    const { error, response } = await api.POST('/session/profile', {
      body: { profileId: profile.id, pin },
    })
    setBusy(false)
    if (error) {
      setFail(failure(error, response))
      setAttempt((n) => n + 1)
      return false
    }
    markProfilePicked()
    await refresh()
    void navigate(from, { replace: true })
    return true
  }

  function pick(profile: Profile) {
    setFail(undefined)
    if (profile.hasPin) setAsking(profile)
    else void select(profile)
  }

  return (
    <div className="pl pf">
      <div className="pf__top">
        <Wordmark />
      </div>
      {asking ? (
        <PinEntry
          key={attempt}
          profile={asking}
          busy={busy}
          error={fail && errorMessage(t, fail)}
          locked={fail?.error?.code === 'profile_locked'}
          onPin={(pin) => select(asking, pin)}
          onCancel={() => {
            setAsking(undefined)
            setFail(undefined)
          }}
        />
      ) : (
        <main className="pf__main">
          <h1 className="pf__title">{t('profiles.title')}</h1>
          {fail && (
            <p role="alert" className="pf__error">
              {errorMessage(t, fail)}
            </p>
          )}
          <ul className="pf__grid">
            {profiles.map((p) => (
              <li key={p.id}>
                <button
                  type="button"
                  className="pf__tile pl-focusable"
                  disabled={busy}
                  onClick={() => pick(p)}
                  aria-label={p.hasPin ? t('profiles.withPin', { name: p.name }) : p.name}
                >
                  <Avatar name={p.name} color={p.avatarColor + 1} size={144} />
                  <span className="pf__name">{p.name}</span>
                  {p.hasPin && <span className="pf__badge">{t('profiles.pin.badge')}</span>}
                </button>
              </li>
            ))}
          </ul>
          <span className="pf__fill" />
          <button type="button" className="pf__link pl-focusable" onClick={() => void signOut()}>
            {t('auth.signOut')}
          </button>
        </main>
      )}
    </div>
  )
}

/**
 * The PIN as four boxes over one numeric field, so screen readers announce
 * it once and paste works; it sends itself at the fourth digit.
 */
function PinEntry({
  profile,
  busy,
  error,
  locked,
  onPin,
  onCancel,
}: {
  profile: Profile
  busy: boolean
  error?: string
  locked: boolean
  onPin: (pin: string) => Promise<boolean>
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const id = useId()
  const [pin, setPin] = useState('')

  function change(value: string) {
    const digits = value.replace(/\D/g, '').slice(0, 4)
    setPin(digits)
    if (digits.length === 4) void onPin(digits)
  }

  return (
    <main className="pf__main pf__main--pin">
      <Avatar name={profile.name} color={profile.avatarColor + 1} size={96} />
      <label htmlFor={id} className="pf__pinTitle">
        {t('profiles.pin.title', { name: profile.name })}
      </label>
      <div className={`pf__pin${error ? ' is-error' : ''}${locked ? ' is-locked' : ''}`}>
        <input
          id={id}
          type="password"
          inputMode="numeric"
          autoComplete="off"
          maxLength={4}
          autoFocus
          disabled={busy || locked}
          value={pin}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? `${id}-error` : undefined}
          onChange={(e) => change(e.target.value)}
        />
        {[0, 1, 2, 3].map((i) => (
          <span
            key={i}
            aria-hidden="true"
            className={`pf__box${i === pin.length ? ' is-current' : ''}`}
          >
            {i < pin.length && <span className="pf__dot" />}
          </span>
        ))}
      </div>
      {error && (
        <p id={`${id}-error`} role="alert" className="pf__error">
          {error}
        </p>
      )}
      <button type="button" className="pf__link pl-focusable" onClick={onCancel}>
        {t('profiles.pin.cancel')}
      </button>
    </main>
  )
}
