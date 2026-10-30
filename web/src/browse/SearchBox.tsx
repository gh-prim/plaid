import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { Icon } from '../ui'
import { SearchResultList, flatHits } from './SearchResults'
import { useSearch } from './useSearch'

/** The search field's id, which the "/" shortcut focuses. */
export const searchFieldId = 'br-search-field'

/**
 * Search while typing (F008-R25): a combobox in the top bar with the
 * results in a dropdown, or the whole page on phones (`page`). ↑ ↓ walk
 * the results, Enter opens one, Esc closes and then clears.
 */
export function SearchBox({ page, initial = '' }: { page?: boolean; initial?: string }) {
  const { t } = useTranslation()
  const { state } = useAuth()
  const serverName = state.status === 'ready' ? state.server.name : ''
  const navigate = useNavigate()
  const id = useId()
  const [q, setQ] = useState(initial)
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const box = useRef<HTMLDivElement>(null)
  const { query, results } = useSearch(q)
  const hits = flatHits(results)
  const shown = (page || open) && query !== ''

  useEffect(() => {
    if (page) return
    const onDown = (e: MouseEvent) => {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [page])

  function onKey(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown' && hits.length) {
      e.preventDefault()
      setOpen(true)
      setActive((a) => (a + 1) % hits.length)
    } else if (e.key === 'ArrowUp' && hits.length) {
      e.preventDefault()
      setActive((a) => (a <= 0 ? hits.length - 1 : a - 1))
    } else if (e.key === 'Enter') {
      const hit = hits[active >= 0 ? active : 0]
      if (hit) {
        e.preventDefault()
        setOpen(false)
        void navigate(hit.href)
      }
    } else if (e.key === 'Escape') {
      if (open && !page) setOpen(false)
      else setQ('')
      setActive(-1)
    }
  }

  return (
    <div ref={box} className={page ? 'br-searchbox br-searchbox--page' : 'br-searchbox'}>
      <label className="br-search pl-focusable-within">
        <Icon name="search" size={18} />
        <input
          id={searchFieldId}
          type="search"
          role="combobox"
          aria-label={t('search.label', { serverName })}
          aria-expanded={shown}
          aria-controls={`${id}-list`}
          aria-autocomplete="list"
          aria-activedescendant={shown && active >= 0 ? `${id}-list-${active}` : undefined}
          placeholder={t('search.placeholder')}
          autoComplete="off"
          autoFocus={page}
          value={q}
          onChange={(e) => {
            setQ(e.target.value)
            setOpen(true)
            setActive(-1)
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKey}
        />
      </label>
      {shown && (
        <div className={page ? 'br-search__page' : 'pl pl-menu br-search__dropdown'}>
          <SearchResultList
            id={`${id}-list`}
            query={query}
            results={results}
            active={active}
            onPick={() => setOpen(false)}
          />
        </div>
      )}
    </div>
  )
}
