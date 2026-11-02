import { useEffect, useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { api, type Profile } from '../api/client'
import { serverOrigin } from '../api/server'
import type { components } from '../api/schema'
import { useAuth } from '../auth/AuthProvider'
import { markProfilePicked } from '../auth/profileSession'
import { errorMessage, failure, fieldCodes, fieldMessage, type Failure } from '../auth/errors'
import { AvatarChoice } from '../avatar/AvatarChoice'
import { checkAccount } from '../onboarding/Account'
import { FieldGroup, FieldNote, InsetField } from '../onboarding/Fields'
import { Card, LoadingCard, Page } from '../onboarding/Page'
import { slide } from '../onboarding/transition'
import { Button } from '../ui'

type Preview = components['schemas']['InvitationPreview']

/** iPhone and iPad, where the PLAID app can take the invitation over. */
function appleDevice(): boolean {
  return (
    /iPhone|iPad/.test(navigator.userAgent) ||
    (/Macintosh/.test(navigator.userAgent) && navigator.maxTouchPoints > 1)
  )
}

/** The app's link for this invitation: it creates the account natively. */
function appLink(token: string): string {
  const q = new URLSearchParams({ server: serverOrigin(), token })
  return `plaid://invite?${q}`
}
type Phase = 'open' | 'account' | 'pin' | 'confirm' | 'avatar' | 'welcome'

/**
 * An invitation link, by link only (canvas step 20, boards Invite01–Invite16):
 * who invites you, your account, your code (required), your face, welcome.
 */
export function Invite() {
  const { t, i18n } = useTranslation()
  const { token = '' } = useParams()
  const navigate = useNavigate()
  const { refresh, device } = useAuth()
  const ids = useId()
  const [preview, setPreview] = useState<Preview>()
  const [expired, setExpired] = useState(false)
  const [phase, setPhase] = useState<Phase>('open')
  const [form, setForm] = useState({ displayName: '', username: '', password: '' })
  const [pin, setPin] = useState('')
  const [confirm, setConfirm] = useState('')
  const [checked, setChecked] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  const [mismatch, setMismatch] = useState(false)
  const [profile, setProfile] = useState<Profile>()

  useEffect(() => {
    void api.POST('/invite/check', { body: { token } }).then(({ data }) => {
      if (!data) return setExpired(true)
      setPreview(data)
      setForm((f) => ({ ...f, displayName: data.name }))
    })
  }, [token])

  const go = (p: Phase, dir?: 'back') => slide(() => setPhase(p), dir)

  if (expired)
    return (
      <Page>
        <Card id="expired" icon title={t('invite.expired.title')} body={t('invite.expired.body')}>
          <Button block className="ob__cta" onClick={() => void navigate('/login')}>
            {t('invite.haveAccount')}
          </Button>
        </Card>
      </Page>
    )
  if (!preview)
    return (
      <Page>
        <LoadingCard />
      </Page>
    )

  const serverName = preview.serverName
  const local = checked ? checkAccount({ ...form, setupCode: '' }) : {}
  const server = fieldCodes(fail)
  const code = fail?.error?.code
  const labels = {
    username: t('onboarding.account.username.label'),
    displayName: t('onboarding.account.displayName.label'),
    password: t('onboarding.account.password.label'),
  }
  const field = (key: keyof typeof labels) => {
    const message = fieldMessage(t, local[key] ?? server[key])
    return message && t('onboarding.field.error', { field: labels[key], error: message })
  }
  const usernameError = code === 'username_taken' ? t('errors.username_taken') : field('username')
  const passwordError = code === 'weak_password' ? t('errors.weak_password') : field('password')

  async function accept() {
    setBusy(true)
    setFail(undefined)
    const { data, error, response } = await api.POST('/invite/accept', {
      body: {
        token,
        displayName: form.displayName.trim(),
        username: form.username.toLowerCase(),
        password: form.password,
        pin,
        locale: i18n.resolvedLanguage,
        device,
      },
    })
    setBusy(false)
    if (error) {
      const f = failure(error, response)
      setFail(f)
      if (f.error?.code === 'invitation_expired' || f.error?.code === 'invitation_not_found')
        return setExpired(true)
      // The account's fields were refused: back to them.
      return go('account', 'back')
    }
    markProfilePicked()
    setProfile(data.profiles[0])
    await refresh()
    go('avatar')
  }

  if (phase === 'open')
    return (
      <Page>
        <Card
          id="open"
          icon
          title={t('invite.title', { invitedBy: preview.invitedBy || serverName, serverName })}
          body={t('invite.lead')}
        >
          <Button block className="ob__cta" onClick={() => go('account')}>
            {t('invite.start')}
          </Button>
          {appleDevice() && (
            <a className="ob__foot ob__link" href={appLink(token)}>
              {t('invite.openApp')}
            </a>
          )}
        </Card>
      </Page>
    )

  if (phase === 'account')
    return (
      <Page onBack={() => go('open', 'back')}>
        <Card
          id="account"
          title={t('invite.account.title')}
          body={t('invite.account.lead', { serverName })}
        >
          <form
            className="ob__form"
            noValidate
            onSubmit={(e: FormEvent) => {
              e.preventDefault()
              setChecked(true)
              if (Object.keys(checkAccount({ ...form, setupCode: '' })).length) return
              go('pin')
            }}
          >
            <FieldGroup
              notes={
                <>
                  {fail &&
                    !['username_taken', 'weak_password', 'validation_failed'].includes(
                      code ?? '',
                    ) && (
                      <FieldNote error alert>
                        {errorMessage(t, fail)}
                      </FieldNote>
                    )}
                  {usernameError && <FieldNote error>{usernameError}</FieldNote>}
                  {field('displayName') && <FieldNote error>{field('displayName')}</FieldNote>}
                  <FieldNote id={`${ids}-password`} error={!!passwordError}>
                    {passwordError ?? t('onboarding.account.password.help')}
                  </FieldNote>
                </>
              }
            >
              <InsetField
                label={labels.displayName}
                value={form.displayName}
                onValue={(v) => setForm((f) => ({ ...f, displayName: v }))}
                invalid={!!field('displayName')}
                autoComplete="name"
              />
              <InsetField
                label={labels.username}
                value={form.username}
                onValue={(v) => setForm((f) => ({ ...f, username: v }))}
                invalid={!!usernameError}
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                autoFocus
              />
              <InsetField
                kind="password"
                label={labels.password}
                value={form.password}
                onValue={(v) => setForm((f) => ({ ...f, password: v }))}
                invalid={!!passwordError}
                describedBy={`${ids}-password`}
                autoComplete="new-password"
              />
            </FieldGroup>
            <Button type="submit" block className="ob__cta">
              {t('invite.next')}
            </Button>
          </form>
        </Card>
      </Page>
    )

  if (phase === 'pin' || phase === 'confirm') {
    const confirming = phase === 'confirm'
    const value = confirming ? confirm : pin
    const setValue = confirming ? setConfirm : setPin
    return (
      <Page onBack={() => go(confirming ? 'pin' : 'account', 'back')}>
        <Card
          id={phase}
          title={confirming ? t('invite.pin.confirm') : t('invite.pin.title')}
          body={t('invite.pin.lead')}
        >
          <form
            className="ob__form"
            noValidate
            onSubmit={(e) => {
              e.preventDefault()
              if (!/^\d{4}$/.test(value)) return
              if (!confirming) {
                setMismatch(false)
                setConfirm('')
                return go('confirm')
              }
              if (confirm !== pin) {
                setMismatch(true)
                setPin('')
                return go('pin', 'back')
              }
              void accept()
            }}
          >
            <FieldGroup
              notes={
                mismatch &&
                !confirming && (
                  <FieldNote error alert>
                    {t('invite.pin.mismatch')}
                  </FieldNote>
                )
              }
            >
              <InsetField
                kind="code"
                label={t('invite.pin.label')}
                value={value}
                onValue={(v) => setValue(v.replace(/\D/g, '').slice(0, 4))}
                inputMode="numeric"
                autoComplete="off"
                autoFocus
              />
            </FieldGroup>
            <Button
              type="submit"
              block
              className="ob__cta"
              loading={busy}
              disabled={!/^\d{4}$/.test(value) || busy}
            >
              {t('invite.next')}
            </Button>
          </form>
        </Card>
      </Page>
    )
  }

  if (phase === 'avatar' && profile)
    return (
      <Page>
        <section className="ob__card ob__card--wide">
          <AvatarChoice
            profile={profile}
            doneLabel={t('invite.next')}
            onDone={(p) => {
              setProfile(p)
              // The top bar shows the new picture from the session's profile.
              void refresh()
              go('welcome')
            }}
          />
        </section>
      </Page>
    )

  return (
    <Page>
      <Card
        id="welcome"
        title={t('invite.welcome.title', { name: form.displayName.trim() })}
        body={t('invite.welcome.lead', { serverName })}
      >
        <Button block className="ob__cta" onClick={() => void navigate('/', { replace: true })}>
          {t('invite.welcome.go')}
        </Button>
      </Card>
    </Page>
  )
}
