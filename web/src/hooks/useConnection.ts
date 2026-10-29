import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'

/** Retry ladder after the server stops answering: 2 s, 4 s, 8 s, then every 15 s (F007 R16). */
export const retryDelays = [2000, 4000, 8000, 15000] as const

export type Connection =
  { status: 'online' } | { status: 'offline'; retryInSeconds: number } | { status: 'back' }

async function healthy(): Promise<boolean> {
  try {
    return (await api.GET('/health')).response.ok
  } catch {
    return false
  }
}

/**
 * Watches the server while a step is on screen. A failed request calls
 * `lost()`; the hook then checks GET /health on the ladder until it
 * answers, says "back" for a moment, and returns to online. `retryNow`
 * checks at once.
 */
export function useConnection(backForMs = 3000) {
  const [state, setState] = useState<Connection>({ status: 'online' })
  const offline = useRef(false)
  const attempt = useRef(0)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const tick = useRef<ReturnType<typeof setInterval>>(undefined)
  // The timer calls the latest retry through this ref: a callback can't
  // schedule itself by name.
  const again = useRef<() => void>(() => {})

  const stop = () => {
    clearTimeout(timer.current)
    clearInterval(tick.current)
  }

  const retry = useCallback(async () => {
    stop()
    if (await healthy()) {
      offline.current = false
      attempt.current = 0
      setState({ status: 'back' })
      timer.current = setTimeout(() => setState({ status: 'online' }), backForMs)
      return
    }
    const delay = retryDelays[Math.min(attempt.current, retryDelays.length - 1)] ?? 15000
    attempt.current++
    const deadline = Date.now() + delay
    const show = () =>
      setState({
        status: 'offline',
        retryInSeconds: Math.max(0, Math.ceil((deadline - Date.now()) / 1000)),
      })
    show()
    tick.current = setInterval(show, 1000)
    timer.current = setTimeout(() => again.current(), delay)
  }, [backForMs])

  useEffect(() => {
    again.current = () => void retry()
  }, [retry])

  const lost = useCallback(() => {
    if (offline.current) return
    offline.current = true
    void retry()
  }, [retry])

  useEffect(() => stop, [])

  return { connection: state, lost, retryNow: retry }
}
