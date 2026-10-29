import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { SUPPORTED_LANGUAGES } from '../i18n'
import { nativeLanguageName } from '../lib/format'
import { Button, ChoiceRow } from '../ui'
import { Page, useServer } from './Page'

/**
 * Step 1: the language, preselected from the browser, which then sets the
 * whole UI, the admin's locale and their libraries' metadata language
 * (F007 R3). One of the three warm moments: the serif line.
 */
export function Start() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { serverName } = useServer()
  const current = i18n.resolvedLanguage ?? 'en'
  return (
    <Page center>
      <div className="ob__form" style={{ maxWidth: 760, gap: 40 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <h1 className="ob__voice">{t('onboarding.start.title', { serverName })}</h1>
          <p className="ob__body" style={{ maxWidth: 560 }}>
            {t('onboarding.start.body')}
          </p>
        </div>
        <fieldset className="ob__fieldset">
          <legend className="ob__legend">{t('onboarding.start.language.label')}</legend>
          <div className="ob__choices">
            {SUPPORTED_LANGUAGES.map((lng) => (
              <ChoiceRow
                key={lng}
                name="language"
                lang={lng}
                label={nativeLanguageName(lng)}
                checked={current === lng}
                onChange={() => void i18n.changeLanguage(lng)}
              />
            ))}
          </div>
          <p className="ob__help">{t('onboarding.start.language.help')}</p>
        </fieldset>
        <div className="ob__actions">
          <Button onClick={() => void navigate('/setup/account')}>
            {t('onboarding.start.continue')}
          </Button>
        </div>
      </div>
    </Page>
  )
}
