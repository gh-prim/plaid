import { useId, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api, type Profile } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { imageSrc } from '../browse/media'
import { PageLight } from '../browse/PageLight'
import { Icon, Spinner } from '../ui'
import '../browse/browse.css'

// The TV's code alphabet (server/internal/auth/token.go): no 0, O, 1, I or L.
const notInCode = /[^ABCDEFGHJKMNPQRSTUVWXYZ2-9]/g
const codeLength = 6

function isPinFailure(f: Failure) {
  return f.error?.code === 'invalid_pin' || f.error?.code === 'profile_locked'
}

/** A profile's circle, as on the profile picker. */
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

/** Signs a TV in with the code it shows (F003-R20), V2 look (canvas step 16). */
export function Pair() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me : null
  const profiles = me?.profiles ?? []
  const active = me?.activeProfile ?? undefined
  const [code, setCode] = useState('')
  const [profileId, setProfileId] = useState(active?.id ?? profiles[0]?.id ?? '')
  const [pin, setPin] = useState('')
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  // Each failure remounts the field it's about, replaying its shake.
  const [codeTry, setCodeTry] = useState(0)
  const [pinTry, setPinTry] = useState(0)
  // The PIN takes the focus once a profile is picked, not before the code.
  const [focusPin, setFocusPin] = useState(false)
  const [paired, setPaired] = useState<{ device: string; profile?: Profile }>()

  const profile = profiles.find((p) => p.id === profileId)
  const needsPin = profile?.hasPin ?? false
  const ready = code.length === codeLength && (!needsPin || pin.length === 4)
  // A wrong PIN is the PIN's fault, not the code's.
  const pinFail = !!fail && isPinFailure(fail)

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!ready || busy) return
    setBusy(true)
    setFail(undefined)
    const { data, error, response } = await api.POST('/pairing/claim', {
      body: { code, profileId, pin: needsPin ? pin : undefined },
    })
    setBusy(false)
    if (error) {
      const f = failure(error, response)
      setFail(f)
      if (isPinFailure(f)) {
        setPin('')
        setPinTry((n) => n + 1)
      } else setCodeTry((n) => n + 1)
      return
    }
    setPaired({ device: data.deviceName, profile })
    setCode('')
    setPin('')
  }

  function again() {
    setPaired(undefined)
    setFocusPin(false)
    setFail(undefined)
  }

  return (
    <div className="pl pf pa">
      <PageLight art={(paired?.profile ?? profile)?.ambient} />
      <header className="pa__top">
        <Link to="/" className="pa__back br-glass pl-focusable">
          <Icon name="chevronLeft" size={18} />
          {t('nav.home')}
        </Link>
        {active && <Face profile={active} size={44} />}
      </header>
      {paired ? (
        <Done
          device={paired.device}
          profile={paired.profile}
          admin={me?.user.role === 'admin'}
          onAgain={again}
          onDone={() => void navigate('/')}
        />
      ) : (
        <main className="pa__main">
          <Tv>
            <span className="pa__tvLogo" aria-hidden="true">
              <Logo />
            </span>
            <CodeGlyphs code={code} />
          </Tv>
          <h1 className="pa__title">{t('pairing.title')}</h1>
          <p className="pa__lead">{t('pairing.lead')}</p>
          <form className="pa__form" onSubmit={(e) => void submit(e)}>
            <CodeBoxes
              key={codeTry}
              code={code}
              error={!!fail && !pinFail}
              disabled={busy}
              onChange={(c) => {
                setCode(c)
                if (!pinFail) setFail(undefined)
              }}
            />
            {fail && !pinFail ? (
              <p role="alert" className="pa__error br-glass">
                <Icon name="alert" size={17} />
                {errorMessage(t, fail)}
              </p>
            ) : (
              <p className="pa__hint">{t('pairing.codeHint')}</p>
            )}
            <fieldset className="pa__profiles">
              <legend>{t('pairing.startProfile')}</legend>
              <div className="pa__faces">
                {profiles.map((p) => (
                  <label key={p.id} className={`pa__pick${p.id === profileId ? ' is-on' : ''}`}>
                    <input
                      type="radio"
                      name="profile"
                      className="br-visually-hidden"
                      value={p.id}
                      checked={p.id === profileId}
                      onChange={() => {
                        setProfileId(p.id)
                        setPin('')
                        setFocusPin(true)
                        if (pinFail) setFail(undefined)
                      }}
                    />
                    <span className="pa__circle">
                      <Face profile={p} size={60} />
                      {p.hasPin && (
                        <span className="pa__lock br-glass">
                          <Icon name="lock" size={10} />
                        </span>
                      )}
                    </span>
                    <span className="pa__name">{p.name}</span>
                  </label>
                ))}
              </div>
              {needsPin && profile && (
                <PinPill
                  key={`${profile.id}-${pinTry}`}
                  name={profile.name}
                  value={pin}
                  autoFocus={focusPin}
                  error={pinFail ? errorMessage(t, fail) : undefined}
                  disabled={busy || fail?.error?.code === 'profile_locked'}
                  onChange={(v) => {
                    setPin(v)
                    if (pinFail) setFail(undefined)
                  }}
                />
              )}
            </fieldset>
            <button
              type="submit"
              className="pa__cta pl-focusable"
              disabled={!ready || busy}
              aria-busy={busy}
            >
              {busy && <Spinner size={18} mono />}
              {t('pairing.confirm')}
            </button>
          </form>
        </main>
      )}
    </div>
  )
}

/** The TV drawn on the page, with its stand; it mirrors what's typed. */
function Tv({ children, lit }: { children: React.ReactNode; lit?: boolean }) {
  return (
    <div className={`pa__tv${lit ? ' is-lit' : ''}`} aria-hidden="true">
      <div className="pa__tvGlow" />
      <div className="pa__tvScreen">{children}</div>
      <div className="pa__tvStand" />
    </div>
  )
}

function CodeGlyphs({ code }: { code: string }) {
  return (
    <span className="pa__tvCode">
      {Array.from({ length: codeLength }, (_, i) => (
        <span key={i} className={i < code.length ? '' : 'is-empty'}>
          {i === 3 && <i className="pa__tvDash" />}
          {code[i] ?? '·'}
        </span>
      ))}
    </span>
  )
}

/**
 * The code as six boxes over one text field, so screen readers announce it
 * once and paste works; "K7P-4QX" and "k7p4qx" both fit.
 */
function CodeBoxes({
  code,
  error,
  disabled,
  onChange,
}: {
  code: string
  error: boolean
  disabled: boolean
  onChange: (code: string) => void
}) {
  const { t } = useTranslation()
  const id = useId()
  const input = useRef<HTMLInputElement>(null)
  const [focused, setFocused] = useState(true)
  return (
    <div className={`pa__code${error ? ' is-error' : ''}`} onClick={() => input.current?.focus()}>
      <label htmlFor={id} className="br-visually-hidden">
        {t('pairing.codeLabel')}
      </label>
      <input
        ref={input}
        id={id}
        autoFocus
        autoCapitalize="characters"
        autoComplete="off"
        spellCheck={false}
        maxLength={7}
        disabled={disabled}
        value={code}
        aria-invalid={error || undefined}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onChange={(e) =>
          onChange(e.target.value.toUpperCase().replace(notInCode, '').slice(0, codeLength))
        }
      />
      {Array.from({ length: codeLength }, (_, i) => (
        <span key={i} className="pa__boxWrap" aria-hidden="true">
          {i === 3 && <span className="pa__dash" />}
          <span
            className={`pa__box${focused && i === Math.min(code.length, codeLength - 1) ? ' is-current' : ''}`}
          >
            {code[i] ?? (focused && i === code.length && <span className="pa__caret" />)}
          </span>
        </span>
      ))}
    </div>
  )
}

/** The chosen profile's PIN, under the faces: four dots over one field. */
function PinPill({
  name,
  value,
  autoFocus,
  error,
  disabled,
  onChange,
}: {
  name: string
  value: string
  autoFocus: boolean
  error?: string
  disabled: boolean
  onChange: (pin: string) => void
}) {
  const { t } = useTranslation()
  const id = useId()
  return (
    <>
      <div className={`pa__pin br-glass${error ? ' is-error' : ''}`}>
        <Icon name="lock" size={17} />
        <label htmlFor={id}>{t('pairing.pin', { name })}</label>
        <input
          id={id}
          type="password"
          inputMode="numeric"
          autoComplete="off"
          maxLength={4}
          autoFocus={autoFocus}
          disabled={disabled}
          value={value}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? `${id}-error` : undefined}
          onChange={(e) => onChange(e.target.value.replace(/\D/g, '').slice(0, 4))}
        />
        <span className="pa__dots" aria-hidden="true">
          {[0, 1, 2, 3].map((i) => (
            <span key={i} className={`pf__dot${i < value.length ? ' is-on' : ''}`} />
          ))}
        </span>
      </div>
      {error && (
        <p id={`${id}-error`} role="alert" className="pa__pinError">
          {error}
        </p>
      )}
    </>
  )
}

function greeting(hour: number) {
  if (hour < 5 || hour >= 18) return 'evening'
  return hour < 12 ? 'morning' : 'afternoon'
}

function Done({
  device,
  profile,
  admin,
  onAgain,
  onDone,
}: {
  device: string
  profile?: Profile
  admin: boolean
  onAgain: () => void
  onDone: () => void
}) {
  const { t } = useTranslation()
  const art = profile?.ambient
  return (
    <main className="pa__main pa__main--done">
      <Tv lit>
        {art && <img className="pa__tvArt" src={imageSrc(art.imageId, 780)} alt="" />}
        <span className="pa__check">
          <Icon name="check" size={35} />
        </span>
        {profile && (
          <span className="pa__hello">
            {t(`pairing.hello.${greeting(new Date().getHours())}`, { name: profile.name })}
          </span>
        )}
      </Tv>
      <h1 className="pa__title pa__title--done" role="status">
        {t('pairing.done.title', { device })}
      </h1>
      <p className="pa__lead">
        {profile && t('pairing.done.profile', { name: profile.name })}
        {admin && <> {t('pairing.done.devices')}</>}
      </p>
      <div className="pa__actions">
        <button type="button" className="pa__ghost br-glass pl-focusable" onClick={onAgain}>
          {t('pairing.done.another')}
        </button>
        <button type="button" className="pa__cta pl-focusable" onClick={onDone} autoFocus>
          {t('pairing.done.finish')}
        </button>
      </div>
    </main>
  )
}

/** The woven play mark, small, as the TV shows it while it waits. */
function Logo() {
  return (
    <svg width="36" height="36" viewBox="0 0 100 100">
      <rect width="100" height="100" rx="24" fill="#1c1c1e" />
      <path
        d="M33 22 L81 50 L33 78 Z"
        fill="#f4f0e9"
        stroke="#f4f0e9"
        strokeWidth="6"
        strokeLinejoin="round"
      />
      <rect x="46" y="0" width="5" height="100" fill="#1c1c1e" />
      <rect x="0" y="47.5" width="100" height="5" fill="#1c1c1e" />
    </svg>
  )
}
