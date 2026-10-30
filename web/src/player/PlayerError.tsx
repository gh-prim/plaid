import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { formatClock } from '../lib/format'
import { Icon } from '../ui'

export type PlayerProblem =
  | { kind: 'api'; code: string; retryAfter?: number }
  | { kind: 'cantPlay' }
  | { kind: 'stopped'; positionMs: number }

/**
 * What went wrong and what to do (F008-R39, R45): the fix in amber, Back.
 * The last frame stays behind the veil.
 */
export function PlayerError({
  problem,
  isAdmin,
  canLower,
  onBack,
  onRetry,
  onLower,
  onCopyLink,
}: {
  problem: PlayerProblem
  isAdmin: boolean
  canLower: boolean
  onBack: () => void
  onRetry: () => void
  onLower: () => void
  onCopyLink: () => Promise<boolean>
}) {
  const { t } = useTranslation()
  const wait = problem.kind === 'api' ? (problem.retryAfter ?? 0) : 0
  const [left, setLeft] = useState(wait)
  const [copied, setCopied] = useState(false)
  const first = useRef<HTMLButtonElement>(null)
  useEffect(() => first.current?.focus(), [])
  useEffect(() => {
    if (wait <= 0) return
    const start = Date.now()
    const id = setInterval(() => {
      const rest = Math.max(0, wait - Math.floor((Date.now() - start) / 1000))
      setLeft(rest)
      if (rest === 0) clearInterval(id)
    }, 1000)
    return () => clearInterval(id)
  }, [wait])

  let title: string
  let body: string | undefined
  const actions: { label: string; onClick: () => void; primary?: boolean; disabled?: boolean }[] =
    []
  const retry = (primary = true) =>
    actions.push({
      label: left > 0 ? t('player.retryIn', { seconds: left }) : t('player.tryAgain'),
      onClick: onRetry,
      primary,
      disabled: left > 0,
    })

  if (problem.kind === 'stopped') {
    title = t('player.stopped.title')
    body = t('player.stopped.body')
    actions.push({
      label: t('player.stopped.restart', { position: formatClock(problem.positionMs) }),
      onClick: onRetry,
      primary: true,
    })
  } else if (problem.kind === 'cantPlay') {
    title = t('player.cantPlay.title')
    body = t('player.cantPlay.body')
    actions.push({
      label: copied ? t('player.copied') : t('player.copyVlcLink'),
      onClick: () => void onCopyLink().then(setCopied),
      primary: true,
    })
  } else {
    title = problemTitle(t, problem.code)
    switch (problem.code) {
      case 'transcode_limit_reached':
        retry()
        if (canLower) actions.push({ label: t('player.lowerQuality'), onClick: onLower })
        break
      case 'ffmpeg_missing':
        body = isAdmin ? t('player.error.ffmpegAdmin') : t('player.error.ffmpegMember')
        if (isAdmin) retry()
        break
      case 'not_playable':
      case 'transcode_storage_full':
        break
      default:
        retry()
    }
  }
  actions.push({ label: t('player.back'), onClick: onBack })

  return (
    <div className="pl-pmsg pv-msg" role="alert">
      <Icon name="error" size={40} className="pl-pmsg__err" />
      <h2 className="pl-pmsg__title">{title}</h2>
      {body && <p className="pl-pmsg__text">{body}</p>}
      <div className="pl-pmsg__actions">
        {actions.map((a, i) => (
          <button
            key={a.label}
            ref={i === 0 ? first : undefined}
            type="button"
            className={`pl-btn ${a.primary ? 'pl-btn--primary' : 'pl-btn--secondary'} pl-focusable`}
            aria-disabled={a.disabled || undefined}
            onClick={() => !a.disabled && a.onClick()}
          >
            {a.label}
          </button>
        ))}
      </div>
    </div>
  )
}

function problemTitle(t: (k: string) => string, code: string): string {
  switch (code) {
    case 'transcode_limit_reached':
      return t('errors.transcode_limit_reached')
    case 'transcode_storage_full':
      return t('errors.transcode_storage_full')
    case 'file_unavailable':
      return t('errors.file_unavailable')
    case 'ffmpeg_missing':
      return t('errors.ffmpeg_missing')
    case 'file_not_analyzed':
      return t('errors.file_not_analyzed')
    case 'not_playable':
      return t('errors.not_playable')
    case 'profile_required':
      return t('errors.profile_required')
    case 'not_found':
      return t('player.error.notFound')
  }
  return t('errors.generic')
}
