import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { formatNumber } from '../lib/format'
import { useServer } from '../onboarding/Page'
import { Icon, Spinner, type IconName } from '../ui'
import { useAdmin } from './AdminLayout'
import { needs, statusLine } from './summary'

type Row = {
  to: string
  icon: IconName
  label: string
  value?: string
  tone?: 'error' | 'warning'
}

/**
 * Settings (V2, like Apple TV): a big glass tile on the left that says
 * where you are, with the server's status in words, and one list of quiet
 * glass rows on the right, each with its value in grey and a chevron.
 */
export function AdminSettings() {
  const { t, i18n } = useTranslation()
  const { serverName } = useServer()
  const { data } = useAdmin()
  if (!data)
    return (
      <div className="adm__empty" role="status" aria-busy="true">
        <Spinner size={28} />
      </div>
    )
  const n = (v: number) => formatNumber(i18n.language, v)
  const unavailable = data.libraries.some((l) => l.folders.some((f) => f.status !== 'ok'))
  const unrecognized = data.libraries.filter((l) => l.counts.unrecognized > 0)
  const unrecognizedCount = unrecognized.reduce((a, l) => a + l.counts.unrecognized, 0)
  const open = data.problems.filter((p) => !p.resolvedAt).length
  const hw = data.capabilities?.hwAccel.active
  const list = needs(t, data)

  const groups: Row[][] = [
    [
      {
        to: '/admin/libraries',
        icon: 'folder',
        label: t('admin.nav.libraries'),
        value: unavailable
          ? t('folder.status.unavailable')
          : t('admin.settings.libraries', { count: data.libraries.length }),
        tone: unavailable ? 'error' : undefined,
      },
      ...(unrecognized[0]
        ? [
            {
              to: `/admin/libraries/${unrecognized[0].id}/unrecognized`,
              icon: 'alert' as const,
              label: t('admin.settings.unrecognized'),
              value: n(unrecognizedCount),
              tone: 'warning' as const,
            },
          ]
        : []),
      { to: '/admin/metadata', icon: 'info', label: t('admin.nav.metadata') },
    ],
    [
      { to: '/admin/people', icon: 'users', label: t('admin.nav.people') },
      { to: '/pair', icon: 'tv', label: t('pairing.title') },
    ],
    [
      {
        to: '/admin/transcoding',
        icon: 'quality',
        label: t('admin.nav.transcoding'),
        value: hw ? hw.toUpperCase() : t('admin.dash.software'),
      },
      {
        to: '/admin/playing',
        icon: 'play',
        label: t('admin.nav.playing'),
        value: n(data.streams.length),
      },
      {
        to: '/admin/problems',
        icon: 'alert',
        label: t('admin.nav.problems'),
        value: open ? n(open) : t('admin.settings.none'),
        tone: open ? 'warning' : undefined,
      },
    ],
  ]

  return (
    <div className="adm-settings">
      <h1 className="adm-settings__title">{t('admin.tab.settings')}</h1>
      <div className="adm-settings__body">
        <div className="adm-settings__tile br-glass" aria-hidden="true">
          <Icon name="tv" size={72} />
          <strong>{serverName}</strong>
          <span
            className={
              list.some((x) => x.tone === 'error')
                ? 'is-error'
                : list.length
                  ? 'is-warning'
                  : 'is-ok'
            }
          >
            {statusLine(t, data, list.length)}
          </span>
        </div>
        <div className="adm-settings__groups">
          {groups.map((g, i) => (
            <ul key={i} className="adm-list">
              {g.map((r) => (
                <li key={r.to}>
                  <Link to={r.to} className="adm-list__row pl-focusable">
                    <Icon name={r.icon} size={20} />
                    <span className="adm-list__label">{r.label}</span>
                    {r.value && (
                      <span className={`adm-list__value${r.tone ? ` is-${r.tone}` : ''}`}>
                        {r.value}
                      </span>
                    )}
                    <Icon name="chevronRight" size={16} />
                  </Link>
                </li>
              ))}
            </ul>
          ))}
        </div>
      </div>
    </div>
  )
}
