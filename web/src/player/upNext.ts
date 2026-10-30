/** What the end of an episode shows (F008-R43, R44). */
export type EndPhase =
  | 'none' // nothing yet
  | 'card' // the next episode's card, with Play now and Cancel
  | 'countdown' // the video ended: the card counts 10 s down
  | 'stillWatching' // 3 countdowns in a row without input
  | 'endOfShow' // the last episode ended
  | 'leave' // ended with nothing to show: back to the details

export const cardBeforeEndMs = 30_000
export const countdownSeconds = 10
export const autoplayLimit = 3

export interface EndInput {
  /** The episode's up-next is known: undefined while it loads. */
  hasNext: boolean | undefined
  isEpisode: boolean
  remainingMs: number
  ended: boolean
  /** Cancel was pressed on the card. */
  cancelled: boolean
  /** The profile's "Play next episode automatically". */
  autoplay: boolean
  /** Episodes started by the countdown in a row, without input. */
  autoplayed: number
}

export function endPhase(i: EndInput): EndPhase {
  if (!i.ended) {
    return i.hasNext && !i.cancelled && i.remainingMs <= cardBeforeEndMs ? 'card' : 'none'
  }
  if (!i.hasNext) return i.isEpisode && i.hasNext === false ? 'endOfShow' : 'leave'
  if (i.cancelled) return 'leave'
  if (!i.autoplay) return 'card'
  return i.autoplayed >= autoplayLimit ? 'stillWatching' : 'countdown'
}
