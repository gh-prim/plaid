import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { imageSrc } from '../browse/media'
import { formatDuration } from '../lib/format'
import { Icon } from '../ui'
import { countdownSeconds } from './upNext'

type Item = components['schemas']['ItemSummary']

function episodeLine(t: ReturnType<typeof useTranslation>['t'], next: Item) {
  return next.seasonNumber !== undefined && next.episodeNumber !== undefined
    ? t('card.episode', {
        season: next.seasonNumber,
        episode: next.episodeNumber,
        title: next.displayTitle,
      })
    : next.displayTitle
}

/**
 * The next episode's card (F008-R43): in the last 30 s, then a 10 s
 * countdown when the video ends. The ring drains once, never pulses.
 */
export function NextEpisode({
  next,
  counting,
  onPlay,
  onCancel,
}: {
  next: Item
  counting: boolean
  onPlay: () => void
  onCancel: () => void
}) {
  const { t, i18n } = useTranslation()
  const [left, setLeft] = useState(countdownSeconds)
  const play = useRef(onPlay)
  useEffect(() => {
    play.current = onPlay
  })
  useEffect(() => {
    if (!counting) return
    const start = Date.now()
    const id = setInterval(() => {
      const rest = Math.max(0, countdownSeconds - Math.floor((Date.now() - start) / 1000))
      setLeft(rest)
      if (rest === 0) {
        clearInterval(id)
        play.current()
      }
    }, 250)
    return () => clearInterval(id)
  }, [counting])

  const art = next.still ?? next.backdrop
  const meta = [
    next.show?.displayTitle,
    next.runtimeMinutes ? formatDuration(i18n.language, next.runtimeMinutes * 60_000) : '',
  ]
    .filter(Boolean)
    .join(' · ')
  const r = 26
  const c = 2 * Math.PI * r
  return (
    <div className="pl-next pv-next" role="region" aria-label={t('player.next.label')}>
      <div className="pl-next__top">
        <div className="pl-next__thumb">
          {art && <img src={imageSrc(art.imageId, 342)} alt="" className="pv-cover" />}
          {counting && (
            <div className="pl-next__ring">
              <svg
                width="60"
                height="60"
                viewBox="0 0 60 60"
                aria-hidden="true"
                className="pv-ring"
              >
                <circle className="ring-track" cx="30" cy="30" r={r} strokeWidth="3.5" />
                <circle
                  className="ring-fill"
                  cx="30"
                  cy="30"
                  r={r}
                  strokeWidth="3.5"
                  strokeDasharray={c}
                  strokeDashoffset={c * (1 - left / countdownSeconds)}
                  transform="rotate(-90 30 30)"
                />
              </svg>
              <b>{left}</b>
            </div>
          )}
        </div>
        <div>
          <div className="pl-next__kicker" aria-live="polite">
            {counting ? t('player.next.countdown', { count: left }) : t('player.next.label')}
          </div>
          <div className="pl-next__title">{episodeLine(t, next)}</div>
          {meta && <div className="pl-next__meta">{meta}</div>}
        </div>
      </div>
      <div className="pl-next__actions">
        <button type="button" className="pl-btn pl-btn--primary pl-focusable" onClick={onPlay}>
          <Icon name="play" size={18} />
          {t('player.next.playNow')}
        </button>
        <button type="button" className="pl-btn pl-btn--secondary pl-focusable" onClick={onCancel}>
          {t('player.next.cancel')}
        </button>
      </div>
    </div>
  )
}

/** "Still watching?" after 3 countdowns in a row without input (F008-R44). */
export function StillWatching({
  next,
  onContinue,
  onStop,
}: {
  next: Item
  onContinue: () => void
  onStop: () => void
}) {
  const { t } = useTranslation()
  const id = useId()
  const first = useRef<HTMLButtonElement>(null)
  useEffect(() => first.current?.focus(), [])
  return (
    <div className="pv-center pv-dim">
      <div className="pv-warm" role="dialog" aria-modal="true" aria-labelledby={id}>
        <h2 id={id} className="pv-warm__title">
          {t('player.stillWatching.title')}
        </h2>
        <p className="pv-warm__text">
          {[t('player.next.label'), episodeLine(t, next)].join(' · ')}
        </p>
        <div className="pv-warm__actions">
          <button
            ref={first}
            type="button"
            className="pl-btn pl-btn--primary pl-focusable"
            onClick={onContinue}
          >
            {t('player.stillWatching.continue')}
          </button>
          <button type="button" className="pl-btn pl-btn--secondary pl-focusable" onClick={onStop}>
            {t('player.stillWatching.stop')}
          </button>
        </div>
      </div>
    </div>
  )
}

/** The last episode ended: one warm line, nothing plays (F008-R43). */
export function EndOfShow({ show, onBack }: { show: string; onBack: () => void }) {
  const { t } = useTranslation()
  const back = useRef<HTMLButtonElement>(null)
  useEffect(() => back.current?.focus(), [])
  return (
    <div className="pv-center pv-dim" role="status">
      <div className="pv-warm">
        <h2 className="pv-warm__title">{t('player.endOfShow.title', { show })}</h2>
        <div className="pv-warm__actions">
          <button
            ref={back}
            type="button"
            className="pl-btn pl-btn--secondary pl-focusable"
            onClick={onBack}
          >
            {t('player.endOfShow.back', { show })}
          </button>
        </div>
      </div>
    </div>
  )
}
