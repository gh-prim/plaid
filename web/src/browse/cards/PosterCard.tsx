import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../../api/schema'
import { imageSrc, imageSrcSet, progress, unwatchedEpisodes } from '../media'
import { Thread } from './Thread'

type ItemSummary = components['schemas']['ItemSummary']

/**
 * A poster card (F008-R8): poster, title and year, a watched mark or the
 * unwatched episodes of a show, the woven progress thread, or the
 * missing-artwork plate. `sizes` is how wide it shows, for the srcset.
 */
export function PosterCard({
  item,
  sizes = '(max-width: 767px) 104px, 128px',
}: {
  item: ItemSummary
  sizes?: string
}) {
  const { t } = useTranslation()
  const p = progress(item)
  const unwatched = unwatchedEpisodes(item)
  const badge = item.userState?.watched
    ? t('card.watched')
    : unwatched > 0
      ? t('card.unwatchedBadge', { count: unwatched })
      : undefined
  return (
    <Link to={`/item/${item.id}`} className="br-card br-card--poster pl-focusable">
      <span className="br-card__art">
        {item.poster ? (
          <img
            src={imageSrc(item.poster.imageId, 342)}
            srcSet={imageSrcSet(item.poster.imageId, [185, 342, 500])}
            sizes={sizes}
            alt=""
            loading="lazy"
            decoding="async"
          />
        ) : (
          <span className="br-card__plate" aria-hidden="true">
            {item.displayTitle}
          </span>
        )}
        {badge && <span className="br-card__badge">{badge}</span>}
        {p && <Thread fraction={p.fraction} />}
      </span>
      <span className="br-card__caption">
        <strong>{item.displayTitle}</strong>
        {item.year !== undefined && <span>{item.year}</span>}
      </span>
    </Link>
  )
}
