import { useEffect, useEffectEvent, useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { resolutionName } from '../browse/itemFacts'
import { formatMbps } from '../lib/format'
import { cx, Icon } from '../ui'
import { audioLabel, qualityLabel, subtitleLabel } from './labels'

type Session = components['schemas']['PlaybackSession']
type MediaFile = components['schemas']['MediaFile']
type Quality = components['schemas']['Quality']

export interface MenuRow {
  key: string
  label: string
  detail?: string
  selected: boolean
  onSelect: () => void
}

/**
 * A player panel anchored above its pill (canvas: a bottom sheet on a
 * portrait phone). A menu of radio rows: arrows move, Enter picks, Esc
 * closes and gives the focus back to the pill.
 */
export function Panel({
  title,
  rows,
  foot,
  onClose,
  wide,
  children,
}: {
  title: string
  rows?: MenuRow[]
  foot?: ReactNode
  onClose: () => void
  wide?: boolean
  children?: ReactNode
}) {
  const ref = useRef<HTMLDivElement>(null)
  // The player re-renders on every timeupdate: focus once, on open.
  useEffect(() => {
    const el = ref.current
    const items = el ? [...el.querySelectorAll<HTMLElement>('[role="menuitemradio"]')] : []
    ;(items.find((i) => i.getAttribute('aria-checked') === 'true') ?? items[0] ?? el)?.focus()
  }, [])
  const close = useEffectEvent(onClose)
  useEffect(() => {
    const onDown = (e: PointerEvent) => {
      const el = ref.current
      const target = e.target as Element
      if (el && !el.contains(target) && !target.closest?.('[data-menu-pill]')) close()
    }
    document.addEventListener('pointerdown', onDown)
    return () => document.removeEventListener('pointerdown', onDown)
  }, [])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.stopPropagation()
      onClose()
      return
    }
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    e.preventDefault()
    e.stopPropagation()
    const items = [...ref.current!.querySelectorAll<HTMLElement>('[role="menuitemradio"]')]
    const i = items.indexOf(document.activeElement as HTMLElement)
    const next = items[(i + (e.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length]
    next?.focus()
  }

  return (
    <div
      ref={ref}
      className={cx('pl-ppanel pv-panel', wide && 'pl-ppanel--wide')}
      role={rows ? 'menu' : 'dialog'}
      aria-label={title}
      tabIndex={-1}
      onKeyDown={onKeyDown}
    >
      <div className="pl-ppanel__head">{title}</div>
      {rows?.map((r) => (
        <div
          key={r.key}
          role="menuitemradio"
          aria-checked={r.selected}
          tabIndex={-1}
          className={cx('pl-prow pv-prow', r.selected && 'is-selected')}
          onClick={r.onSelect}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault()
              e.stopPropagation()
              r.onSelect()
            }
          }}
        >
          <div className="pl-prow__main">
            <div className="pl-prow__label">{r.label}</div>
            {r.detail && <div className="pl-prow__detail">{r.detail}</div>}
          </div>
          {r.selected && <Icon name="check" size={18} className="pv-prow__check" />}
        </div>
      ))}
      {children}
      {foot && <div className="pl-ppanel__foot">{foot}</div>}
    </div>
  )
}

/** Subtitles: off, every text track (switched in the page), image tracks (reload). */
export function SubtitlesMenu({
  session,
  file,
  current,
  onPick,
  onClose,
}: {
  session: Session
  file?: MediaFile
  current: number | null
  onPick: (index: number | null) => void
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const sdh = sdhOf(file)
  const rows: MenuRow[] = [
    {
      key: 'off',
      label: t('player.subtitles.off'),
      selected: current === null,
      onSelect: () => onPick(null),
    },
    ...session.subtitles.map((s) => ({
      key: String(s.index),
      label: subtitleLabel(t, i18n.language, s, sdh.has(s.index)),
      detail: s.delivery === 'burn_in' ? t('player.subtitles.imageNote') : s.title,
      selected: current === s.index,
      onSelect: () => onPick(s.index),
    })),
  ]
  return (
    <Panel
      title={t('player.subtitles.label')}
      rows={rows}
      foot={sdh.size > 0 ? t('player.subtitles.sdhNote') : undefined}
      onClose={onClose}
    />
  )
}

export function AudioMenu({
  session,
  onPick,
  onClose,
}: {
  session: Session
  onPick: (index: number) => void
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  return (
    <Panel
      title={t('player.audio.label')}
      rows={session.audioTracks.map((a) => ({
        key: String(a.index),
        label: audioLabel(t, i18n.language, a),
        detail: a.title,
        selected: a.selected,
        onSelect: () => onPick(a.index),
      }))}
      foot={t('player.audio.reloadNote')}
      onClose={onClose}
    />
  )
}

export function QualityMenu({
  session,
  file,
  onPick,
  onClose,
}: {
  session: Session
  file?: MediaFile
  onPick: (q: Quality) => void
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const original = [
    resolutionName(file?.width, file?.height),
    file?.bitrate ? formatMbps(i18n.language, file.bitrate) : undefined,
    t('player.quality.asIs'),
  ]
    .filter(Boolean)
    .join(' · ')
  return (
    <Panel
      title={t('player.quality.label')}
      rows={session.qualities.map((q) => ({
        key: q,
        label: qualityLabel(t, q),
        detail: q === 'original' ? original : t('player.quality.converted'),
        selected: q === session.quality,
        onSelect: () => onPick(q),
      }))}
      foot={t('player.quality.note')}
      onClose={onClose}
    />
  )
}

/** Subtitle stream indexes the file marks as SDH (the session doesn't say). */
export function sdhOf(file?: MediaFile): Set<number> {
  return new Set(file?.streams.filter((s) => s.type === 'subtitle' && s.sdh).map((s) => s.index))
}
