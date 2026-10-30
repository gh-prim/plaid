import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { Avatar } from '../ui'

type ActivePlayback = components['schemas']['ActivePlayback']

/** The admin's server card in the sidebar: who is watching what (F008-R1, F007-R17). */
export function ServerCard({ name, streams }: { name: string; streams?: ActivePlayback[] }) {
  const { t } = useTranslation()
  const count = streams?.length ?? 0
  return (
    <Link to="/admin" className="br-server pl-focusable">
      <span className="br-server__head">
        <span className="br-server__mark" aria-hidden="true" />
        <span className="br-server__name">{name}</span>
        <span className="br-server__state">
          {count > 0 ? t('nav.serverCard.watching', { count }) : t('nav.serverCard.idle')}
        </span>
      </span>
      {streams?.map((s) => (
        <span key={s.session.id} className="br-server__stream">
          <Avatar name={s.profileName} size={24} />
          <span className="br-server__who">
            <strong>{s.profileName}</strong>
            <span>{s.itemTitle}</span>
          </span>
        </span>
      ))}
    </Link>
  )
}

/** The same, as a pill in the phone's top bar. */
export function ServerPill({ name, streams }: { name: string; streams?: ActivePlayback[] }) {
  const { t } = useTranslation()
  const count = streams?.length ?? 0
  return (
    <Link to="/admin" className="br-pill pl-focusable">
      {count > 0 ? t('nav.serverStrip', { serverName: name, count }) : name}
    </Link>
  )
}
