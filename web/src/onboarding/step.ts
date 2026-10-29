import type { components } from '../api/schema'

type Library = components['schemas']['Library']

/** Where the wizard stands (F007 R1, R2, R10). */
export type OnboardingStep = 'start' | 'account' | 'libraries' | 'scan' | 'done'

/** Remembered on this browser only: the admin chose "Add libraries later". */
export const laterKey = 'plaid.onboarding.later'
/** Remembered on this browser only: the admin left the end of the first scan. */
export const doneKey = 'plaid.onboarding.done'

function flag(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

export function setFlag(key: string) {
  try {
    localStorage.setItem(key, '1')
  } catch {
    // Private mode: the wizard then derives the step from the server alone.
  }
}

/**
 * The step a signed-in admin is on, derived from the server so leaving
 * and coming back lands in the right place (R10): no library yet means
 * the libraries step, unless they chose "later" on this browser; a
 * library that never finished a scan means the scan, until they leave its
 * end screen. Members never see the wizard.
 */
export function adminStep(libraries: Library[], isAdmin: boolean): 'libraries' | 'scan' | 'done' {
  if (!isAdmin || flag(doneKey)) return 'done'
  if (libraries.length === 0) return flag(laterKey) ? 'done' : 'libraries'
  if (libraries.some((l) => !l.lastScanAt)) return 'scan'
  return 'done'
}
