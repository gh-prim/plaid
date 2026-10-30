import { useEffect, useId, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { imageSrc } from '../browse/media'
import { formatClock } from '../lib/format'
import { Icon } from '../ui'

type Item = components['schemas']['ItemDetail']

/** "Pick up where you left off?" before a plain Play (F008-R40). */
export function ResumePrompt({
  item,
  positionMs,
  onResume,
  onStartOver,
}: {
  item: Item
  positionMs: number
  onResume: () => void
  onStartOver: () => void
}) {
  const { t } = useTranslation()
  const id = useId()
  const resume = useRef<HTMLButtonElement>(null)
  useEffect(() => resume.current?.focus(), [])
  const art = item.still ?? item.backdrop
  const duration = item.userState?.durationMs ?? 0
  const position = formatClock(positionMs)
  return (
    <div className="pv-center pv-dim">
      <div className="pl-resume pv-resume" role="dialog" aria-modal="true" aria-labelledby={id}>
        <div className="pl-resume__art">
          {art && <img src={imageSrc(art.imageId, 780)} alt="" className="pv-cover" />}
          {duration > 0 && (
            <div className="pv-resume__bar">
              <div style={{ width: `${Math.min(100, (positionMs / duration) * 100)}%` }} />
            </div>
          )}
        </div>
        <div className="pl-resume__body">
          <h2 id={id} className="pl-resume__title">
            {t('player.resume.title')}
          </h2>
          <p className="pl-resume__text">
            {t('player.resume.text', { position, title: item.displayTitle })}
          </p>
          <div className="pl-resume__actions">
            <button
              ref={resume}
              type="button"
              className="pl-btn pl-btn--primary pl-focusable"
              onClick={onResume}
            >
              <Icon name="play" size={18} />
              {t('playback.resume', { position })}
            </button>
            <button
              type="button"
              className="pl-btn pl-btn--secondary pl-focusable"
              onClick={onStartOver}
            >
              {t('playback.startOver')}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
