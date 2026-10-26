import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { LanguageSwitcher } from '../components/LanguageSwitcher'

type ServerState = { kind: 'loading' } | { kind: 'ok'; version: string } | { kind: 'error' }

export function Welcome() {
  const { t } = useTranslation()
  const [server, setServer] = useState<ServerState>({ kind: 'loading' })

  useEffect(() => {
    let cancelled = false
    api
      .GET('/health')
      .then(({ data }) => {
        if (cancelled) return
        setServer(data?.status === 'ok' ? { kind: 'ok', version: data.version } : { kind: 'error' })
      })
      .catch(() => {
        if (!cancelled) setServer({ kind: 'error' })
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <main className="welcome">
      <h1>{t('welcome.title')}</h1>
      {server.kind === 'loading' && <p role="status">{t('welcome.status.loading')}</p>}
      {server.kind === 'ok' && (
        <>
          <p role="status">{t('welcome.status.ok')}</p>
          <p className="welcome__version">{t('welcome.version', { version: server.version })}</p>
        </>
      )}
      {server.kind === 'error' && <p role="alert">{t('welcome.status.error')}</p>}
      <LanguageSwitcher />
    </main>
  )
}
