// Ported from the PLAID design system: the admin components (Avatar, Tag,
// SectionIndexRow, NeedsYouItem, StreamSpeed, StreamCard, LibraryRow,
// FileRow, CandidateCard, PersonRow, DeviceRow, SettingsGroup,
// ProblemEntry). Every label comes from the caller or the catalogs.
import type { CSSProperties, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { Button } from './Button'
import { CommandBlock } from './CommandBlock'
import { cx } from './cx'
import { Icon } from './Icon'
import { PosterSlot } from './scan'

export type Tone = 'success' | 'warning' | 'error' | 'info'

/** The small square mark. Never alone: a word always names the state. */
export function Mark({ tone }: { tone?: Tone }) {
  return <span className={cx('pl-mark', tone && `pl-mark--${tone}`)} aria-hidden="true" />
}

export function Tag({ children }: { children: ReactNode }) {
  return <span className="pl-tag">{children}</span>
}

/** A person's initial on their party color (F003 profile color index 0–7). */
export function Avatar({
  name,
  color,
  size = 40,
}: {
  name: string
  color?: number
  size?: number
}) {
  const c = color ?? ((name.codePointAt(0) ?? 0) % 8) + 1
  return (
    <span
      className="pl pl-avatar"
      style={{ '--pl-s': `${size}px`, '--pc': `var(--party-${c})` } as CSSProperties}
      aria-hidden="true"
    >
      <span>{name.trim().charAt(0).toLocaleUpperCase()}</span>
    </span>
  )
}

/** One admin section in the index, with its live one-line summary. */
export function SectionIndexRow({
  to,
  label,
  summary,
  tone,
  selected,
}: {
  to: string
  label: string
  summary?: string
  tone?: Tone
  selected?: boolean
}) {
  return (
    <Link
      to={to}
      className={cx('pl pl-index-row pl-focusable', selected && 'is-selected')}
      aria-current={selected ? 'page' : undefined}
    >
      <span className="pl-index-row__text">
        <span className="pl-index-row__label">{label}</span>
        {summary && (
          <span className={cx('pl-index-row__summary', tone && `pl-tone--${tone}`)}>{summary}</span>
        )}
      </span>
      <Icon name="chevronRight" size={20} className="pl-index-row__chev" />
    </Link>
  )
}

/** Something that needs the admin, linking to where it's fixed. */
export function NeedsYouItem({
  to,
  title,
  tone = 'warning',
  card,
  children,
}: {
  to: string
  title: string
  tone?: Tone
  /** A bordered card (desktop home) instead of a row (phone index). */
  card?: boolean
  children?: ReactNode
}) {
  return (
    <Link to={to} className={cx('pl pl-needs pl-focusable', card && 'pl-needs--card')}>
      <Mark tone={tone} />
      <span className="pl-needs__text">
        <span className="pl-needs__title">{title}</span>
        {children && <span className="pl-needs__body">{children}</span>}
      </span>
    </Link>
  )
}

/** "Keeps up (1.4×)" or "Can't keep up (0.8×): it will buffer." */
export function StreamSpeed({ speed }: { speed: number }) {
  const { t, i18n } = useTranslation()
  const n = new Intl.NumberFormat(i18n.resolvedLanguage, { maximumFractionDigits: 1 }).format(speed)
  const slow = speed < 1
  return (
    <span className={cx('pl pl-speed', slow ? 'pl-tone--warning' : 'pl-tone--success')}>
      <Mark tone={slow ? 'warning' : 'success'} />
      {slow ? t('playing.speed.slow', { speed: n }) : t('playing.speed.ok', { speed: n })}
    </span>
  )
}

export type StreamMethod = 'direct_play' | 'direct_stream' | 'transcode'

/** One stream in Playing now, its reasons in sentences (F007 R19). */
export function StreamCard({
  title,
  poster,
  who,
  position,
  progress,
  method,
  methodLabel,
  reasons,
  quality,
  speed,
  advice,
  onStop,
}: {
  title: string
  poster?: string
  who: string
  position?: string
  progress?: number
  method: StreamMethod
  methodLabel: string
  reasons: string[]
  quality?: string
  speed?: number
  advice?: string
  onStop: () => void
}) {
  const { t } = useTranslation()
  const slow = speed != null && speed < 1
  return (
    <article className={cx('pl pl-stream', slow && 'is-slow')}>
      <div className="pl-stream__top">
        {poster ? (
          <img className="pl-stream__poster" src={poster} alt="" />
        ) : (
          <PosterSlot title={title} />
        )}
        <div className="pl-stream__who">
          <span className="pl-stream__title">{title}</span>
          <span className="pl-stream__meta">{who}</span>
          {position && <span className="pl-stream__meta">{position}</span>}
          {progress != null && (
            <span className="pl-stream__bar">
              <span style={{ width: `${progress}%` }} />
            </span>
          )}
        </div>
      </div>
      <div className="pl-stream__how">
        <span className="pl-stream__method">{methodLabel}</span>
        {reasons.map((r) => (
          <span key={r} className="pl-stream__reason">
            {r}
          </span>
        ))}
        {quality && <span className="pl-stream__meta">{quality}</span>}
        {speed != null && method === 'transcode' && <StreamSpeed speed={speed} />}
      </div>
      {advice && <p className="pl-stream__advice">{advice}</p>}
      <div className="pl-stream__actions">
        <Button variant="secondary" size="sm" onClick={onStop}>
          {t('playing.stop.label')}
        </Button>
      </div>
    </article>
  )
}

export function LibraryRow({
  to,
  name,
  counts,
  scan,
  issue,
}: {
  to: string
  name: string
  counts: string
  scan?: string
  issue?: string
}) {
  return (
    <Link to={to} className="pl pl-lib-row pl-focusable">
      <span className="pl-lib-row__text">
        <span className="pl-lib-row__name">{name}</span>
        <span className="pl-lib-row__meta">{counts}</span>
        {scan && <span className="pl-lib-row__meta">{scan}</span>}
        {issue && (
          <span className="pl-lib-row__issue pl-tone--error">
            <Mark tone="error" />
            {issue}
          </span>
        )}
      </span>
      <Icon name="chevronRight" size={20} className="pl-index-row__chev" />
    </Link>
  )
}

/** A file whose name PLAID couldn't read (F007 R21). */
export function FileRow({
  name,
  folder,
  identified,
  onIdentify,
}: {
  name: string
  folder?: string
  identified?: boolean
  onIdentify: () => void
}) {
  const { t } = useTranslation()
  return (
    <li className="pl pl-file-row">
      <span className="pl-file-row__text">
        <span className="pl-file-row__name">{name}</span>
        {folder && <span className="pl-file-row__dir">{folder}</span>}
      </span>
      {identified ? (
        <span className="pl-file-row__done pl-tone--success">
          <Icon name="check" size={16} />
          {t('review.identified')}
        </span>
      ) : (
        <Button variant="secondary" size="sm" onClick={onIdentify}>
          {t('review.identify')}
        </Button>
      )}
    </li>
  )
}

/** A TMDB candidate to pick (F007 R22). */
export function CandidateCard({
  title,
  meta,
  score,
  poster,
  selected,
  name,
  onSelect,
}: {
  title: string
  meta?: string
  /** 0 to 1. */
  score?: number
  poster?: string
  selected: boolean
  name: string
  onSelect: () => void
}) {
  const { t } = useTranslation()
  return (
    <label className={cx('pl pl-cand', selected && 'is-selected')}>
      <input type="radio" name={name} checked={selected} onChange={onSelect} />
      {poster ? (
        <img className="pl-cand__poster" src={poster} alt="" />
      ) : (
        <PosterSlot title={title} />
      )}
      <span className="pl-cand__text">
        <span className="pl-cand__title">{title}</span>
        {meta && <span className="pl-cand__meta">{meta}</span>}
        {score != null && <span className="pl-cand__meta">{t('review.match', { score })}</span>}
      </span>
    </label>
  )
}

export function PersonRow({
  to,
  name,
  color,
  role,
  meta,
  disabled,
  mustChangePassword,
}: {
  to: string
  name: string
  color?: number
  role: 'admin' | 'member'
  meta?: string
  disabled?: boolean
  mustChangePassword?: boolean
}) {
  const { t } = useTranslation()
  return (
    <Link to={to} className={cx('pl pl-person pl-focusable', disabled && 'is-disabled')}>
      <Avatar name={name} color={color} />
      <span className="pl-person__text">
        <span className="pl-person__name">
          {name} <Tag>{role === 'admin' ? t('people.role.admin') : t('people.role.user')}</Tag>
        </span>
        {meta && <span className="pl-person__meta">{meta}</span>}
        {disabled && <span className="pl-person__flag">{t('people.disabled')}</span>}
        {mustChangePassword && <span className="pl-person__flag">{t('people.mustChange')}</span>}
      </span>
      <Icon name="chevronRight" size={20} className="pl-index-row__chev" />
    </Link>
  )
}

export function DeviceRow({
  name,
  lastSeen,
  current,
  onSignOut,
}: {
  name: string
  lastSeen?: string
  current?: boolean
  onSignOut: () => void
}) {
  const { t } = useTranslation()
  return (
    <li className="pl pl-device">
      <span className="pl-device__text">
        <span className="pl-device__name">{name}</span>
        <span className="pl-device__meta">{current ? t('people.device.current') : lastSeen}</span>
      </span>
      <Button variant="secondary" size="sm" onClick={onSignOut}>
        {t('people.device.signOut')}
      </Button>
    </li>
  )
}

export type GroupState = 'saved' | 'changed' | 'saving' | 'error'

/** Settings in a small group, saved on its own (F007 R24). */
export function SettingsGroup({
  title,
  state,
  error,
  onSave,
  onCancel,
  children,
}: {
  title: string
  state: GroupState
  error?: string
  onSave: () => void
  onCancel: () => void
  children: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <section className={cx('pl pl-settings', `is-${state}`)} aria-label={title}>
      <h3 className="pl-settings__title">{title}</h3>
      {children}
      {state === 'error' && error && (
        <p className="pl-settings__error pl-tone--error" role="alert">
          <Icon name="error" size={16} />
          {error}
        </p>
      )}
      {state !== 'saved' && (
        <div className="pl-settings__actions">
          <Button loading={state === 'saving'} onClick={onSave}>
            {t('common.save')}
          </Button>
          <Button variant="ghost" onClick={onCancel}>
            {t('common.cancel')}
          </Button>
        </div>
      )}
    </section>
  )
}

/** One entry of Problems: what happened, what to do (F007 R26). */
export function ProblemEntry({
  when,
  fixed,
  todo,
  command,
  children,
}: {
  when: string
  fixed?: string
  todo?: string
  command?: string
  children: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <li className={cx('pl pl-problem', fixed && 'is-fixed')}>
      <Mark tone={fixed ? undefined : 'warning'} />
      <div className="pl-problem__body">
        <p className="pl-problem__when">
          {when}
          {fixed && ` · ${t('problems.resolved', { time: fixed })}`}
        </p>
        <p className="pl-problem__text">{children}</p>
        {todo && (
          <p className="pl-problem__todo">
            <strong>{t('problems.whatToDo')}</strong> {todo}
          </p>
        )}
        {command && <CommandBlock command={command} />}
      </div>
    </li>
  )
}
