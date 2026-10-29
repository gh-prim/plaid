// Ported from the PLAID design system: one folder on the server's disks.
// The browse API gives a name, a path and whether PLAID can read it, never
// files, so a row never shows a file count (F007 R8, R9).
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { mountCommand, setfaclCommand } from '../lib/shellQuote'
import { Button, Spinner } from './Button'
import { CommandBlock } from './CommandBlock'
import { cx } from './cx'
import { Icon } from './Icon'

export type FolderStatus = 'ok' | 'noAccess' | 'unavailable' | 'checking' | 'fixed' | 'disabled'

type FolderRowProps = {
  name: string
  path: string
  status?: FolderStatus
  /** Paths at the roots level are shown in the code font. */
  mono?: boolean
  /** Admin: "Reachable · Watched live · Local disk (ext4)". */
  meta?: string
  expanded?: boolean
  /** Admin: replaces the unavailable folder's explanation. */
  reason?: ReactNode
  /** Primary when Check again is the screen's fix. */
  checkVariant?: 'primary' | 'secondary'
  onOpen?: () => void
  onToggle?: () => void
  onCheck?: () => void
}

export function FolderRow({
  name,
  path,
  status = 'ok',
  mono,
  meta,
  expanded,
  reason,
  checkVariant = 'secondary',
  onOpen,
  onToggle,
  onCheck,
}: FolderRowProps) {
  const { t } = useTranslation()
  const unavailable = status === 'unavailable'
  const blocked = status === 'noAccess' || status === 'checking' || unavailable
  const word: Partial<Record<FolderStatus, string>> = {
    unavailable: t('folder.status.unavailable'),
    noAccess: t('picker.noAccess'),
    checking: t('picker.checking'),
    fixed: t('picker.readable'),
    disabled: t('picker.notAllowed'),
  }
  const icon =
    status === 'noAccess' ? 'lock' : unavailable ? 'alert' : status === 'fixed' ? 'check' : 'folder'
  const label = (
    <>
      <Icon name={icon} size={20} className="pl-folder__icon" />
      <span className="pl-folder__label">
        <span className={cx('pl-folder__name', mono && 'pl-folder__name--mono')}>{name}</span>
        {meta && <span className="pl-folder__meta">{meta}</span>}
      </span>
      {word[status] && (
        <span className="pl-folder__word">
          {status === 'checking' && <Spinner size={14} mono />}
          {word[status]}
        </span>
      )}
    </>
  )
  const head =
    blocked || status === 'disabled' ? (
      <button
        type="button"
        className="pl-folder__head pl-focusable"
        disabled={status === 'disabled'}
        aria-expanded={blocked ? !!expanded : undefined}
        onClick={onToggle}
      >
        {label}
      </button>
    ) : (
      <button type="button" className="pl-folder__head pl-focusable" onClick={onOpen}>
        {label}
        <Icon name="chevronRight" size={20} className="pl-folder__chev" />
      </button>
    )
  return (
    <li className={cx('pl pl-folder', `pl-folder--${status}`, expanded && 'is-expanded')}>
      {head}
      {blocked && expanded && (
        <div className="pl-folder__fix">
          <p className="pl-folder__title">
            {unavailable ? t('folder.unavailable.title', { path }) : t('errors.folder_unreadable')}
          </p>
          <p className="pl-folder__text">
            {unavailable
              ? (reason ?? t('folder.unavailable.check'))
              : t('picker.unreadable.why', { name })}
          </p>
          <CommandBlock command={unavailable ? mountCommand(path) : setfaclCommand(path)} />
          {!unavailable && <p className="pl-folder__hint">{t('picker.unreadable.docker')}</p>}
          <div>
            <Button
              variant={checkVariant}
              size="sm"
              loading={status === 'checking'}
              onClick={onCheck}
            >
              {t('picker.checkAgain')}
            </Button>
          </div>
        </div>
      )}
    </li>
  )
}
