import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Me, type ServerInfo } from '../api/client'
import { detectDevice } from './device'

export type AuthState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; server: ServerInfo; me: Me | null }

type AuthContextValue = {
  state: AuthState
  /** Reloads the server state and the signed-in account. */
  refresh: () => Promise<void>
  signOut: () => Promise<void>
  /** The device sent with sign-in and setup ("Firefox on macOS"). */
  device: { name: string; type: 'web' }
}

const AuthContext = createContext<AuthContextValue | null>(null)

async function load(): Promise<AuthState> {
  const server = await api.GET('/server')
  if (!server.data) return { status: 'error' }
  if (!server.data.setupComplete) return { status: 'ready', server: server.data, me: null }
  const me = await api.GET('/me')
  if (me.response.status === 401) return { status: 'ready', server: server.data, me: null }
  if (!me.data) return { status: 'error' }
  return { status: 'ready', server: server.data, me: me.data }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const { t, i18n } = useTranslation()
  const [state, setState] = useState<AuthState>({ status: 'loading' })

  const refresh = useCallback(async () => {
    try {
      setState(await load())
    } catch {
      setState({ status: 'error' })
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    load()
      .catch((): AuthState => ({ status: 'error' }))
      .then((s) => {
        if (!cancelled) setState(s)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const signOut = useCallback(async () => {
    await api.POST('/auth/logout').catch(() => undefined)
    await refresh()
  }, [refresh])

  // The active profile's language, when it has one, overrides the UI's
  // (F008-R3); switching to a profile without one goes back to the
  // account's.
  const overridden = useRef(false)
  const profileLocale = state.status === 'ready' ? state.me?.activeProfile?.locale : undefined
  const accountLocale = state.status === 'ready' ? state.me?.user.locale : undefined
  useEffect(() => {
    if (profileLocale) {
      overridden.current = true
      if (i18n.resolvedLanguage !== profileLocale) void i18n.changeLanguage(profileLocale)
    } else if (overridden.current) {
      overridden.current = false
      if (accountLocale) void i18n.changeLanguage(accountLocale)
    }
  }, [profileLocale, accountLocale, i18n])

  const { browser, os } = detectDevice(navigator.userAgent)
  const deviceName = os ? t('device.webName', { browser, os }) : browser
  const value = useMemo(
    () => ({ state, refresh, signOut, device: { name: deviceName, type: 'web' as const } }),
    [state, refresh, signOut, deviceName],
  )
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside <AuthProvider>')
  return ctx
}
