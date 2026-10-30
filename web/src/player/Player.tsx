import {
  useCallback,
  useEffect,
  useEffectEvent,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { presentFiles } from '../browse/itemFacts'
import { formatDuration } from '../lib/format'
import { cx, Toast } from '../ui'
import { capabilityProfile, nativeHls } from './capabilities'
import { Controls, type Menu } from './Controls'
import { attach, showSubtitle, type Engine, type EngineFailure } from './engine'
import { InfoPanel } from './InfoPanel'
import { audioLabel, languageLabel, qualityLabel, subtitleLabel } from './labels'
import { setMediaSession } from './mediaSession'
import { EndOfShow, NextEpisode, StillWatching } from './NextEpisode'
import { PlayerError, type PlayerProblem } from './PlayerError'
import { ResumePrompt } from './ResumePrompt'
import {
  Progress,
  startSession,
  updateSession,
  type PlaybackRequest,
  type PlaybackSession,
  type PlaybackUpdate,
} from './session'
import { shortcutFor } from './shortcuts'
import { ShortcutsDialog } from './ShortcutsDialog'
import { AudioMenu, QualityMenu, sdhOf, SubtitlesMenu } from './TrackMenus'
import {
  cycleSubtitle,
  loadAutoplay,
  loadChoice,
  loadVolume,
  pickTracks,
  rememberAudio,
  rememberSubtitles,
  rememberVolume,
} from './trackMemory'
import { endPhase } from './upNext'
import { useCues, useMedia } from './useMedia'
import './player.css'

type Item = components['schemas']['ItemDetail']
type Summary = components['schemas']['ItemSummary']
type MediaFile = components['schemas']['MediaFile']
type Quality = components['schemas']['Quality']

type Phase =
  | { k: 'loading' }
  | { k: 'ask'; positionMs: number }
  | { k: 'starting' }
  | { k: 'playing' }
  | { k: 'problem'; problem: PlayerProblem; positionMs: number }

const hideAfterMs = 3000
const ladder: Quality[] = ['2160p', '1080p', '720p', '480p', '360p']
const ladderHeight: Record<string, number> = {
  '2160p': 2160,
  '1080p': 1080,
  '720p': 720,
  '480p': 480,
  '360p': 360,
}

/**
 * The player (F008-R33): `/watch/:itemId?fileId=&t=`, full screen in the
 * page and always dark. The shell stays across episodes, so full screen
 * does too; each item gets its own playback.
 */
export default function Player() {
  const { itemId = '' } = useParams()
  const [params] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const shell = useRef<HTMLDivElement>(null)
  const t = params.get('t')
  const startMs = t !== null && /^\d+$/.test(t) ? Number(t) * 1000 : null
  const autoplayed = (location.state as { autoplayed?: number } | null)?.autoplayed ?? 0
  const fromApp = location.key !== 'default'
  const back = useCallback(
    (fallback: string) => {
      if (fromApp) void navigate(-1)
      else void navigate(fallback, { replace: true })
    },
    [fromApp, navigate],
  )
  return (
    <div ref={shell} className="pl pl-player pv-player" data-theme="dark">
      <Playback
        key={itemId}
        itemId={itemId}
        fileId={params.get('fileId') ?? undefined}
        startMs={startMs}
        autoplayed={autoplayed}
        shell={shell}
        onBack={back}
      />
    </div>
  )
}

function Playback({
  itemId,
  fileId,
  startMs,
  autoplayed,
  shell,
  onBack,
}: {
  itemId: string
  fileId?: string
  startMs: number | null
  autoplayed: number
  shell: RefObject<HTMLDivElement | null>
  onBack: (fallback: string) => void
}) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const navigate = useNavigate()
  const { state: auth } = useAuth()
  const me = auth.status === 'ready' ? auth.me : undefined
  const profileId = me?.activeProfile?.id ?? ''
  const isAdmin = me?.user.role === 'admin'
  const video = useRef<HTMLVideoElement>(null)
  const media = useMedia(video)
  const profile = useMemo(() => capabilityProfile(), [])

  const [item, setItem] = useState<Item | null>(null)
  const [phase, setPhase] = useState<Phase>({ k: 'loading' })
  const [session, setSession] = useState<PlaybackSession | null>(null)
  const [sub, setSub] = useState<number | null>(null)
  const [menu, setMenu] = useState<Menu | null>(null)
  const [reload, setReload] = useState<string | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const [next, setNext] = useState<Summary | null | undefined>(undefined)
  const [cancelled, setCancelled] = useState(false)
  const [chain, setChain] = useState(autoplayed)
  const [shown, setShown] = useState(true)
  const [hovering, setHovering] = useState(false)
  const [focusIn, setFocusIn] = useState(false)
  const [fullscreen, setFullscreen] = useState(false)
  const [nativeFs, setNativeFs] = useState(false)
  const engine = useRef<Engine | null>(null)
  const progress = useRef<Progress | null>(null)
  const retried = useRef(false)
  const hideTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const lastTap = useRef<{ at: number; x: number } | null>(null)

  const file: MediaFile | undefined = item
    ? (item.files.find((f) => f.id === (session?.fileId ?? fileId)) ?? presentFiles(item.files)[0])
    : undefined
  const sdh = sdhOf(file)
  const detailPath = `/item/${item?.show?.id ?? itemId}`
  const pos = () => Math.round((video.current?.currentTime ?? 0) * 1000)

  // ---- Sessions ----

  async function attachStream(s: PlaybackSession, atMs: number, it: Item | null = item) {
    engine.current?.destroy()
    const v = video.current!
    engine.current = await attach({
      video: v,
      url: s.url,
      method: s.method,
      startMs: atMs,
      nativeHls: nativeHls(),
      onFatal: (f) => onFatal(f, it),
    })
    v.play().catch(() => {
      // Autoplay refused: the play button is there.
    })
  }

  function tracksFor(it: Item): Pick<PlaybackRequest, 'audioStreamIndex' | 'subtitleStreamIndex'> {
    const choice = loadChoice(profileId)
    const files = fileId ? it.files.filter((f) => f.id === fileId) : presentFiles(it.files)
    const picks = files.map((f) => pickTracks(choice, f.streams))
    // Indexes differ between files: apply them only when every file agrees.
    const same = picks.every((p) => JSON.stringify(p) === JSON.stringify(picks[0]))
    return same && picks[0] ? picks[0] : {}
  }

  async function start(
    it: Item,
    atMs: number,
    extra: { forceMethod?: 'transcode'; quality?: Quality } = {},
  ) {
    setPhase({ k: 'starting' })
    setMenu(null)
    const r = await startSession(itemId, {
      profile,
      startPositionMs: atMs,
      burnSubtitles: false,
      ...(fileId ? { fileId } : {}),
      ...tracksFor(it),
      ...extra,
    })
    if ('failure' in r) {
      if (r.failure.code === 'profile_required') {
        void navigate('/profiles', { state: { from: { pathname: `/watch/${itemId}` } } })
        return
      }
      setPhase({ k: 'problem', problem: { kind: 'api', ...r.failure }, positionMs: atMs })
      return
    }
    const s = r.session
    setSession(s)
    setSub(s.subtitles.find((x) => x.selected && x.delivery !== 'burn_in')?.index ?? null)
    const p = new Progress(s.id, () => gone())
    progress.current = p
    p.follow(video.current!)
    setPhase({ k: 'playing' })
    await attachStream(s, atMs, it)
  }

  function halt(atMs: number) {
    progress.current?.stop(atMs)
    engine.current?.destroy()
    engine.current = null
  }

  /** The session ended on the server (F008-R39). */
  function gone() {
    const at = pos()
    engine.current?.destroy()
    engine.current = null
    setReload(null)
    setPhase({ k: 'problem', problem: { kind: 'stopped', positionMs: at }, positionMs: at })
  }

  // `it` comes with the stream: the state may not have it yet on the first start.
  function onFatal(f: EngineFailure, it: Item | null) {
    const at = pos()
    if (f.kind === 'network' && f.status === 404) {
      gone()
      return
    }
    halt(at)
    if (f.kind === 'media' && !retried.current && it) {
      // One automatic retry as a conversion (F008-R45).
      retried.current = true
      void start(it, at, { forceMethod: 'transcode' })
      return
    }
    setPhase({
      k: 'problem',
      problem: f.kind === 'media' ? { kind: 'cantPlay' } : { kind: 'api', code: 'generic' },
      positionMs: at,
    })
  }

  async function patch(body: Omit<PlaybackUpdate, 'positionMs'>, status: string) {
    if (!session) return
    const at = pos()
    const r = await updateSession(session.id, { positionMs: at, ...body })
    if ('failure' in r) {
      if (r.failure.code === 'playback_session_not_found') gone()
      else setToast(r.failure.code)
      return
    }
    const s = r.session
    setSession(s)
    if (s.url !== session.url) {
      // A new decision: reload at the same moment, the last frame stays.
      setReload(status)
      video.current?.addEventListener('playing', () => setReload(null), { once: true })
      await attachStream(s, at)
    }
  }

  function chooseSubtitle(index: number | null) {
    setMenu(null)
    if (!session) return
    const target = session.subtitles.find((s) => s.index === index)
    rememberSubtitles(
      profileId,
      target
        ? { language: target.language, forced: target.forced, sdh: sdh.has(target.index) }
        : 'off',
    )
    const burned = session.subtitles.find((s) => s.selected && s.delivery === 'burn_in')
    if (target?.delivery === 'burn_in') {
      setSub(null)
      void patch(
        { subtitleStreamIndex: target.index, burnSubtitles: true },
        t('player.reload.subtitles', { language: languageLabel(t, lng, target.language) }),
      )
      return
    }
    setSub(index)
    if (burned) void patch({ subtitleStreamIndex: index, burnSubtitles: false }, '')
  }

  function chooseAudio(index: number) {
    setMenu(null)
    const track = session?.audioTracks.find((a) => a.index === index)
    if (!track || track.selected) return
    rememberAudio(profileId, track.language)
    void patch(
      { audioStreamIndex: index },
      t('player.reload.audio', { track: audioLabel(t, lng, track) }),
    )
  }

  function chooseQuality(q: Quality) {
    setMenu(null)
    if (q === session?.quality) return
    void patch({ quality: q }, t('player.reload.quality', { quality: qualityLabel(t, q) }))
  }

  const lower = ladder.find((q) => file?.height && ladderHeight[q]! < file.height)

  function retry() {
    if (!item || phase.k !== 'problem') return
    retried.current = false
    void start(item, phase.positionMs)
  }

  async function copyLink(): Promise<boolean> {
    const r = await startSession(itemId, {
      profile: { preset: 'external', canSwitchAudioTrack: false },
      startPositionMs: 0,
      burnSubtitles: false,
      ...(file ? { fileId: file.id } : {}),
    })
    if ('failure' in r) return false
    try {
      await navigator.clipboard.writeText(new URL(r.session.url, window.location.origin).href)
      return true
    } catch {
      return false
    }
  }

  // ---- Loading, leaving ----

  const loaded = useEffectEvent(
    async (data: Item | undefined, status: number, alive: () => boolean) => {
      if (!data) {
        setPhase({
          k: 'problem',
          problem: { kind: 'api', code: status === 404 ? 'not_found' : 'generic' },
          positionMs: 0,
        })
        return
      }
      if (data.kind !== 'movie' && data.kind !== 'episode') {
        void navigate(`/item/${itemId}`, { replace: true })
        return
      }
      setItem(data)
      const resume = data.userState?.watched ? 0 : (data.userState?.positionMs ?? 0)
      if (startMs !== null) void start(data, startMs)
      else if (resume > 0) setPhase({ k: 'ask', positionMs: resume })
      else void start(data, 0)
      if (data.kind === 'episode') {
        // Without an answer, the episode simply ends on its page.
        const up = await api
          .GET('/items/{itemId}/up-next', { params: { path: { itemId } } })
          .catch(() => undefined)
        if (alive()) setNext(up?.data?.item ?? null)
      }
    },
  )

  useEffect(() => {
    let alive = true
    void api.GET('/items/{itemId}', { params: { path: { itemId } } }).then(
      ({ data, response }) => {
        if (alive) void loaded(data, response.status, () => alive)
      },
      () => {
        // Offline or the server is down: the error screen says so.
        if (alive) void loaded(undefined, 0, () => alive)
      },
    )
    const v = video.current
    const saved = loadVolume()
    if (v && saved) {
      v.volume = saved.volume
      v.muted = saved.muted
    }
    const leave = () => progress.current?.stop(pos(), true)
    window.addEventListener('pagehide', leave)
    return () => {
      alive = false
      window.removeEventListener('pagehide', leave)
      progress.current?.stop(Math.round((v?.currentTime ?? 0) * 1000))
      engine.current?.destroy()
      clearTimeout(hideTimer.current)
    }
    // Mount only: Playback is keyed by item.
  }, [itemId])

  // ---- Subtitles, volume, full screen, OS controls ----

  useEffect(() => {
    const v = video.current
    if (!v) return
    const apply = () => showSubtitle(v, sub, nativeFs ? 'showing' : 'hidden')
    apply()
    const tracks = v.textTracks as TextTrackList | undefined
    tracks?.addEventListener?.('addtrack', apply)
    return () => tracks?.removeEventListener?.('addtrack', apply)
  }, [sub, nativeFs, session])
  const cues = useCues(video, nativeFs ? null : sub)

  useEffect(() => {
    if (media.durationMs > 0) rememberVolume(media.volume, media.muted)
  }, [media.volume, media.muted, media.durationMs])

  useEffect(() => {
    const v = video.current
    const onFs = () => setFullscreen(document.fullscreenElement === shell.current)
    const begin = () => setNativeFs(true)
    const end = () => setNativeFs(false)
    document.addEventListener('fullscreenchange', onFs)
    v?.addEventListener('webkitbeginfullscreen', begin)
    v?.addEventListener('webkitendfullscreen', end)
    return () => {
      document.removeEventListener('fullscreenchange', onFs)
      v?.removeEventListener('webkitbeginfullscreen', begin)
      v?.removeEventListener('webkitendfullscreen', end)
    }
  }, [shell])

  const episodeLine =
    item?.kind === 'episode' && item.seasonNumber !== undefined && item.episodeNumber !== undefined
      ? t('card.episode', {
          season: item.seasonNumber,
          episode: item.episodeNumber,
          title: item.displayTitle,
        })
      : undefined
  const playing = phase.k === 'playing'
  useEffect(() => {
    const v = video.current
    if (!item || !playing || !v) return
    return setMediaSession(item, episodeLine, v)
  }, [item, playing, episodeLine])

  // ---- The end of an episode ----

  const end = endPhase({
    hasNext: next === undefined ? undefined : next !== null,
    isEpisode: item?.kind === 'episode',
    remainingMs: media.durationMs > 0 ? media.durationMs - media.timeMs : Infinity,
    ended: media.ended,
    cancelled,
    autoplay: loadAutoplay(profileId),
    autoplayed: chain,
  })

  useEffect(() => {
    if (media.ended) progress.current?.stop(media.durationMs)
  }, [media.ended, media.durationMs])

  useEffect(() => {
    if (playing && end === 'leave') void navigate(detailPath, { replace: true })
  }, [playing, end, navigate, detailPath])

  function playNext(auto: boolean) {
    if (!next) return
    halt(pos())
    const st = next.userState
    const at = st && !st.watched && st.positionMs > 0 ? Math.floor(st.positionMs / 1000) : 0
    void navigate(`/watch/${next.id}?t=${at}`, {
      replace: true,
      state: { autoplayed: auto ? chain + 1 : 0 },
    })
  }

  // ---- Input ----

  function poke() {
    setShown(true)
    clearTimeout(hideTimer.current)
    hideTimer.current = setTimeout(() => setShown(false), hideAfterMs)
  }

  function toggle() {
    const v = video.current
    if (!v) return
    if (v.paused) void v.play().catch(() => {})
    else v.pause()
  }

  function seekTo(ms: number) {
    const v = video.current
    if (!v) return
    v.currentTime = Math.max(0, Math.min(ms, media.durationMs || ms)) / 1000
  }

  function toggleFullscreen() {
    const el = shell.current
    if (document.fullscreenElement) void document.exitFullscreen().catch(() => {})
    else if (el?.requestFullscreen) void el.requestFullscreen().catch(() => {})
    else
      (
        video.current as HTMLVideoElement & { webkitEnterFullscreen?: () => void }
      )?.webkitEnterFullscreen?.()
  }

  const textSubs = (session?.subtitles ?? [])
    .filter((s) => s.url && s.delivery !== 'burn_in')
    .map((s) => ({ ...s, sdh: sdh.has(s.index) }))

  const onKey = useEffectEvent((e: KeyboardEvent) => {
    setChain(0)
    poke()
    if (phase.k !== 'playing' || menu) return
    const target = e.target as HTMLElement
    if (target.closest?.('input, textarea, select, [contenteditable="true"]')) return
    const s = shortcutFor(e)
    if (!s) return
    if (e.key === ' ' && target.closest?.('button, a, [role="menuitemradio"]')) return
    e.preventDefault()
    const v = video.current!
    switch (s.type) {
      case 'toggle':
        return toggle()
      case 'seek':
        return seekTo(pos() + s.seconds * 1000)
      case 'volume':
        v.muted = false
        v.volume = Math.max(0, Math.min(1, Math.round((v.volume + s.by) * 10) / 10))
        return
      case 'mute':
        v.muted = !v.muted
        return
      case 'fullscreen':
        return toggleFullscreen()
      case 'subtitles':
        return chooseSubtitle(cycleSubtitle(sub, textSubs, loadChoice(profileId).subtitles))
      case 'next':
        return playNext(false)
      case 'escape':
        if (document.fullscreenElement) void document.exitFullscreen().catch(() => {})
        return
      case 'jump':
        return seekTo(s.fraction * media.durationMs)
      case 'start':
        return seekTo(0)
      case 'end':
        return seekTo(media.durationMs)
      case 'help':
        return setMenu('shortcuts')
    }
  })

  useEffect(() => {
    const onDown = (e: KeyboardEvent) => onKey(e)
    window.addEventListener('keydown', onDown)
    return () => window.removeEventListener('keydown', onDown)
  }, [])

  function onSurface(e: React.PointerEvent) {
    setChain(0)
    if (e.pointerType !== 'touch') {
      toggle()
      poke()
      return
    }
    // Touch: a tap shows or hides the controls; a double tap on a side seeks.
    const now = Date.now()
    const rect = e.currentTarget.getBoundingClientRect()
    const x = (e.clientX - rect.left) / (rect.width || 1)
    const prev = lastTap.current
    lastTap.current = { at: now, x }
    if (prev && now - prev.at < 300 && (x < 0.4 || x > 0.6)) {
      seekTo(pos() + (x < 0.5 ? -10_000 : 10_000))
      lastTap.current = null
      poke()
      return
    }
    if (shown && playing) {
      clearTimeout(hideTimer.current)
      setShown(false)
    } else poke()
  }

  // ---- Render ----

  const blocking = end === 'countdown' || end === 'stillWatching' || end === 'endOfShow'
  const visible =
    playing && !blocking && (shown || media.paused || menu !== null || hovering || focusIn)
  const title =
    item?.kind === 'episode' ? (item.show?.displayTitle ?? item.displayTitle) : item?.displayTitle
  const line =
    item?.kind === 'episode'
      ? episodeLine
      : [item?.year, item?.runtimeMinutes ? formatDuration(lng, item.runtimeMinutes * 60_000) : '']
          .filter(Boolean)
          .join(' · ')
  const selectedAudio = session?.audioTracks.find((a) => a.selected)
  const subValue =
    sub !== null
      ? (() => {
          const s = session!.subtitles.find((x) => x.index === sub)!
          return subtitleLabel(t, lng, s, sdh.has(s.index))
        })()
      : (() => {
          const burned = session?.subtitles.find((s) => s.selected && s.delivery === 'burn_in')
          return burned
            ? subtitleLabel(t, lng, burned, sdh.has(burned.index))
            : t('player.subtitles.off')
        })()
  const pills = session
    ? [
        ...(session.subtitles.length
          ? [
              {
                menu: 'subtitles' as const,
                icon: 'subtitles' as const,
                label: t('player.subtitles.label'),
                value: subValue,
              },
            ]
          : []),
        ...(session.audioTracks.length > 1 && selectedAudio
          ? [
              {
                menu: 'audio' as const,
                icon: 'audio' as const,
                label: t('player.audio.label'),
                value: audioLabel(t, lng, selectedAudio),
              },
            ]
          : []),
        ...(session.qualities.length > 1
          ? [
              {
                menu: 'quality' as const,
                icon: 'quality' as const,
                label: t('player.quality.label'),
                value: qualityLabel(t, session.quality),
              },
            ]
          : []),
      ]
    : []
  const closeMenu = () => setMenu(null)
  const panel =
    session &&
    (menu === 'subtitles' ? (
      <SubtitlesMenu
        session={session}
        file={file}
        current={
          sub ??
          session.subtitles.find((s) => s.selected && s.delivery === 'burn_in')?.index ??
          null
        }
        onPick={chooseSubtitle}
        onClose={closeMenu}
      />
    ) : menu === 'audio' ? (
      <AudioMenu session={session} onPick={chooseAudio} onClose={closeMenu} />
    ) : menu === 'quality' ? (
      <QualityMenu session={session} file={file} onPick={chooseQuality} onClose={closeMenu} />
    ) : menu === 'info' ? (
      <InfoPanel
        session={session}
        file={file}
        subtitle={sub}
        profile={profile}
        onClose={closeMenu}
      />
    ) : null)

  return (
    <div
      className={cx(
        'pv-stage',
        visible ? 'is-shown' : 'is-hidden',
        menu && 'has-menu',
        end === 'card' && 'has-next',
      )}
      onPointerMove={(e) => e.pointerType === 'mouse' && poke()}
      onPointerDown={() => setChain(0)}
    >
      <video
        ref={video}
        className={cx('pv-video', !playing && phase.k !== 'problem' && 'is-waiting')}
        playsInline
      >
        {textSubs.map((s) => (
          <track
            key={`${s.index}-${s.url}`}
            id={`sub-${s.index}`}
            kind="subtitles"
            src={s.url}
            label={subtitleLabel(t, lng, s, s.sdh)}
          />
        ))}
      </video>
      {playing && <div className="pv-surface" onPointerUp={onSurface} />}
      {visible && <div className="pl-player__scrim" />}

      {cues.length > 0 && (
        <div className="pv-subs" aria-hidden="true">
          {cues.map((c, i) => (
            <span key={i}>{c}</span>
          ))}
        </div>
      )}

      {playing && item && (
        <div
          className="pv-controls"
          onPointerEnter={(e) => e.pointerType === 'mouse' && setHovering(true)}
          onPointerLeave={() => setHovering(false)}
          onFocus={(e) => setFocusIn(e.target.matches(':focus-visible'))}
          onBlur={(e) => {
            if (!e.currentTarget.contains(e.relatedTarget as Node)) setFocusIn(false)
          }}
          inert={!visible}
        >
          <Controls
            title={title ?? ''}
            line={line || undefined}
            media={media}
            pills={pills}
            menu={menu === 'shortcuts' ? null : menu}
            panel={panel}
            canPip={!!document.pictureInPictureEnabled}
            fullscreen={fullscreen}
            onBack={() => onBack(detailPath)}
            onToggle={toggle}
            onSkip={(s) => seekTo(pos() + s * 1000)}
            onSeek={seekTo}
            onVolume={(v) => {
              const el = video.current!
              el.volume = v
              el.muted = v === 0
            }}
            onMute={() => {
              const el = video.current!
              el.muted = !el.muted
            }}
            onMenu={setMenu}
            onPip={() => void video.current?.requestPictureInPicture?.().catch(() => {})}
            onFullscreen={toggleFullscreen}
          />
        </div>
      )}

      {reload && (
        <div className="pv-reload" role="status">
          <span>{reload}</span>
          <span className="pv-reload__line" aria-hidden="true" />
        </div>
      )}
      {toast && (
        <div className="pv-toast">
          <Toast variant="error" onClose={() => setToast(null)}>
            {toastText(t, toast)}
          </Toast>
        </div>
      )}

      {phase.k === 'ask' && item && (
        <ResumePrompt
          item={item}
          positionMs={phase.positionMs}
          onResume={() => void start(item, phase.positionMs)}
          onStartOver={() => void start(item, 0)}
        />
      )}
      {phase.k === 'problem' && (
        <PlayerError
          key={JSON.stringify(phase.problem)}
          problem={phase.problem}
          isAdmin={isAdmin}
          canLower={!!lower}
          onBack={() => onBack(detailPath)}
          onRetry={retry}
          onLower={() => item && lower && void start(item, phase.positionMs, { quality: lower })}
          onCopyLink={copyLink}
        />
      )}
      {playing && next && (end === 'card' || end === 'countdown') && (
        <NextEpisode
          next={next}
          counting={end === 'countdown'}
          onPlay={() => playNext(end === 'countdown')}
          onCancel={() => setCancelled(true)}
        />
      )}
      {playing && next && end === 'stillWatching' && (
        <StillWatching
          next={next}
          onContinue={() => playNext(false)}
          onStop={() => void navigate(detailPath, { replace: true })}
        />
      )}
      {playing && end === 'endOfShow' && item?.show && (
        <EndOfShow
          show={item.show.displayTitle}
          onBack={() => void navigate(detailPath, { replace: true })}
        />
      )}
      {menu === 'shortcuts' && <ShortcutsDialog onClose={closeMenu} />}
    </div>
  )
}

function toastText(t: ReturnType<typeof useTranslation>['t'], code: string): string {
  switch (code) {
    case 'transcode_limit_reached':
      return t('errors.transcode_limit_reached')
    case 'transcode_storage_full':
      return t('errors.transcode_storage_full')
    case 'ffmpeg_missing':
      return t('errors.ffmpeg_missing')
  }
  return t('errors.generic')
}
