import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { Shell } from '../browse/Shell'
import { adminStep } from '../onboarding/step'

/** The signed-in home until the browsing screens (F008) exist. */
export function Home() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me : null
  const server = state.status === 'ready' ? state.server : undefined
  const isAdmin = me?.user.role === 'admin'

  // An admin who hasn't finished the onboarding goes back to it (F007 R1, R10).
  useEffect(() => {
    if (!isAdmin) return
    let cancelled = false
    void api.GET('/libraries').then(({ data }) => {
      if (cancelled || !data) return
      const step = adminStep(data, true)
      if (step !== 'done') void navigate(`/setup/${step}`, { replace: true })
    })
    return () => {
      cancelled = true
    }
  }, [isAdmin, navigate])

  return (
    <Shell>
      <div className="welcome">
        <h1>{t('welcome.title')}</h1>
        {server && (
          <p className="welcome__version">{t('welcome.version', { version: server.version })}</p>
        )}
      </div>
    </Shell>
  )
}
