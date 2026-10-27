import { render } from '@testing-library/react'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter } from 'react-router'
import { vi } from 'vitest'
import { AppRoutes } from '../App'
import { createI18n } from '../i18n'

/** Renders the whole app at `path`, in `lang`. */
export async function renderApp(path: string, lang = 'en-US') {
  vi.spyOn(navigator, 'languages', 'get').mockReturnValue([lang])
  const i18n = await createI18n()
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[path]}>
        <AppRoutes />
      </MemoryRouter>
    </I18nextProvider>,
  )
}
