import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import type { Connection } from '../hooks/useConnection'
import { Button, Icon } from '../ui'
import { Card, Page, useServer } from './Page'

/** "Already set up", "nothing to watch yet" (boards P05, P14), as V2 cards. */
export function StatusScreen({ kind }: { kind: 'done' | 'zero' }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { refresh } = useAuth()
  if (kind === 'done') {
    return (
      <Page>
        <Card
          id="done"
          icon
          title={t('onboarding.doneElsewhere.title')}
          body={t('onboarding.doneElsewhere.body')}
        >
          <Button
            block
            className="ob__cta"
            onClick={() => void refresh().then(() => navigate('/login'))}
          >
            {t('onboarding.doneElsewhere.action')}
          </Button>
        </Card>
      </Page>
    )
  }
  return (
    <Page step={4}>
      <Card id="zero" title={t('home.empty.title')} body={t('home.empty.body')}>
        <Button block className="ob__cta" onClick={() => void navigate('/setup/libraries')}>
          {t('home.empty.action')}
        </Button>
        <p className="ob__foot">{t('home.apps')}</p>
      </Card>
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
      <Page>
        <Card id="offline" title={t('offline.title', { serverName })} body={t('offline.body')}>
          <Icon name="alert" size={28} className="ob__status-icon" />
          <p className="ob__foot">
            {t('offline.retrying', { seconds: connection.retryInSeconds })}
          </p>
          <Button block className="ob__cta" onClick={onRetry}>
            {t('offline.now')}
          </Button>
        </Card>
      </Page>
    </div>
  )
}
