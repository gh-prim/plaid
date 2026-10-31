import { useState } from 'react'
import type { components } from '../api/schema'
import { imageSrc } from './media'

type Artwork = components['schemas']['Artwork']

/**
 * The page lit by a title's own colors (V2): its backdrop, blurred far
 * and dimmed, behind everything, fading into the base color. A new title
 * crossfades over the last one, which stays underneath meanwhile.
 */
export function PageLight({ art }: { art?: Artwork }) {
  const id = art?.imageId
  const [layers, setLayers] = useState<string[]>(id ? [id] : [])
  if (id && layers[layers.length - 1] !== id) setLayers([...layers.slice(-1), id])
  if (!id) return null
  return (
    <div className="br-light" aria-hidden="true">
      {layers.map((l) => (
        <img key={l} src={imageSrc(l, 780)} alt="" decoding="async" />
      ))}
    </div>
  )
}
