import type { ReactNode } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { Sidebar } from './Sidebar'
import { TabBar } from './TabBar'
import { TopBar } from './TopBar'
import { useLibraries, useStreams } from './useShell'
import './browse.css'

/**
 * The signed-in shell of every browsing route (F008-R1): a sidebar from
 * 768 px, a top bar, and the bottom tab bar on phones. The player has none.
 */
export function Shell({ children }: { children: ReactNode }) {
  const { state } = useAuth()
  const me = state.status === 'ready' ? state.me : null
  const serverName = state.status === 'ready' ? state.server.name : ''
  const admin = me?.user.role === 'admin'
  const libraries = useLibraries()
  // Members never ask who is watching (F008-R1).
  const streams = useStreams(admin)
  const profile = me?.activeProfile
  return (
    <div className="pl br-shell">
      <Sidebar
        libraries={libraries}
        profile={profile}
        admin={admin}
        serverName={serverName}
        streams={streams}
      />
      <div className="br-main">
        <TopBar profile={profile} admin={admin} serverName={serverName} streams={streams} />
        <main className="br-content">{children}</main>
      </div>
      <TabBar libraries={libraries} />
    </div>
  )
}
