import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { findServer, setServerOrigin, shortLinkId } from '../api/server'
import { Card, Page } from '../onboarding/Page'
import { Button, Spinner } from '../ui'

type State = { k: 'finding' } | { k: 'ready' } | { k: 'unknown' } | { k: 'unreachable' }

/**
 * Hosted mode (F012): before the app starts, find the server behind the
 * short link, on the LAN at home or through its public address elsewhere.
 */
export function HostedGate({ children }: { children: (basename: string) => ReactNode }) {
  const { t } = useTranslation()
  const id = shortLinkId()
  const [state, setState] = useState<State>(id ? { k: 'finding' } : { k: 'unknown' })

  // Each retry bumps the attempt, which runs the search again.
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    if (!id) return
    let stale = false
    findServer(id)
      .catch(() => undefined)
      .then((origin) => {
        if (stale) return
        if (origin) setServerOrigin(origin)
        setState(origin ? { k: 'ready' } : { k: 'unreachable' })
      })
    return () => {
      stale = true
    }
  }, [id, attempt])

  if (state.k === 'ready' && id) return <>{children(`/${id}`)}</>
  return (
    <Page className="ob--signin">
      {state.k === 'finding' && (
        <Card id="hosted-finding" icon title={t('hosted.finding')}>
          <Spinner />
        </Card>
      )}
      {state.k === 'unknown' && (
        <Card
          id="hosted-unknown"
          icon
          title={t('hosted.unknown.title')}
          body={t('hosted.unknown.body')}
        />
      )}
      {state.k === 'unreachable' && (
        <Card
          id="hosted-unreachable"
          icon
          title={t('hosted.unreachable.title')}
          body={t('hosted.unreachable.body')}
        >
          <Button
            variant="primary"
            block
            onClick={() => {
              setState({ k: 'finding' })
              setAttempt((n) => n + 1)
            }}
          >
            {t('hosted.retry')}
          </Button>
        </Card>
      )}
    </Page>
  )
}
