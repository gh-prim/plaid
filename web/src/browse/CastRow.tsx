import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { imageSrc } from './media'
import { Img } from './Img'

type Credit = components['schemas']['Credit']

const crewRoles = ['creator', 'director', 'writer'] as const

/**
 * Cast and crew (F008-R19, V2): up to 20 actors then the creators,
 * directors and writers, in big circles (portraits, or initials until
 * there are some), each to their page.
 */
export function CastRow({ credits }: { credits: Credit[] }) {
  const { t } = useTranslation()
  const cast = [
    ...credits.filter((c) => c.role === 'cast').slice(0, 20),
    ...crewRoles.flatMap((r) => credits.filter((c) => c.role === r)),
  ]
  if (cast.length === 0) return null
  const role = (c: Credit) =>
    c.role === 'cast'
      ? c.character
      : c.role === 'creator'
        ? t('item.crew.creator')
        : c.role === 'director'
          ? t('item.crew.director')
          : t('item.crew.writer')
  return (
    <section className="br-row" aria-label={t('item.castAndCrew')}>
      <div className="br-row__head">
        <h2 className="br-row__title">{t('item.castAndCrew')}</h2>
      </div>
      <ul className="br-row__list br-cast">
        {cast.map((c) => (
          <li key={`${c.personId}-${c.role}-${c.character ?? ''}`}>
            <Link to={`/person/${c.personId}`} className="br-person pl-focusable">
              {c.portrait ? (
                <Img
                  className="br-person__img"
                  src={imageSrc(c.portrait.imageId, 342)}
                  alt=""
                  loading="lazy"
                />
              ) : (
                <span className="br-person__img br-person__initials" aria-hidden="true">
                  {initials(c.name)}
                </span>
              )}
              <strong>{c.name}</strong>
              {role(c) && <span>{role(c)}</span>}
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
