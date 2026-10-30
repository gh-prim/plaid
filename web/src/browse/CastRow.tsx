import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { imageSrc } from './media'

type Credit = components['schemas']['Credit']

/** The cast (F008-R19): up to 20, round portraits or initials, each to their page. */
export function CastRow({ credits }: { credits: Credit[] }) {
  const { t } = useTranslation()
  const cast = credits.filter((c) => c.role === 'cast').slice(0, 20)
  if (cast.length === 0) return null
  return (
    <section className="br-row" aria-labelledby="cast">
      <div className="br-row__head">
        <h2 id="cast" className="br-row__title">
          {t('item.cast')}
        </h2>
      </div>
      <ul className="br-row__list br-cast">
        {cast.map((c) => (
          <li key={`${c.personId}-${c.character ?? ''}`}>
            <Link to={`/person/${c.personId}`} className="br-person pl-focusable">
              {c.portrait ? (
                <img
                  className="br-person__img"
                  src={imageSrc(c.portrait.imageId, 185)}
                  alt=""
                  loading="lazy"
                />
              ) : (
                <span className="br-person__img br-person__initials" aria-hidden="true">
                  {initials(c.name)}
                </span>
              )}
              <strong>{c.name}</strong>
              {c.character && <span>{c.character}</span>}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

export function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w.charAt(0).toLocaleUpperCase())
    .join('')
}
