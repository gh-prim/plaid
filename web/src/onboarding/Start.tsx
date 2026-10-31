import { useTranslation } from 'react-i18next'
import { SUPPORTED_LANGUAGES } from '../i18n'
import { nativeLanguageName } from '../lib/format'
import { Button, SegmentedControl } from '../ui'
import { Card, Page, useServer } from './Page'
import { useStepNav } from './transition'

/**
 * The start (board TO01): the app icon, the warm line, the server it
 * found, one button; the language, preselected from the browser, which
 * then sets the whole UI, the admin's locale and their libraries'
 * metadata language (F007 R3).
 */
export function Start() {
  const { t, i18n } = useTranslation()
  const go = useStepNav()
  const { serverName, version } = useServer()
  const current = i18n.resolvedLanguage ?? 'en'
  return (
    <Page>
      <Card id="start" icon title={t('onboarding.start.title', { serverName })}>
        <div className="ob__server ob-glassbtn">
          <span className="ob__server-dot" aria-hidden="true" />
          <span className="ob__server-text">
            <b>{serverName}</b>
            {version && <span>{t('onboarding.start.version', { version })}</span>}
          </span>
        </div>
        <Button block className="ob__cta" onClick={() => go('/setup/account')}>
          {t('onboarding.start.continue', { serverName })}
        </Button>
        <div className="ob__language">
          <SegmentedControl
            label={t('onboarding.start.language.label')}
            value={current}
            options={SUPPORTED_LANGUAGES.map((lng) => ({
              value: lng,
              label: nativeLanguageName(lng),
            }))}
            onChange={(lng) => void i18n.changeLanguage(lng)}
          />
          <p className="ob__foot">{t('onboarding.start.language.help')}</p>
        </div>
      </Card>
    </Page>
  )
}
