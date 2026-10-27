import { vi } from 'vitest'

type Reply = { status: number; body?: unknown; headers?: Record<string, string> }
type Handler = (req: Request, body: unknown) => Reply | Promise<Reply>

/**
 * Stubs fetch with routes keyed "METHOD /path" (relative to /api/v1).
 * Unmatched requests fail the test with a 599. Returns the recorded calls.
 */
export function fakeApi(routes: Record<string, Handler | Reply>) {
  const calls: { method: string; path: string; body: unknown; headers: Headers }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (req: Request) => {
      const path = new URL(req.url).pathname.replace(/^\/api\/v1/, '')
      const text = req.body ? await req.text() : ''
      const body = text ? JSON.parse(text) : undefined
      calls.push({ method: req.method, path, body, headers: req.headers })
      const route = routes[`${req.method} ${path}`]
      if (!route) return new Response(`no fake for ${req.method} ${path}`, { status: 599 })
      const reply = typeof route === 'function' ? await route(req, body) : route
      return new Response(reply.body === undefined ? null : JSON.stringify(reply.body), {
        status: reply.status,
        headers: { 'Content-Type': 'application/json', ...reply.headers },
      })
    }),
  )
  return calls
}

export const apiError = (
  status: number,
  code: string,
  headers?: Record<string, string>,
): Reply => ({
  status,
  body: { error: { code, message: code } },
  headers,
})

export const profile = (id: string, name: string, extra: Record<string, unknown> = {}) => ({
  id,
  name,
  avatarColor: 0,
  hasPin: false,
  maxAge: null,
  allowUnrated: true,
  locale: null,
  isDefault: id === 'p1',
  ...extra,
})

export const user = (extra: Record<string, unknown> = {}) => ({
  id: 'u1',
  username: 'ada',
  displayName: 'Ada',
  role: 'admin',
  locale: 'en',
  mustChangePassword: false,
  ...extra,
})

export const server = (setupComplete: boolean): Reply => ({
  status: 200,
  body: { name: 'nas', version: '0.1.0-test', setupComplete },
})

export const health: Reply = {
  status: 200,
  body: { status: 'ok', version: '0.1.0-test', commit: 'abc', uptimeSeconds: 1, database: 'ok' },
}
