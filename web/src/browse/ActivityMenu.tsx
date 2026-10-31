import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import { formatNumber } from '../lib/format'
import { methodLabel } from '../player/labels'
import { cx, Icon, ProgressBar } from '../ui'

type Library = components['schemas']['Library']
type LibraryMetadataCounts = components['schemas']['LibraryMetadataCounts']
type ActivePlayback = components['schemas']['ActivePlayback']

interface Activity {
  libraries: Library[]
  metadata: LibraryMetadataCounts[]
}

/** How often the button checks for work; faster while the panel is open. */
const idleEvery = 5000
const openEvery = 2000

/**
 * The admin's activity menu in the top bar, like Plex's: what the server is
 * doing right now (scans, metadata matching, who is watching), with progress.
 */
export function ActivityMenu({ streams }: { streams?: ActivePlayback[] }) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLSpanElement>(null)
  const { data } = usePolling<Activity>(
    async () => {
      const [libs, meta] = await Promise.all([api.GET('/libraries'), api.GET('/metadata/status')])
      return { libraries: libs.data ?? [], metadata: meta.data?.libraries ?? [] }
    },
    open ? openEvery : idleEvery,
  )

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const n = (v: number) => formatNumber(lng, v)
  const tasks: { key: string; title: string; detail?: string; value?: number }[] = []
  for (const lib of data?.libraries ?? []) {
    const s = lib.scan
    if (s.state === 'idle') continue
    const c = s.counters
    if (s.state === 'queued') {
      tasks.push({
        key: `scan-${lib.id}`,
        title: t('activity.scan', { library: lib.name }),
        detail: t('activity.queued'),
      })
    } else if (s.state === 'analyzing') {
      tasks.push({
        key: `scan-${lib.id}`,
        title: t('activity.analyze', { library: lib.name }),
        detail: t('activity.ofTotal', { done: n(c.analyzed), total: n(c.found) }),
        value: c.found ? (c.analyzed / c.found) * 100 : 0,
      })
    } else {
      tasks.push({
        key: `scan-${lib.id}`,
        title: t('activity.scan', { library: lib.name }),
        detail: [t('activity.found', { count: c.found }), s.currentFolder?.split('/').pop()]
          .filter(Boolean)
          .join(' · '),
      })
    }
  }
  for (const m of data?.metadata ?? []) {
    if (m.pending <= 0) continue
    const done = m.matched + m.needsReview + m.unmatched
    const total = done + m.pending
    const lib = data?.libraries.find((l) => l.id === m.libraryId)
    tasks.push({
      key: `meta-${m.libraryId}`,
      title: t('activity.match', { library: lib?.name ?? '' }),
      detail: t('activity.ofTotal', { done: n(done), total: n(total) }),
      value: total ? (done / total) * 100 : 0,
    })
  }
  for (const m of data?.metadata ?? []) {
    // Titles already looked up, fetched again (artwork, credits): no total
    // to count against, so the bar just runs.
    const refreshing = m.refreshing ?? 0
    if (refreshing <= 0) continue
    const lib = data?.libraries.find((l) => l.id === m.libraryId)
    tasks.push({
      key: `refresh-${m.libraryId}`,
      title: t('activity.refresh', { library: lib?.name ?? '' }),
      detail: t('activity.titlesLeft', { count: refreshing }),
    })
  }
  for (const s of streams ?? []) {
    tasks.push({
      key: `play-${s.session.id}`,
      title: t('activity.watching', { name: s.profileName, title: s.itemTitle }),
      detail: methodLabel(t, s.session.method),
    })
  }

  return (
    <span ref={ref} className="br-activity">
      <button
        type="button"
        className={cx('br-activity__button pl-focusable', tasks.length > 0 && 'is-busy')}
        aria-label={t('activity.label', { count: tasks.length })}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        {tasks.length > 0 && (
          // Work is running: a ring turns around the icon, like Plex's.
          <svg className="br-activity__ring" viewBox="0 0 40 40" aria-hidden="true">
            <circle cx="20" cy="20" r="18" />
          </svg>
        )}
        <Icon name="activity" size={22} />
        {tasks.length > 0 && <span className="br-activity__count">{n(tasks.length)}</span>}
      </button>
      {open && (
        <div
          className="pl pl-menu br-menu br-menu--down br-activity__panel"
          role="dialog"
          aria-label={t('activity.title')}
        >
          <div className="br-activity__head">{t('activity.title')}</div>
          {!data ? (
            // First answer not in yet: placeholder rows that shimmer.
            <ul className="br-activity__list" aria-busy="true" aria-label={t('activity.loading')}>
              {[0, 1].map((i) => (
                <li key={i}>
                  <span
                    className="br-skeleton br-activity__shimmer"
                    style={{ width: '70%', height: 16 }}
                  />
                  <span
                    className="br-skeleton br-activity__shimmer"
                    style={{ width: '45%', height: 12 }}
                  />
                  <span
                    className="br-skeleton br-activity__shimmer"
                    style={{ width: '100%', height: 6 }}
                  />
                </li>
              ))}
            </ul>
          ) : tasks.length === 0 ? (
            <p className="br-activity__empty">{t('activity.empty')}</p>
          ) : (
            <ul className="br-activity__list">
              {tasks.map((task) => (
                <li key={task.key}>
                  <div className="br-activity__title">{task.title}</div>
                  {task.detail && <div className="br-activity__detail">{task.detail}</div>}
                  {task.value !== undefined ? (
                    <ProgressBar
                      variant="scan"
                      value={task.value}
                      valueLabel={new Intl.NumberFormat(lng, { style: 'percent' }).format(
                        task.value / 100,
                      )}
                    />
                  ) : task.key.startsWith('scan-') || task.key.startsWith('refresh-') ? (
                    <ProgressBar variant="scan" indeterminate />
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </span>
  )
}
