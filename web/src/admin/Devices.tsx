import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { formatRelative } from '../lib/format'
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
      glyph="devices"
      paneTitle={devices ? t('admin.devices.count', { count: devices.length }) : undefined}
      paneSub={devices ? t('admin.devices.tvs', { count: tvs.length }) : undefined}
    >
      {!devices && <Loading />}
      {devices && !devices.length && <EmptyState title={t('admin.devices.empty')} />}
      {tvs.length > 0 && <Group title={t('admin.devices.tvGroup')}>{tvs.map(row)}</Group>}
      {others.length > 0 && <Group title={t('admin.devices.otherGroup')}>{others.map(row)}</Group>}
      {devices && (
        <Group foot={t('admin.devices.foot')}>
          <Row to="/pair" label={t('pairing.title')} sub={t('admin.devices.pairHelp')} />
        </Group>
      )}
    </Section>
  )
}
