import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { api, type Profile } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { markProfilePicked } from '../auth/profileSession'
import { formatNumber } from '../lib/format'
import { Button, Dialog, Icon, SegmentedControl, Spinner } from '../ui'
import { loadAutoplay, rememberAutoplay } from '../player/trackMemory'
import { setPreviewsOn, usePreviewsOn } from './previewPref'
import { useTheme, type Theme } from './theme'

/** How many other profiles fit next to "Manage" before it says the rest. */
const shown = 4

/** A profile's circle: its party color with a soft top light, its initial. */
function Face({ profile, size }: { profile: Profile; size: number }) {
  return (
    <span
      className="br-pmenu__face"
      style={
        {
          '--pf-s': `${size}px`,
          '--pc': `var(--party-${profile.avatarColor + 1})`,
        } as React.CSSProperties
      }
      aria-hidden="true"
    >
      {profile.name.trim().charAt(0).toLocaleUpperCase()}
    </span>
  )
}

/**
 * The profile menu (F008-R1, R4, R50; V2 Step 12): who's watching, with
 * the role and the server; the other profiles one tap away (a PIN goes
 * through the picker); admin with what needs you, pair a TV; appearance
 * and autoplay; about, sign out. `extra` goes first (the phone's More tab
 * lists the libraries that don't fit the tab bar there).
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
  const { t, i18n } = useTranslation()
  const { state, refresh, signOut } = useAuth()
  const navigate = useNavigate()
  const [theme, setTheme] = useTheme()
  const [about, setAbout] = useState(false)
  const [switching, setSwitching] = useState<string>()
  const [problems, setProblems] = useState(0)
  const me = state.status === 'ready' ? state.me : undefined
  const profile = me?.activeProfile
  const profileId = profile?.id ?? ''
  const admin = me?.user.role === 'admin'
  const others = (me?.profiles ?? []).filter((p) => p.id !== profileId)
  const serverName = state.status === 'ready' ? state.server.name : ''
  const [autoplay, setAutoplay] = useState(() => loadAutoplay(profileId))
  const previews = usePreviewsOn(profileId)
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

  // The admin row counts the open problems, like the dashboard's "Needs you".
  useEffect(() => {
    if (!admin) return
    let live = true
    void api.GET('/problems').then(({ data }) => {
      if (live) setProblems((data?.problems ?? []).filter((p) => !p.resolvedAt).length)
    })
    return () => {
      live = false
    }
  }, [admin])

  async function switchTo(p: Profile) {
    if (p.hasPin) {
      onClose()
      void navigate('/profiles')
      return
    }
    setSwitching(p.id)
    const { error } = await api.POST('/session/profile', { body: { profileId: p.id } })
    if (error) {
      setSwitching(undefined)
      onClose()
      void navigate('/profiles')
      return
    }
    markProfilePicked()
    await refresh()
    onClose()
  }

  const role = admin ? t('people.role.admin') : t('people.role.user')
  const attributions = state.status === 'ready' ? (state.server.attributions ?? []) : []
  return (
    <div
      ref={ref}
      className={`pl pl-menu br-menu br-pmenu ${className ?? ''}`}
      role="menu"
      aria-label={t('nav.profileMenu', { name: profile?.name ?? '' })}
    >
      {extra}
      {profile && (
        <div className="br-pmenu__me">
          <Face profile={profile} size={52} />
          <span className="br-pmenu__who">
            <strong>{profile.name}</strong>
            <span>{serverName ? `${role} · ${serverName}` : role}</span>
          </span>
        </div>
      )}
      {profile && (
        <div className="br-pmenu__others">
          {others.slice(0, shown).map((p) => (
            <button
              key={p.id}
              type="button"
              role="menuitem"
              className="br-pmenu__other pl-focusable"
              disabled={!!switching}
              aria-label={t('profiles.switchTo', { name: p.name })}
              onClick={() => void switchTo(p)}
            >
              <span className="br-pmenu__circle">
                <Face profile={p} size={40} />
                {switching === p.id && (
                  <span className="br-pmenu__busy">
                    <Spinner size={18} />
                  </span>
                )}
                {p.hasPin && (
                  <span className="br-pmenu__lock">
                    <Icon name="lock" size={9} stroke={3} />
                  </span>
                )}
              </span>
              <span className="br-pmenu__name">{p.name}</span>
            </button>
          ))}
          <Link
            to="/profiles"
            role="menuitem"
            className="br-pmenu__other pl-focusable"
            onClick={onClose}
          >
            <span className="br-pmenu__circle br-pmenu__circle--more">
              <Icon name="more" size={20} />
            </span>
            <span className="br-pmenu__name">{t('profiles.manage')}</span>
          </Link>
        </div>
      )}
      <div className="br-pmenu__sep" />
      {admin && (
        <Link to="/admin" role="menuitem" className="br-pmenu__row pl-focusable" onClick={onClose}>
          <Icon name="activity" size={20} />
          <span>{t('nav.admin')}</span>
          {problems > 0 && (
            <b className="br-pmenu__badge">{formatNumber(i18n.language, problems)}</b>
          )}
          <Icon name="chevronRight" size={14} />
        </Link>
      )}
      <Link to="/pair" role="menuitem" className="br-pmenu__row pl-focusable" onClick={onClose}>
        <Icon name="tv" size={20} />
        <span>{t('pairing.title')}</span>
        <Icon name="chevronRight" size={14} />
      </Link>
      <div className="br-pmenu__sep" />
      <div className="br-pmenu__group">
        <span className="br-pmenu__label">{t('nav.appearance.label')}</span>
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
      <label className={`br-pmenu__switch pl-focusable-within ${autoplay ? 'is-on' : ''}`}>
        <Icon name="playNext" size={20} />
        <span>{t('player.next.auto')}</span>
        <input
          type="checkbox"
          role="switch"
          checked={autoplay}
          onChange={(e) => {
            setAutoplay(e.target.checked)
            rememberAutoplay(profileId, e.target.checked)
          }}
        />
        <span className="br-pmenu__track" aria-hidden="true" />
      </label>
      <label className={`br-pmenu__switch pl-focusable-within ${previews ? 'is-on' : ''}`}>
        <Icon name="tv" size={20} />
        <span>{t('item.preview.autoplay')}</span>
        <input
          type="checkbox"
          role="switch"
          checked={previews}
          onChange={(e) => setPreviewsOn(profileId, e.target.checked)}
        />
        <span className="br-pmenu__track" aria-hidden="true" />
      </label>
      <div className="br-pmenu__sep" />
      <button
        type="button"
        role="menuitem"
        className="br-pmenu__row br-menu__button pl-focusable"
        onClick={() => setAbout(true)}
      >
        <Icon name="info" size={20} />
        <span>{t('nav.about')}</span>
      </button>
      <button
        type="button"
        role="menuitem"
        className="br-pmenu__row br-pmenu__row--out br-menu__button pl-focusable"
        onClick={() => void signOut()}
      >
        <Icon name="signOut" size={20} />
        <span>{t('auth.signOut')}</span>
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
