import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { components } from '../api/schema'
import { LandscapeCard } from './cards/LandscapeCard'
import { PosterCard } from './cards/PosterCard'
import { Icon } from '../ui'

type ItemSummary = components['schemas']['ItemSummary']
type Page = { items: ItemSummary[]; nextCursor?: string }

/**
 * A shelf (F008-R5): a titled horizontal row with "See all", landscape
 * cards for Continue watching and posters otherwise. Scrolling to its end
 * loads the next page with `more`.
 */
export function Row({
  title,
  seeAll,
  page,
  more,
  landscape,
  onChange,
}: {
  title: string
  seeAll?: string
  page: Page
  more?: (cursor: string) => Promise<Page | undefined>
  landscape?: boolean
  onChange?: () => void
}) {
  const { t } = useTranslation()
  const [items, setItems] = useState(page.items)
  const [cursor, setCursor] = useState(page.nextCursor)
  const [loading, setLoading] = useState(false)
  const scroller = useRef<HTMLUListElement>(null)
  const end = useRef<HTMLLIElement>(null)

  // A new first page (the home reloaded) replaces what was loaded.
  const [first, setFirst] = useState(page)
  if (first !== page) {
    setFirst(page)
    setItems(page.items)
    setCursor(page.nextCursor)
  }

  useEffect(() => {
    if (!cursor || !more || loading || !end.current || typeof IntersectionObserver === 'undefined')
      return
    const io = new IntersectionObserver(
      (entries) => {
        if (!entries.some((e) => e.isIntersecting)) return
        io.disconnect()
        setLoading(true)
        void more(cursor).then((next) => {
          setLoading(false)
          if (!next) return
          setItems((prev) => [
            ...prev,
            ...next.items.filter((n) => !prev.some((p) => p.id === n.id)),
          ])
          setCursor(next.nextCursor)
        })
      },
      { root: scroller.current, rootMargin: '0px 600px 0px 0px' },
    )
    io.observe(end.current)
    return () => io.disconnect()
  }, [cursor, more, loading])

  return (
    <section className="br-row" aria-label={title}>
      <div className="br-row__head">
        {/* One row title, nothing else (V2): the title itself opens the whole list. */}
        <h2 className="br-row__title">
          {seeAll ? (
            <Link
              to={seeAll}
              className="br-row__all pl-focusable"
              aria-label={`${title}, ${t('home.row.seeAll')}`}
            >
              {title}
              <Icon name="chevronRight" size={18} />
            </Link>
          ) : (
            title
          )}
        </h2>
      </div>
      <ul ref={scroller} className={`br-row__list${landscape ? ' br-row__list--landscape' : ''}`}>
        {items.map((it) => (
          <li key={it.id}>
            {landscape ? (
              <LandscapeCard
                item={it}
                onChange={onChange}
                sizes="(max-width: 767px) 260px, 316px"
              />
            ) : (
              <PosterCard item={it} caption={false} sizes="(max-width: 767px) 120px, 204px" />
            )}
          </li>
        ))}
        {cursor && <li ref={end} className="br-row__end" aria-hidden="true" />}
      </ul>
    </section>
  )
}
