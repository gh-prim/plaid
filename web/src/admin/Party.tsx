import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { errorMessage, failure } from '../auth/errors'
import { formatNumber } from '../lib/format'
import { Group, Loading, Row, Section, Switch } from './v2'

type Party = components['schemas']['PartySettings']
type Update = components['schemas']['PartySettingsUpdate']

const sizes = [2, 3, 4, 5, 6, 8, 10, 12, 16]

/**
 * Plaid Party (board AdmV2 Plaid Party): watching together is on or off,
 * how many per party, and whether guests without an account can join.
 * Who can start one is each group's party right. Parties themselves come
 * with F010; the settings are stored now.
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
        <Group title={t('party.parties')}>
          <Row label={t('party.none')} sub={t('party.noneHelp')} muted />
        </Group>
      }
    >
      {!p ? (
        <Loading />
      ) : (
        <Group title={t('party.title')}>
          <Row
            label={t('party.title')}
            sub={t('party.help')}
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
            label={t('party.people')}
            sub={t('party.peopleHelp')}
            muted={!p.enabled}
            control={
              <select
                className="adm__select"
                aria-label={t('party.people')}
                value={p.maxPeople}
                disabled={saving || !p.enabled}
                onChange={(e) => void save({ maxPeople: Number(e.target.value) })}
              >
                {[...new Set([...sizes, p.maxPeople])]
                  .sort((a, b) => a - b)
                  .map((n) => (
                    <option key={n} value={n}>
                      {t('party.upTo', { n: formatNumber(i18n.language, n) })}
                    </option>
                  ))}
              </select>
            }
          />
          <Row
            label={t('party.guests')}
            sub={t('party.guestsHelp')}
            muted={!p.enabled}
            control={
              <Switch
                label={t('party.guests')}
                on={p.guests}
                disabled={saving || !p.enabled}
                onChange={(on) => void save({ guests: on })}
              />
            }
          />
          <Row
            to="/admin/groups"
            label={t('party.who')}
            value={t('party.whoValue')}
            sub={t('party.whoHelp')}
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
