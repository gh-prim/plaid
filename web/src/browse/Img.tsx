import { useState, type ImgHTMLAttributes, type ReactNode } from 'react'
import { cx } from '../ui'

/**
 * An artwork image that comes into focus once loaded (from a 12 px blur,
 * V2 motion), or at once when the browser already had it, over the shimmer its frame
 * shows meanwhile (`.br-img` in browse.css), and gives way to `fallback`
 * when the server has no file for it yet (artwork still downloading).
 */
export function Img({
  fallback,
  className,
  ...props
}: ImgHTMLAttributes<HTMLImageElement> & { fallback?: ReactNode }) {
  // cached: already there when it mounted, shown at once without the fade.
  const [state, setState] = useState<'loading' | 'loaded' | 'cached' | 'failed'>('loading')
  if (state === 'failed') return <>{fallback ?? null}</>
  return (
    <img
      {...props}
      // A cached image can finish before React listens: check on mount.
      ref={(el) => {
        if (el?.complete && el.naturalWidth > 0 && state === 'loading') setState('cached')
      }}
      className={cx(
        'br-img',
        state !== 'loading' && 'is-loaded',
        state === 'cached' && 'is-cached',
        className,
      )}
      onLoad={() => setState('loaded')}
      onError={() => setState('failed')}
    />
  )
}
