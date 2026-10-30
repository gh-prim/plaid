import { useEffect, useState, type RefObject } from 'react'

export interface MediaState {
  timeMs: number
  durationMs: number
  bufferedMs: number
  paused: boolean
  ended: boolean
  waiting: boolean
  volume: number
  muted: boolean
}

const initial: MediaState = {
  timeMs: 0,
  durationMs: 0,
  bufferedMs: 0,
  paused: true,
  ended: false,
  waiting: false,
  volume: 1,
  muted: false,
}

const events = [
  'timeupdate',
  'durationchange',
  'loadedmetadata',
  'progress',
  'play',
  'playing',
  'pause',
  'ended',
  'waiting',
  'seeking',
  'seeked',
  'volumechange',
  'emptied',
]

/** The <video>'s state, read again on each of its events. */
export function useMedia(ref: RefObject<HTMLVideoElement | null>): MediaState {
  const [state, setState] = useState(initial)
  useEffect(() => {
    const v = ref.current
    if (!v) return
    let waiting = false
    const read = (e?: Event) => {
      if (e?.type === 'waiting') waiting = true
      else if (e && ['playing', 'pause', 'seeked', 'emptied', 'ended'].includes(e.type))
        waiting = false
      const b = v.buffered
      const duration = Number.isFinite(v.duration) ? v.duration : 0
      setState({
        timeMs: v.currentTime * 1000,
        durationMs: duration * 1000,
        bufferedMs: b && b.length ? b.end(b.length - 1) * 1000 : 0,
        paused: v.paused,
        ended: v.ended,
        waiting,
        volume: v.volume,
        muted: v.muted,
      })
    }
    events.forEach((e) => v.addEventListener(e, read))
    return () => events.forEach((e) => v.removeEventListener(e, read))
  }, [ref])
  return state
}

/** The lines of the cues showing on a text track, rendered by the player. */
export function useCues(ref: RefObject<HTMLVideoElement | null>, index: number | null): string[] {
  const [lines, setLines] = useState<string[]>([])
  useEffect(() => {
    const tracks = ref.current?.textTracks as TextTrackList | undefined
    if (!tracks || index === null) return
    let track: TextTrack | null = null
    const read = () => {
      const cues = track?.activeCues
      const out: string[] = []
      for (let i = 0; cues && i < cues.length; i++) {
        const text = (cues[i] as VTTCue).text ?? ''
        out.push(...text.replace(/<[^>]+>/g, '').split('\n'))
      }
      setLines(out)
    }
    const find = () => {
      let next: TextTrack | null = null
      for (let i = 0; i < tracks.length; i++)
        if (tracks[i]!.id === `sub-${index}`) next = tracks[i]!
      if (next === track) return
      track?.removeEventListener('cuechange', read)
      track = next
      track?.addEventListener('cuechange', read)
      read()
    }
    find()
    tracks.addEventListener?.('addtrack', find)
    return () => {
      tracks.removeEventListener?.('addtrack', find)
      track?.removeEventListener('cuechange', read)
      setLines([])
    }
  }, [ref, index])
  return lines
}
