import { useEffect, useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { formatDuration, formatNumber, formatClock } from '../lib/format'
import { cx, Icon, type IconName } from '../ui'
import { Timeline } from './Timeline'
import type { MediaState } from './useMedia'

export type Menu = 'subtitles' | 'audio' | 'quality' | 'info' | 'shortcuts'

interface Pill {
  menu: Exclude<Menu, 'shortcuts' | 'info'>
  icon: IconName
  label: string
  value: string
}

/**
 * The controls, direction A of the canvas (F008-R36): words on the scrim,
 * the title on top, the timeline with both times, then the buttons and
 * the pills that name what plays.
 */
export function Controls({
  title,
  line,
  media,
  pills,
  menu,
  panel,
  canPip,
  fullscreen,
  onBack,
  onToggle,
  onSkip,
  onSeek,
  onVolume,
  onMute,
  onMenu,
  onPip,
  onFullscreen,
}: {
  title: string
  line?: string
  media: MediaState
  pills: Pill[]
  menu: Menu | null
  /** The open menu's panel, anchored above its pill. */
  panel: ReactNode
  canPip: boolean
  fullscreen: boolean
  onBack: () => void
  onToggle: () => void
  onSkip: (seconds: number) => void
  onSeek: (ms: number) => void
  onVolume: (v: number) => void
  onMute: () => void
  onMenu: (m: Menu | null) => void
  onPip: () => void
  onFullscreen: () => void
}) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const ten = formatNumber(lng, 10)
  const toggle = (m: Menu) => onMenu(menu === m ? null : m)
  // A closed menu gives the focus back to its pill (keyboard users).
  const open = useRef<Menu | null>(null)
  useEffect(() => {
    const was = open.current
    open.current = menu
    if (was && !menu && document.activeElement === document.body)
      document.querySelector<HTMLElement>(`[data-menu-pill="${was}"]`)?.focus()
  }, [menu])
  const muted = media.muted || media.volume === 0
  const left = Math.max(0, media.durationMs - media.timeMs)

  return (
    <>
      <div className="pl-player__top pv-top">
        <button
          type="button"
          className="pl-pbtn pl-focusable"
          aria-label={t('player.back')}
          onClick={onBack}
        >
          <Icon name="arrowLeft" />
        </button>
        <div className="pl-player__titles">
          <div className="pl-player__title">{title}</div>
          {line && <div className="pl-player__sub">{line}</div>}
        </div>
      </div>

      <div className="pl-player__bottom pv-bottom">
        <div className="pl-player__tlrow">
          <span className="pl-player__time">{formatClock(media.timeMs)}</span>
          <Timeline
            timeMs={media.timeMs}
            durationMs={media.durationMs}
            bufferedMs={media.bufferedMs}
            onSeek={onSeek}
          />
          <span className="pl-player__time pv-left">
            {t('player.remaining', { duration: formatDuration(lng, left) })}
          </span>
        </div>
        <div className="pl-player__row pv-row">
          <button
            type="button"
            className="pl-pbtn pl-focusable pv-main"
            aria-label={media.paused ? t('player.play') : t('player.pause')}
            onClick={onToggle}
          >
            <Icon name={media.paused ? 'play' : 'pause'} />
          </button>
          <button
            type="button"
            className="pl-pbtn pl-focusable pv-skip"
            aria-label={t('player.back10')}
            onClick={() => onSkip(-10)}
          >
            <Icon name="back10" />
            <span className="pv-skip__n" aria-hidden="true">
              {ten}
            </span>
          </button>
          <button
            type="button"
            className="pl-pbtn pl-focusable pv-skip"
            aria-label={t('player.forward10')}
            onClick={() => onSkip(10)}
          >
            <Icon name="forward10" />
            <span className="pv-skip__n" aria-hidden="true">
              {ten}
            </span>
          </button>
          <div className={cx('pl-vol pv-vol', muted && 'is-muted')}>
            <button
              type="button"
              className="pl-pbtn pl-focusable"
              aria-label={muted ? t('player.unmute') : t('player.mute')}
              onClick={onMute}
            >
              <Icon name={muted ? 'volumeOff' : 'volume'} />
            </button>
            <input
              type="range"
              className="pv-vol__range"
              min={0}
              max={1}
              step={0.05}
              value={media.muted ? 0 : media.volume}
              aria-label={t('player.volume.label')}
              aria-valuetext={t('player.volume.valueText', {
                percent: Math.round((media.muted ? 0 : media.volume) * 100),
              })}
              style={{ ['--v' as string]: `${(media.muted ? 0 : media.volume) * 100}%` }}
              onChange={(e) => onVolume(Number(e.target.value))}
            />
          </div>
          <span className="pl-player__spacer" />
          {pills.map((p) => (
            <div key={p.menu} className="pv-menuwrap">
              <button
                type="button"
                data-menu-pill={p.menu}
                className={cx(
                  'pl-pbtn pl-pbtn--pill pl-focusable pv-pill',
                  menu === p.menu && 'is-open',
                )}
                aria-label={`${p.label}: ${p.value}`}
                aria-haspopup="menu"
                aria-expanded={menu === p.menu}
                onClick={() => toggle(p.menu)}
              >
                <Icon name={p.icon} size={20} />
                <span className="pv-pill__value">{p.value}</span>
              </button>
              {menu === p.menu && panel}
            </div>
          ))}
          <div className="pv-menuwrap">
            <button
              type="button"
              data-menu-pill="info"
              className={cx('pl-pbtn pl-focusable', menu === 'info' && 'is-open')}
              aria-label={t('player.info.label')}
              aria-haspopup="dialog"
              aria-expanded={menu === 'info'}
              onClick={() => toggle('info')}
            >
              <Icon name="info" />
            </button>
            {menu === 'info' && panel}
          </div>
          {canPip && (
            <button
              type="button"
              className="pl-pbtn pl-focusable"
              aria-label={t('player.pip')}
              onClick={onPip}
            >
              <Icon name="pip" />
            </button>
          )}
          <button
            type="button"
            className="pl-pbtn pl-focusable"
            aria-label={fullscreen ? t('player.exitFullscreen') : t('player.fullscreen')}
            onClick={onFullscreen}
          >
            <Icon name={fullscreen ? 'fullscreenExit' : 'fullscreen'} />
          </button>
        </div>
      </div>
    </>
  )
}
