import { useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Profile } from '../api/client'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { Button, cx, Icon } from '../ui'
import { AvatarBuilder, avatarColors } from './AvatarBuilder'
import { PhotoCrop } from './PhotoCrop'
import './avatar.css'

/**
 * "Your face in PLAID" (board Invite15): build an avatar, use a photo, or
 * keep the initial on a color. Used by the invitation and by Change the
 * picture.
 */
export function AvatarChoice({
  profile,
  onDone,
  doneLabel,
}: {
  profile: Profile
  onDone: (p: Profile) => void
  doneLabel: string
}) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'choose' | 'builder' | 'photo'>('choose')
  const [color, setColor] = useState(profile.avatarColor)
  const [saving, setSaving] = useState(false)
  const [fail, setFail] = useState<Failure>()

  if (mode === 'builder')
    return <AvatarBuilder profile={profile} onDone={onDone} onCancel={() => setMode('choose')} />
  if (mode === 'photo')
    return <PhotoCrop profile={profile} onDone={onDone} onCancel={() => setMode('choose')} />

  async function keepInitial() {
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
      body: { kind: 'initial' },
    })
    setSaving(false)
    if (error) return setFail(failure(error, response))
    onDone(data)
  }

  const initial = profile.name.trim().charAt(0).toLocaleUpperCase()
  return (
    <div className="av av--choose">
      <h1 className="av__title av__title--big">{t('avatar.title')}</h1>
      <p className="av__lead">{t('avatar.lead')}</p>
      <ul className="av__choices">
        <li>
          <button
            type="button"
            className="av__choice pl-focusable"
            onClick={() => setMode('builder')}
          >
            <span className="av__choice-art av__choice-art--build" aria-hidden="true" />
            <span>
              <strong>{t('avatar.build')}</strong>
              <small>{t('avatar.buildHelp')}</small>
            </span>
            <Icon name="chevronRight" size={18} />
          </button>
        </li>
        <li>
          <button
            type="button"
            className="av__choice pl-focusable"
            onClick={() => setMode('photo')}
          >
            <span className="av__choice-art av__choice-art--photo" aria-hidden="true">
              <Icon name="users" size={22} />
            </span>
            <span>
              <strong>{t('avatar.photo')}</strong>
              <small>{t('avatar.photoHelp')}</small>
            </span>
            <Icon name="chevronRight" size={18} />
          </button>
        </li>
        <li className="av__choice av__choice--static">
          <span
            className="av__choice-art"
            style={{ background: `var(--party-${color + 1})` } as CSSProperties}
            aria-hidden="true"
          >
            {initial}
          </span>
          <span>
            <strong>{t('avatar.initial')}</strong>
            <small>{t('avatar.initialHelp')}</small>
          </span>
        </li>
      </ul>
      <div className="av__swatches" role="radiogroup" aria-label={t('avatar.builder.background')}>
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
      {fail && (
        <p role="alert" className="av__error">
          {errorMessage(t, fail)}
        </p>
      )}
      <Button block loading={saving} onClick={() => void keepInitial()}>
        {doneLabel}
      </Button>
    </div>
  )
}
