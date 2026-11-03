import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { GlassSheet } from '../browse/GlassSheet'
import { Button } from '../ui'

/** "Join a party" with the code a friend sent (Party07). */
export function JoinCode({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [code, setCode] = useState('')
  const clean = code.toUpperCase().replace(/[^23456789ABCDEFGHJKMNPQRSTUVWXYZ]/g, '')
  function submit(e: FormEvent) {
    e.preventDefault()
    if (clean.length === 6) void navigate(`/p/${clean}`)
  }
  return (
    <GlassSheet title={t('party.join.title')} subtitle={t('party.join.hint')} onClose={onClose}>
      <form className="pv-party-stack" onSubmit={submit}>
        <input
          className="pl-field__input pv-party-code"
          value={code}
          inputMode="text"
          autoCapitalize="characters"
          autoComplete="off"
          maxLength={7}
          placeholder="K7P-4QX"
          aria-label={t('party.join.codeLabel')}
          onChange={(e) => setCode(e.target.value)}
        />
        <Button variant="cream" type="submit" block disabled={clean.length !== 6}>
          {t('party.join.cta')}
        </Button>
      </form>
    </GlassSheet>
  )
}
