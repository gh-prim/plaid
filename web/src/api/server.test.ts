import { afterEach, describe, expect, it, vi } from 'vitest'
import { findServer, serverUrl, setServerOrigin, shortLinkId } from './server'

const lan = 'https://192-168-1-20.living-room-k3x7qa.getplaid.direct:32501'
const pub = 'https://living-room-k3x7qa.getplaid.direct:32501'

function stubFetch(answers: Record<string, () => Promise<Response>>) {
  vi.stubGlobal(
    'fetch',
    vi.fn((input: string) => {
      const answer = Object.entries(answers).find(([prefix]) => input.startsWith(prefix))
      return answer ? answer[1]() : Promise.reject(new TypeError('Failed to fetch'))
    }),
  )
}

const connections = () =>
  Promise.resolve(
    Response.json({
      connections: [
        { url: lan, local: true },
        { url: pub, local: false },
      ],
    }),
  )
const ok = () => Promise.resolve(new Response('{}'))

afterEach(() => {
  vi.unstubAllGlobals()
  setServerOrigin(window.location.origin)
})

describe('hosted mode', () => {
  it('reads the id from the short link', () => {
    expect(shortLinkId('/k3x7qa')).toBe('k3x7qa')
    expect(shortLinkId('/k3x7qa/movies/1')).toBe('k3x7qa')
    expect(shortLinkId('/app/assets')).toBeUndefined()
    expect(shortLinkId('/')).toBeUndefined()
  })

  it('prefers the LAN address when both answer', async () => {
    stubFetch({ '/api/v1/servers/': connections, [lan]: ok, [pub]: ok })
    await expect(findServer('k3x7qa')).resolves.toBe(lan)
  })

  it('falls back to the public address away from home', async () => {
    stubFetch({ '/api/v1/servers/': connections, [pub]: ok })
    await expect(findServer('k3x7qa')).resolves.toBe(pub)
  })

  it('gives up when nothing answers', async () => {
    stubFetch({ '/api/v1/servers/': connections })
    await expect(findServer('k3x7qa')).resolves.toBeUndefined()
  })

  it('points server paths at the server found', () => {
    setServerOrigin(pub + '/')
    expect(serverUrl('/api/v1/images/a?w=342')).toBe(pub + '/api/v1/images/a?w=342')
    expect(serverUrl('blob:x')).toBe('blob:x')
  })
})
