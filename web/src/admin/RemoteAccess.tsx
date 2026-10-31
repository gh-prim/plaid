import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { errorMessage, failure } from '../auth/errors'
import { formatRelative } from '../lib/format'
import { Button, CommandBlock } from '../ui'
import { Group, Loading, Row, Section, Switch, type Tone } from './v2'

type Remote = components['schemas']['RemoteAccess']
type Step = components['schemas']['RemoteStep']
type Update = components['schemas']['RemoteAccessUpdate']

/** While a step is still pending, the page asks again this often. */
const pollMs = 3000

const busy = (r: Remote) =>
  r.enabled &&
  (r.state === 'starting' ||
    [r.portMapping, r.certificate, r.reachability].some((s) => s.state === 'pending'))

/**
 * Remote access (F012): one switch, and the steps that make the server
 * reachable from outside at a getplaid.direct address, each with its
 * state in words.
 */
export function RemoteAccess() {
  const { t, i18n } = useTranslation()
  const [r, setR] = useState<Remote>()
  const [error, setError] = useState<string>()
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [portInput, setPortInput] = useState('')

  useEffect(() => {
    api.GET('/remote-access').then(({ data, error: e, response }) => {
      if (e) return setError(errorMessage(t, failure(e, response)))
      setR(data)
      if (data?.manualPort) setPortInput(String(data.manualPort))
    })
  }, [t])

  const polling = r ? busy(r) : false
  useEffect(() => {
    if (!polling) return
    const timer = setInterval(() => {
      api.GET('/remote-access').then(({ data }) => data && setR(data))
    }, pollMs)
    return () => clearInterval(timer)
  }, [polling])

  async function save(body: Update) {
    setSaving(true)
    setError(undefined)
    const { data, error: e, response } = await api.PATCH('/remote-access', { body })
    setSaving(false)
    if (e) return setError(errorMessage(t, failure(e, response)))
    setR(data)
  }

  async function test() {
    setTesting(true)
    setError(undefined)
    const { data, error: e, response } = await api.POST('/remote-access/test')
    setTesting(false)
    if (e) return setError(errorMessage(t, failure(e, response)))
    setR(data)
  }

  const title = t('remoteAccess.title')
  if (!r)
    return (
      <Section title={title} glyph="remote">
        {error ? (
          <p role="alert" className="ob__folder-error">
            {error}
          </p>
        ) : (
          <Loading />
        )}
      </Section>
    )

  const portValue = Number(portInput)
  const portError =
    Number.isInteger(portValue) && portValue >= 1 && portValue <= 65535
      ? undefined
      : t('fieldErrors.out_of_range')
  const stateTone: Record<Remote['state'], Tone | undefined> = {
    off: undefined,
    starting: undefined,
    ok: 'ok',
    degraded: 'warning',
  }
  const stepTone = (s: Step): Tone | undefined =>
    s.state === 'ok' ? 'ok' : s.state === 'failed' ? 'error' : undefined
  // Spelled out so the i18n extractor sees every key.
  const errorText: Record<string, string> = {
    no_gateway: t('remoteAccess.error.no_gateway'),
    mapping_refused: t('remoteAccess.error.mapping_refused'),
    direct_unreachable: t('remoteAccess.error.direct_unreachable'),
    rate_limited: t('remoteAccess.error.rate_limited'),
    acme_failed: t('remoteAccess.error.acme_failed'),
    timeout: t('remoteAccess.error.timeout'),
    refused: t('remoteAccess.error.refused'),
    not_plaid: t('remoteAccess.error.not_plaid'),
    tls: t('remoteAccess.error.tls', { port: r.internalPort }),
  }
  const stepText: Record<Step['state'], string> = {
    off: t('remoteAccess.step.off'),
    pending: t('remoteAccess.step.pending'),
    ok: t('remoteAccess.step.ok'),
    failed: t('remoteAccess.step.failed'),
  }
  const stateText: Record<Remote['state'], string> = {
    off: t('remoteAccess.state.off'),
    starting: t('remoteAccess.state.starting'),
    ok: t('remoteAccess.state.ok'),
    degraded: t('remoteAccess.state.degraded'),
  }
  const port = r.externalPort ?? 0
  const methodText = (m?: string) =>
    m === 'upnp'
      ? t('remoteAccess.port.method.upnp', { port })
      : m === 'natpmp'
        ? t('remoteAccess.port.method.natpmp', { port })
        : t('remoteAccess.port.method.manual', { port })
  const stepWhy = (s: Step) =>
    s.state === 'failed' && s.error
      ? (errorText[s.error] ?? t('remoteAccess.error.unknown'))
      : undefined

  return (
    <Section
      title={title}
      glyph="remote"
      paneTitle={r.hostname ?? t('remoteAccess.pane')}
      paneSub={t('remoteAccess.paneSub')}
      status={{ text: stateText[r.state], tone: stateTone[r.state] }}
    >
      <Group foot={t('remoteAccess.privacy')}>
        <Row
          label={t('remoteAccess.enable')}
          sub={t('remoteAccess.enableHelp')}
          busy={saving}
          control={
            <Switch
              label={t('remoteAccess.enable')}
              on={r.enabled}
              disabled={saving}
              onChange={(on) => void save({ enabled: on })}
            />
          }
        />
      </Group>

      {r.enabled && r.url && (
        <Group title={t('remoteAccess.address')} foot={t('remoteAccess.addressHelp')}>
          <li className="adm-row-item">
            <div className="adm-row">
              <CommandBlock command={r.shortUrl ?? r.url} />
            </div>
          </li>
          <Row label={t('remoteAccess.fullUrl')} value={r.url} mono />
        </Group>
      )}

      {r.enabled && (
        <Group title={t('remoteAccess.steps')}>
          <Row
            label={t('remoteAccess.port.label')}
            value={
              r.portMapping.state === 'ok' && r.externalPort
                ? methodText(r.portMapping.method)
                : stepText[r.portMapping.state]
            }
            tone={stepTone(r.portMapping)}
            sub={stepWhy(r.portMapping)}
            subTone={r.portMapping.state === 'failed' ? 'error' : undefined}
            busy={r.portMapping.state === 'pending'}
          />
          <Row
            label={t('remoteAccess.ip')}
            value={r.publicIp ?? stepText.pending}
            mono={!!r.publicIp}
          />
          <Row
            label={t('remoteAccess.cert.label')}
            value={
              r.certificate.state === 'ok' && r.certificate.at
                ? t('remoteAccess.cert.until', {
                    when: new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' }).format(
                      new Date(r.certificate.at),
                    ),
                  })
                : stepText[r.certificate.state]
            }
            tone={stepTone(r.certificate)}
            sub={stepWhy(r.certificate)}
            subTone={r.certificate.state === 'failed' ? 'error' : undefined}
            busy={r.certificate.state === 'pending'}
          />
          <Row
            label={t('remoteAccess.reach.label')}
            value={stepText[r.reachability.state]}
            tone={stepTone(r.reachability)}
            sub={
              stepWhy(r.reachability) ??
              (r.reachability.at
                ? t('remoteAccess.reach.checked', {
                    when: formatRelative(i18n.language, new Date(r.reachability.at)),
                  })
                : undefined)
            }
            subTone={r.reachability.state === 'failed' ? 'error' : undefined}
            busy={r.reachability.state === 'pending'}
          />
          <Row
            label={testing ? t('remoteAccess.testing') : t('remoteAccess.test')}
            busy={testing}
            disabled={!r.hostname}
            onClick={() => void test()}
          />
        </Group>
      )}

      <Group
        title={t('remoteAccess.port.title')}
        foot={t('remoteAccess.port.help', { port: r.internalPort })}
      >
        <Row
          label={t('remoteAccess.port.auto')}
          sub={t('remoteAccess.port.autoHelp')}
          onClick={() => r.portMode !== 'auto' && void save({ portMode: 'auto' })}
          value={r.portMode === 'auto' ? '✓' : undefined}
          tone={r.portMode === 'auto' ? 'ok' : undefined}
          disabled={saving}
        />
        <Row
          label={t('remoteAccess.port.manual')}
          sub={t('remoteAccess.port.manualHelp')}
          value={r.portMode === 'manual' ? '✓' : undefined}
          tone={r.portMode === 'manual' ? 'ok' : undefined}
        >
          <form
            className="adm__prefs"
            noValidate
            onSubmit={(e) => {
              e.preventDefault()
              if (portError) return
              void save({ portMode: 'manual', manualPort: portValue })
            }}
          >
            <label className="adm-row__label" htmlFor="remote-port">
              {t('remoteAccess.port.external')}
            </label>
            <input
              id="remote-port"
              className="adm__select"
              inputMode="numeric"
              value={portInput}
              aria-invalid={portError && portInput ? 'true' : undefined}
              onChange={(e) => setPortInput(e.target.value.trim())}
            />
            <Button type="submit" loading={saving} disabled={!portInput}>
              {t('remoteAccess.port.use')}
            </Button>
          </form>
        </Row>
      </Group>

      {error && (
        <p role="alert" className="ob__folder-error">
          {error}
        </p>
      )}
    </Section>
  )
}
