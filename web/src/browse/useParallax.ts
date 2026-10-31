import { useEffect, type RefObject } from 'react'

/**
 * Hero parallax (V2 motion): as the page scrolls, the hero's picture moves
 * at 0.6× the content and fades to 40% by the time the hero has left. It
 * writes --parallax-y and --parallax-o on the hero; nothing with reduced
 * motion.
 */
export function useParallax(ref: RefObject<HTMLElement | null>) {
  useEffect(() => {
    const el = ref.current
    if (!el || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
    let frame = 0
    const update = () => {
      frame = 0
      const y = Math.max(0, window.scrollY)
      const gone = Math.min(1, y / Math.max(1, el.offsetTop + el.offsetHeight))
      el.style.setProperty('--parallax-y', `${Math.round(y * 0.4)}px`)
      el.style.setProperty('--parallax-o', String(1 - gone * 0.6))
    }
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(update)
    }
    update()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      window.removeEventListener('scroll', onScroll)
      cancelAnimationFrame(frame)
    }
  }, [ref])
}
