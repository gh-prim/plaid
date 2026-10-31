import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../auth/AuthProvider'
import { Icon } from '../ui'
import { usePreviewsOn } from './previewPref'

/** The backdrop shows alone this long before the scenes fade in. */
const backdropFirst = 2000
/** Asking again for a preview being cut stops after this many tries. */
const maxTries = 40

/**
 * Whether this browser should play a preview at all: not with reduced
 * motion, not when saving data, not without IntersectionObserver (it's
 * how the preview pauses off screen).
 */
function canPreview(): boolean {
  if (typeof IntersectionObserver === 'undefined') return false
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return false
  const conn = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection
  return !conn?.saveData
}

/**
 * Asks the server for the item's preview until it's cut: the URL once
 * it's ready, undefined while waiting or when there's none.
 */
function usePreviewUrl(itemId: string, on: boolean): string | undefined {
  const [ready, setReady] = useState<{ id: string; url: string }>()
  useEffect(() => {
    if (!on) return
    const url = `${window.location.origin}/api/v1/items/${encodeURIComponent(itemId)}/preview`
    let stop = false
    let timer = 0
    let tries = 0
    const ask = async () => {
      tries++
      try {
        const r = await fetch(url, { method: 'HEAD', headers: { 'X-Plaid-Client': 'web' } })
        if (stop) return
        if (r.status === 200) setReady({ id: itemId, url })
        else if (r.status === 202 && tries < maxTries) {
          const wait = Number(r.headers.get('Retry-After')) || 5
          timer = window.setTimeout(() => void ask(), wait * 1000)
        }
      } catch {
        // No preview: the backdrop stays.
      }
    }
    void ask()
    return () => {
      stop = true
      window.clearTimeout(timer)
    }
  }, [itemId, on])
  return on && ready?.id === itemId ? ready.url : undefined
}

/**
 * The hero's moving picture (V2): a few silent scenes from the title,
 * looping over the backdrop. The backdrop shows first, then the scenes
 * fade in once they play; they pause off screen and in a hidden tab, and
 * a button pauses them for good on this page.
 */
export function HeroPreview({ itemId }: { itemId: string }) {
  const { t } = useTranslation()
  const { state } = useAuth()
  const profileId = state.status === 'ready' ? (state.me?.activeProfile?.id ?? '') : ''
  const allowed = usePreviewsOn(profileId)
  const [supported] = useState(canPreview)
  const url = usePreviewUrl(itemId, allowed && supported)
  const [waited, setWaited] = useState(false)
  const [playing, setPlaying] = useState(false)
  const [paused, setPaused] = useState(false)
  const video = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const id = window.setTimeout(() => setWaited(true), backdropFirst)
    return () => window.clearTimeout(id)
  }, [])

  const show = !!url && waited
  // Plays only on screen, in a visible tab, and unless paused.
  useEffect(() => {
    const v = video.current
    if (!show || !v) return
    let visible = true
    const sync = () => {
      if (visible && !paused && document.visibilityState === 'visible')
        void v.play().catch(() => {})
      else v.pause()
    }
    const io = new IntersectionObserver(
      ([e]) => {
        visible = e?.isIntersecting ?? true
        sync()
      },
      { threshold: 0.25 },
    )
    io.observe(v)
    document.addEventListener('visibilitychange', sync)
    sync()
    return () => {
      io.disconnect()
      document.removeEventListener('visibilitychange', sync)
    }
  }, [show, paused])

  if (!show) return null
  return (
    <>
      <video
        ref={video}
        className={`br-detail__video${playing ? ' is-playing' : ''}`}
        src={url}
        muted
        loop
        playsInline
        preload="auto"
        aria-hidden="true"
        onPlaying={() => setPlaying(true)}
      />
      {playing && (
        <button
          type="button"
          className="br-detail__pause br-glass pl-focusable"
          aria-label={paused ? t('item.preview.play') : t('item.preview.pause')}
          aria-pressed={paused}
          onClick={() => setPaused((p) => !p)}
        >
          <Icon name={paused ? 'play' : 'pause'} size={16} />
        </button>
      )}
    </>
  )
}
