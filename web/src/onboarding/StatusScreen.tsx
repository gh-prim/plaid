import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import type { Connection } from '../hooks/useConnection'
import { Button, PosterSlot } from '../ui'
import { Page, useServer } from './Page'

/** "Already set up", "nothing to watch yet" (boards P05, P14). */
export function StatusScreen({ kind }: { kind: 'done' | 'zero' }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { refresh } = useAuth()
  if (kind === 'done') {
    return (
      <Page center>
        <div className="ob__head">
          <h1 className="ob__title">{t('onboarding.doneElsewhere.title')}</h1>
          <p className="ob__body">{t('onboarding.doneElsewhere.body')}</p>
        </div>
        <div className="ob__actions">
          <Button onClick={() => void refresh().then(() => navigate('/login'))}>
            {t('onboarding.doneElsewhere.action')}
          </Button>
        </div>
      </Page>
    )
  }
  return (
    <Page>
      <div className="ob__head">
        <h1 className="ob__title">{t('home.empty.title')}</h1>
        <p className="ob__body">{t('home.empty.body')}</p>
      </div>
      <div className="ob__actions">
        <Button onClick={() => void navigate('/setup/libraries')}>{t('home.empty.action')}</Button>
      </div>
      <ul className="scan__wall" aria-hidden="true" style={{ marginTop: 16 }}>
        {Array.from({ length: 8 }, (_, i) => (
          <li key={i}>
            <PosterSlot />
          </li>
        ))}
      </ul>
      <p className="ob__help">{t('home.apps')}</p>
    </Page>
  )
}

/**
 * Server unreachable (board P15), over whatever step is on screen, which
 * keeps its state underneath (F007 R16).
 */
export function OfflineScreen({
  connection,
  onRetry,
}: {
  connection: Connection
  onRetry: () => void
}) {
  const { t } = useTranslation()
  const { serverName } = useServer()
  if (connection.status !== 'offline') return null
  return (
    <div className="ob__offline" role="alert">
      <Page center>
        <div className="ob__head">
          <svg
            className="ob__status-icon"
            width="28"
            height="28"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.75"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M3 9a14 14 0 0 1 18 0M6.5 12.5a9 9 0 0 1 11 0M10 16a4 4 0 0 1 4 0M12 19.5h.01M4 4l16 16" />
          </svg>
          <h1 className="ob__title">{t('offline.title', { serverName })}</h1>
          <p className="ob__body">{t('offline.body')}</p>
          <p className="ob__help">
            {t('offline.retrying', { seconds: connection.retryInSeconds })}
          </p>
        </div>
        <div className="ob__actions">
          <Button onClick={onRetry}>{t('offline.now')}</Button>
        </div>
      </Page>
    </div>
  )
}
