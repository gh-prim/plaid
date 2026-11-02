import { useEffect, useMemo, useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Profile } from '../api/client'
import type { components } from '../api/schema'
import { serverUrl } from '../api/server'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { Button, cx, Icon, Spinner } from '../ui'
import './avatar.css'

type Builder = components['schemas']['AvatarBuilder']
type Options = components['schemas']['AvatarOptions']
type PartName = components['schemas']['AvatarPart']['name']

/** The avatar colors: the `party-*` tokens, 0–7 as profiles store them. */
export const avatarColors = [0, 1, 2, 3, 4, 5, 6, 7]

/** The server's drawing of options, for previews and tiles. */
export function renderSrc(options: Options, size: number, color?: number): string {
  const q = new URLSearchParams({ options: JSON.stringify(options), size: String(size) })
  if (color !== undefined) q.set('color', String(color))
  return serverUrl(`/api/v1/avatar-builder/render?${q}`)
}

const pick = <T,>(list: readonly T[]) => list[Math.floor(Math.random() * list.length)]!

/** An avatar no one chose: every part at random, optional ones half the time. */
export function randomOptions(b: Builder): Options {
  const o = { ...b.defaults }
  for (const p of b.parts) {
    const v = p.optional && Math.random() < 0.6 ? null : pick(p.variants)
    ;(o as Record<string, unknown>)[p.name] = v
  }
  o.hairColor = pick(b.hairColors)
  o.skinColor = pick(b.skinColors)
  return o
}

/** Tabs in the order of board Avatar01; mouth, nose and the rest follow. */
const tabOrder: (PartName | 'background')[] = [
  'head',
  'hair',
  'eyes',
  'glasses',
  'beard',
  'background',
  'eyebrows',
  'mouth',
  'nose',
  'earrings',
  'freckles',
]

/**
 * The avatar builder (board Avatar01): the avatar in big on top, a tab per
 * part, the part's choices as drawn tiles, its colors as swatches, Surprise
 * me, and OK which saves it on the profile.
 */
export function AvatarBuilder({
  profile,
  onDone,
  onCancel,
}: {
  profile: Profile
  onDone: (p: Profile) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [builder, setBuilder] = useState<Builder>()
  const [options, setOptions] = useState<Options>()
  const [color, setColor] = useState(profile.avatarColor)
  const [tab, setTab] = useState<PartName | 'background'>('hair')
  const [saving, setSaving] = useState(false)
  const [fail, setFail] = useState<Failure>()

  useEffect(() => {
    void api.GET('/avatar-builder').then(({ data }) => {
      if (!data) return
      setBuilder(data)
      setOptions(profile.avatar.options ?? randomOptions(data))
    })
  }, [profile.avatar.options])

  const part = useMemo(() => builder?.parts.find((p) => p.name === tab), [builder, tab])

  if (!builder || !options)
    return (
      <div className="av av--loading" role="status">
        <Spinner />
        <span>{t('avatar.builder.loading')}</span>
      </div>
    )

  async function save() {
    setSaving(true)
    setFail(undefined)
    if (color !== profile.avatarColor) {
      const { error, response } = await api.PATCH('/me/profiles/{profileId}', {
        params: { path: { profileId: profile.id } },
        body: { avatarColor: color },
      })
      if (error) {
        setSaving(false)
        return setFail(failure(error, response))
      }
    }
    const { data, error, response } = await api.PUT('/me/profiles/{profileId}/avatar', {
      params: { path: { profileId: profile.id } },
      body: { kind: 'builder', options },
    })
    setSaving(false)
    if (error) return setFail(failure(error, response))
    onDone(data)
  }

  const set = (name: PartName, v: string | null) => setOptions((o) => ({ ...o!, [name]: v }))
  const swatches =
    tab === 'hair' || tab === 'eyebrows' || tab === 'beard'
      ? { key: 'hairColor' as const, list: builder.hairColors, label: t('avatar.builder.color') }
      : tab === 'head'
        ? { key: 'skinColor' as const, list: builder.skinColors, label: t('avatar.builder.skin') }
        : undefined

  return (
    <div className="av">
      <header className="av__top">
        <button
          type="button"
          className="ob-glassbtn pl-focusable av__back"
          aria-label={t('common.back')}
          onClick={onCancel}
        >
          <Icon name="chevronLeft" size={20} />
        </button>
        <h1 className="av__title">{t('avatar.builder.title')}</h1>
        <Button size="sm" loading={saving} onClick={() => void save()}>
          {t('avatar.builder.ok')}
        </Button>
      </header>

      <div className="av__stage">
        <img
          className="av__big"
          src={renderSrc(options, 340, color)}
          alt=""
          width={170}
          height={170}
          style={{ '--pc': `var(--party-${color + 1})` } as CSSProperties}
        />
        <button
          type="button"
          className="av__random pl-focusable"
          onClick={() => setOptions(randomOptions(builder))}
        >
          <Icon name="restart" size={15} />
          {t('avatar.builder.random')}
        </button>
      </div>

      <section className="av__sheet">
        <div className="av__tabs" role="tablist" aria-label={t('avatar.builder.title')}>
          {tabOrder.map((name) => (
            <button
              key={name}
              type="button"
              role="tab"
              aria-selected={tab === name}
              className={cx('av__tab pl-focusable', tab === name && 'is-selected')}
              onClick={() => setTab(name)}
            >
              {t(`avatar.builder.tab.${name}`)}
            </button>
          ))}
        </div>

        {tab === 'background' ? (
          <div
            className="av__swatches av__swatches--big"
            role="radiogroup"
            aria-label={t('avatar.builder.background')}
          >
            {avatarColors.map((c) => (
              <button
                key={c}
                type="button"
                role="radio"
                aria-checked={color === c}
                aria-label={t('avatar.builder.variant', { n: c + 1 })}
                className={cx('av__swatch pl-focusable', color === c && 'is-selected')}
                style={{ background: `var(--party-${c + 1})` }}
                onClick={() => setColor(c)}
              />
            ))}
          </div>
        ) : (
          part && (
            <div
              className="av__grid"
              role="radiogroup"
              aria-label={t(`avatar.builder.tab.${part.name}`)}
            >
              {part.optional && (
                <button
                  type="button"
                  role="radio"
                  aria-checked={!options[part.name]}
                  className={cx(
                    'av__tile av__tile--none pl-focusable',
                    !options[part.name] && 'is-selected',
                  )}
                  onClick={() => set(part.name, null)}
                >
                  {t('avatar.builder.none')}
                </button>
              )}
              {part.variants.map((v, i) => (
                <button
                  key={v}
                  type="button"
                  role="radio"
                  aria-checked={options[part.name] === v}
                  aria-label={t('avatar.builder.variant', { n: i + 1 })}
                  className={cx('av__tile pl-focusable', options[part.name] === v && 'is-selected')}
                  onClick={() => set(part.name, v)}
                >
                  <img src={renderSrc({ ...options, [part.name]: v }, 200)} alt="" loading="lazy" />
                </button>
              ))}
            </div>
          )
        )}

        {swatches && (
          <>
            <p className="av__label">{swatches.label}</p>
            <div className="av__swatches" role="radiogroup" aria-label={swatches.label}>
              {swatches.list.map((c, i) => (
                <button
                  key={c}
                  type="button"
                  role="radio"
                  aria-checked={options[swatches.key] === c}
                  aria-label={t('avatar.builder.variant', { n: i + 1 })}
                  className={cx(
                    'av__swatch pl-focusable',
                    options[swatches.key] === c && 'is-selected',
                  )}
                  style={{ background: c }}
                  onClick={() => setOptions((o) => ({ ...o!, [swatches.key]: c }))}
                />
              ))}
            </div>
          </>
        )}
        {fail && (
          <p role="alert" className="av__error">
            {errorMessage(t, fail)}
          </p>
        )}
      </section>
    </div>
  )
}
