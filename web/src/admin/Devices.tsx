import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure } from '../auth/errors'
import { formatNumber, formatRelative } from '../lib/format'
import { Button, EmptyState } from '../ui'
import { useAdmin } from './AdminLayout'
import { Group, Loading, Row, Section } from './v2'

type AdminUser = components['schemas']['AdminUser']
type SessionItem = components['schemas']['SessionListItem']
type Device = { user: AdminUser; session: SessionItem }

const tvTypes = new Set(['tvos', 'androidtv', 'tv'])

/** Every signed-in device, from each account's sessions. */
async function loadDevices(): Promise<Device[]> {
  const { data: users } = await api.GET('/users')
  const lists = await Promise.all(
    (users ?? []).map((user) =>
      api
        .GET('/users/{userId}/sessions', { params: { path: { userId: user.id } } })
        .then(({ data }) => (data ?? []).map((session) => ({ user, session })))
        .catch(() => [] as Device[]),
    ),
  )
  return lists.flat().sort((a, b) => b.session.lastSeenAt.localeCompare(a.session.lastSeenAt))
}

/**
 * Devices (board TA07): every signed-in device across accounts, TVs first,
 * with who it belongs to and when it was last seen; signing one out ends
 * that session only. Pairing a TV stays on its own page.
 */
export function Devices() {
  const { t, i18n } = useTranslation()
  const { toast } = useAdmin()
  const locale = i18n.resolvedLanguage ?? 'en'
  const [devices, setDevices] = useState<Device[]>()
  const [busy, setBusy] = useState<string>()
  const load = useCallback(() => loadDevices().then(setDevices), [])
  useEffect(() => {
    loadDevices().then(setDevices)
  }, [])

  async function signOut(d: Device) {
    setBusy(d.session.id)
    await api
      .DELETE('/users/{userId}/sessions/{sessionId}', {
        params: { path: { userId: d.user.id, sessionId: d.session.id } },
      })
      .catch(() => undefined)
    setBusy(undefined)
    toast(t('admin.devices.signedOut', { device: d.session.deviceName }))
    await load()
  }

  const tvs = (devices ?? []).filter((d) => tvTypes.has(d.session.deviceType))
  const others = (devices ?? []).filter((d) => !tvTypes.has(d.session.deviceType))
  const row = (d: Device) => (
    <Row
      key={d.session.id}
      label={d.session.deviceName}
      sub={[d.user.displayName, d.session.current ? t('people.device.current') : undefined]
        .filter(Boolean)
        .join(' · ')}
      value={formatRelative(locale, new Date(d.session.lastSeenAt))}
      control={
        d.session.current ? undefined : (
          <Button
            variant="secondary"
            size="sm"
            loading={busy === d.session.id}
            aria-label={t('admin.devices.signOut', { device: d.session.deviceName })}
            onClick={() => void signOut(d)}
          >
            {t('people.device.signOut')}
          </Button>
        )
      }
    />
  )

  return (
    <Section
      title={t('admin.nav.devices')}
      sub={devices ? t('admin.devices.count', { count: devices.length }) : undefined}
    >
      <PairCard onPaired={() => void load()} />
      {!devices && <Loading />}
      {devices && !devices.length && <EmptyState title={t('admin.devices.empty')} />}
      {tvs.length > 0 && (
        <Group title={t('admin.devices.tvGroup')} meta={formatNumber(locale, tvs.length)}>
          {tvs.map(row)}
        </Group>
      )}
      {others.length > 0 && (
        <Group
          title={t('admin.devices.otherGroup')}
          meta={formatNumber(locale, others.length)}
          foot={t('admin.devices.foot')}
        >
          {others.map(row)}
        </Group>
      )}
    </Section>
  )
}

// The TV's code alphabet (server/internal/auth/token.go): no 0, O, 1, I or L.
const notInCode = /[^ABCDEFGHJKMNPQRSTUVWXYZ2-9]/g

/**
 * Pair a TV (board AdmV2 14): the code the TV shows, signed in on the
 * admin's current profile. A profile with a PIN goes to the full pairing
 * page, which asks for it.
 */
function PairCard({ onPaired }: { onPaired: () => void }) {
  const { t } = useTranslation()
  const { state } = useAuth()
  const { toast } = useAdmin()
  const navigate = useNavigate()
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const me = state.status === 'ready' ? state.me : null
  const profile = me?.activeProfile ?? me?.profiles[0]

  async function pair(e: FormEvent) {
    e.preventDefault()
    if (code.length !== 6 || busy || !profile) return
    if (profile.hasPin) return void navigate(`/pair?code=${code}`)
    setBusy(true)
    setError(undefined)
    const {
      data,
      error: err,
      response,
    } = await api.POST('/pairing/claim', {
      body: { code, profileId: profile.id },
    })
    setBusy(false)
    if (err) return setError(errorMessage(t, failure(err, response)))
    setCode('')
    toast(t('admin.devices.paired', { device: data.deviceName }))
    onPaired()
  }

  return (
    <section className="ax-card ax-pair">
      <header className="ax-card__head">
        <h2>{t('pairing.title')}</h2>
      </header>
      <form className="ax-pair__form" onSubmit={(e) => void pair(e)}>
        <label className="ax-pair__label" htmlFor="pair-code">
          {t('admin.devices.pairHelp')}
        </label>
        <input
          id="pair-code"
          className="ax-pair__code"
          value={code}
          maxLength={6}
          autoComplete="off"
          spellCheck={false}
          placeholder="••••••"
          aria-invalid={error ? 'true' : undefined}
          onChange={(e) => setCode(e.target.value.toUpperCase().replace(notInCode, '').slice(0, 6))}
        />
        <Button type="submit" size="sm" loading={busy} disabled={code.length !== 6}>
          {t('admin.devices.pair')}
        </Button>
        {error && (
          <p role="alert" className="ax-pair__error">
            {error}
          </p>
        )}
      </form>
    </section>
  )
}
