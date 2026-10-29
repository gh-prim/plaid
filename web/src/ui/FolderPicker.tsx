// Ported from the PLAID design system: browse the server's folders one
// level at a time (F007 R8). Presentational: the caller fetches
// /fs/browse and owns the state.
import { useTranslation } from 'react-i18next'
import { Button } from './Button'
import { cx } from './cx'
import { FolderRow, type FolderStatus } from './FolderRow'
import { Icon } from './Icon'

export type FolderEntry = {
  name: string
  path: string
  status: FolderStatus
  expanded?: boolean
}

type FolderPickerProps = {
  serverName: string
  /** The folder being listed; empty at the roots. */
  path: string
  entries: FolderEntry[]
  layout?: 'sheet' | 'panel'
  /** Listing the PLAID_BROWSE_ROOTS. */
  atRoots?: boolean
  /** Shown when there are more than 20 sub-folders. */
  filter?: { value: string; total: number; onChange: (v: string) => void }
  onOpen: (entry: FolderEntry) => void
  onToggle: (entry: FolderEntry) => void
  onCheck: (entry: FolderEntry) => void
  onParent?: () => void
  onUse: () => void
  onClose: () => void
  onTypePath?: () => void
}

export function FolderPicker({
  serverName,
  path,
  entries,
  layout = 'sheet',
  atRoots,
  filter,
  onOpen,
  onToggle,
  onCheck,
  onParent,
  onUse,
  onClose,
  onTypePath,
}: FolderPickerProps) {
  const { t } = useTranslation()
  const crumbs = path.split('/').filter(Boolean)
  const tail = atRoots ? t('picker.rootsLabel') : (crumbs.at(-1) ?? '/')
  const head =
    atRoots || crumbs.length === 0
      ? ''
      : '/' + crumbs.slice(0, -1).join('/') + (crumbs.length > 1 ? '/' : '')
  const title = t('picker.title', { serverName })
  return (
    <section className={cx('pl pl-picker', `pl-picker--${layout}`)} aria-label={title}>
      <header className="pl-picker__head">
        <div className="pl-picker__top">
          <h2 className="pl-picker__title">{title}</h2>
          {layout === 'panel' ? (
            <Button variant="icon" icon="x" label={t('common.close')} onClick={onClose} />
          ) : (
            <Button variant="ghost" onClick={onClose}>
              {t('common.cancel')}
            </Button>
          )}
        </div>
        <div className="pl-picker__path">
          <Button
            variant="icon"
            icon="chevronLeft"
            label={t('picker.parent')}
            disabled={atRoots || !onParent}
            onClick={onParent}
          />
          <p className={cx('pl-picker__crumbs', atRoots && 'is-label')}>
            {head}
            <b>{tail}</b>
          </p>
        </div>
        {filter && (
          <div className="pl-picker__filter">
            <Icon name="search" size={18} />
            <input
              value={filter.value}
              onChange={(e) => filter.onChange(e.target.value)}
              aria-label={t('picker.filter', { total: filter.total })}
              placeholder={t('picker.filter', { total: filter.total })}
            />
            <span>{t('picker.filterCount', { shown: entries.length, total: filter.total })}</span>
          </div>
        )}
      </header>
      {atRoots && (
        <p className="pl-picker__note">
          <Icon name="info" size={18} />
          <span>{t('picker.roots')}</span>
        </p>
      )}
      <ul className="pl-picker__list">
        {entries.length > 0 ? (
          entries.map((e) => (
            <FolderRow
              key={e.path}
              name={e.name}
              path={e.path}
              mono={atRoots}
              status={e.status}
              expanded={e.expanded}
              onOpen={() => onOpen(e)}
              onToggle={() => onToggle(e)}
              onCheck={() => onCheck(e)}
            />
          ))
        ) : filter?.value ? (
          <li className="pl-picker__empty">
            <p>{t('picker.noMatch', { query: filter.value })}</p>
          </li>
        ) : (
          <li className="pl-picker__empty">
            <Icon name="folder" size={28} />
            <p className="pl-picker__empty-title">{t('picker.empty.title')}</p>
            <p>{t('picker.empty.body')}</p>
          </li>
        )}
      </ul>
      <footer className="pl-picker__foot">
        {atRoots && <p className="pl-picker__hint">{t('picker.openHint')}</p>}
        <Button variant="primary" block={layout === 'sheet'} disabled={atRoots} onClick={onUse}>
          {t('picker.use')}
        </Button>
        {onTypePath && (
          <Button variant="ghost" block={layout === 'sheet'} onClick={onTypePath}>
            {t('picker.typePath')}
          </Button>
        )}
      </footer>
    </section>
  )
}
