import type { TFunction } from 'i18next'
import type { ApiError } from '../api/client'

/** What a failed API call tells the UI: the error, and how long to wait if throttled. */
export type Failure = { error: ApiError | undefined; retryAfterSeconds?: number }

/** Builds a Failure from an openapi-fetch result. */
export function failure(error: unknown, response: Response | undefined): Failure {
  const body = (error as { error?: ApiError } | undefined)?.error
  const retry = Number(response?.headers.get('Retry-After'))
  return { error: body, retryAfterSeconds: Number.isFinite(retry) && retry > 0 ? retry : undefined }
}

/**
 * The one-line message for a failed request. Every API error `code` maps to
 * `errors.<code>`; unknown codes get `errors.generic`. Keys are spelled out so
 * the i18n extractor sees each of them.
 */
export function errorMessage(t: TFunction, f: Failure): string {
  const minutes = Math.max(1, Math.ceil((f.retryAfterSeconds ?? 60) / 60))
  switch (f.error?.code) {
    case 'invalid_credentials':
      return t('errors.invalid_credentials')
    case 'too_many_attempts':
      return t('errors.too_many_attempts', { minutes })
    case 'profile_locked':
      return t('errors.profile_locked', { minutes })
    case 'invalid_pin':
      return t('errors.invalid_pin')
    case 'weak_password':
      return t('errors.weak_password')
    case 'username_taken':
      return t('errors.username_taken')
    case 'setup_complete':
      return t('errors.setup_complete')
    case 'setup_code_required':
      return t('errors.setup_code_required')
    case 'password_change_required':
      return t('errors.password_change_required')
    case 'pairing_expired':
      return t('errors.pairing_expired')
    case 'pairing_not_found':
      return t('errors.pairing_not_found')
    case 'validation_failed':
      return t('errors.validation_failed')
    case 'unauthorized':
      return t('errors.unauthorized')
    case 'forbidden':
      return t('errors.forbidden')
    case 'csrf':
      return t('errors.csrf')
    case 'last_admin':
      return t('errors.last_admin')
    case 'last_profile':
      return t('errors.last_profile')
    default:
      return t('errors.generic')
  }
}

/** The message under one field, from a `details` entry of the API error. */
export function fieldMessage(t: TFunction, code: string | undefined): string | undefined {
  switch (code) {
    case undefined:
      return undefined
    case 'required':
      return t('fieldErrors.required')
    case 'too_short':
      return t('fieldErrors.too_short')
    case 'too_long':
      return t('fieldErrors.too_long')
    case 'too_common':
      return t('fieldErrors.too_common')
    case 'out_of_range':
      return t('fieldErrors.out_of_range')
    case 'too_many':
      return t('fieldErrors.too_many')
    case 'not_found':
      return t('fieldErrors.not_found')
    case 'mismatch':
      return t('errors.password_mismatch')
    default:
      return t('fieldErrors.invalid')
  }
}

/** Field problems reported by the server, by field name. */
export function fieldCodes(f: Failure | undefined): Record<string, string> {
  const out: Record<string, string> = {}
  for (const d of f?.error?.details ?? []) out[d.field] = d.code
  return out
}
