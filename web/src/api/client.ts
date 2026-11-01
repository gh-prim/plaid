import createClient from 'openapi-fetch'
import type { components, paths } from './schema'
import { hosted, serverOrigin } from './server'

// Absolute base URL: the API is served by the same origin (or proxied by Vite in dev).
// In hosted mode each request goes to the server found at start, with the
// session cookie (a credentialed cross-origin request on the same site).
// fetch is looked up on each call so tests can stub it.
export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api/v1`,
  fetch: async (request) => {
    if (!hosted) return globalThis.fetch(request)
    // The body is buffered, not streamed: streamed uploads need HTTP/2.
    const body =
      request.method === 'GET' || request.method === 'HEAD'
        ? undefined
        : await request.arrayBuffer()
    return globalThis.fetch(serverOrigin() + request.url.slice(window.location.origin.length), {
      method: request.method,
      headers: request.headers,
      body,
      signal: request.signal,
      credentials: 'include',
    })
  },
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
