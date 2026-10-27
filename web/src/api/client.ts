import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

// Absolute base URL: the API is served by the same origin (or proxied by Vite in dev).
// fetch is looked up on each call so tests can stub it.
export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api/v1`,
  fetch: (request) => globalThis.fetch(request),
})

// Marks every request as coming from the web app: the server then answers
// sign-in with the session cookie, and accepts that cookie on state-changing
// requests (CSRF protection, F003-R10).
api.use({
  onRequest({ request }) {
    request.headers.set('X-Plaid-Client', 'web')
    return request
  },
})

export type ApiError = components['schemas']['Error']['error']
export type Me = components['schemas']['Me']
export type Profile = components['schemas']['Profile']
export type ServerInfo = components['schemas']['ServerInfo']
