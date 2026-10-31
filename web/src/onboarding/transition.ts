import { useCallback, useLayoutEffect } from 'react'
import { useNavigate } from 'react-router'

/**
 * Onboarding steps slide (V2 motion, board TM25): the card's content
 * leaves 40 px to the left and fades, the next arrives from the right, the
 * card animates its height, the active dot stretches into a bar; Back
 * reverses. Done with a view transition (the CSS is in motion.css under
 * "Onboarding steps"); browsers without one simply switch.
 */
export type Direction = 'forward' | 'back'

/** Resolves the running transition once the next step has rendered. */
let arrived: (() => void) | undefined

/** The longest a transition waits for the next step before it plays anyway. */
const waitAtMost = 300

type ViewTransitionDocument = Document & {
  startViewTransition?: (update: () => Promise<void>) => unknown
}

/** Runs `update` (a navigation or a phase change) inside a slide. */
export function slide(update: () => void, direction: Direction = 'forward') {
  const doc = document as ViewTransitionDocument
  if (typeof doc.startViewTransition !== 'function') {
    update()
    return
  }
  doc.documentElement.dataset.obDir = direction
  doc.startViewTransition(
    () =>
      new Promise<void>((resolve) => {
        arrived = resolve
        update()
        setTimeout(resolve, waitAtMost)
      }),
  )
}

/** Called by the card each time new content is on screen: the slide may play. */
export function useArrived(key: string) {
  useLayoutEffect(() => {
    arrived?.()
    arrived = undefined
  }, [key])
}

/** Navigates to another onboarding step with the slide. */
export function useStepNav() {
  const navigate = useNavigate()
  return useCallback(
    (to: string, direction: Direction = 'forward') => slide(() => void navigate(to), direction),
    [navigate],
  )
}
