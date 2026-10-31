import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { languageName } from '../lib/format'
import { Icon, cx } from '../ui'
import { GlassSheet } from './GlassSheet'
import { firstYear } from './MovieHeader'
import { imageSrc } from './media'

type ItemDetail = components['schemas']['ItemDetail']
type Candidate = components['schemas']['ArtworkCandidate']
type Kind = 'poster' | 'backdrop' | 'logo'

function candidates(itemId: string, kind: Kind) {
  return api
    .GET('/items/{itemId}/artwork', { params: { path: { itemId }, query: { kind } } })
    .then(({ data }) => ({ kind, list: data?.candidates ?? [] }))
    .catch(() => ({ kind, list: [] as Candidate[] }))
}

/** Provider names, never translated. */
const sources = { tmdb: 'TMDB', fanart: 'Fanart.tv' }

/**
 * The artwork sheet (F008-R23, F005's choices; V2): a kind, its options
 * with their source, language and size. Picking one applies at once for
 * every profile and locks it; "Back to automatic" unlocks.
 */
export function ArtworkPicker({
  item,
  onClose,
  onChange,
}: {
  item: ItemDetail
  onClose: () => void
  onChange: () => Promise<void>
}) {
  const { t, i18n } = useTranslation()
  const [kind, setKind] = useState<Kind>('poster')
  const [loaded, setLoaded] = useState<{ kind: Kind; list: Candidate[] }>()
  const [busy, setBusy] = useState<string>()

  useEffect(() => {
    let cancelled = false
    void candidates(item.id, kind).then((c) => !cancelled && setLoaded(c))
    return () => {
      cancelled = true
    }
  }, [item.id, kind])

  const ready = loaded?.kind === kind
  const list = ready ? loaded.list : []
  const locked = list.some((c) => c.locked)

  async function choose(c?: Candidate) {
    if (busy) return
    setBusy(c?.id ?? 'auto')
    const path = { itemId: item.id, kind }
    try {
      if (c)
        await api.PUT('/items/{itemId}/artwork/{kind}', {
          params: { path },
          body: { candidateId: c.id },
        })
      else await api.DELETE('/items/{itemId}/artwork/{kind}', { params: { path } })
      setLoaded(await candidates(item.id, kind))
      await onChange()
    } finally {
      setBusy(undefined)
    }
  }

  const year = firstYear(item)
  const kinds: Kind[] = ['poster', 'backdrop', 'logo']
  const kindLabel = (k: Kind) =>
    k === 'poster'
      ? t('item.edit.artwork.poster')
      : k === 'backdrop'
        ? t('item.edit.artwork.backdrop')
        : t('item.edit.artwork.logo')
  return (
    <GlassSheet
      title={t('item.edit.artwork.sheet')}
      subtitle={[item.displayTitle, year].filter(Boolean).join(t('card.separator'))}
      wide={kind !== 'poster'}
      onClose={onClose}
      footer={
        <>
          <span className="br-sheet__note">
            {locked ? t('item.edit.artwork.byHand') : t('item.edit.artwork.picked')}
          </span>
          <span className="br-filters__fill" />
          {locked && (
            <button
              type="button"
              className="br-sheet__pill pl-focusable"
              disabled={!!busy}
              aria-busy={busy === 'auto'}
              onClick={() => void choose()}
            >
              {t('item.edit.artwork.automatic')}
            </button>
          )}
        </>
      }
    >
      <div
        className="br-segmented br-sheet__glass"
        role="radiogroup"
        aria-label={t('item.edit.artwork.kind')}
      >
        {kinds.map((k) => (
          <button
            key={k}
            type="button"
            role="radio"
            aria-checked={kind === k}
            className={cx('br-segmented__item pl-focusable', kind === k && 'is-on')}
            onClick={() => setKind(k)}
          >
            {kindLabel(k)}
          </button>
        ))}
      </div>
      <ul className={`br-art br-art--${kind}`} aria-busy={!ready || !!busy}>
        {!ready &&
          Array.from({ length: kind === 'poster' ? 5 : 3 }, (_, i) => (
            <li key={i} aria-hidden="true">
              <span className="br-art__img br-skeleton" />
            </li>
          ))}
        {ready && list.length === 0 && (
          <li className="br-art__none">{t('item.edit.artwork.none')}</li>
        )}
        {list.map((c) => {
          const lang = c.language
            ? languageName(i18n.language, c.language)
            : t('item.edit.artwork.noLanguage')
          const size = c.width && c.height ? `${c.width} × ${c.height}` : ''
          const source =
            c.source === 'local' ? t('item.edit.artwork.source.local') : sources[c.source]
          return (
            <li key={c.id}>
              <button
                type="button"
                className="br-art__choice pl-focusable"
                aria-pressed={c.selected}
                aria-label={[source, lang, size].filter(Boolean).join(', ')}
                disabled={!!busy}
                onClick={() => void choose(c)}
              >
                <span className={cx('br-art__img', busy === c.id && 'is-busy')}>
                  <img src={imageSrc(c.id, kind === 'poster' ? 185 : 342)} alt="" loading="lazy" />
                  {c.selected && (
                    <span className="br-art__check" aria-hidden="true">
                      <Icon name="check" size={14} />
                    </span>
                  )}
                </span>
                <span className="br-art__caption">
                  {source}
                  <span>{[lang, size].filter(Boolean).join(t('card.separator'))}</span>
                </span>
              </button>
            </li>
          )
        })}
      </ul>
    </GlassSheet>
  )
}
