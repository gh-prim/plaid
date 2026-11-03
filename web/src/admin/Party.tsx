import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { errorMessage, failure } from '../auth/errors'
import { formatNumber } from '../lib/format'
import { Group, Loading, Row, Section, Switch } from './v2'

type Party = components['schemas']['PartySettings']
type Update = components['schemas']['PartySettingsUpdate']

const sizes = [2, 4, 6, 8, 12, 16, 20]

/** Remote streams the upload carries at 8 Mbps each, and what a full party
 * means for them (Party26, Party27; F010-R27). */
function uploadHelp(t: ReturnType<typeof useTranslation>['t'], lng: string, p: Party): string {
  if (!p.uploadMbps) return t('party.settings.uploadHelpUnknown')
  const fit = Math.floor((p.uploadMbps * 0.8) / 8)
  return t('party.settings.uploadHelp', {
    count: fit,
    remote: formatNumber(lng, p.maxPeople - 1),
    mbps: formatNumber(lng, p.uploadMbps),
  })
}

/**
 * Plaid Party (Party26, Party27): watching together is on or off, how many
 * per party (2 to 20), whether guests without an account can join, and the
 * home upload speed. Who can start one is each group's party right (F009).
 */
export function PartyPage() {
  const { t, i18n } = useTranslation()
  const [p, setP] = useState<Party>()
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string>()
  useEffect(() => {
    api.GET('/settings/party').then(({ data }) => setP(data))
  }, [])

  async function save(body: Update) {
    setSaving(true)
    setError(undefined)
    const { data, error: e, response } = await api.PATCH('/settings/party', { body })
    setSaving(false)
    if (e) return setError(errorMessage(t, failure(e, response)))
    setP(data)
  }

  return (
    <Section
      title={t('party.title')}
      aside={
        <Group title={t('party.settings.parties')}>
          <Row label={t('party.settings.none')} sub={t('party.settings.noneHelp')} muted />
        </Group>
      }
    >
      {!p ? (
        <Loading />
      ) : (
        <Group title={t('party.title')}>
          <Row
            label={t('party.title')}
            sub={t('party.settings.help')}
            control={
              <Switch
                label={t('party.title')}
                on={p.enabled}
                disabled={saving}
                onChange={(on) => void save({ enabled: on })}
              />
            }
          />
          <Row
            label={t('party.settings.people')}
            sub={t('party.settings.peopleHelp')}
            muted={!p.enabled}
            control={
              <select
                className="adm__select"
                aria-label={t('party.settings.people')}
                value={p.maxPeople}
                disabled={saving || !p.enabled}
                onChange={(e) => void save({ maxPeople: Number(e.target.value) })}
              >
                {[...new Set([...sizes, p.maxPeople])]
                  .sort((a, b) => a - b)
                  .map((n) => (
                    <option key={n} value={n}>
                      {t('party.settings.upTo', { n: formatNumber(i18n.language, n) })}
                    </option>
                  ))}
              </select>
            }
          />
          <Row
            label={t('party.settings.guests')}
            sub={t('party.settings.guestsHelp')}
            muted={!p.enabled}
            control={
              <Switch
                label={t('party.settings.guests')}
                on={p.guests}
                disabled={saving || !p.enabled}
                onChange={(on) => void save({ guests: on })}
              />
            }
          />
          <Row
            label={t('party.settings.upload')}
            sub={uploadHelp(t, i18n.language, p)}
            muted={!p.enabled}
            control={
              <input
                className="adm__select"
                type="number"
                min={0}
                max={10000}
                inputMode="numeric"
                aria-label={t('party.settings.uploadLabel')}
                defaultValue={p.uploadMbps || ''}
                placeholder={t('party.settings.uploadUnknown')}
                disabled={saving || !p.enabled}
                onBlur={(e) => {
                  const n = Number(e.target.value || 0)
                  if (n !== p.uploadMbps && n >= 0 && n <= 10000) void save({ uploadMbps: n })
                }}
              />
            }
          />
        </Group>
      )}
      {error && (
        <p role="alert" className="ob__folder-error">
          {error}
        </p>
      )}
    </Section>
  )
}
