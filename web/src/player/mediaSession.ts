import { imageSrc } from '../browse/media'
import type { components } from '../api/schema'

type Item = components['schemas']['ItemDetail']

/**
 * The OS media controls (F008-R46): title, show and episode, artwork,
 * play/pause/seek. Returns the cleanup.
 */
export function setMediaSession(
  item: Item,
  episodeLine: string | undefined,
  video: HTMLVideoElement,
): () => void {
  const ms = navigator.mediaSession
  if (!ms || typeof MediaMetadata === 'undefined') return () => {}
  const art = item.kind === 'episode' ? (item.still ?? item.poster) : item.poster
  ms.metadata = new MediaMetadata({
    title: item.displayTitle,
    artist: item.show?.displayTitle ?? '',
    album: episodeLine ?? '',
    artwork: art ? [{ src: imageSrc(art.imageId, 500), sizes: '500x750' }] : [],
  })
  const handlers: [MediaSessionAction, MediaSessionActionHandler][] = [
    ['play', () => void video.play()],
    ['pause', () => video.pause()],
    ['seekbackward', (d) => (video.currentTime -= d.seekOffset ?? 10)],
    ['seekforward', (d) => (video.currentTime += d.seekOffset ?? 10)],
    ['seekto', (d) => d.seekTime !== undefined && (video.currentTime = d.seekTime)],
  ]
  for (const [action, h] of handlers) {
    try {
      ms.setActionHandler(action, h)
    } catch {
      // Not every browser knows every action.
    }
  }
  return () => {
    ms.metadata = null
    for (const [action] of handlers) {
      try {
        ms.setActionHandler(action, null)
      } catch {
        // Same.
      }
    }
  }
}
