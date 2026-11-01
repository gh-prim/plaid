import { useEffect, useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { shortLinkId } from '../api/server'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { SUPPORTED_LANGUAGES } from '../i18n'
import { useServer } from '../onboarding/Page'
import { cx } from '../ui'
import './remote-signin.css'

const base = import.meta.env.BASE_URL

/**
 * Sign in through the short link (V2 step 15, boards TR01–TR04): the server's
 * name over a film still, the username and password in one glass bar, and
 * the play button that signs in. Same rules as SignIn: a wrong password
 * rings the field, too many attempts lock the button for the time the
 * server asks.
 */
export function RemoteSignIn() {
  const { t, i18n } = useTranslation()
  const { refresh, device } = useAuth()
  const { serverName } = useServer()
  const errorId = useId()
  const langId = useId()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [reveal, setReveal] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  // Bumped on each wrong attempt so the shake replays.
  const [tries, setTries] = useState(0)
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
      setTries((n) => n + 1)
      return
    }
    await refresh()
  }

  const wrong = fail?.error?.code === 'invalid_credentials'
  const described = fail ? errorId : undefined
  return (
    <div className="rs">
      <div className="rs__still" aria-hidden="true">
        <img
          src={`${base}remote/tears-of-steel.jpg`}
          srcSet={`${base}remote/tears-of-steel-800.jpg 800w, ${base}remote/tears-of-steel.jpg 1600w`}
          sizes="100vw"
          alt=""
        />
      </div>
      <div className="rs__shade" aria-hidden="true" />

      <header className="rs__top">
        <div className="rs__brand rs-glass" aria-label="PLAID">
          <BrandMark />
        </div>
        <div className="rs__lang rs-glass">
          <GlobeIcon />
          <span aria-hidden="true">
            {t(`language.${i18n.resolvedLanguage === 'fr' ? 'fr' : 'en'}`)}
          </span>
          <ChevronIcon />
          <label htmlFor={langId} className="rs__sr">
            {t('language.label')}
          </label>
          <select
            id={langId}
            value={i18n.resolvedLanguage}
            onChange={(e) => void i18n.changeLanguage(e.target.value)}
          >
            {SUPPORTED_LANGUAGES.map((lng) => (
              <option key={lng} value={lng} lang={lng}>
                {t(`language.${lng}`)}
              </option>
            ))}
          </select>
        </div>
      </header>

      <main className="rs__main">
        <p className="rs__kicker rs-in-1">
          {t('remoteSignIn.kicker')}
          <span className="rs__dot" aria-hidden="true" />
          <code>{shortLinkId()}</code>
        </p>
        <h1 className="rs__title rs-in-2">{serverName}</h1>
        <p className="rs__lead rs-in-2">{t('remoteSignIn.lead')}</p>

        <form className="rs__form rs-in-3" onSubmit={(e) => void submit(e)}>
          {fail && (
            <p className="rs__error rs-glass-b" id={errorId} role="alert">
              <AlertIcon />
              {errorMessage(t, fail)}
            </p>
          )}
          <div className={cx('rs__row', wrong && 'is-wrong')} key={tries}>
            <div className="rs__bar rs-glass">
              <label className="rs__field">
                <PersonIcon />
                <input
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder={t('auth.username')}
                  aria-label={t('auth.username')}
                  aria-describedby={described}
                  aria-invalid={wrong || undefined}
                  autoComplete="username"
                  autoCapitalize="none"
                  spellCheck={false}
                  required
                  autoFocus
                />
              </label>
              <label className={cx('rs__field', wrong && 'is-wrong')}>
                <LockIcon />
                <input
                  type={reveal ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t('auth.password')}
                  aria-label={t('auth.password')}
                  aria-describedby={described}
                  aria-invalid={wrong || undefined}
                  autoComplete="current-password"
                  required
                />
                <button
                  type="button"
                  className="rs__reveal rs-glass-b"
                  aria-label={reveal ? t('ui.hidePassword') : t('ui.showPassword')}
                  aria-pressed={reveal}
                  onClick={() => setReveal((r) => !r)}
                >
                  <EyeIcon />
                </button>
              </label>
            </div>
            <button
              type="submit"
              className="rs__go"
              aria-label={t('auth.signIn')}
              disabled={busy || lockedFor > 0}
            >
              {busy ? <span className="rs__spin" aria-hidden="true" /> : <PlayIcon />}
              <span className="rs__go-label" aria-hidden="true">
                {t('auth.signIn')}
              </span>
            </button>
          </div>
          <p className="rs__foot">
            <span className="rs__foot-status" aria-live="polite">
              {busy ? t('remoteSignIn.signingIn') : t('remoteSignIn.hint')}
            </span>
            <span className="rs__dot" aria-hidden="true" />
            <span className="rs__forgot-long">{t('remoteSignIn.forgot')}</span>
            <span className="rs__forgot-short">{t('remoteSignIn.forgotShort')}</span>
          </p>
        </form>

        <div className="rs__secure">
          <p>
            <ShieldIcon />
            {t('remoteSignIn.secure')}
          </p>
          <p>{t('remoteSignIn.credit')}</p>
        </div>
      </main>
    </div>
  )
}

const stroke = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.7,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
} as const

function Svg({
  size = 19,
  dim = true,
  children,
}: {
  size?: number
  dim?: boolean
  children: React.ReactNode
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      aria-hidden="true"
      style={dim ? { color: 'var(--rs-ink-2)' } : undefined}
      {...stroke}
    >
      {children}
    </svg>
  )
}

const PersonIcon = () => (
  <Svg>
    <circle cx="12" cy="8" r="4" />
    <path d="M4 21c1.5-4 4.5-6 8-6s6.5 2 8 6" />
  </Svg>
)
const LockIcon = () => (
  <Svg>
    <rect x="5" y="11" width="14" height="10" rx="2.5" />
    <path d="M8 11V8a4 4 0 0 1 8 0v3" />
  </Svg>
)
const EyeIcon = () => (
  <Svg size={17}>
    <path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z" />
    <circle cx="12" cy="12" r="3" />
  </Svg>
)
const GlobeIcon = () => (
  <Svg size={17}>
    <circle cx="12" cy="12" r="9" />
    <path d="M3 12h18M12 3c2.5 2.5 3.5 5.5 3.5 9s-1 6.5-3.5 9c-2.5-2.5-3.5-5.5-3.5-9s1-6.5 3.5-9z" />
  </Svg>
)
const ChevronIcon = () => (
  <Svg size={14}>
    <path d="M6 9l6 6 6-6" />
  </Svg>
)
const AlertIcon = () => (
  <Svg size={17} dim={false}>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 7.5v5.5M12 16.5h.01" />
  </Svg>
)
const ShieldIcon = () => (
  <Svg size={15} dim={false}>
    <path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z" />
    <path d="M9 12l2 2 4-4" />
  </Svg>
)
const PlayIcon = () => (
  <svg
    width="22"
    height="22"
    viewBox="0 0 24 24"
    aria-hidden="true"
    style={{ display: 'block', marginLeft: 3 }}
  >
    <path
      d="M7 4.5v15l12.5-7.5z"
      fill="#111"
      stroke="#111"
      strokeWidth="2"
      strokeLinejoin="round"
    />
  </svg>
)

/** The app icon and the "plaid" wordmark, as drawn on the V2 boards. */
function BrandMark() {
  const ink = '#f4f0e9'
  const line = { stroke: ink, strokeWidth: 12, fill: 'none' }
  return (
    <>
      <svg
        width="34"
        height="34"
        viewBox="0 0 100 100"
        aria-hidden="true"
        style={{ display: 'block' }}
      >
        <rect width="100" height="100" rx="26" fill="#1c1c1e" />
        <path
          d="M33 22 L81 50 L33 78 Z"
          fill={ink}
          stroke={ink}
          strokeWidth="6"
          strokeLinejoin="round"
        />
        <rect x="46" y="0" width="5" height="100" fill="#1c1c1e" />
        <rect x="0" y="47.5" width="100" height="5" fill="#1c1c1e" />
      </svg>
      <svg
        height="22"
        viewBox="-2 14 245 122"
        aria-hidden="true"
        style={{ display: 'block', overflow: 'visible' }}
      >
        <circle cx="31" cy="75" r="19" {...line} />
        <path d="M6 56 V130" {...line} />
        <path d="M78 20 V84 Q78 100 94 100" {...line} />
        <circle cx="125" cy="75" r="19" {...line} />
        <path d="M144 56 V100" {...line} />
        <path d="M170 56 V100" {...line} />
        <path
          d="M164 26 L180 35 L164 44 Z"
          fill={ink}
          stroke={ink}
          strokeWidth="3"
          strokeLinejoin="round"
        />
        <circle cx="214" cy="75" r="19" {...line} />
        <path d="M233 20 V100" {...line} />
      </svg>
    </>
  )
}
