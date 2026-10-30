import type { components } from '../api/schema'
import { imageSrc } from './media'

type ItemSummary = components['schemas']['ItemSummary']

/**
 * The top of the home (F008-R6): the ambient item's backdrop, blurred and
 * dimmed, fading into the base color behind the first shelves. No text,
 * no button.
 */
export function AmbientTop({ item }: { item?: ItemSummary }) {
  const art = item?.backdrop
  if (!art) return null
  return (
    <div className="br-ambient" aria-hidden="true">
      <img src={imageSrc(art.imageId, 1280)} alt="" decoding="async" />
      <div className="br-ambient__fade" />
    </div>
  )
}
