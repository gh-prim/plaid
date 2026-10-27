import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate } from 'react-router'
import { useAuth, type AuthState } from './AuthProvider'

/** The steps a visitor goes through before reaching the app, in order. */
export type Step = 'setup' | 'login' | 'change-password' | 'profiles' | 'app'

export const stepPath: Record<Step, string> = {
  setup: '/setup',
  login: '/login',
  'change-password': '/change-password',
  profiles: '/profiles',
  app: '/',
}

/** Where a visitor must be: setup → sign-in → password change → profile → app. */
export function currentStep(state: Extract<AuthState, { status: 'ready' }>): Step {
  if (!state.server.setupComplete) return 'setup'
  if (!state.me) return 'login'
  if (state.me.user.mustChangePassword) return 'change-password'
  if (!state.me.activeProfile) return 'profiles'
  return 'app'
}

/**
 * Renders children only when the visitor's current step is one of `allow`,
 * else redirects to the page of their step. The profile picker is also
 * reachable from the app, to switch profile.
 */
export function RequireStep({ allow, children }: { allow: Step[]; children: ReactNode }) {
  const { t } = useTranslation()
  const { state, refresh } = useAuth()

  if (state.status === 'loading') {
    return (
      <main className="page">
        <p role="status">{t('welcome.status.loading')}</p>
      </main>
    )
  }
  if (state.status === 'error') {
    return (
      <main className="page">
        <p role="alert">{t('welcome.status.error')}</p>
        <button type="button" onClick={() => void refresh()}>
          {t('common.retry')}
        </button>
      </main>
    )
  }
  const step = currentStep(state)
  if (!allow.includes(step)) return <Navigate to={stepPath[step]} replace />
  return <>{children}</>
}
