import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

/** A title or library that doesn't exist for this profile (F008-R24). */
export function Unavailable() {
  const { t } = useTranslation()
  return (
    <section className="br-page br-empty">
      <h1 className="br-empty__title br-empty__title--small">{t('item.unavailable.title')}</h1>
      <p className="br-empty__body">{t('item.unavailable.body')}</p>
      <Link to="/" className="pl-btn pl-btn--secondary pl-focusable br-empty__action">
        {t('item.unavailable.back')}
      </Link>
    </section>
  )
}
