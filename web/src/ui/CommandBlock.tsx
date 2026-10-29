// Ported from the PLAID design system: a shell command the admin copies
// to their server. Copied lasts 2 s and is announced.
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cx } from './cx'
import { Icon } from './Icon'

export const copiedFor = 2000

export function CommandBlock({ command }: { command: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), copiedFor)
    return () => clearTimeout(timer)
  }, [copied])
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command)
      setCopied(true)
    } catch {
      // No clipboard (plain HTTP on another host): the command stays selectable.
    }
  }
  return (
    <div className="pl pl-cmd">
      <code className="pl-cmd__code">{command}</code>
      <button
        type="button"
        className={cx('pl-cmd__btn pl-focusable', copied && 'is-copied')}
        onClick={() => void copy()}
      >
        {copied && <Icon name="check" size={16} />}
        {copied ? t('common.copied') : t('common.copy')}
      </button>
      <span className="pl-sr" aria-live="polite">
        {copied ? t('common.copied') : ''}
      </span>
    </div>
  )
}
