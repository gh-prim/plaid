import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { PosterCard } from './cards/PosterCard'
import { initials } from './CastRow'
import { imageSrc } from './media'
import { Unavailable } from './Unavailable'
import { Img } from './Img'
import { PageLight } from './PageLight'

type Person = components['schemas']['Person']
type Role = components['schemas']['PersonCredit']['role']

const roles: Role[] = ['cast', 'director', 'writer', 'creator']

/** A person (F008-R22): portrait, then what they're in on this server by role, newest first. */
export function PersonPage() {
  const { t } = useTranslation()
  const { personId = '' } = useParams()
  const { state } = useAuth()
  const serverName = state.status === 'ready' ? state.server.name : ''
  const [p, setP] = useState<Person | null>()

  useEffect(() => {
    let cancelled = false
    void api.GET('/people/{personId}', { params: { path: { personId } } }).then(({ data }) => {
      if (!cancelled) setP(data ?? null)
    })
    return () => {
      cancelled = true
    }
  }, [personId])

  if (p === null) return <Unavailable />
  if (!p) return <div className="br-page" aria-busy="true" aria-label={t('common.loading')} />
  const roleName: Record<Role, string> = {
    cast: t('person.role.cast'),
    director: t('person.role.director'),
    writer: t('person.role.writer'),
    creator: t('person.role.creator'),
  }
  const titles = new Set(p.credits.map((c) => c.item.id)).size
  // What they're known for here: the role with the most titles.
  const main = [...roles].sort(
    (a, b) =>
      p.credits.filter((c) => c.role === b).length - p.credits.filter((c) => c.role === a).length,
  )[0]
  // Lit by their best-known title on the server (V2).
  const light = p.credits.find((c) => c.item.backdrop)?.item.backdrop
  return (
    <div className="br-grid-page br-personpage">
      <PageLight art={light} />
      <header className="br-personpage__head">
        {p.portrait ? (
          <Img
            className="br-person__img br-personpage__img"
            src={imageSrc(p.portrait.imageId, 342)}
            alt=""
          />
        ) : (
          <span
            className="br-person__img br-person__initials br-personpage__img"
            aria-hidden="true"
          >
            {initials(p.name)}
          </span>
        )}
        <div className="br-personpage__titles">
          {main && p.credits.length > 0 && <p className="br-personpage__known">{roleName[main]}</p>}
          <h1 className="br-personpage__name">{p.name}</h1>
          <p className="br-grid__count">{t('person.summary', { serverName, count: titles })}</p>
        </div>
      </header>
      {roles.map((role) => {
        const credits = p.credits.filter((c) => c.role === role)
        if (credits.length === 0) return null
        return (
          <section key={role} className="br-personpage__role" aria-labelledby={`role-${role}`}>
            <h2 id={`role-${role}`} className="br-row__title">
              {roleName[role]}
            </h2>
            <ul className="br-tiles">
              {credits.map((c) => (
                <li key={c.item.id}>
                  <PosterCard item={c.item} sizes="(max-width: 767px) 120px, 204px" />
                  {c.character && <span className="br-current">{c.character}</span>}
                </li>
              ))}
            </ul>
          </section>
        )
      })}
    </div>
  )
}
