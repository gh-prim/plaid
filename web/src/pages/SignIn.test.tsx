import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { apiError, fakeApi, server } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

async function signIn(password = 'wrong') {
  await userEvent.type(await screen.findByLabelText('Username'), 'ada')
  await userEvent.type(screen.getByLabelText('Password'), password)
  await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))
}

describe('SignIn', () => {
  it('shows a translated error for wrong credentials', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': apiError(401, 'unauthorized'),
      'POST /auth/login': apiError(401, 'invalid_credentials'),
    })
    await renderApp('/login')
    expect(await screen.findByRole('heading', { name: 'Sign in to nas' })).toBeInTheDocument()
    await signIn()
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong username or password.')
    // Both fields turn red and point at the message (board TL02).
    expect(screen.getByLabelText('Username')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText('Password')).toHaveAccessibleDescription(
      'Wrong username or password.',
    )
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeEnabled()
  })

  it('shows and hides the password', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': apiError(401, 'unauthorized'),
    })
    await renderApp('/login')
    await userEvent.type(await screen.findByLabelText('Password'), 'secret')
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password')
    await userEvent.click(screen.getByRole('button', { name: 'Show password' }))
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'text')
    await userEvent.click(screen.getByRole('button', { name: 'Hide password' }))
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password')
  })

  it('says when to retry after too many attempts', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': apiError(401, 'unauthorized'),
      'POST /auth/login': apiError(429, 'too_many_attempts', { 'Retry-After': '240' }),
    })
    await renderApp('/login')
    await signIn()
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Too many attempts. Try again in 4 min.',
    )
    // Sign in waits for the time the server asked (board TL03).
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeDisabled()
  })

  it('signs in again once the wait is over', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    fakeApi({
      'GET /server': server(true),
      'GET /me': apiError(401, 'unauthorized'),
      'POST /auth/login': apiError(429, 'too_many_attempts', { 'Retry-After': '2' }),
    })
    await renderApp('/login')
    await signIn()
    await screen.findByRole('alert')
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeDisabled()
    await act(() => vi.advanceTimersByTimeAsync(2000))
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeEnabled()
    vi.useRealTimers()
  })

  it('falls back to a generic message for unknown codes, in French', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': apiError(401, 'unauthorized'),
      'POST /auth/login': apiError(500, 'internal_error'),
    })
    await renderApp('/login', 'fr-FR')
    await userEvent.type(await screen.findByLabelText("Nom d'utilisateur"), 'ada')
    await userEvent.type(screen.getByLabelText('Mot de passe'), 'x')
    await userEvent.click(screen.getByRole('button', { name: 'Se connecter' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      "Une erreur s'est produite. Réessaie.",
    )
  })

  it('sends the device and the web client header', async () => {
    const calls = fakeApi({
      'GET /server': server(true),
      'GET /me': apiError(401, 'unauthorized'),
      'POST /auth/login': apiError(401, 'invalid_credentials'),
    })
    await renderApp('/login')
    await signIn()
    await screen.findByRole('alert')
    const login = calls.find((c) => c.path === '/auth/login')!
    expect(login.headers.get('X-Plaid-Client')).toBe('web')
    expect(login.body).toMatchObject({ username: 'ada', device: { type: 'web' } })
    expect((login.body as { device: { name: string } }).device.name).not.toBe('')
  })
})
