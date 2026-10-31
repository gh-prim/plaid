import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure } from '../auth/errors'
import { parseNetworks } from '../lib/cidr'
import { formatRelative, formatTime } from '../lib/format'
import { mountCommand } from '../lib/shellQuote'
import { Button, Mark, CommandBlock, TextField } from '../ui'
import { useAdmin } from './AdminLayout'
import { Group, Loading, Row, Section } from './v2'

type PlaybackSettings = components['schemas']['PlaybackSettings']
type Update = components['schemas']['PlaybackSettingsUpdate']
type HwAccel = components['schemas']['HwAccelSetting']
type Problem = components['schemas']['Problem']

const gb = 1e9
const remoteOptions = [0, 4e6, 8e6, 12e6, 20e6, 40e6]

/** Hardware test and playback settings (boards AP12–AP14, TA09, F007 R24, R28). */
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

  if (!s)
    return (
      <Section title={t('transcoding.title')} glyph="transcoding">
        <Loading />
      </Section>
    )
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

  const fallback = hw.disabledReason === 'repeated_failures'
  return (
    <Section
      title={t('transcoding.title')}
      glyph="transcoding"
      paneTitle={
        hw.active
          ? t('admin.transcoding.hardware', { accelerator: hw.active.toUpperCase() })
          : t('admin.transcoding.software')
      }
      paneSub={hw.active ? hw.device : undefined}
      status={
        ffmpeg === false
          ? { text: t('transcoding.ffmpeg.title'), tone: 'error' }
          : fallback
            ? { text: t('transcoding.fallback.title'), tone: 'warning' }
            : hw.active
              ? { text: t('admin.transcoding.working'), tone: 'ok' }
              : undefined
      }
    >
      {fallback && (
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
      <Group title={t('transcoding.found')}>
        <Row
          label={t('transcoding.hw.label')}
          value={hw.active ? hw.active.toUpperCase() : t('admin.dash.software')}
          tone={hw.active ? 'ok' : undefined}
          sub={
            hw.active
              ? t('transcoding.test.ok', {
                  accelerator: hw.active.toUpperCase(),
                  device: hw.device ?? t('transcoding.device.default'),
                })
              : t('transcoding.test.none')
          }
        />
        {hw.active && (
          <>
            <Row label={t('transcoding.encodersLabel')} value={hw.encoders.join(', ') || '—'} />
            <Row label={t('transcoding.decodersLabel')} value={hw.decoders.join(', ') || '—'} />
            <Row
              label={t('transcoding.tonemapLabel')}
              value={hw.tonemap ? t('transcoding.tonemap.gpu') : t('transcoding.tonemap.software')}
            />
          </>
        )}
        {!fallback && (
          <Row
            label={testing ? t('transcoding.hw.testing') : t('transcoding.retest')}
            busy={testing}
            onClick={() => void retest()}
          />
        )}
      </Group>
      {testError && (
        <p role="alert" className="ob__folder-error">
          {testError}
        </p>
      )}

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
    <li className="adm-row-item">
      <div className={`adm-row adm-pref${wide ? ' adm-pref--wide' : ''}`}>
        <span className="adm-row__text">
          <label className="adm-row__label" htmlFor={id}>
            {label}
          </label>
          {(error ?? help) && (
            <span className={`adm-row__sub${error ? ' is-error' : ''}`}>{error ?? help}</span>
          )}
        </span>
        <span className="adm-pref__control">{children}</span>
      </div>
    </li>
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
      <Group title={t('admin.transcoding.prefs')}>
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
        <Row
          label={advanced ? t('admin.prefs.hideAdvanced') : t('admin.prefs.showAdvanced')}
          onClick={() => setAdvanced(!advanced)}
        />
      </Group>
      {advanced && (
        <Group title={t('admin.transcoding.advanced')}>
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
        </Group>
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

/** Provider keys (write-only), metadata progress and attributions (boards AP15, TA11, F007 R25). */
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

  const providers = status?.providers ?? []
  const totals = (status?.libraries ?? []).reduce(
    (a, c) => ({
      matched: a.matched + c.matched,
      total: a.total + c.matched + c.needsReview + c.unmatched,
    }),
    { matched: 0, total: 0 },
  )
  const down = providers.filter((p) => p.state !== 'ok' && p.state !== 'no_api_key')

  return (
    <Section
      title={t('metadata.title')}
      glyph="metadata"
      paneTitle={t('admin.metadata.sources')}
      paneSub={
        status
          ? t('admin.metadata.matched', { matched: totals.matched, total: totals.total })
          : undefined
      }
      status={
        status
          ? down.length
            ? { text: t('admin.metadata.down', { count: down.length }), tone: 'warning' }
            : { text: t('admin.metadata.allOk'), tone: 'ok' }
          : undefined
      }
    >
      {!status && !settings && <Loading />}
      {settings && !status && <p className="adm__text">{source(settings.tmdb.keySource)}</p>}
      {providers.length > 0 && (
        <Group title={t('metadata.attributions')}>
          {providers.map((p) => (
            <Row
              key={p.provider}
              label={providerName(p.provider)}
              value={p.state === 'ok' ? t('metadata.state.ok') : undefined}
              tone={p.state === 'ok' ? 'ok' : undefined}
              sub={[source(p.keySource), p.state === 'ok' ? undefined : stateText(p)]
                .filter(Boolean)
                .join(' · ')}
              subTone={p.state === 'ok' || p.state === 'no_api_key' ? undefined : 'warning'}
              control={
                p.provider === 'tmdb' && !editing ? (
                  <Button variant="secondary" size="sm" onClick={() => setEditing(true)}>
                    {t('metadata.useOwn')}
                  </Button>
                ) : undefined
              }
            >
              {p.provider === 'tmdb' && editing && (
                <div className="adm__form">
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
            </Row>
          ))}
        </Group>
      )}
      {(status?.libraries.length ?? 0) > 0 && (
        <Group title={t('metadata.perLibrary')}>
          {status!.libraries.map((c) => (
            <Row
              key={c.libraryId}
              to={`/admin/libraries/${c.libraryId}`}
              label={admin?.libraries.find((l) => l.id === c.libraryId)?.name ?? c.libraryId}
              sub={t('metadata.progress', {
                matched: c.matched,
                needsReview: c.needsReview,
                unmatched: c.unmatched,
              })}
            />
          ))}
        </Group>
      )}
      {attributions.length > 0 && (
        <Group title={t('admin.metadata.credits')}>
          {attributions.map((a) => (
            <Row
              key={a.provider}
              label={
                <a href={a.url} rel="noreferrer" target="_blank" className="adm__link">
                  {a.name}
                </a>
              }
              sub={a.notice}
            />
          ))}
        </Group>
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

/** The last 7 days of problems, going on first (board TA10, F007 R26). */
export function Problems() {
  const { t, i18n } = useTranslation()
  const { data } = useAdmin()
  const locale = i18n.resolvedLanguage ?? 'en'
  const list = data?.problems ?? []
  const open = list.filter((p) => !p.resolvedAt)
  const solved = list.filter((p) => p.resolvedAt)

  const row = (p: Problem) => {
    const time = formatTime(locale, new Date(p.lastSeen))
    const d = describe(t, p, time)
    const fixed = p.resolvedAt ? formatRelative(locale, new Date(p.resolvedAt)) : undefined
    return (
      <Row
        key={p.id}
        label={d.text}
        value={formatRelative(locale, new Date(p.lastSeen))}
        sub={[
          p.count > 1 ? t('problems.count', { count: p.count }) : undefined,
          fixed ? t('problems.resolved', { time: fixed }) : undefined,
        ]
          .filter(Boolean)
          .join(' · ')}
        tone={fixed ? undefined : 'warning'}
      >
        {!fixed && (d.todo || d.command) ? (
          <>
            {d.todo && (
              <p className="adm__text">
                <strong>{t('problems.whatToDo')}</strong> {d.todo}
              </p>
            )}
            {d.command && <CommandBlock command={d.command} />}
          </>
        ) : undefined}
      </Row>
    )
  }

  return (
    <Section
      title={t('problems.title')}
      glyph="problems"
      paneTitle={data ? t('admin.problems.week', { count: list.length }) : undefined}
      paneSub={t('problems.lead')}
      status={
        data
          ? open.length
            ? { text: t('admin.problems.open', { count: open.length }), tone: 'error' }
            : { text: t('admin.problems.allClear'), tone: 'ok' }
          : undefined
      }
    >
      {!data && <Loading />}
      {data && !list.length && (
        <div className="adm__empty">
          <p className="title">{t('problems.empty')}</p>
        </div>
      )}
      {open.length > 0 && <Group title={t('admin.problems.goingOn')}>{open.map(row)}</Group>}
      {solved.length > 0 && <Group title={t('admin.problems.solved')}>{solved.map(row)}</Group>}
    </Section>
  )
}
