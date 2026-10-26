import { describe, expect, it, vi } from 'vitest'
import { createI18n, LANGUAGE_STORAGE_KEY } from './i18n'

function browserLanguages(langs: string[]) {
  vi.spyOn(navigator, 'languages', 'get').mockReturnValue(langs)
  vi.spyOn(navigator, 'language', 'get').mockReturnValue(langs[0] ?? 'en')
}

describe('i18n', () => {
  it('picks French from a fr-FR browser and sets <html lang>', async () => {
    browserLanguages(['fr-FR', 'fr'])
    const i18n = await createI18n()
    expect(i18n.resolvedLanguage).toBe('fr')
    expect(i18n.t('welcome.status.ok')).toBe('Ton serveur est prêt. Attrape le plaid.')
    expect(document.documentElement.lang).toBe('fr')
  })

  it('falls back to English for an unsupported browser language', async () => {
    browserLanguages(['ja-JP'])
    const i18n = await createI18n()
    expect(i18n.resolvedLanguage).toBe('en')
  })

  it('persists a manual switch and prefers it over the browser language', async () => {
    browserLanguages(['fr-FR'])
    const first = await createI18n()
    await first.changeLanguage('en')
    expect(document.documentElement.lang).toBe('en')
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe('en')

    const afterReload = await createI18n()
    expect(afterReload.resolvedLanguage).toBe('en')
  })
})
