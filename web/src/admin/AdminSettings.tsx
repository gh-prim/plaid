import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { formatNumber } from '../lib/format'
import { useServer } from '../onboarding/Page'
import { useAdmin } from './AdminLayout'
import { needs, statusLine } from './summary'
import { Group, Loading, Row, Section, type Tone } from './v2'

type Item = { to: string; label: string; value?: string; tone?: Tone }

/**
 * Settings (V2, boards TA01 and TQ02, like Apple TV): a big glass tile on
 * the left with the server's name and its state in words, and the sections
 * as grouped rows on the right, each with its value in grey and a chevron.
 * "Needs you" comes first, and only when something does.
 */
export function AdminSettings() {
  const { t, i18n } = useTranslation()
  const { serverName, version } = useServer()
  const { data } = useAdmin()
  const list = data ? needs(t, data) : []
  const tone: Tone = list.some((x) => x.tone === 'error') ? 'error' : list.length ? 'warning' : 'ok'

  const section = (children: ReactNode) => (
    <Section
      title={t('admin.tab.settings')}
      back={null}
      glyph="root"
      paneTitle={serverName}
      paneSub={version ? t('welcome.version', { version }) : undefined}
      status={data ? { text: statusLine(t, data, list.length), tone } : undefined}
      phoneTitle="pane"
    >
      {children}
    </Section>
  )
  if (!data) return section(<Loading />)

  const n = (v: number) => formatNumber(i18n.language, v)
  const unavailable = data.libraries.some((l) => l.folders.some((f) => f.status !== 'ok'))
  const unrecognized = data.libraries.filter((l) => l.counts.unrecognized > 0)
  const unrecognizedCount = unrecognized.reduce((a, l) => a + l.counts.unrecognized, 0)
  const open = data.problems.filter((p) => !p.resolvedAt).length
  const hw = data.capabilities?.hwAccel.active

  const groups: { title?: string; items: Item[] }[] = [
    {
      items: list.length
        ? [
            {
              to: '/admin',
              label: t('admin.needs.title'),
              value: t('admin.settings.things', { count: list.length }),
              tone: tone,
            },
          ]
        : [],
    },
    {
      title: t('admin.settings.group.library'),
      items: [
        {
          to: '/admin/libraries',
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
                label: t('admin.settings.unrecognized'),
                value: n(unrecognizedCount),
                tone: 'warning' as const,
              },
            ]
          : []),
        { to: '/admin/metadata', label: t('admin.nav.metadata') },
      ],
    },
    {
      title: t('admin.settings.group.people'),
      items: [
        { to: '/admin/people', label: t('admin.nav.people') },
        { to: '/admin/devices', label: t('admin.nav.devices') },
        { to: '/pair', label: t('pairing.title') },
      ],
    },
    {
      title: t('admin.settings.group.playback'),
      items: [
        {
          to: '/admin/transcoding',
          label: t('admin.nav.transcoding'),
          value: hw ? hw.toUpperCase() : t('admin.dash.software'),
        },
        {
          to: '/admin/playing',
          label: t('admin.nav.playing'),
          value: n(data.streams.length),
        },
      ],
    },
    {
      title: t('admin.settings.group.server'),
      items: [
        { to: '/admin/remote-access', label: t('admin.nav.remoteAccess') },
        {
          to: '/admin/problems',
          label: t('admin.nav.problems'),
          value: open ? n(open) : t('admin.settings.none'),
          tone: open ? 'warning' : undefined,
        },
      ],
    },
  ]

  return section(
    groups
      .filter((g) => g.items.length)
      .map((g, i) => (
        <Group key={i} title={g.title}>
          {g.items.map((r) => (
            <Row key={r.to} to={r.to} label={r.label} value={r.value} tone={r.tone} />
          ))}
        </Group>
      )),
  )
}
