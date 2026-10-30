import type { components } from '../api/schema'

type ItemSummary = components['schemas']['ItemSummary']

/** F005's artwork widths (R19); the server snaps any other to these. */
export const artWidths = [92, 185, 342, 500, 780, 1280, 1920] as const

export function imageSrc(imageId: string, width: number): string {
  return `/api/v1/images/${encodeURIComponent(imageId)}?w=${width}`
}

/** A srcset over the widths, so the browser fetches what it displays (F008 NFR grid). */
export function imageSrcSet(imageId: string, widths: readonly number[]): string {
  return widths.map((w) => `${imageSrc(imageId, w)} ${w}w`).join(', ')
}

/** Where an item's progress stands for the active profile. */
export function progress(item: ItemSummary): { fraction: number; leftMs?: number } | undefined {
  const st = item.userState
  if (!st || st.watched || st.positionMs <= 0) return undefined
  if (!st.durationMs) return { fraction: 0 }
  return {
    fraction: Math.min(1, st.positionMs / st.durationMs),
    leftMs: Math.max(0, st.durationMs - st.positionMs),
  }
}

/** Episodes of a show not watched yet, when above 0. */
export function unwatchedEpisodes(item: ItemSummary): number {
  const st = item.userState
  if (!st?.totalEpisodes) return 0
  return Math.max(0, st.totalEpisodes - (st.watchedEpisodes ?? 0))
}
