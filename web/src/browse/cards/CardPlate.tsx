import { useTranslation } from 'react-i18next'
import { cx } from '../../ui'

/**
 * Where artwork isn't there: the title on a plate. While the scan is still
 * looking the item up (`pending`), the plate shimmers and says so.
 */
export function CardPlate({ title, pending }: { title: string; pending?: boolean }) {
  const { t } = useTranslation()
  return (
    <span className={cx('br-card__plate', pending && 'is-pending')} aria-hidden="true">
      {pending && <span className="br-card__pending">{t('card.pending')}</span>}
      {title}
    </span>
  )
}
