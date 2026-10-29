import { useEffect, useRef, useState } from 'react'
import type { MatchedItem } from '../hooks/useScanEvents'
import { PosterSlot } from '../ui'

/** The design system's pacing: one poster every 400 ms; past 24 waiting, the rest land together. */
export const arriveEvery = 400
export const flushOver = 24

export function posterSrc(imageId: string, width = 342): string {
  return `/api/v1/images/${encodeURIComponent(imageId)}?w=${width}`
}

/**
 * The wall that fills during the scan (F007 R12). `items` come newest
 * first; the wall places them left to right, paced, and keeps the latest
 * `slots` on screen so the DOM stays small on a 1,240-movie NAS.
 */
export function PosterWall({
  items,
  slots,
  label,
}: {
  items: MatchedItem[]
  slots: number
  label: string
}) {
  const [shown, setShown] = useState<{ item: MatchedItem; fresh: boolean }[]>([])
  const seen = useRef(new Set<string>())
  const queue = useRef<MatchedItem[]>([])

  useEffect(() => {
    const fresh = [...items].reverse().filter((i) => !seen.current.has(i.id))
    for (const i of fresh) seen.current.add(i.id)
    queue.current.push(...fresh)
  }, [items])

  useEffect(() => {
    const timer = setInterval(() => {
      if (!queue.current.length) return
      const batch =
        queue.current.length > flushOver ? queue.current.splice(0) : queue.current.splice(0, 1)
      setShown((s) =>
        [
          ...s.map((x) => ({ ...x, fresh: false })),
          ...batch.map((item) => ({ item, fresh: true })),
        ].slice(-slots),
      )
    }, arriveEvery)
    return () => clearInterval(timer)
  }, [slots])

  return (
    <ul className="scan__wall" aria-label={label}>
      {Array.from({ length: slots }, (_, i) => {
        const s = shown[i]
        if (!s) {
          return (
            <li key={`empty-${i}`}>
              <PosterSlot />
            </li>
          )
        }
        const src = s.item.poster ? posterSrc(s.item.poster.imageId) : undefined
        return (
          <li key={s.item.id}>
            <PosterSlot
              src={src}
              title={s.item.title}
              state={src ? (s.fresh ? 'arriving' : 'arrived') : 'missing'}
            />
          </li>
        )
      })}
    </ul>
  )
}
