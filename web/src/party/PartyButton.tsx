import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { Icon } from '../ui'
import { LaunchSheet } from './LaunchSheet'

/** "Plaid Party" on a movie or an episode page (Party04). */
export function PartyButton({ itemId, title }: { itemId: string; title: string }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <>
      <button
        type="button"
        className="br-hero__btn br-glass pl-focusable"
        onClick={() => setOpen(true)}
      >
        <Icon name="party" size={18} />
        {t('party.title')}
      </button>
      {open && <LaunchSheet itemId={itemId} title={title} onClose={() => setOpen(false)} />}
    </>
  )
}

/** "Follow" on a show page: its releases go to Upcoming (F010-R35). */
export function FollowButton({ showId }: { showId: string }) {
  const { t } = useTranslation()
  const [state, setState] = useState<{ following: boolean; explicit: boolean } | null>(null)
  useEffect(() => {
    void api
      .GET('/shows/{showId}/follow', { params: { path: { showId } } })
      .then(({ data }) => data && setState(data))
      .catch(() => {})
  }, [showId])
  if (!state) return null
  async function toggle() {
    const call = state!.explicit
      ? api.DELETE('/shows/{showId}/follow', { params: { path: { showId } } })
      : api.PUT('/shows/{showId}/follow', { params: { path: { showId } } })
    const { data } = await call.catch(() => ({ data: undefined }))
    if (data) setState(data)
  }
  return (
    <button
      type="button"
      className="br-hero__btn br-glass pl-focusable"
      aria-pressed={state.explicit}
      onClick={() => void toggle()}
    >
      <Icon name={state.following ? 'check' : 'bell'} size={18} />
      {state.following ? t('upcoming.following') : t('upcoming.follow')}
    </button>
  )
}
