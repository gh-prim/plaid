import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../auth/AuthProvider'
import { cx } from '../ui'
import './onboarding.css'

/** The server's name and version, as the onboarding shows them. */
export function useServer() {
  const { state } = useAuth()
  const server = state.status === 'ready' ? state.server : undefined
  return { serverName: server?.name ?? 'PLAID', version: server?.version ?? '' }
}

/** One onboarding screen: the "mediabox · PLAID 0.9.2" line, then the step. */
export function Page({ center, children }: { center?: boolean; children: ReactNode }) {
  const { t } = useTranslation()
  const { serverName, version } = useServer()
  return (
    <div className="pl ob">
      <p className="ob__meta">{t('onboarding.start.version', { version, serverName })}</p>
      <main className={cx('ob__main', center && 'ob__main--center')}>{children}</main>
    </div>
  )
}

/** "Step 2 of 3", the title and the line under it. */
export function StepHead({
  step,
  title,
  body,
}: {
  step?: number
  title: string
  body?: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <div className="ob__head">
      {step && <p className="ob__step">{t('onboarding.step', { step, total: 3 })}</p>}
      <h1 className="ob__title">{title}</h1>
      {body && <p className="ob__body">{body}</p>}
    </div>
  )
}
