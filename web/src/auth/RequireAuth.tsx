import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation } from 'react-router'
import { useAuth, type AuthState } from './AuthProvider'
import { profilePicked } from './profileSession'

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
export function RequireStep({
  allow,
  admin,
  children,
}: {
  allow: Step[]
  /** Admins only: members go home. */
  admin?: boolean
  children: ReactNode
}) {
  const { t } = useTranslation()
  const { state, refresh } = useAuth()
  const location = useLocation()

  if (state.status === 'loading') {
    return (
      <main className="page">
        <p role="status">{t('common.loading')}</p>
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
  // Two or more profiles: each new tab asks who is watching first (F008-R2).
  if (
    step === 'app' &&
    location.pathname !== stepPath.profiles &&
    (state.me?.profiles.length ?? 0) >= 2 &&
    !profilePicked()
  ) {
    return (
      <Navigate
        to={stepPath.profiles}
        replace
        state={{ from: location.pathname + location.search }}
      />
    )
  }
  if (admin && state.me?.user.role !== 'admin') return <Navigate to="/" replace />
  return <>{children}</>
}
