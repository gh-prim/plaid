import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { Welcome } from './Welcome'

/** The signed-in home until the browsing screens (F008) exist. */
export function Home() {
  const { t } = useTranslation()
  const { state, signOut } = useAuth()
  const profile = state.status === 'ready' ? state.me?.activeProfile : undefined
  return (
    <>
      <nav className="account-bar" aria-label={t('account.label')}>
        {profile && <span>{t('account.watchingAs', { name: profile.name })}</span>}
        <Link to="/profiles">{t('profiles.switch')}</Link>
        <Link to="/pair">{t('pairing.title')}</Link>
        <button type="button" className="link-button" onClick={() => void signOut()}>
          {t('auth.signOut')}
        </button>
      </nav>
      <Welcome />
    </>
  )
}
