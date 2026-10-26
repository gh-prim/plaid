import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it, vi } from 'vitest'
import { createI18n } from '../i18n'
import { Welcome } from './Welcome'

function mockFetch(status: number, body: unknown) {
  const fetchMock = vi.fn(
    async (_request: Request) =>
      new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      }),
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

const healthy = {
  status: 'ok',
  version: '0.1.0-test',
  commit: 'abc1234',
  uptimeSeconds: 3,
  database: 'ok',
}

async function renderWelcome() {
  vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['en-US'])
  const i18n = await createI18n()
  render(
    <I18nextProvider i18n={i18n}>
      <Welcome />
    </I18nextProvider>,
  )
}

describe('Welcome', () => {
  it('shows the loading state, then the server status and version', async () => {
    const fetchMock = mockFetch(200, healthy)
    await renderWelcome()

    expect(screen.getByText('Warming up the couch…')).toBeInTheDocument()
    expect(await screen.findByText('Your server is up. Grab the blanket.')).toBeInTheDocument()
    expect(screen.getByText('Version 0.1.0-test')).toBeInTheDocument()
    const request = fetchMock.mock.calls[0]?.[0]
    expect(new URL(request!.url).pathname).toBe('/api/v1/health')
  })

  it('shows the error state when the server is degraded', async () => {
    mockFetch(503, { ...healthy, status: 'degraded', database: 'error' })
    await renderWelcome()
    expect(
      await screen.findByText("Can't reach the server. Is it still plugged in?"),
    ).toBeInTheDocument()
  })

  it('shows the error state when the network fails', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('network down')
      }),
    )
    await renderWelcome()
    expect(
      await screen.findByText("Can't reach the server. Is it still plugged in?"),
    ).toBeInTheDocument()
  })

  it('switches language without reloading', async () => {
    mockFetch(200, healthy)
    await renderWelcome()
    await screen.findByText('Your server is up. Grab the blanket.')

    await userEvent.selectOptions(screen.getByLabelText('Language'), 'fr')

    expect(await screen.findByText('Ton serveur est prêt. Attrape le plaid.')).toBeInTheDocument()
    expect(screen.getByLabelText('Langue')).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('fr')
  })
})
