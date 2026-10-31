import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { candidatePoster } from './matchQuery'
import { ArtworkPicker } from './ArtworkPicker'
import { FixMatchSheet } from './FixMatchSheet'
import { GlassToast } from './GlassSheet'

type ItemDetail = components['schemas']['ItemDetail']
type Candidate = components['schemas']['MatchCandidate']

/**
 * The admin's Edit menu on detail pages (F008-R23, V2): artwork and fix
 * match open glass sheets over the page; refresh metadata. Members never
 * see it.
 */
export function EditMenu({ item, onChange }: { item: ItemDetail; onChange: () => Promise<void> }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [picking, setPicking] = useState(false)
  const [fixing, setFixing] = useState(false)
  // What the last fix changed, with the match it replaced (Undo).
  const [fixed, setFixed] = useState<{ c: Candidate; previous?: number }>()
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
        className="br-hero__btn br-glass pl-focusable"
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
          <button
            type="button"
            role="menuitem"
            className="pl-menu__item pl-focusable br-menu__button"
            onClick={() => {
              setOpen(false)
              setFixing(true)
            }}
          >
            {t('item.edit.fixMatch')}
          </button>
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
      {fixing && (
        <FixMatchSheet
          item={item}
          onClose={() => setFixing(false)}
          onMatched={(c, previous) => {
            setFixing(false)
            setFixed({ c, previous })
            void onChange()
          }}
        />
      )}
      {fixed && (
        <GlassToast
          art={fixed.c.posterPath ? candidatePoster(fixed.c) : undefined}
          onDone={() => setFixed(undefined)}
          action={
            fixed.previous !== undefined
              ? {
                  label: t('fixMatch.undo'),
                  onClick: () => {
                    const previous = fixed.previous!
                    setFixed(undefined)
                    void api
                      .POST('/items/{itemId}/match', {
                        params: { path: { itemId: item.id } },
                        body: { tmdbId: previous },
                      })
                      .then(() => onChange())
                      .catch(() => {})
                  },
                }
              : undefined
          }
        >
          {fixed.c.year
            ? t('fixMatch.done', { title: fixed.c.title, year: fixed.c.year })
            : t('fixMatch.doneNoYear', { title: fixed.c.title })}
        </GlassToast>
      )}
    </div>
  )
}
