import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { formatClock, formatSpokenTime } from '../lib/format'
import { cx } from '../ui'

/**
 * The timeline (F008-R36): played (woven), buffered, a thumb, the time
 * under the pointer. A slider with the time in words; the arrows, Home
 * and End are the player's shortcuts.
 */
export function Timeline({
  timeMs,
  durationMs,
  bufferedMs,
  onSeek,
}: {
  timeMs: number
  durationMs: number
  bufferedMs: number
  onSeek: (ms: number) => void
}) {
  const { t, i18n } = useTranslation()
  const ref = useRef<HTMLDivElement>(null)
  const [hover, setHover] = useState<number | null>(null)
  const [scrub, setScrub] = useState<number | null>(null)
  const pct = (ms: number) => (durationMs > 0 ? Math.min(100, (ms / durationMs) * 100) : 0)
  const at = (clientX: number) => {
    const r = ref.current!.getBoundingClientRect()
    return Math.max(0, Math.min(1, (clientX - r.left) / (r.width || 1))) * durationMs
  }
  const shown = scrub ?? timeMs
  const lng = i18n.language

  return (
    <div
      ref={ref}
      className={cx('pl-tl', hover !== null && 'is-hover', scrub !== null && 'is-scrubbing')}
      role="slider"
      tabIndex={0}
      aria-label={t('player.timeline.label')}
      aria-valuemin={0}
      aria-valuemax={Math.round(durationMs / 1000)}
      aria-valuenow={Math.round(shown / 1000)}
      aria-valuetext={t('player.timeline.valueText', {
        elapsed: formatSpokenTime(lng, shown),
        duration: formatSpokenTime(lng, durationMs),
      })}
      onPointerMove={(e) => {
        if (e.pointerType === 'mouse') setHover(at(e.clientX))
        if (scrub !== null) setScrub(at(e.clientX))
      }}
      onPointerLeave={() => setHover(null)}
      onPointerDown={(e) => {
        e.currentTarget.setPointerCapture?.(e.pointerId)
        setScrub(at(e.clientX))
      }}
      onPointerUp={(e) => {
        if (scrub === null) return
        onSeek(at(e.clientX))
        setScrub(null)
      }}
      onPointerCancel={() => setScrub(null)}
    >
      <div className="pl-tl__track">
        <div className="pl-tl__seg" style={{ flex: 1 }}>
          <div className="pl-tl__buf" style={{ width: `${pct(bufferedMs)}%` }} />
          {hover !== null && hover > shown && (
            <div className="pl-tl__hoverfill" style={{ width: `${pct(hover)}%` }} />
          )}
          <div className="pl-tl__played" style={{ width: `${pct(shown)}%` }} />
        </div>
      </div>
      <div className="pl-tl__thumb" style={{ left: `${pct(shown)}%` }} />
      {(scrub ?? hover) !== null && (
        <div className="pv-tip" style={{ left: `${pct((scrub ?? hover)!)}%` }} aria-hidden="true">
          {formatClock((scrub ?? hover)!)}
        </div>
      )}
    </div>
  )
}
