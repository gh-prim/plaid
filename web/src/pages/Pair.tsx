import { useState } from 'react'
import type { FormEvent } from 'react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { Field, FormError } from '../components/Field'
import { PinInput } from '../components/PinInput'

/** Signs a TV in with the code it shows (F003-R20). */
export function Pair() {
  const { t } = useTranslation()
  const { state } = useAuth()
  const selectId = useId()
  const me = state.status === 'ready' ? state.me : null
  const profiles = me?.profiles ?? []
  const [code, setCode] = useState('')
  const [profileId, setProfileId] = useState(me?.activeProfile?.id ?? profiles[0]?.id ?? '')
  const [pin, setPin] = useState('')
  const [busy, setBusy] = useState(false)
  const [fail, setFail] = useState<Failure>()
  const [paired, setPaired] = useState<string>()

  const profile = profiles.find((p) => p.id === profileId)
  const normalized = code.replace(/[\s-]/g, '').toUpperCase()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setFail(undefined)
    const { data, error, response } = await api.POST('/pairing/claim', {
      body: { code: normalized, profileId, pin: profile?.hasPin ? pin : undefined },
    })
    setBusy(false)
    if (error) {
      setFail(failure(error, response))
      return
    }
    setPaired(data.deviceName)
    setCode('')
    setPin('')
  }

  return (
    <main className="page">
      <h1>{t('pairing.title')}</h1>
      {paired && (
        <p role="status" className="form-success">
          {t('pairing.done', { device: paired })}
        </p>
      )}
      <form onSubmit={(e) => void submit(e)}>
        <FormError message={fail && errorMessage(t, fail)} />
        <Field
          label={t('pairing.codeLabel')}
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          autoCapitalize="characters"
          autoComplete="off"
          spellCheck={false}
          maxLength={7}
          placeholder="K7P-4QX"
          required
          autoFocus
        />
        <div className="field">
          <label htmlFor={selectId}>{t('pairing.startProfile')}</label>
          <select id={selectId} value={profileId} onChange={(e) => setProfileId(e.target.value)}>
            {profiles.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </div>
        {profile?.hasPin && (
          <PinInput
            label={t('profiles.enterPin', { name: profile.name })}
            value={pin}
            onChange={setPin}
          />
        )}
        <button type="submit" disabled={busy || normalized.length !== 6} aria-busy={busy}>
          {t('pairing.confirm')}
        </button>
      </form>
      <Link to="/">{t('common.back')}</Link>
    </main>
  )
}
