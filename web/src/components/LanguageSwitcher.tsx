import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { SUPPORTED_LANGUAGES } from '../i18n'

export function LanguageSwitcher() {
  const { t, i18n } = useTranslation()
  const id = useId()
  const languageNames = {
    en: t('language.en'),
    fr: t('language.fr'),
  }

  return (
    <div className="language-switcher">
      <label htmlFor={id}>{t('language.label')}</label>
      <select
        id={id}
        value={i18n.resolvedLanguage}
        onChange={(e) => void i18n.changeLanguage(e.target.value)}
      >
        {SUPPORTED_LANGUAGES.map((lng) => (
          <option key={lng} value={lng} lang={lng}>
            {languageNames[lng]}
          </option>
        ))}
      </select>
    </div>
  )
}
