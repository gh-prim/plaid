import createClient from 'openapi-fetch'
import type { paths } from './schema'

// Absolute base URL: the API is served by the same origin (or proxied by Vite in dev).
// fetch is looked up on each call so tests can stub it.
export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api/v1`,
  fetch: (request) => globalThis.fetch(request),
})
