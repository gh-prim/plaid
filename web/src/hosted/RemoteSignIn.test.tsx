import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { apiError, fakeApi, server } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

// The app as getplaid.direct serves it, on the short link k3x7qa.
vi.mock('../api/server', async (original) => ({
  ...(await original<typeof import('../api/server')>()),
  hosted: true,
  shortLinkId: () => 'k3x7qa',
}))

describe('RemoteSignIn', () => {
  it('shows the server over the still, and rings the password on a wrong attempt', async () => {
    const calls = fakeApi({
      'GET /server': server(true),
      'GET /me': apiError(401, 'unauthorized'),
      'POST /auth/login': apiError(401, 'invalid_credentials'),
    })
    await renderApp('/login')
    expect(await screen.findByRole('heading', { name: 'nas' })).toBeInTheDocument()
    expect(screen.getByText('k3x7qa')).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Username'), 'ada')
    await userEvent.type(screen.getByLabelText('Password'), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong username or password.')
    expect(screen.getByLabelText('Password')).toHaveAttribute('aria-invalid', 'true')
    expect(calls.find((c) => c.path === '/auth/login')?.body).toMatchObject({ username: 'ada' })
  })

  it('shows and hides the password', async () => {
    fakeApi({ 'GET /server': server(true), 'GET /me': apiError(401, 'unauthorized') })
    await renderApp('/login')
    await userEvent.type(await screen.findByLabelText('Password'), 'secret')
    await userEvent.click(screen.getByRole('button', { name: 'Show password' }))
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'text')
  })
})
