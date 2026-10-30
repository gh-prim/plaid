import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { Button, Dialog, SegmentedControl } from '../ui'
import { useTheme, type Theme } from './theme'

/**
 * The profile menu (F008-R1, R4, R50): switch profile, appearance, pair a
 * TV, about, sign out. `extra` goes first (the phone's More tab lists the
 * libraries that don't fit the tab bar there).
 */
export function ProfileMenu({
  onClose,
  extra,
  className,
}: {
  onClose: () => void
  extra?: React.ReactNode
  className?: string
}) {
  const { t } = useTranslation()
  const { state, signOut } = useAuth()
  const [theme, setTheme] = useTheme()
  const [about, setAbout] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    const onDown = (e: MouseEvent) => {
      if (!about && ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onDown)
    }
  }, [onClose, about])

  const attributions = state.status === 'ready' ? (state.server.attributions ?? []) : []
  return (
    <div ref={ref} className={`pl pl-menu br-menu ${className ?? ''}`} role="menu">
      {extra}
      <Link to="/profiles" role="menuitem" className="pl-menu__item pl-focusable" onClick={onClose}>
        {t('profiles.switch')}
      </Link>
      <div className="br-menu__group">
        <span className="br-menu__label">{t('nav.appearance.label')}</span>
        <SegmentedControl<Theme>
          label={t('nav.appearance.label')}
          value={theme}
          onChange={setTheme}
          options={[
            { value: 'system', label: t('nav.appearance.system') },
            { value: 'dark', label: t('nav.appearance.dark') },
            { value: 'light', label: t('nav.appearance.light') },
          ]}
        />
      </div>
      <Link to="/pair" role="menuitem" className="pl-menu__item pl-focusable" onClick={onClose}>
        {t('pairing.title')}
      </Link>
      <button
        type="button"
        role="menuitem"
        className="pl-menu__item pl-focusable br-menu__button"
        onClick={() => setAbout(true)}
      >
        {t('nav.about')}
      </button>
      <div className="pl-menu__sep" />
      <button
        type="button"
        role="menuitem"
        className="pl-menu__item pl-focusable br-menu__button"
        onClick={() => void signOut()}
      >
        {t('auth.signOut')}
      </button>
      {about && (
        <Dialog
          title={t('nav.about')}
          actions={
            <Button variant="secondary" onClick={() => setAbout(false)}>
              {t('common.close')}
            </Button>
          }
        >
          {state.status === 'ready' && (
            <p>{t('welcome.version', { version: state.server.version })}</p>
          )}
          {attributions.length > 0 && (
            <h3 className="br-menu__label">{t('metadata.attributions')}</h3>
          )}
          {attributions.map((a) => (
            <p key={a.provider}>
              <a href={a.url} rel="noreferrer" target="_blank">
                {a.name}
              </a>{' '}
              · {a.notice}
            </p>
          ))}
        </Dialog>
      )}
    </div>
  )
}
