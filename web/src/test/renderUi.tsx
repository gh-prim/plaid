import { render } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { vi } from 'vitest'
import { createI18n } from '../i18n'

/** Renders one component with the app's real catalogs, in `lang`. */
export async function renderUi(ui: ReactElement, lang = 'en-US') {
  vi.spyOn(navigator, 'languages', 'get').mockReturnValue([lang])
  const i18n = await createI18n()
  // A wrapper, not a parent: rerender() keeps the provider.
  const wrapper = ({ children }: { children: ReactNode }) => (
    <I18nextProvider i18n={i18n}>{children}</I18nextProvider>
  )
  return render(ui, { wrapper })
}
