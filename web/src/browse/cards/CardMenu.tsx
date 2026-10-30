import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../../api/client'
import type { components } from '../../api/schema'
import { Icon } from '../../ui'

type ItemSummary = components['schemas']['ItemSummary']

/** A card's "More actions" menu (F008-R7): mark watched, and go to the show or the item. */
export function CardMenu({
  item,
  title,
  onChange,
}: {
  item: ItemSummary
  title: string
  onChange?: () => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
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

  const watched = item.userState?.watched ?? false
  async function toggleWatched() {
    setOpen(false)
    const params = { params: { path: { itemId: item.id } } }
    if (watched) await api.DELETE('/items/{itemId}/watched', params)
    else await api.PUT('/items/{itemId}/watched', params)
    onChange?.()
  }

  const detail = item.kind === 'episode' && item.show ? `/item/${item.show.id}` : `/item/${item.id}`
  return (
    <div className="br-cardmenu" ref={ref}>
      <button
        type="button"
        className="br-cardmenu__button pl-focusable"
        aria-label={t('card.menu', { title })}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <Icon name="more" size={18} />
      </button>
      {open && (
        <div className="pl pl-menu br-menu br-menu--down" role="menu">
          <button
            type="button"
            role="menuitem"
            className="pl-menu__item pl-focusable br-menu__button"
            onClick={() => void toggleWatched()}
          >
            {watched ? t('item.markUnwatched') : t('item.markWatched')}
          </button>
          <Link to={detail} role="menuitem" className="pl-menu__item pl-focusable">
            {item.kind === 'episode' ? t('card.goToShow') : t('card.moreInfo')}
          </Link>
        </div>
      )}
    </div>
  )
}
