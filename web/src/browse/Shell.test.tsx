import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'
import { storedTheme } from './theme'

const library = (id: string, name: string, type = 'movies') => ({
  id,
  name,
  type,
  scanIntervalSeconds: null,
  metadataLanguage: 'en',
  metadataRegion: 'US',
  lastScanAt: '2026-10-30T12:00:00Z',
  folders: [],
  counts: { movies: 1240, shows: 0, episodes: 0, files: 1240, unrecognized: 0 },
  scan: { state: 'idle', queued: false, full: false, counters: {} },
})

const facets = (total: number) => ({
  status: 200,
  body: { total, genres: [], decades: [], resolutions: [], hdr: 0, letters: [] },
})

function signedIn(role: 'admin' | 'member') {
  const ada = profile('p1', 'Ada')
  return { status: 200, body: { user: user({ role }), profiles: [ada], activeProfile: ada } }
}

function routes(role: 'admin' | 'member') {
  return {
    'GET /server': server(true),
    'GET /me': signedIn(role),
    'GET /libraries': {
      status: 200,
      body: [library('lm', 'Movies'), library('ls', 'Shows', 'shows')],
    },
    // A child profile's counts come from the facets, not the library's raw counts.
    'GET /libraries/lm/facets': facets(214),
    'GET /libraries/ls/facets': facets(18),
    'GET /playback': {
      status: 200,
      body: {
        sessions: [
          { profileName: 'Sam', itemTitle: 'The Bear · S3 E2', session: { id: 's1' } },
          { profileName: 'Mia', itemTitle: 'Past Lives', session: { id: 's2' } },
        ],
      },
    },
  }
}

afterEach(() => {
  localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
})

describe('shell', () => {
  it('puts the libraries in the glass tabs, and tells the admin who is watching', async () => {
    const calls = fakeApi(routes('admin'))
    await renderApp('/')
    const nav = (await screen.findAllByRole('navigation', { name: 'Main' }))[0]!
    expect(await within(nav).findByRole('link', { name: 'Shows' })).toHaveAttribute(
      'href',
      '/library/ls',
    )
    expect(within(nav).getByRole('link', { name: 'Search' })).toHaveAttribute('href', '/search')
    await userEvent.click(await screen.findByRole('button', { name: /Activity/ }))
    expect(await screen.findByText(/Past Lives/)).toBeInTheDocument()
    expect(calls.some((c) => c.path === '/playback')).toBe(true)
  })

  it('never asks a member’s browser who is watching', async () => {
    const calls = fakeApi(routes('member'))
    await renderApp('/')
    await screen.findAllByRole('navigation', { name: 'Main' })
    expect(screen.queryByText('Server')).not.toBeInTheDocument()
    expect(calls.some((c) => c.path === '/playback')).toBe(false)
  })

  it('switches appearance from the profile menu and remembers it', async () => {
    fakeApi(routes('member'))
    await renderApp('/')
    expect(storedTheme()).toBe('dark')
    const [button] = await screen.findAllByRole('button', { name: 'Profile menu for Ada' })
    await userEvent.click(button!)
    await userEvent.click(screen.getByRole('radio', { name: 'Light' }))
    expect(document.documentElement).toHaveAttribute('data-theme', 'light')
    expect(storedTheme()).toBe('light')
    await userEvent.click(screen.getByRole('radio', { name: 'System' }))
    expect(document.documentElement).not.toHaveAttribute('data-theme')
  })
})
