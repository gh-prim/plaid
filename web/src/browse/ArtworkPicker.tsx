import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { languageName } from '../lib/format'
import { Button, SegmentedControl } from '../ui'
import { imageSrc } from './media'

type ItemDetail = components['schemas']['ItemDetail']
type Candidate = components['schemas']['ArtworkCandidate']
type Kind = 'poster' | 'backdrop' | 'logo'

function candidates(itemId: string, kind: Kind) {
  return api
    .GET('/items/{itemId}/artwork', { params: { path: { itemId }, query: { kind } } })
    .then(({ data }) => ({ kind, list: data?.candidates ?? [] }))
}

/**
 * The artwork picker (F008-R23, F005's choices): a kind, its candidates
 * with their language and size, "Back to automatic" first. Choosing one
 * locks it; automatic unlocks.
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
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    void candidates(item.id, kind).then((c) => !cancelled && setLoaded(c))
    return () => {
      cancelled = true
    }
  }, [item.id, kind])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  const list = loaded?.kind === kind ? loaded.list : []
  const locked = list.some((c) => c.locked)

  async function choose(c?: Candidate) {
    setBusy(true)
    const path = { itemId: item.id, kind }
    if (c)
      await api.PUT('/items/{itemId}/artwork/{kind}', {
        params: { path },
        body: { candidateId: c.id },
      })
    else await api.DELETE('/items/{itemId}/artwork/{kind}', { params: { path } })
    setLoaded(await candidates(item.id, kind))
    setBusy(false)
    await onChange()
  }

  const width = kind === 'poster' ? 185 : 342
  // Over the whole page, whatever holds the menu.
  return createPortal(
    <div
      className="pl pl-scrim br-dialog"
      role="presentation"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <div
        className="pl-dialog br-artwork"
        role="dialog"
        aria-modal="true"
        aria-labelledby="artwork-title"
      >
        <h2 id="artwork-title" className="pl-dialog__title">
          {t('item.edit.artwork.title', { title: item.displayTitle })}
        </h2>
        <SegmentedControl<Kind>
          label={t('item.edit.artwork.kind')}
          value={kind}
          onChange={setKind}
          options={[
            { value: 'poster', label: t('item.edit.artwork.poster') },
            { value: 'backdrop', label: t('item.edit.artwork.backdrop') },
            { value: 'logo', label: t('item.edit.artwork.logo') },
          ]}
        />
        <ul
          className={`br-artwork__grid br-artwork__grid--${kind}`}
          aria-busy={busy || loaded?.kind !== kind}
        >
          <li>
            <button
              type="button"
              className="br-artwork__choice br-artwork__auto pl-focusable"
              aria-pressed={!locked}
              disabled={busy}
              onClick={() => void choose()}
            >
              <strong>{t('item.edit.artwork.automatic')}</strong>
              <span>{t('item.edit.artwork.auto')}</span>
            </button>
          </li>
          {list.map((c) => (
            <li key={c.id}>
              <button
                type="button"
                className="br-artwork__choice pl-focusable"
                aria-pressed={c.locked}
                disabled={busy}
                onClick={() => void choose(c)}
              >
                <img src={imageSrc(c.id, width)} alt="" loading="lazy" />
                <span>
                  {t('item.edit.artwork.option', {
                    language: c.language
                      ? languageName(i18n.language, c.language)
                      : t('item.edit.artwork.noLanguage'),
                    width: c.width ?? 0,
                    height: c.height ?? 0,
                  })}
                </span>
              </button>
            </li>
          ))}
        </ul>
        <div className="pl-dialog__actions">
          <Button variant="secondary" onClick={onClose}>
            {t('common.close')}
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
