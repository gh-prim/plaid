import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure } from '../auth/errors'
import { parseNetworks } from '../lib/cidr'
import { formatRelative, formatTime } from '../lib/format'
import { mountCommand } from '../lib/shellQuote'
import { Button, ProblemEntry, Mark, CommandBlock, TextField } from '../ui'
import { Section, useAdmin } from './AdminLayout'

type PlaybackSettings = components['schemas']['PlaybackSettings']
type Update = components['schemas']['PlaybackSettingsUpdate']
type HwAccel = components['schemas']['HwAccelSetting']
type Problem = components['schemas']['Problem']

const gb = 1e9
const remoteOptions = [0, 4e6, 8e6, 12e6, 20e6, 40e6]

/** Hardware test and playback settings (boards AP12–AP14, F007 R24, R28). */
export function Transcoding() {
  const { t } = useTranslation()
  const { refresh } = useAdmin()
  const [s, setS] = useState<PlaybackSettings>()
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState<string>()
  useEffect(() => {
    api.GET('/settings/playback').then(({ data }) => setS(data))
  }, [])
  const [ffmpeg, setFfmpeg] = useState<boolean>()
  useEffect(() => {
    api.GET('/server/capabilities').then(({ data }) => setFfmpeg(data?.ffmpeg.found))
  }, [])

  if (!s) return <Section>{null}</Section>
  const hw = s.hardware
  const onSaved = (next: PlaybackSettings) => {
    setS(next)
    void refresh()
  }

  async function retest() {
    setTesting(true)
    setTestError(undefined)
    const { data, error, response } = await api.POST('/server/capabilities/hardware-test')
    setTesting(false)
    if (error) return setTestError(errorMessage(t, failure(error, response)))
    if (data && s) setS({ ...s, hardware: data })
    void refresh()
  }

  const accelOptions: { value: HwAccel; label: string }[] = [
    { value: 'auto', label: t('transcoding.hw.auto') },
    { value: 'qsv', label: t('transcoding.hw.qsv') },
    { value: 'vaapi', label: t('transcoding.hw.vaapi') },
    { value: 'nvenc', label: t('transcoding.hw.nvenc') },
    { value: 'v4l2m2m', label: t('transcoding.hw.v4l2m2m') },
    { value: 'none', label: t('transcoding.hw.none') },
  ]

  return (
    <Section>
      <h2 className="adm__h2">{t('transcoding.title')}</h2>
      {hw.disabledReason === 'repeated_failures' && (
        <section className="adm__alert">
          <p className="adm__word pl-tone--warning">
            <Mark tone="warning" />
            {t('transcoding.fallback.title')}
          </p>
          <p>{t('transcoding.fallback.body')}</p>
          <div>
            <Button loading={testing} onClick={() => void retest()}>
              {t('transcoding.retest')}
            </Button>
          </div>
        </section>
      )}
      {ffmpeg === false && (
        <section className="adm__alert">
          <p className="adm__word pl-tone--warning">
            <Mark tone="warning" />
            {t('transcoding.ffmpeg.title')}
          </p>
          <p>{t('transcoding.ffmpeg.missing')}</p>
          <CommandBlock command="sudo apt install ffmpeg" />
          <p className="adm__help">{t('transcoding.ffmpeg.docker')}</p>
        </section>
      )}
      <section className="adm__section" style={{ gap: 8 }}>
        <h3 className="adm__h3">{t('transcoding.found')}</h3>
        <p className="adm__text" style={{ display: 'flex', gap: 8, alignItems: 'baseline' }}>
          <Mark tone={hw.active ? 'success' : 'warning'} />
          <span>
            {hw.active
              ? t('transcoding.test.ok', {
                  accelerator: hw.active.toUpperCase(),
                  device: hw.device ?? t('transcoding.device.default'),
                })
              : t('transcoding.test.none')}
          </span>
        </p>
        {hw.active && (
          <dl className="adm__dl">
            <dt>{t('transcoding.encodersLabel')}</dt>
            <dd>{hw.encoders.join(', ') || '—'}</dd>
            <dt>{t('transcoding.decodersLabel')}</dt>
            <dd>{hw.decoders.join(', ') || '—'}</dd>
            <dt>{t('transcoding.tonemapLabel')}</dt>
            <dd>{hw.tonemap ? t('transcoding.tonemap.gpu') : t('transcoding.tonemap.software')}</dd>
          </dl>
        )}
        {hw.disabledReason !== 'repeated_failures' && (
          <div>
            <Button variant="secondary" loading={testing} onClick={() => void retest()}>
              {testing ? t('transcoding.hw.testing') : t('transcoding.retest')}
            </Button>
          </div>
        )}
        {testError && (
          <p role="alert" className="ob__folder-error">
            {testError}
          </p>
        )}
      </section>

      <Prefs settings={s} accelOptions={accelOptions} onSaved={onSaved} />
    </Section>
  )
}

type Draft = {
  hwAccel: HwAccel
  maxTranscodes: string
  remoteMaxBitrate: number
  localNetworks: string
  encoderPreset: PlaybackSettings['encoderPreset']
  capGb: string
}

function draftOf(s: PlaybackSettings): Draft {
  return {
    hwAccel: s.hwAccel,
    maxTranscodes: String(s.maxTranscodes),
    remoteMaxBitrate: s.remoteMaxBitrate,
    localNetworks: s.localNetworks.join('\n'),
    encoderPreset: s.encoderPreset,
    capGb: String(Math.round((s.transcodeDirMaxBytes || s.transcodeDirMaxBytesDefault) / gb)),
  }
}

/** Only what changed, as the PATCH body (F007 R24). */
export function changes(before: Draft, after: Draft): Update {
  const out: Update = {}
  if (after.hwAccel !== before.hwAccel) out.hwAccel = after.hwAccel
  if (after.maxTranscodes !== before.maxTranscodes) out.maxTranscodes = Number(after.maxTranscodes)
  if (after.remoteMaxBitrate !== before.remoteMaxBitrate)
    out.remoteMaxBitrate = after.remoteMaxBitrate
  if (after.localNetworks !== before.localNetworks)
    out.localNetworks = parseNetworks(after.localNetworks).networks
  if (after.encoderPreset !== before.encoderPreset) out.encoderPreset = after.encoderPreset
  if (after.capGb !== before.capGb) out.transcodeDirMaxBytes = Number(after.capGb) * gb
  return out
}

/** One row: the label, the control, the help under it, like a server's settings page. */
function Pref({
  label,
  id,
  help,
  error,
  wide,
  children,
}: {
  label: string
  id: string
  help?: string
  error?: string
  wide?: boolean
  children: ReactNode
}) {
  return (
    <div className="adm__pref">
      <label className="adm__pref-label" htmlFor={id}>
        {label}
      </label>
      <div className={wide ? 'adm__pref-control adm__pref-control--wide' : 'adm__pref-control'}>
        {children}
      </div>
      {(error ?? help) && (
        <p className={error ? 'adm__help pl-tone--error' : 'adm__help'}>{error ?? help}</p>
      )}
    </div>
  )
}

/**
 * Playback settings as one page of rows, saved together; the rarely
 * changed ones stay behind "Show advanced" (F007 R24, revised: inspired by
 * Plex's server settings).
 */
function Prefs({
  settings,
  accelOptions,
  onSaved,
}: {
  settings: PlaybackSettings
  accelOptions: { value: HwAccel; label: string }[]
  onSaved: (s: PlaybackSettings) => void
}) {
  const { t } = useTranslation()
  const [base, setBase] = useState(() => draftOf(settings))
  const [draft, setDraft] = useState(base)
  const [advanced, setAdvanced] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string>()
  const set = <K extends keyof Draft>(k: K, v: Draft[K]) => {
    setDraft((d) => ({ ...d, [k]: v }))
    setSaved(false)
  }
  const maxError =
    /^\d+$/.test(draft.maxTranscodes) &&
    Number(draft.maxTranscodes) >= 1 &&
    Number(draft.maxTranscodes) <= 16
      ? undefined
      : t('fieldErrors.out_of_range')
  const capError =
    /^\d+$/.test(draft.capGb) && Number(draft.capGb) >= 1
      ? undefined
      : t('fieldErrors.out_of_range')
  const netError = parseNetworks(draft.localNetworks).invalidLines.length
    ? t('transcoding.networks.invalid')
    : undefined
  const body = changes(base, draft)
  const dirty = Object.keys(body).length > 0

  async function save() {
    if (maxError || capError || netError || !dirty) return
    setSaving(true)
    setError(undefined)
    const { data, error: e, response } = await api.PATCH('/settings/playback', { body })
    setSaving(false)
    if (e) return setError(errorMessage(t, failure(e, response)))
    if (data) {
      const next = draftOf(data)
      setBase(next)
      setDraft(next)
      onSaved(data)
    }
    setSaved(true)
  }

  return (
    <form
      className="adm__prefs"
      onSubmit={(e) => {
        e.preventDefault()
        void save()
      }}
      noValidate
    >
      <div className="adm__prefs-bar">
        <Button variant="secondary" size="sm" onClick={() => setAdvanced(!advanced)}>
          {advanced ? t('admin.prefs.hideAdvanced') : t('admin.prefs.showAdvanced')}
        </Button>
      </div>
      <Pref id="hw" label={t('transcoding.hw.label')} help={t('transcoding.hw.help')}>
        <select
          id="hw"
          className="adm__select"
          value={draft.hwAccel}
          onChange={(e) => set('hwAccel', e.target.value as HwAccel)}
        >
          {accelOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </Pref>
      <Pref
        id="max"
        label={t('transcoding.max.label')}
        help={t('transcoding.max.help', { default: settings.maxTranscodesDefault })}
        error={maxError}
      >
        <input
          id="max"
          className="adm__select"
          inputMode="numeric"
          aria-invalid={maxError ? 'true' : undefined}
          value={draft.maxTranscodes}
          onChange={(e) => set('maxTranscodes', e.target.value)}
        />
      </Pref>
      <Pref id="remote" label={t('transcoding.remote.label')} help={t('transcoding.remote.help')}>
        <select
          id="remote"
          className="adm__select"
          value={draft.remoteMaxBitrate}
          onChange={(e) => set('remoteMaxBitrate', Number(e.target.value))}
        >
          {[...new Set([...remoteOptions, settings.remoteMaxBitrate])]
            .sort((a, b) => a - b)
            .map((b) => (
              <option key={b} value={b}>
                {b ? t('transcoding.remote.mbps', { n: b / 1e6 }) : t('transcoding.remote.none')}
              </option>
            ))}
        </select>
      </Pref>
      {advanced && (
        <>
          <Pref
            id="nets"
            label={t('transcoding.networks.label')}
            help={t('transcoding.networks.help')}
            error={netError}
            wide
          >
            <textarea
              id="nets"
              className="adm__textarea"
              aria-invalid={netError ? 'true' : undefined}
              value={draft.localNetworks}
              onChange={(e) => set('localNetworks', e.target.value)}
            />
          </Pref>
          <Pref
            id="preset"
            label={t('transcoding.preset.label')}
            help={t('transcoding.preset.help')}
          >
            <select
              id="preset"
              className="adm__select"
              value={draft.encoderPreset}
              onChange={(e) => set('encoderPreset', e.target.value as Draft['encoderPreset'])}
            >
              <option value="fast">{t('transcoding.preset.fast')}</option>
              <option value="balanced">{t('transcoding.preset.balanced')}</option>
              <option value="quality">{t('transcoding.preset.quality')}</option>
            </select>
          </Pref>
          <Pref
            id="cap"
            label={t('transcoding.cap.gb')}
            help={t('transcoding.cap.help')}
            error={capError}
          >
            <input
              id="cap"
              className="adm__select"
              inputMode="numeric"
              aria-invalid={capError ? 'true' : undefined}
              value={draft.capGb}
              onChange={(e) => set('capGb', e.target.value)}
            />
          </Pref>
        </>
      )}
      <div className="adm__prefs-save">
        <Button type="submit" loading={saving} disabled={!dirty || saving}>
          {t('admin.prefs.save')}
        </Button>
        {dirty && (
          <Button variant="ghost" onClick={() => setDraft(base)}>
            {t('common.cancel')}
          </Button>
        )}
        {saved && !dirty && (
          <span className="adm__help" role="status">
            {t('common.saved')}
          </span>
        )}
        {error && (
          <span role="alert" className="adm__help pl-tone--error">
            {error}
          </span>
        )}
      </div>
    </form>
  )
}

/** Provider keys (write-only), metadata progress and attributions (board AP15, F007 R25). */
export function Metadata() {
  const { t } = useTranslation()
  const { state } = useAuth()
  const { data: admin } = useAdmin()
  const [settings, setSettings] = useState<components['schemas']['MetadataSettings']>()
  const [status, setStatus] = useState<components['schemas']['MetadataStatus']>()
  const [key, setKey] = useState('')
  const [editing, setEditing] = useState(false)
  const [error, setError] = useState<string>()
  useEffect(() => {
    Promise.all([api.GET('/metadata/settings'), api.GET('/metadata/status')]).then(([a, b]) => {
      setSettings(a.data)
      setStatus(b.data)
    })
  }, [])
  const attributions = state.status === 'ready' ? (state.server.attributions ?? []) : []

  const source = (s?: string) =>
    s === 'custom'
      ? t('metadata.source.custom')
      : s === 'env'
        ? t('metadata.source.env', { var: 'PLAID_TMDB_API_KEY' })
        : s === 'builtin'
          ? t('metadata.source.builtin')
          : t('metadata.source.none')

  async function saveKey() {
    setError(undefined)
    const {
      data,
      error: e,
      response,
    } = await api.PATCH('/metadata/settings', { body: { tmdbApiKey: key.trim() || null } })
    if (e) return setError(errorMessage(t, failure(e, response)))
    setSettings(data)
    setKey('')
    setEditing(false)
  }

  const providerName = (p: string) => (p === 'tmdb' ? 'TMDB' : 'fanart.tv')
  const stateText = (p: { provider: string; state: string }) =>
    p.state === 'ok'
      ? t('metadata.state.ok')
      : p.state === 'invalid_key'
        ? t('metadata.state.invalid_key', { provider: providerName(p.provider) })
        : p.state === 'no_api_key'
          ? t('metadata.state.no_api_key', { provider: providerName(p.provider) })
          : t('metadata.state.unreachable', { provider: providerName(p.provider) })

  return (
    <Section>
      <h2 className="adm__h2">{t('metadata.title')}</h2>
      <div className="adm__rows">
        {(status?.providers ?? []).map((p) => (
          <div
            key={p.provider}
            className="adm__row"
            style={{ padding: '16px 0', flexWrap: 'wrap' }}
          >
            <Mark
              tone={p.state === 'ok' ? 'success' : p.state === 'no_api_key' ? undefined : 'warning'}
            />
            <span className="adm__row-body" style={{ minWidth: 220 }}>
              <span className="adm__h3">{providerName(p.provider)}</span>
              <span className="adm__word" style={{ color: 'var(--text-secondary)' }}>
                {source(p.keySource)}
              </span>
              <span className="adm__help">{stateText(p)}</span>
            </span>
            {p.provider === 'tmdb' && !editing && (
              <Button variant="secondary" size="sm" onClick={() => setEditing(true)}>
                {t('metadata.useOwn')}
              </Button>
            )}
            {p.provider === 'tmdb' && editing && (
              <div className="adm__form" style={{ flexBasis: '100%' }}>
                <TextField
                  kind="password"
                  label={t('metadata.useOwn')}
                  help={t('metadata.key.help')}
                  value={key}
                  onValue={setKey}
                  autoComplete="off"
                  error={error}
                />
                <div className="adm__actions">
                  <Button onClick={() => void saveKey()}>{t('common.save')}</Button>
                  <Button variant="ghost" onClick={() => setEditing(false)}>
                    {t('common.cancel')}
                  </Button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
      {settings && !status && <p className="adm__text">{source(settings.tmdb.keySource)}</p>}
      <section className="adm__section" style={{ gap: 0 }}>
        <h3 className="adm__h3" style={{ marginBottom: 8 }}>
          {t('metadata.perLibrary')}
        </h3>
        <div className="adm__rows">
          {(status?.libraries ?? []).map((c) => (
            <div key={c.libraryId} className="adm__row">
              <span className="adm__row-body">
                <span className="adm__row-title">
                  {admin?.libraries.find((l) => l.id === c.libraryId)?.name ?? c.libraryId}
                </span>
                <span className="adm__help">
                  {t('metadata.progress', {
                    matched: c.matched,
                    needsReview: c.needsReview,
                    unmatched: c.unmatched,
                  })}
                </span>
              </span>
            </div>
          ))}
        </div>
      </section>
      {attributions.length > 0 && (
        <section className="adm__section">
          <h3 className="adm__h3">{t('metadata.attributions')}</h3>
          {attributions.map((a) => (
            <p key={a.provider} className="adm__help">
              <a href={a.url} rel="noreferrer" target="_blank" className="adm__link">
                {a.name}
              </a>{' '}
              · {a.notice}
            </p>
          ))}
        </section>
      )}
    </Section>
  )
}

/** What happened and what to do, for each problem code (F007 R26, R29). */
function describe(
  t: ReturnType<typeof useTranslation>['t'],
  p: Problem,
  time: string,
): { text: string; todo?: string; command?: string } {
  const path = p.params.path ?? ''
  switch (p.code) {
    case 'hw_fallback':
      return {
        text: t('problems.hwFallback.label', { accel: (p.params.accel ?? '').toUpperCase(), time }),
        todo: t('problems.hwFallback.todo'),
      }
    case 'ffmpeg_restarted':
      return { text: t('problems.ffmpeg', { title: p.params.title ?? '', time }) }
    case 'folder_unavailable':
      return {
        text: t('problems.folderGone', { path, time }),
        todo: t('folder.unavailable.check'),
        command: mountCommand(path),
      }
    case 'folder_unreadable':
      return {
        text: t('problems.folderUnreadable', { path, time }),
        todo: t('picker.unreadable.why', { name: path }),
      }
    case 'watch_limit':
      return {
        text: t('problems.watchLimit', { path, minutes: 15 }),
        command:
          'echo fs.inotify.max_user_watches=524288 | sudo tee /etc/sysctl.d/90-plaid.conf && sudo sysctl --system',
      }
    default:
      return {
        text: t('problems.keyRefused.label', { provider: 'TMDB' }),
        todo: t('problems.keyRefused.todo'),
      }
  }
}

/** The last 7 days of problems (board AP16, F007 R26). */
export function Problems() {
  const { t, i18n } = useTranslation()
  const { data } = useAdmin()
  const locale = i18n.resolvedLanguage ?? 'en'
  const list = data?.problems ?? []
  return (
    <Section>
      <div>
        <h2 className="adm__h2">{t('problems.title')}</h2>
        <p className="adm__lead">{t('problems.lead')}</p>
      </div>
      {data && !list.length && (
        <div className="adm__empty">
          <p className="title">{t('problems.empty')}</p>
        </div>
      )}
      <ul className="adm__rows">
        {list.map((p) => {
          const time = formatTime(locale, new Date(p.lastSeen))
          const d = describe(t, p, time)
          return (
            <ProblemEntry
              key={p.id}
              when={
                formatRelative(locale, new Date(p.lastSeen)) +
                (p.count > 1 ? ` · ${t('problems.count', { count: p.count })}` : '')
              }
              fixed={p.resolvedAt ? formatRelative(locale, new Date(p.resolvedAt)) : undefined}
              todo={p.resolvedAt ? undefined : d.todo}
              command={p.resolvedAt ? undefined : d.command}
            >
              {d.text}
            </ProblemEntry>
          )
        })}
      </ul>
    </Section>
  )
}
