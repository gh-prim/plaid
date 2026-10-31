import { useEffect, useState, type ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { searchFieldId } from './SearchResults'
import { TabBar } from './TabBar'
import { TopBar } from './TopBar'
import { useLibraries, useStreams, type ShellLibrary } from './useShell'
import './browse.css'

/**
 * The signed-in shell of every browsing route (F008-R1, V2): the top bar
 * with its glass tabs, and the floating tab bar on phones. The player has none.
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
  const { pathname } = useLocation()
  // Going right in the tabs slides the new page in from the right (V2).
  const order = tabOrder(pathname, libraries)
  const [nav, setNav] = useState<{ path: string; order: number | null; dir?: 'left' | 'right' }>({
    path: pathname,
    order,
  })
  if (nav.path !== pathname) {
    const dir =
      order !== null && nav.order !== null && order !== nav.order
        ? order > nav.order
          ? 'right'
          : 'left'
        : undefined
    setNav({ path: pathname, order, dir })
  }
  const dir = nav.dir

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
      <div className="br-main">
        <TopBar
          libraries={libraries}
          profile={profile}
          admin={admin}
          serverName={serverName}
          streams={streams}
        />
        {/* Keyed: each page arrives with the same short rise (V2 motion). */}
        <main key={pathname} className="br-content pl-page-enter" data-dir={dir}>
          {children}
        </main>
      </div>
      <TabBar libraries={libraries} />
    </div>
  )
}

/** Where a page sits in the tab bar, for the direction of travel; null off it. */
function tabOrder(pathname: string, libraries?: ShellLibrary[]): number | null {
  if (pathname === '/') return 0
  if (pathname === '/search') return 1 + (libraries?.length ?? 0)
  const i = libraries?.findIndex((l) => pathname === `/library/${l.id}`) ?? -1
  return i >= 0 ? 1 + i : null
}
