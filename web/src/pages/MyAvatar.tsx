import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { AvatarChoice } from '../avatar/AvatarChoice'
import { LoadingCard, Page } from '../onboarding/Page'

/** Change the picture of the active profile (F009), from the profile menu. */
export function MyAvatar() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { state, refresh } = useAuth()
  const me = state.status === 'ready' ? state.me : null
  const profile = me?.activeProfile ?? me?.profiles[0]
  return (
    <Page onBack={() => void navigate(-1)}>
      {profile ? (
        <section className="ob__card ob__card--wide">
          <AvatarChoice
            profile={profile}
            doneLabel={t('common.done')}
            onDone={() => void refresh().then(() => navigate(-1))}
          />
        </section>
      ) : (
        <LoadingCard />
      )}
    </Page>
  )
}
