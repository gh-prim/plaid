import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Calls `load` now and every `everyMs` while the page is visible, so the
 * admin screens never poll the server from a background tab (F007 NFR).
 * `refresh` runs it again at once.
 */
export function usePolling<T>(load: () => Promise<T>, everyMs: number) {
  const [data, setData] = useState<T>()
  const [error, setError] = useState<unknown>()
  const loadRef = useRef(load)
  useEffect(() => {
    loadRef.current = load
  })

  const run = useCallback(async () => {
    try {
      setData(await loadRef.current())
      setError(undefined)
    } catch (err) {
      setError(err)
    }
  }, [])

  useEffect(() => {
    let timer: ReturnType<typeof setInterval> | undefined
    const start = () => {
      if (timer) return
      void run()
      timer = setInterval(() => void run(), everyMs)
    }
    const stop = () => {
      clearInterval(timer)
      timer = undefined
    }
    const onVisibility = () => (document.hidden ? stop() : start())
    if (!document.hidden) start()
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      stop()
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [run, everyMs])

  return { data, error, refresh: run }
}
