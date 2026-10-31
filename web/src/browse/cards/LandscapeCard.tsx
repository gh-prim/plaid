import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../../api/schema'
import { formatDuration } from '../../lib/format'
import { imageSrc, imageSrcSet, progress, watchLink } from '../media'
import { CardMenu } from './CardMenu'
import { Thread } from './Thread'
import { Img } from '../Img'
import { CardPlate } from './CardPlate'

type ItemSummary = components['schemas']['ItemSummary']

/**
 * A Continue watching card (F008-R7): the episode's still or the item's
 * backdrop at 16:9, the show and `S1 E4 · Title` for episodes, the time
 * left, the thread. It plays directly; its menu marks it watched or goes
 * to the show. `onChange` reloads the row after a change.
 */
export function LandscapeCard({
  item,
  onChange,
  sizes = '(max-width: 767px) 232px, 256px',
}: {
  item: ItemSummary
  onChange?: () => void
  sizes?: string
}) {
  const { t, i18n } = useTranslation()
  const p = progress(item)
  const art = item.still ?? item.backdrop
  const episode = item.kind === 'episode'
  const title = episode && item.show ? item.show.displayTitle : item.displayTitle
  const parts: string[] = []
  if (episode && item.seasonNumber !== undefined && item.episodeNumber !== undefined) {
    parts.push(
      item.displayTitle
        ? t('card.episode', {
            season: item.seasonNumber,
            episode: item.episodeNumber,
            title: item.displayTitle,
          })
        : t('card.episodeShort', { season: item.seasonNumber, episode: item.episodeNumber }),
    )
  }
  if (p?.leftMs !== undefined) {
    parts.push(t('card.timeLeft', { duration: formatDuration(i18n.language, p.leftMs) }))
  }
  const plate = <CardPlate title={title} pending={item.matchStatus === 'pending'} />
  return (
    <div className="br-card br-card--landscape">
      <Link to={watchLink(item)} className="br-card__link pl-focusable">
        <span className="br-card__art">
          {art ? (
            <Img
              src={imageSrc(art.imageId, 500)}
              srcSet={imageSrcSet(art.imageId, [342, 500, 780])}
              sizes={sizes}
              alt=""
              loading="lazy"
              decoding="async"
              fallback={plate}
            />
          ) : (
            plate
          )}
          {p && <Thread fraction={p.fraction} />}
        </span>
        <span className="br-card__caption">
          <strong>{title}</strong>
          {parts.length > 0 && <span>{parts.join(t('card.separator'))}</span>}
        </span>
      </Link>
      <CardMenu item={item} title={title} onChange={onChange} />
    </div>
  )
}
