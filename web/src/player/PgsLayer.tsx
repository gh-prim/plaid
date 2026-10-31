import { useEffect, useRef, type RefObject } from 'react'
import { decodeObject, parsePgs, setAt, type PgsDisplaySet } from './pgs'
import { useQuickSubtitle } from './quickSubtitles'

const readPgs = async (r: Response) => parsePgs(await r.arrayBuffer())

/**
 * A PGS track drawn over the video (V2): fetched from its own stream,
 * then the set showing at the current time is drawn on a canvas the size
 * of the picture (object-fit: contain), raised a little while the controls
 * show. `onLoading` says when the track is still on its way.
 */
export function PgsLayer({
  url,
  video,
  raised,
  onLoading,
}: {
  url: string
  video: RefObject<HTMLVideoElement | null>
  raised: boolean
  onLoading: (loading: boolean) => void
}) {
  const canvas = useRef<HTMLCanvasElement>(null)
  // The part around the position first, then the whole track (V2).
  const { data: sets, loading } = useQuickSubtitle(url, video, readPgs)
  useEffect(() => onLoading(loading), [loading, onLoading])

  useEffect(() => {
    const v = video.current
    const c = canvas.current
    if (!v || !c || !sets) return
    const ctx = c.getContext('2d')
    if (!ctx) return
    let shown: PgsDisplaySet | undefined
    let size = ''
    let frame = 0
    const draw = () => {
      frame = requestAnimationFrame(draw)
      // The picture's box inside the element (object-fit: contain).
      const ew = v.clientWidth
      const eh = v.clientHeight
      const vw = v.videoWidth || 16
      const vh = v.videoHeight || 9
      const scale = Math.min(ew / vw, eh / vh)
      const pw = Math.round(vw * scale)
      const ph = Math.round(vh * scale)
      const ratio = window.devicePixelRatio || 1
      const key = `${pw}x${ph}@${ratio}`
      const set = setAt(sets, v.currentTime * 1000)
      if (set === shown && key === size) return
      shown = set
      size = key
      c.style.width = `${pw}px`
      c.style.height = `${ph}px`
      c.style.left = `${Math.round((ew - pw) / 2)}px`
      c.style.top = `${Math.round((eh - ph) / 2)}px`
      c.width = Math.round(pw * ratio)
      c.height = Math.round(ph * ratio)
      ctx.clearRect(0, 0, c.width, c.height)
      if (!set || set.placements.length === 0) return
      const sx = c.width / (set.width || vw)
      const sy = c.height / (set.height || vh)
      for (const p of set.placements) {
        const px = decodeObject(p)
        const img = new ImageData(px.data, px.width, px.height)
        const tmp = document.createElement('canvas')
        tmp.width = img.width
        tmp.height = img.height
        tmp.getContext('2d')?.putImageData(img, 0, 0)
        ctx.imageSmoothingQuality = 'high'
        ctx.drawImage(tmp, p.x * sx, p.y * sy, p.width * sx, p.height * sy)
      }
    }
    frame = requestAnimationFrame(draw)
    return () => cancelAnimationFrame(frame)
  }, [sets, video])

  return (
    <canvas ref={canvas} className={`pv-pgs${raised ? ' is-raised' : ''}`} aria-hidden="true" />
  )
}
