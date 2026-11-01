import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { I18nextProvider } from 'react-i18next'
import { App } from './App'
import { hosted } from './api/server'
import { applyTheme, storedTheme } from './browse/theme'
import { HostedGate } from './hosted/HostedGate'
import { createI18n } from './i18n'
import './design/fonts.css'
import './design/tokens.css'
import './design/components.css'
import './design/base.css'
import './design/motion.css'
import './styles.css'

applyTheme(storedTheme())
const i18n = await createI18n()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <I18nextProvider i18n={i18n}>
      {hosted ? <HostedGate>{(basename) => <App basename={basename} />}</HostedGate> : <App />}
    </I18nextProvider>
  </StrictMode>,
)
