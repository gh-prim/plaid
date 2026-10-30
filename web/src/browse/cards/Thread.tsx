import { useTranslation } from 'react-i18next'

/** The woven progress thread on a card (the tartan's one place in the UI). */
export function Thread({ fraction }: { fraction: number }) {
  const { t } = useTranslation()
  const percent = Math.round(fraction * 100)
  return (
    <span
      className="pl-progress br-card__thread"
      role="progressbar"
      aria-label={t('card.progressLabel', { percent })}
      aria-valuenow={percent}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <span className="pl-progress__fill" style={{ width: `${percent}%` }} />
    </span>
  )
}
