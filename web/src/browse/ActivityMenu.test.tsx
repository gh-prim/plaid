import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { fakeApi, profile, server, user } from '../test/fakeApi'
import { stubEventSource } from '../test/fakeEventSource'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada')
const counters = (found: number, analyzed: number) => ({
  found,
  new: 0,
  changed: 0,
  moved: 0,
  missing: 0,
  analyzed,
  failed: 0,
  unrecognized: 0,
})
const library = (id: string, name: string, scan: Record<string, unknown>) => ({
  id,
  name,
  type: 'movies',
  scanIntervalSeconds: null,
  metadataLanguage: 'en',
  metadataRegion: 'US',
  folders: [],
  counts: {},
  scan: { queued: false, full: false, ...scan },
})

function setup(role: 'admin' | 'member') {
  stubEventSource()
  sessionStorage.setItem('plaid.profilePicked', '1')
  fakeApi({
    'GET /server': server(true),
    'GET /me': { status: 200, body: { user: user({ role }), profiles: [ada], activeProfile: ada } },
    'GET /libraries': {
      status: 200,
      body: [
        library('l1', 'Movies', {
          state: 'scanning',
          currentFolder: '/truenas/MOVIES/Dune (2021)',
          counters: counters(120, 0),
        }),
        library('l2', 'Shows', { state: 'analyzing', counters: counters(40, 10) }),
      ],
    },
    'GET /metadata/status': {
      status: 200,
      body: {
        providers: [],
        libraries: [
          { libraryId: 'l1', pending: 300, matched: 240, needsReview: 5, unmatched: 0 },
          {
            libraryId: 'l2',
            pending: 0,
            matched: 80,
            needsReview: 0,
            unmatched: 0,
            refreshing: 12,
          },
        ],
      },
    },
    'GET /playback': {
      status: 200,
      body: {
        sessions: [
          { profileName: 'Mia', itemTitle: 'Her', session: { id: 's1', method: 'transcode' } },
        ],
      },
    },
  })
}

describe('activity menu', () => {
  it('lists what the server is doing, with progress', async () => {
    setup('admin')
    // Not the home: with a first scan running, it sends the admin to the scan screen.
    await renderApp('/search')
    await userEvent.click(await screen.findByRole('button', { name: /^Activity: 5 tasks running/ }))
    const panel = screen.getByRole('dialog', { name: 'Activity' })
    expect(within(panel).getByText('Scanning Movies')).toBeInTheDocument()
    expect(within(panel).getByText('120 files found · Dune (2021)')).toBeInTheDocument()
    expect(within(panel).getByText('Analyzing files in Shows')).toBeInTheDocument()
    expect(within(panel).getByText('10 of 40')).toBeInTheDocument()
    expect(within(panel).getByText('Matching Movies')).toBeInTheDocument()
    expect(within(panel).getByText('245 of 545')).toBeInTheDocument()
    // Titles already matched, fetched again: shown too.
    expect(within(panel).getByText('Refreshing details in Shows')).toBeInTheDocument()
    expect(within(panel).getByText('12 titles to go')).toBeInTheDocument()
    expect(within(panel).getByText('Mia is watching Her')).toBeInTheDocument()
  })

  it('shimmers until the first answer', async () => {
    setup('admin')
    // The libraries never answer in this test.
    const fetch = globalThis.fetch
    vi.stubGlobal('fetch', (req: Request) =>
      new URL(req.url).pathname.endsWith('/api/v1/libraries') ? new Promise(() => {}) : fetch(req),
    )
    await renderApp('/search')
    await userEvent.click(await screen.findByRole('button', { name: /^Activity/ }))
    expect(screen.getByRole('list', { name: 'Loading activity' })).toHaveAttribute(
      'aria-busy',
      'true',
    )
  })

  it('is for admins only', async () => {
    setup('member')
    await renderApp('/search')
    await screen.findAllByRole('button', { name: 'Profile menu for Ada' })
    expect(screen.queryByRole('button', { name: /^Activity/ })).not.toBeInTheDocument()
  })
})
