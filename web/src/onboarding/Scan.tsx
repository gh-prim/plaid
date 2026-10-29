import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useConnection } from '../hooks/useConnection'
import { useScanEvents, type ScanState } from '../hooks/useScanEvents'
import { AmbientBackdrop, ProgressBar, ServerFact } from '../ui'
import { useServer } from './Page'
import { posterSrc, PosterWall } from './PosterWall'
import { ScanEnd } from './ScanEnd'
import { OfflineScreen } from './StatusScreen'

type Library = components['schemas']['Library']
type Capabilities = components['schemas']['Capabilities']
type ItemSummary = components['schemas']['ItemSummary']

/** The serif line changes at most every 3 s on screen, every 10 s for screen readers (F007 R12, NFR). */
export const voiceEvery = 3000
export const announceEvery = 10000

/** Follows one library's stream and reports it up; renders nothing. */
function Follow({ id, onState }: { id: string; onState: (id: string, s: ScanState) => void }) {
  const state = useScanEvents(id)
  useEffect(() => onState(id, state), [id, state, onState])
  return null
}

/** A value that follows `value` at most once every `every` ms. */
function useThrottled<T>(value: T, every: number): T {
  const [shown, setShown] = useState(value)
  const last = useRef(0)
  useEffect(() => {
    const wait = Math.max(0, last.current + every - Date.now())
    const timer = setTimeout(() => {
      last.current = Date.now()
      setShown(value)
    }, wait)
    return () => clearTimeout(timer)
  }, [value, every])
  return shown
}

/**
 * Done: idle, nothing queued, nothing left to match. What's left to match
 * comes from the stream's metadata counters, or from GET /metadata/status
 * before the first counter arrives (they're throttled to one every 2 s):
 * unknown is never "nothing".
 */
export function scanDone(s: ScanState | undefined, pendingFromStatus?: number): boolean {
  const pending = s?.metadata?.pending ?? pendingFromStatus
  return !!s?.progress && s.progress.state === 'idle' && !s.progress.queued && pending === 0
}

/** How often the end of the scan asks the server what's left to match. */
export const statusEvery = 2000

/** The thread's fill: files analyzed while analyzing, then items matched (F007 R11). */
export function scanFill(s: ScanState | undefined): number {
  const c = s?.progress?.counters
  if (!c) return 0
  if (s?.progress?.state === 'scanning' || s?.progress?.state === 'analyzing') {
    return c.found ? (100 * c.analyzed) / c.found : 0
  }
  const m = s?.metadata
  if (!m) return 100
  const total = m.pending + m.matched + m.needsReview + m.unmatched
  return total ? 100 * (1 - m.pending / total) : 100
}

/** The first scan, live (F007 R11–R13), then its end (R14). */
export function Scan() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { serverName, version } = useServer()
  const { connection, lost, retryNow } = useConnection()
  const [libraries, setLibraries] = useState<Library[]>()
  const [caps, setCaps] = useState<Capabilities>()
  const [maxTranscodes, setMaxTranscodes] = useState<number>()
  const [states, setStates] = useState<Record<string, ScanState>>({})
  const [seed, setSeed] = useState<Record<string, ItemSummary[]>>({})
  const [tab, setTab] = useState<string>()
  const desktop = typeof matchMedia === 'function' && matchMedia('(min-width: 1024px)').matches

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const [libs, c, s] = await Promise.all([
          api.GET('/libraries'),
          api.GET('/server/capabilities'),
          api.GET('/settings/playback'),
        ])
        if (cancelled) return
        if (libs.data?.length === 0) return void navigate('/setup/libraries', { replace: true })
        setLibraries(libs.data)
        setCaps(c.data)
        setMaxTranscodes(s.data?.maxTranscodes)
        // Titles matched before this page opened refill the wall once.
        const pages = await Promise.all(
          (libs.data ?? []).map((l) =>
            api.GET('/libraries/{libraryId}/items', {
              params: { path: { libraryId: l.id }, query: { limit: 24 } },
            }),
          ),
        )
        if (!cancelled) {
          setSeed(
            Object.fromEntries(
              (libs.data ?? []).map((l, i) => [
                l.id,
                (pages[i]?.data?.items ?? []).filter((it) => it.poster),
              ]),
            ),
          )
        }
      } catch {
        lost()
      }
    })()
    return () => {
      cancelled = true
    }
  }, [navigate, lost])

  // Files are all scanned but no metadata counter arrived yet: ask the
  // server what's left to match instead of assuming nothing is.
  const [statusPending, setStatusPending] = useState<Record<string, number>>({})
  const waitingForCounts =
    !!libraries?.length &&
    libraries.every((l) => states[l.id]?.progress?.state === 'idle') &&
    libraries.some((l) => !states[l.id]?.metadata)
  useEffect(() => {
    if (!waitingForCounts) return
    const ask = () =>
      api
        .GET('/metadata/status')
        .then(({ data }) =>
          setStatusPending(
            Object.fromEntries((data?.libraries ?? []).map((c) => [c.libraryId, c.pending])),
          ),
        )
        .catch(lost)
    ask()
    const timer = setInterval(ask, statusEvery)
    return () => clearInterval(timer)
  }, [waitingForCounts, lost])

  const onState = useCallback(
    (id: string, s: ScanState) => setStates((all) => ({ ...all, [id]: s })),
    [],
  )

  const current = tab ?? libraries?.[0]?.id
  const lib = libraries?.find((l) => l.id === current)
  const state = current ? states[current] : undefined
  const matched = useMemo(() => {
    const fromSeed = (current ? (seed[current] ?? []) : []).map((it) => ({
      id: it.id,
      kind: it.kind === 'show' ? ('show' as const) : ('movie' as const),
      title: it.displayTitle ?? it.title,
      year: it.year,
      poster: it.poster,
      backdrop: it.backdrop,
    }))
    const live = state?.matched ?? []
    return [...live, ...fromSeed.filter((s) => !live.some((m) => m.id === s.id))]
  }, [current, seed, state])

  const latest = matched[0]
  const latestBackdrop = matched.find((m) => m.backdrop)?.backdrop
  const voice = useThrottled(
    latest ? { title: latest.title, count: state?.metadata?.matched ?? matched.length } : undefined,
    voiceEvery,
  )
  const announced = useThrottled(voice, announceEvery)

  const allDone =
    !!libraries?.length && libraries.every((l) => scanDone(states[l.id], statusPending[l.id]))
  if (allDone && libraries) {
    return <ScanEnd libraries={libraries} states={states} />
  }

  const p = state?.progress
  const c = p?.counters
  const m = state?.metadata
  const stateLine = (l: Library) => {
    const s = states[l.id]?.progress
    if (!s) return ''
    if (s.queued && s.state === 'idle') return t('scan.state.queued')
    switch (s.state) {
      case 'scanning':
        return t('scan.state.scanning', { path: s.currentFolder ?? '' })
      case 'analyzing':
        return t('scan.state.analyzing')
      case 'queued':
        return t('scan.state.queued')
      default:
        return t('scan.state.idle')
    }
  }
  const networkFolders = (libraries ?? []).flatMap((l) =>
    l.folders.filter((f) => !f.watched && f.fsType),
  )

  return (
    <div className="pl scan">
      {libraries?.map((l) => (
        <Follow key={l.id} id={l.id} onState={onState} />
      ))}
      <AmbientBackdrop src={latestBackdrop ? posterSrc(latestBackdrop.imageId, 1280) : undefined} />
      <div className="scan__frame">
        <header className="scan__bar">
          <span className="scan__wordmark">{wordmark}</span>
          <span className="ob__meta">{t('onboarding.start.version', { version, serverName })}</span>
        </header>
        <div className="scan__body">
          <main className="scan__main">
            <div
              style={{
                display: 'flex',
                alignItems: 'flex-end',
                justifyContent: 'space-between',
                gap: 24,
                flexWrap: 'wrap',
              }}
            >
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <p className="ob__step">{t('scan.title')}</p>
                {voice && (
                  <h1 className="scan__voice" aria-hidden="true">
                    {t('scan.voice', { latestTitle: voice.title, count: voice.count })}
                  </h1>
                )}
                <p className="pl-sr" aria-live="polite">
                  {announced
                    ? t('scan.voice', { latestTitle: announced.title, count: announced.count })
                    : ''}
                </p>
              </div>
              {libraries && libraries.length > 1 && (
                <div className="scan__tabs" role="tablist" aria-label={t('scan.libraries')}>
                  {libraries.map((l) => (
                    <button
                      key={l.id}
                      type="button"
                      role="tab"
                      className="scan__tab pl-focusable"
                      aria-selected={l.id === current}
                      onClick={() => setTab(l.id)}
                    >
                      {l.name}
                    </button>
                  ))}
                </div>
              )}
            </div>
            {c && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <ProgressBar
                  variant="scan"
                  value={scanFill(state)}
                  label={lib ? stateLine(lib) : ''}
                  valueLabel={
                    p?.state === 'idle'
                      ? ''
                      : t('scan.analyzed', { done: c.analyzed, total: c.found })
                  }
                  error={!!state?.failed}
                />
                <p className="scan__counts">
                  {[
                    m &&
                      t('scan.metadata.summary', {
                        matched: m.matched,
                        needsReview: m.needsReview,
                        unmatched: m.unmatched,
                      }),
                    c.unrecognized > 0 && t('scan.unrecognized', { count: c.unrecognized }),
                    c.failed > 0 && t('scan.failed', { count: c.failed }),
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
              </div>
            )}
            <PosterWall
              items={matched}
              slots={desktop ? 24 : 12}
              label={t('scan.wall', { library: lib?.name ?? '' })}
            />
          </main>
          <aside className="scan__aside">
            <p>{t('scan.leave', { serverName })}</p>
            {libraries && libraries.length > 1 && (
              <section>
                <h2>{t('scan.libraries')}</h2>
                {libraries.map((l) => (
                  <p key={l.id}>
                    <strong>{l.name}</strong>
                    <br />
                    {stateLine(l)}
                  </p>
                ))}
              </section>
            )}
            <section>
              <h2>{serverName}</h2>
              <ServerFacts
                caps={caps}
                maxTranscodes={maxTranscodes}
                networkFolders={networkFolders.map((f) => f.path)}
              />
            </section>
            <button
              type="button"
              className="ob__link"
              style={{ alignSelf: 'flex-start' }}
              onClick={() => void navigate('/')}
            >
              {t('scan.startWatching')}
            </button>
          </aside>
        </div>
      </div>
      <OfflineScreen connection={connection} onRetry={() => void retryNow()} />
    </div>
  )
}

/** The brand wordmark, never translated. */
const wordmark = 'PLAID'

/** Only the facts that change something for the admin (F007 R13). */
function ServerFacts({
  caps,
  maxTranscodes,
  networkFolders,
}: {
  caps?: Capabilities
  maxTranscodes?: number
  networkFolders: string[]
}) {
  const { t } = useTranslation()
  if (!caps) return null
  const active = caps.hwAccel.active
  const device = (a: string) => {
    switch (a) {
      case 'nvenc':
        return t('server.accel.nvenc')
      case 'qsv':
        return t('server.accel.qsv')
      case 'vaapi':
        return t('server.accel.vaapi')
      case 'v4l2m2m':
        return t('server.accel.v4l2m2m')
      default:
        return t('server.accel.videotoolbox')
    }
  }
  return (
    <>
      {!caps.ffmpeg.found ? (
        <ServerFact tone="warning" command="sudo apt install ffmpeg">
          {t('server.ffmpeg.missing')}
        </ServerFact>
      ) : active ? (
        <ServerFact tone="hardware">
          {t('server.hw.active', {
            device: device(active),
            method: active.toUpperCase(),
            max: maxTranscodes ?? 1,
          })}
        </ServerFact>
      ) : (
        <ServerFact tone="hardware">{t('server.hw.none')}</ServerFact>
      )}
      {!caps.ffprobe.found && <ServerFact tone="warning">{t('server.ffprobe.missing')}</ServerFact>}
      {networkFolders.map((path) => (
        <ServerFact key={path} tone="info">
          {t('server.watch.network', { path, minutes: 15 })}
        </ServerFact>
      ))}
      {caps.watchLimitReached.map((path) => (
        <ServerFact
          key={path}
          tone="warning"
          command="echo fs.inotify.max_user_watches=524288 | sudo tee /etc/sysctl.d/90-plaid.conf && sudo sysctl --system"
        >
          {t('server.watch.limit', { path, minutes: 15 })}
        </ServerFact>
      ))}
    </>
  )
}
