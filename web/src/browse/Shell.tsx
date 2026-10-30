import { useEffect, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { searchFieldId } from './SearchBox'
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
  const navigate = useNavigate()

  // "/" goes to the search, from anywhere but a field (F008-R25).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== '/' || e.metaKey || e.ctrlKey || e.altKey) return
      const target = e.target as HTMLElement
      if (target.closest('input, textarea, select, [contenteditable="true"]')) return
      e.preventDefault()
      const field = document.getElementById(searchFieldId)
      if (field && field.offsetParent !== null) field.focus()
      else void navigate('/search')
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [navigate])

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
