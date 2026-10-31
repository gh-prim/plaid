import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { imageSrc, imageSrcSet } from './media'
import { Img } from './Img'

type CollectionSummary = components['schemas']['CollectionSummary']

/** A library's collections (F008-R16, F005-R23), as poster cards. */
export function Collections() {
  const { t } = useTranslation()
  const { libraryId = '' } = useParams()
  const [name, setName] = useState<string>()
  const [collections, setCollections] = useState<CollectionSummary[]>()

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      api.GET('/libraries/{libraryId}', { params: { path: { libraryId } } }),
      api.GET('/collections', { params: { query: { libraryId } } }),
    ]).then(([l, c]) => {
      if (cancelled) return
      setName(l.data?.name)
      setCollections(c.data?.collections ?? [])
    })
    return () => {
      cancelled = true
    }
  }, [libraryId])

  return (
    <div className="br-grid-page">
      <header className="br-grid__head">
        <div>
          <Link to={`/library/${libraryId}`} className="br-row__all pl-focusable">
            {name}
          </Link>
          <h1 className="br-grid__title">{t('grid.collections')}</h1>
        </div>
      </header>
      <ul className="br-tiles">
        {collections?.map((c) => (
          <li key={c.id}>
            <Link to={`/collection/${c.id}`} className="br-card br-card--poster pl-focusable">
              <span className="br-card__art">
                {c.poster ? (
                  <Img
                    src={imageSrc(c.poster.imageId, 342)}
                    srcSet={imageSrcSet(c.poster.imageId, [185, 342, 500])}
                    sizes="(max-width: 767px) 104px, 160px"
                    alt=""
                    loading="lazy"
                  />
                ) : (
                  <span className="br-card__plate" aria-hidden="true">
                    {c.name}
                  </span>
                )}
              </span>
              <span className="br-card__caption">
                <strong>{c.name}</strong>
                <span>{t('grid.count.movies', { count: c.movieCount })}</span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}
