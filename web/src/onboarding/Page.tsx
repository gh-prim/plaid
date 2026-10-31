import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../auth/AuthProvider'
import { Wordmark } from '../browse/Wordmark'
import { cx, Icon, Spinner } from '../ui'
import { useArrived } from './transition'
import '../browse/browse.css'
import './onboarding.css'

/** The server's name and version, as the onboarding shows them. */
export function useServer() {
  const { state } = useAuth()
  const server = state.status === 'ready' ? state.server : undefined
  return { serverName: server?.name ?? 'PLAID', version: server?.version ?? '' }
}

/** The onboarding has four steps: account, libraries, scan, end (V2). */
export const totalSteps = 4

/**
 * One onboarding or sign-in screen (V2, boards TO01–TO07, TL01–TL04): the
 * wordmark on top, Back on the left, the step dots on the right, then the
 * screen, usually a glass card over a warm light.
 */
export function Page({
  step,
  onBack,
  className,
  behind,
  children,
}: {
  /** 1 to 4: lights the dots. */
  step?: number
  onBack?: () => void
  className?: string
  /** What sits behind the screen: the poster wall during the scan. */
  behind?: ReactNode
  children: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <div className={cx('pl ob', className)}>
      {behind}
      <header className="ob__top">
        {onBack && (
          <button
            type="button"
            className="ob__back ob-glassbtn pl-focusable"
            aria-label={t('common.back')}
            onClick={onBack}
          >
            <Icon name="chevronLeft" size={20} />
          </button>
        )}
        <Wordmark />
        {step && <Dots step={step} />}
      </header>
      <main className="ob__main">{children}</main>
    </div>
  )
}

/** The step dots: the active one stretched into a bar, those done lit. */
function Dots({ step }: { step: number }) {
  return (
    <span className="ob__dots" aria-hidden="true">
      {Array.from({ length: totalSteps }, (_, i) => (
        <span
          key={i}
          className={cx(
            'ob__dot',
            `ob__dot--${i + 1}`,
            i + 1 <= step && 'is-done',
            i + 1 === step && 'is-active',
          )}
        />
      ))}
    </span>
  )
}

/**
 * The glass card: the app icon (start, sign-in), a kicker, the title, the
 * line under it, then the step's content. `id` names the content, so a new
 * step or phase slides in (TM25).
 */
export function Card({
  id,
  icon,
  kicker,
  title,
  body,
  wide,
  children,
}: {
  id: string
  icon?: boolean
  kicker?: string
  title: string
  body?: ReactNode
  wide?: boolean
  children?: ReactNode
}) {
  useArrived(id)
  return (
    <section className={cx('ob__card', wide && 'ob__card--wide')}>
      <div className="ob__content" key={id}>
        {icon && <img className="ob__icon" src="/icon.svg" alt="" width={96} height={96} />}
        {kicker && <p className="ob__kicker">{kicker}</p>}
        <h1 className="ob__title">{title}</h1>
        {body && <p className="ob__body">{body}</p>}
        {children}
      </div>
    </section>
  )
}

/** "Step 2 of 4", the kicker above a step's title. */
export function useStepLabel() {
  const { t } = useTranslation()
  return (step: number) => t('onboarding.step', { step, total: totalSteps })
}

/** A card that waits for the server. */
export function LoadingCard() {
  return (
    <section className="ob__card ob__card--loading">
      <Spinner />
    </section>
  )
}
