import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { Icon } from '../ui'
import { imageSrc } from './media'

type Trailer = components['schemas']['Trailer']
type Artwork = components['schemas']['Artwork']

/**
 * The trailer (F008-R20, canvas TrailerSlot): nothing comes from YouTube
 * until "Play trailer" is pressed; then the privacy-enhanced embed plays
 * in place.
 */
export function TrailerSlot({ trailer, backdrop }: { trailer: Trailer; backdrop?: Artwork }) {
  const { t } = useTranslation()
  const [playing, setPlaying] = useState(false)
  const key = encodeURIComponent(trailer.key)
  return (
    <section className="br-row br-trailer" aria-labelledby="trailer">
      <div className="br-row__head">
        <h2 id="trailer" className="br-row__title">
          {t('item.trailer.title')}
        </h2>
      </div>
      {playing ? (
        <>
          <iframe
            className="br-trailer__frame"
            src={`https://www.youtube-nocookie.com/embed/${key}?autoplay=1`}
            title={trailer.name ?? t('item.trailer.title')}
            allow="autoplay; encrypted-media; picture-in-picture; fullscreen"
            allowFullScreen
          />
          <a
            className="br-link"
            href={`https://www.youtube.com/watch?v=${key}`}
            rel="noreferrer"
            target="_blank"
          >
            {t('item.trailer.openYouTube')}
          </a>
        </>
      ) : (
        <>
          <button
            type="button"
            className="br-trailer__poster pl-focusable"
            onClick={() => setPlaying(true)}
          >
            {backdrop && <img src={imageSrc(backdrop.imageId, 780)} alt="" loading="lazy" />}
            <span className="br-trailer__play">
              <Icon name="play" size={20} />
              {t('item.trailer.play')}
            </span>
          </button>
          <p className="br-trailer__notice">{t('item.trailer.notice')}</p>
        </>
      )}
    </section>
  )
}
