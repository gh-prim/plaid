// Where the PLAID server is. Normally the origin that served the app. In
// hosted mode (the app getplaid.direct serves on a server's short link,
// F012) it is the connection found at start: the server's LAN address at
// home, its public address elsewhere. The app and the server then share a
// site (getplaid.direct), so the session cookie works across them.

/** True for the build getplaid.direct serves (`pnpm build:hosted`). */
export const hosted = import.meta.env.MODE === 'hosted'

let origin = window.location.origin

export function serverOrigin(): string {
  return origin
}

export function setServerOrigin(o: string): void {
  origin = o.replace(/\/+$/, '')
}

/** A path on the server ("/api/v1/…") as a URL the browser can load from here:
 * unchanged when the server is where the app came from. */
export function serverUrl(path: string): string {
  return path.startsWith('/') && origin !== window.location.origin ? origin + path : path
}

export type Connection = { url: string; local: boolean }

/** The short link's id: the first path segment, as getplaid.direct/<id>/… */
export function shortLinkId(pathname = window.location.pathname): string | undefined {
  const first = pathname.split('/')[1] ?? ''
  return /^[a-z2-7]{6}$/.test(first) ? first : undefined
}

/**
 * Asks getplaid.direct for the server's addresses and keeps the first that
 * answers its health check, giving the LAN one a head start: at home both
 * may answer, and the LAN one keeps the video inside the house.
 */
export async function findServer(id: string, timeoutMs = 6000): Promise<string | undefined> {
  const r = await fetch(`/api/v1/servers/${encodeURIComponent(id)}/connections`)
  if (!r.ok) return undefined
  const { connections } = (await r.json()) as { connections: Connection[] }
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), timeoutMs)
  const probe = async (c: Connection) => {
    if (!c.local) await new Promise((ok) => setTimeout(ok, 400))
    const res = await fetch(`${c.url}/api/v1/health`, {
      signal: ctrl.signal,
      credentials: 'include',
    })
    if (!res.ok) throw new Error(String(res.status))
    return c.url
  }
  try {
    return await Promise.any(connections.map(probe))
  } catch {
    return undefined
  } finally {
    ctrl.abort()
    clearTimeout(timer)
  }
}
