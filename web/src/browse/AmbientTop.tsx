import type { components } from '../api/schema'
import { imageSrc } from './media'

type Artwork = components['schemas']['Artwork']

/**
 * The top of the home (F008-R6), and of a collection: a backdrop, blurred
 * and dimmed, fading into the base color behind what follows. No text, no
 * button.
 */
export function AmbientTop({ art }: { art?: Artwork }) {
  if (!art) return null
  return (
    <div className="br-ambient" aria-hidden="true">
      <img src={imageSrc(art.imageId, 1280)} alt="" decoding="async" />
      <div className="br-ambient__fade" />
    </div>
  )
}
