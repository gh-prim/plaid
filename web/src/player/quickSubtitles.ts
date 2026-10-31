import { useEffect, useState, type RefObject } from 'react'

/** How often a partial track is asked again, until the whole one is there. */
const retryMs = 15_000

/**
 * Fetches a subtitle track the quick way (V2, like Plex): first the part
 * around the playback position, which the server extracts in seconds, then
 * the whole track once it's ready. `X-Subtitle-Until` marks a partial
 * answer; the next ask starts from where the picture is then.
 */
export function useQuickSubtitle<T>(
  url: string | undefined,
  video: RefObject<HTMLVideoElement | null>,
  read: (r: Response) => Promise<T>,
): { data?: T; loading: boolean } {
  const [state, setState] = useState<{ url?: string; data?: T }>({})

  useEffect(() => {
    if (!url) return
    const ctrl = new AbortController()
    let timer: ReturnType<typeof setTimeout> | undefined
    const ask = () => {
      const from = Math.max(0, Math.round((video.current?.currentTime ?? 0) * 1000))
      const sep = url.includes('?') ? '&' : '?'
      fetch(`${url}${sep}from=${from}`, { signal: ctrl.signal })
        .then(async (r) => {
          if (!r.ok) throw new Error(String(r.status))
          const partial = r.headers.get('X-Subtitle-Until') !== null
          const data = await read(r)
          if (ctrl.signal.aborted) return
          setState({ url, data })
          if (partial) timer = setTimeout(ask, retryMs)
        })
        .catch(() => {
          if (!ctrl.signal.aborted) timer = setTimeout(ask, retryMs)
        })
    }
    ask()
    return () => {
      ctrl.abort()
      clearTimeout(timer)
    }
    // read is a stable module function.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url, video])

  return state.url === url ? { data: state.data, loading: false } : { loading: !!url }
}

/** A WebVTT answer as an object URL a <track> can load. */
export async function vttObjectUrl(r: Response): Promise<string> {
  return URL.createObjectURL(await r.blob())
}
