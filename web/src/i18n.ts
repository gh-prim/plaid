import i18next, { type i18n } from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'
import en from './locales/en/common.json'
import fr from './locales/fr/common.json'

export const SUPPORTED_LANGUAGES = ['en', 'fr'] as const
export type Language = (typeof SUPPORTED_LANGUAGES)[number]
export const LANGUAGE_STORAGE_KEY = 'plaid.language'

/**
 * Creates an initialized i18next instance. English is the source locale.
 * Language order: saved choice, then the browser, then English.
 */
export async function createI18n(): Promise<i18n> {
  const instance = i18next.createInstance()
  instance.on('languageChanged', () => {
    document.documentElement.lang = instance.resolvedLanguage ?? 'en'
  })
  await instance
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
      resources: { en: { common: en }, fr: { common: fr } },
      defaultNS: 'common',
      ns: ['common'],
      supportedLngs: SUPPORTED_LANGUAGES,
      nonExplicitSupportedLngs: true,
      load: 'languageOnly',
      fallbackLng: 'en',
      interpolation: { escapeValue: false }, // React already escapes
      detection: {
        order: ['localStorage', 'navigator'],
        lookupLocalStorage: LANGUAGE_STORAGE_KEY,
        caches: ['localStorage'],
      },
    })
  return instance
}
