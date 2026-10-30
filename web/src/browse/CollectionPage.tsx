import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { AmbientTop } from './AmbientTop'
import { PosterCard } from './cards/PosterCard'
import { Unavailable } from './Unavailable'

type CollectionDetail = components['schemas']['CollectionDetail']

/** A collection (F008-R16): its backdrop, overview and movies by release date. */
export function CollectionPage() {
  const { t } = useTranslation()
  const { collectionId = '' } = useParams()
  const [c, setC] = useState<CollectionDetail | null>()

  useEffect(() => {
    let cancelled = false
    void api
      .GET('/collections/{collectionId}', { params: { path: { collectionId } } })
      .then(({ data }) => !cancelled && setC(data ?? null))
    return () => {
      cancelled = true
    }
  }, [collectionId])

  if (c === null) return <Unavailable />
  if (!c) return <div className="br-page" aria-busy="true" aria-label={t('common.loading')} />
  return (
    <div className="br-home">
      <AmbientTop art={c.backdrop} />
      <div className="br-grid-page br-collection">
        <h1 className="br-grid__title">{c.name}</h1>
        <p className="br-grid__count">{t('grid.count.movies', { count: c.movieCount })}</p>
        {c.overview && <p className="br-collection__overview">{c.overview}</p>}
        <ul className="br-tiles">
          {c.items.map((it) => (
            <li key={it.id}>
              <PosterCard item={it} sizes="(max-width: 767px) 104px, 160px" />
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
