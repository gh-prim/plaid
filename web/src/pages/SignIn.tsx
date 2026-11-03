import { useEffect, useId, useState } from 'react'
import type { CSSProperties, FormEvent, InputHTMLAttributes } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { serverUrl } from '../api/server'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { PlayMark, Wordmark } from '../browse/Wordmark'
import { SUPPORTED_LANGUAGES } from '../i18n'
import { useServer } from '../onboarding/Page'
import { Button, cx, Icon } from '../ui'
import '../browse/browse.css'
import './signin.css'

/**
 * Sign in (V2, board LoginWeb, step 24): a glass card over a slow wall of
 * the server's posters. A wrong username or password rings the fields
 * red; too many attempts disables Sign in for the time the server asks
 * (F003 rate limit).
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
  const describedBy = fail ? noteId : undefined
  return (
    <div className="pl si">
      <Wall />
      <header className="si__top">
        <Wordmark />
      </header>
      <main className="si__main">
        <section className="si__card" aria-labelledby="signin-title">
          <PlayMark className="si__icon" />
          <h1 id="signin-title" className="si__title">
            {t('signIn.title')}
          </h1>
          <p className="si__server">
            <ServerIcon />
            <span className="pl-sr">{t('signIn.server')}</span>
            {serverName}
          </p>
          <form className="si__form" onSubmit={(e) => void submit(e)}>
            <div className="si__fields">
              <Field
                label={t('auth.username')}
                value={username}
                onValue={setUsername}
                invalid={wrong}
                describedBy={describedBy}
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                required
                autoFocus
              />
              <Field
                kind="password"
                label={t('auth.password')}
                value={password}
                onValue={setPassword}
                invalid={wrong}
                describedBy={describedBy}
                autoComplete="current-password"
                required
              />
            </div>
            {fail && (
              <p id={noteId} className="si__error" role="alert">
                {errorMessage(t, fail)}
              </p>
            )}
            <Button
              type="submit"
              variant="ghost"
              block
              className="si__cta si-glass"
              loading={busy}
              disabled={busy || locked}
            >
              {t('auth.signIn')}
            </Button>
          </form>
          <p className="si__forgot">{t('signIn.forgot')}</p>
        </section>
      </main>
      <footer className="si__foot">
        <span>{t('signIn.footer', { host: window.location.hostname })}</span>
        <Language />
      </footer>
    </div>
  )
}

type FieldProps = {
  label: string
  value: string
  onValue: (value: string) => void
  kind?: 'text' | 'password'
  invalid?: boolean
  describedBy?: string
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type' | 'id'>

/**
 * One field of the card: the label sits in the middle while it's empty
 * and rises above the value once focused or filled. A password gets a
 * round glass Show/Hide button.
 */
function Field({
  label,
  value,
  onValue,
  kind = 'text',
  invalid,
  describedBy,
  ...input
}: FieldProps) {
  const { t } = useTranslation()
  const [reveal, setReveal] = useState(false)
  return (
    <div className={cx('si-field', value && 'is-filled', invalid && 'is-error')}>
      <label className="si-field__body">
        <span className="si-field__label">{label}</span>
        <input
          className={cx('si-field__input', kind === 'password' && !reveal && 'is-secret')}
          type={kind === 'password' && !reveal ? 'password' : 'text'}
          value={value}
          aria-invalid={invalid ? 'true' : undefined}
          aria-describedby={describedBy}
          onChange={(e) => onValue(e.target.value)}
          {...input}
        />
      </label>
      {kind === 'password' && (
        <button
          type="button"
          className="si-field__show si-glass pl-focusable"
          aria-label={reveal ? t('ui.hidePassword') : t('ui.showPassword')}
          onClick={() => setReveal(!reveal)}
        >
          <Icon name={reveal ? 'eyeOff' : 'eye'} size={17} />
        </button>
      )}
    </div>
  )
}

/** The language pill: a native select under a glass label. */
function Language() {
  const { t, i18n } = useTranslation()
  const id = useId()
  const names = { en: t('language.en'), fr: t('language.fr') }
  const current = (i18n.resolvedLanguage ?? 'en') as keyof typeof names
  return (
    <span className="si__lang si-glass">
      <Icon name="globe" size={15} />
      <span aria-hidden="true">{names[current]}</span>
      <Icon name="chevronDown" size={14} />
      <label className="pl-sr" htmlFor={id}>
        {t('language.label')}
      </label>
      <select id={id} value={current} onChange={(e) => void i18n.changeLanguage(e.target.value)}>
        {SUPPORTED_LANGUAGES.map((lng) => (
          <option key={lng} value={lng} lang={lng}>
            {names[lng]}
          </option>
        ))}
      </select>
    </span>
  )
}

/** Two stacked rack units: the server, beside its name. */
function ServerIcon() {
  return (
    <svg
      className="si__server-icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.9"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <rect x="3.5" y="4" width="17" height="7" rx="2" />
      <rect x="3.5" y="13" width="17" height="7" rx="2" />
      <path d="M7 7.5h.01M7 16.5h.01" />
    </svg>
  )
}

/** Nine columns of posters, drifting up and down in turn. */
const columns = 9
const perColumn = 6

/**
 * The wall behind the card: the server's posters (public, family-rated,
 * GET /auth/backdrop), dimmed and tilted. Each column holds its posters
 * twice so the loop is seamless. Empty tiles wait for the list and for
 * each image; a server with no posters keeps them empty.
 */
function Wall() {
  const [posters, setPosters] = useState<string[]>([])
  useEffect(() => {
    let live = true
    void api.GET('/auth/backdrop').then(({ data }) => {
      if (live && data) setPosters(data.posters)
    })
    return () => {
      live = false
    }
  }, [])
  return (
    <div className="si__wall" aria-hidden="true">
      <div className="si__cols">
        {Array.from({ length: columns }, (_, c) => {
          const slots = Array.from({ length: perColumn }, (_, i) =>
            posters.length ? posters[(c * perColumn + i) % posters.length] : undefined,
          )
          return (
            <div
              key={c}
              className={cx('si__col', c % 2 ? 'is-up' : 'is-down')}
              // Columns start 0, 90 or 180 px higher.
              style={{ '--si-i': c, marginTop: (c % 3) * -90 } as CSSProperties}
            >
              {[...slots, ...slots].map((id, i) => (
                <Poster key={i} id={id} />
              ))}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function Poster({ id }: { id?: string }) {
  const [loaded, setLoaded] = useState(false)
  return (
    <span className="si__poster">
      {id && (
        <img
          src={serverUrl(`/api/v1/auth/backdrop/${encodeURIComponent(id)}?w=342`)}
          alt=""
          className={cx(loaded && 'is-loaded')}
          onLoad={() => setLoaded(true)}
        />
      )}
    </span>
  )
}
