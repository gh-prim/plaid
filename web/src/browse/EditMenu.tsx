import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { ArtworkPicker } from './ArtworkPicker'

type ItemDetail = components['schemas']['ItemDetail']

/**
 * The admin's Edit menu on detail pages (F008-R23): change artwork, fix
 * the match in F007's review screen, refresh metadata. Members never see it.
 */
export function EditMenu({ item, onChange }: { item: ItemDetail; onChange: () => Promise<void> }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [picking, setPicking] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onDown)
    }
  }, [open])

  async function refresh() {
    setOpen(false)
    setRefreshing(true)
    await api.POST('/items/{itemId}/refresh', {
      params: { path: { itemId: item.id } },
      body: { images: false },
    })
    await onChange()
  }

  return (
    <div className="br-filters__anchor" ref={ref}>
      <button
        type="button"
        className="pl-btn pl-btn--secondary pl-focusable"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        {t('item.edit.label')} <span className="br-caret" aria-hidden="true" />
      </button>
      {open && (
        <div className="pl pl-menu br-menu br-menu--down" role="menu">
          <button
            type="button"
            role="menuitem"
            className="pl-menu__item pl-focusable br-menu__button"
            onClick={() => {
              setOpen(false)
              setPicking(true)
            }}
          >
            {t('item.edit.artwork.label')}
          </button>
          {item.libraryId && (
            <Link
              to={`/admin/libraries/${item.libraryId}/review`}
              role="menuitem"
              className="pl-menu__item pl-focusable"
            >
              {t('item.edit.fixMatch')}
            </Link>
          )}
          <button
            type="button"
            role="menuitem"
            className="pl-menu__item pl-focusable br-menu__button"
            onClick={() => void refresh()}
          >
            {t('item.edit.refresh')}
          </button>
        </div>
      )}
      {refreshing && (
        <p role="status" className="br-trailer__notice">
          {t('item.edit.refreshed')}
        </p>
      )}
      {picking && (
        <ArtworkPicker item={item} onClose={() => setPicking(false)} onChange={onChange} />
      )}
    </div>
  )
}
