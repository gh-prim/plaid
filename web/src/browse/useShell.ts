import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'

type ActivePlayback = components['schemas']['ActivePlayback']

export type ShellLibrary = { id: string; name: string; type: 'movies' | 'shows'; total?: number }

/**
 * The profile's libraries in GET /libraries order, each with its count
 * from the facets, so a child profile sees its own numbers (F008-R1).
 */
export function useLibraries(): ShellLibrary[] | undefined {
  const [libs, setLibs] = useState<ShellLibrary[]>()
  useEffect(() => {
    let cancelled = false
    void (async () => {
      const { data } = await api.GET('/libraries')
      if (cancelled || !data) return
      const base = data.map((l) => ({ id: l.id, name: l.name, type: l.type }))
      setLibs(base)
      const totals = await Promise.all(
        base.map((l) =>
          api.GET('/libraries/{libraryId}/facets', { params: { path: { libraryId: l.id } } }),
        ),
      )
      if (!cancelled) setLibs(base.map((l, i) => ({ ...l, total: totals[i]?.data?.total })))
    })()
    return () => {
      cancelled = true
    }
  }, [])
  return libs
}

/** The admin's view of who is watching, every 30 s while the tab is visible. */
export const streamsEvery = 30000

export function useStreams(enabled: boolean): ActivePlayback[] | undefined {
  const { data } = usePolling(
    async () => (enabled ? ((await api.GET('/playback')).data?.sessions ?? []) : undefined),
    enabled ? streamsEvery : 24 * 3600 * 1000,
  )
  return data
}
