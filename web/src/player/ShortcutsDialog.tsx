import { useEffect, useId, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Icon } from '../ui'
import { shortcutList } from './shortcuts'

/** "?" in the player: every shortcut of F008-R42. */
export function ShortcutsDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const id = useId()
  const close = useRef<HTMLButtonElement>(null)
  useEffect(() => close.current?.focus(), [])
  const labels: Record<(typeof shortcutList)[number]['label'], string> = {
    playPause: t('player.key.playPause'),
    seek: t('player.key.seek'),
    volume: t('player.key.volume'),
    mute: t('player.key.mute'),
    fullscreen: t('player.key.fullscreen'),
    subtitles: t('player.key.subtitles'),
    next: t('player.key.next'),
    jump: t('player.key.jump'),
    startEnd: t('player.key.startEnd'),
    close: t('player.key.close'),
  }
  return (
    <div className="pv-center pv-dim" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div
        className="pv-keys"
        role="dialog"
        aria-modal="true"
        aria-labelledby={id}
        onKeyDown={(e) => {
          if (e.key === 'Escape' || e.key === '?') {
            e.stopPropagation()
            onClose()
          }
        }}
      >
        <div className="pv-keys__head">
          <h2 id={id}>{t('player.shortcuts.label')}</h2>
          <button
            ref={close}
            type="button"
            className="pl-pbtn pl-focusable"
            aria-label={t('common.close')}
            onClick={onClose}
          >
            <Icon name="x" size={20} />
          </button>
        </div>
        <ul className="pv-keys__list">
          {shortcutList.map((s) => (
            <li key={s.label}>
              <span className="pv-keys__keys">
                {s.keys.map((k) => (
                  <kbd key={k}>{k}</kbd>
                ))}
              </span>
              <span>{labels[s.label]}</span>
            </li>
          ))}
        </ul>
        <p className="pv-keys__hint">{t('player.shortcuts.hint')}</p>
      </div>
    </div>
  )
}
