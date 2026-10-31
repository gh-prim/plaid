import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'
import { api, type Profile } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { markProfilePicked } from '../auth/profileSession'
import { PageLight } from '../browse/PageLight'
import { Wordmark } from '../browse/Wordmark'
import { Icon } from '../ui'
import '../browse/browse.css'

/** A profile's circle: its party color with a soft top light, its initial. */
function Face({ profile, size }: { profile: Profile; size: number }) {
  return (
    <span
      className="pf__face"
      style={
        {
          '--pf-s': `${size}px`,
          '--pc': `var(--party-${profile.avatarColor + 1})`,
        } as React.CSSProperties
      }
      aria-hidden="true"
    >
      {profile.name.trim().charAt(0).toLocaleUpperCase()}
    </span>
  )
}

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
  // The page is lit by what the profile in focus watched last (V2, R51).
  const [lit, setLit] = useState<Profile>()

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
      {asking ? null : <PageLight art={lit?.ambient} />}
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
                  onPointerEnter={() => setLit(p)}
                  onFocus={() => setLit(p)}
                  aria-label={p.hasPin ? t('profiles.withPin', { name: p.name }) : p.name}
                >
                  <span className="pf__circle">
                    <Face profile={p} size={176} />
                    {p.hasPin && (
                      <span className="pf__lock">
                        <Icon name="lock" size={14} />
                      </span>
                    )}
                  </span>
                  <span className="pf__name">{p.name}</span>
                </button>
              </li>
            ))}
          </ul>
          <button
            type="button"
            className="pf__glassbtn br-glass pl-focusable"
            onClick={() => void signOut()}
          >
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
      <Face profile={profile} size={120} />
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
          <span key={i} aria-hidden="true" className={`pf__dot${i < pin.length ? ' is-on' : ''}`} />
        ))}
      </div>
      <p id={`${id}-error`} role="alert" className="pf__error">
        {error}
      </p>
      {!locked && <p className="pf__hint">{t('profiles.pin.hint')}</p>}
      <button type="button" className="pf__glassbtn br-glass pl-focusable" onClick={onCancel}>
        {t('profiles.pin.cancel')}
      </button>
    </main>
  )
}
