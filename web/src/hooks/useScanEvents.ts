import { useEffect, useState } from 'react'
import type { components } from '../api/schema'

export type ScanProgress = components['schemas']['ScanProgress']
export type MetadataCounts = components['schemas']['MetadataCounts']
export type MatchedItem = components['schemas']['MatchedItem']
type ScanEvent = components['schemas']['ScanEvent']

/** How many matched items the hook keeps: the poster wall shows at most 48 (F007 R12). */
export const keptMatches = 48

export type ScanState = {
  progress?: ScanProgress
  metadata?: MetadataCounts
  /** Newest first, at most keptMatches. */
  matched: MatchedItem[]
  failed?: string
  connected: boolean
}

const initial: ScanState = { matched: [], connected: false }

/**
 * Follows a library's scan through its server-sent events (F004-R21,
 * F005 R35, F007 R27). The stream is a GET on the same origin, so the
 * session cookie authenticates it and CSRF doesn't apply. EventSource
 * reconnects on its own after a drop.
 */
export function useScanEvents(libraryId: string | undefined): ScanState {
  const [state, setState] = useState<ScanState>(initial)
  useEffect(() => {
    if (!libraryId) return
    const source = new EventSource(`/api/v1/libraries/${encodeURIComponent(libraryId)}/scan/events`)
    const on = (
      type: ScanEvent['type'],
      apply: (ev: ScanEvent) => Partial<ScanState> | ((s: ScanState) => ScanState),
    ) =>
      source.addEventListener(type, (e) => {
        const ev = JSON.parse((e as MessageEvent<string>).data) as ScanEvent
        const change = apply(ev)
        setState((s) => (typeof change === 'function' ? change(s) : { ...s, ...change }))
      })
    on('progress', (ev) => ({ progress: ev.progress, failed: undefined }))
    on('completed', (ev) => ({ progress: ev.progress }))
    on('failed', (ev) => ({ progress: ev.progress, failed: ev.error ?? 'failed' }))
    on('metadata', (ev) => ({ metadata: ev.metadata }))
    on('matched', (ev) => (s: ScanState) => {
      const item = ev.item
      if (!item || s.matched.some((m) => m.id === item.id)) return s
      return { ...s, matched: [item, ...s.matched].slice(0, keptMatches) }
    })
    source.onopen = () => setState((s) => ({ ...s, connected: true }))
    source.onerror = () => setState((s) => ({ ...s, connected: false }))
    return () => {
      source.close()
      setState(initial)
    }
  }, [libraryId])
  return state
}
