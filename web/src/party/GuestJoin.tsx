import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { serverUrl } from '../api/server'
import { useAuth } from '../auth/AuthProvider'
import { Button, Spinner } from '../ui'
import { PartyPlayer } from './PartyPlayer'
import './party.css'

type Preview = components['schemas']['PartyPreview']

/**
 * A party link, `/p/:code` (Party14): a signed-in person joins with the
 * code; anyone else gives a first name and joins as a guest, when the
 * admin allows it (F010-R5).
 */
export default function GuestJoin() {
  const { code = '' } = useParams()
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me : null
  const [preview, setPreview] = useState<Preview | null | undefined>(undefined)
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [joined, setJoined] = useState(false)
  const [loadedAt, setLoadedAt] = useState(0)
  const target = useMemo(() => ({ code }), [code])

  useEffect(() => {
    void api
      .GET('/party-codes/{code}', { params: { path: { code } } })
      .then(({ data }) => {
        setLoadedAt(Date.now())
        setPreview(data ?? null)
      })
      .catch(() => setPreview(null))
  }, [code])

  async function join(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    const { data, error: err } = await api
      .POST('/party-codes/{code}/guest', {
        params: { path: { code } },
        body: { firstName: name.trim() },
      })
      .catch(() => ({ data: undefined, error: { error: { code: 'network' } } }))
    setBusy(false)
    if (!data) {
      setError((err as { error?: { code?: string } })?.error?.code ?? 'generic')
      return
    }
    setJoined(true)
  }

  const leave = () => void navigate('/', { replace: true })
  // Signed in with a profile: straight into the party with the code.
  if (me?.activeProfile || joined) return <PartyPlayer target={target} onLeave={leave} />

  if (preview === undefined || state.status === 'loading') {
    return (
      <div className="pl pv-party-page" data-theme="dark">
        <Spinner size={34} mono />
      </div>
    )
  }
  if (preview === null) {
    return (
      <div className="pl pv-party-page" data-theme="dark">
        <div className="pv-party-center pv-glass pv-party-card">
          <p className="pv-party-title">{t('party.error.party_not_found')}</p>
        </div>
      </div>
    )
  }
  const started = preview.startedAt
    ? new Intl.RelativeTimeFormat(i18n.language, { numeric: 'auto' }).format(
        -Math.max(1, Math.round((loadedAt - new Date(preview.startedAt).getTime()) / 60_000)),
        'minute',
      )
    : ''
  return (
    <div className="pl pv-party-page" data-theme="dark">
      {preview.item && (
        <img
          className="pv-party-backdrop"
          alt=""
          src={serverUrl(`/api/v1/items/${preview.item.id}/artwork/backdrop?w=1280`)}
          onError={(e) => (e.currentTarget.style.display = 'none')}
        />
      )}
      <form className="pv-party-center pv-glass pv-party-card" onSubmit={join}>
        <p className="pv-party-kicker">{t('party.guest.invite', { host: preview.hostName })}</p>
        {preview.item && <p className="pv-party-display">{preview.item.title}</p>}
        {started && (
          <p className="pv-party-muted">
            {t('party.guest.started', { ago: started, count: preview.watching })}
          </p>
        )}
        {preview.full ? (
          <p>{t('party.full.title')}</p>
        ) : preview.guests ? (
          <>
            <label className="pl-field">
              <span className="pl-field__label">{t('party.guest.firstName')}</span>
              <input
                className="pl-field__input"
                value={name}
                maxLength={32}
                autoComplete="given-name"
                required
                onChange={(e) => setName(e.target.value)}
              />
            </label>
            <Button type="submit" block disabled={busy || !name.trim()}>
              {t('party.guest.join')}
            </Button>
            <p className="pv-party-muted">{t('party.guest.hint')}</p>
          </>
        ) : (
          <p className="pv-party-muted">{t('party.guest.accountNeeded')}</p>
        )}
        {error && (
          <p className="pv-party-error" role="alert">
            {t(`party.error.${error}`, t('party.error.generic'))}
          </p>
        )}
        <Link to="/login" state={{ from: { pathname: `/p/${code}` } }} className="pl-link">
          {t('party.guest.signIn')}
        </Link>
      </form>
    </div>
  )
}
