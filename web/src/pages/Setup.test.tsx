import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { apiError, fakeApi, health, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

async function fill(password = 'plaid and chill forever', confirm = password) {
  await userEvent.type(screen.getByLabelText('Name'), 'Ada')
  await userEvent.type(screen.getByLabelText('Username'), 'ada')
  await userEvent.type(screen.getByLabelText('Password'), password)
  await userEvent.type(screen.getByLabelText('Confirm password'), confirm)
}

describe('Setup', () => {
  it('creates the admin and lands on the home page', async () => {
    let setupDone = false
    const p = profile('p1', 'Ada')
    const calls = fakeApi({
      'GET /server': () => server(setupDone),
      'POST /setup/admin': () => {
        setupDone = true
        return { status: 201, body: {} }
      },
      'GET /me': { status: 200, body: { user: user(), profiles: [p], activeProfile: p } },
      'GET /health': health,
    })
    await renderApp('/setup')
    await screen.findByRole('heading', { name: 'Welcome to PLAID.' })
    await fill()
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    expect(await screen.findByText('Watching as Ada')).toBeInTheDocument()
    const setup = calls.find((c) => c.path === '/setup/admin')!
    expect(setup.headers.get('X-Plaid-Client')).toBe('web')
    expect(setup.body).toMatchObject({
      displayName: 'Ada',
      username: 'ada',
      password: 'plaid and chill forever',
      locale: 'en',
      device: { type: 'web' },
    })
  })

  it("refuses passwords that don't match without calling the server", async () => {
    const calls = fakeApi({ 'GET /server': server(false) })
    await renderApp('/setup')
    await screen.findByRole('heading', { name: 'Welcome to PLAID.' })
    await fill('plaid and chill forever', 'plaid and chill')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    const confirm = screen.getByLabelText('Confirm password')
    expect(confirm).toHaveAttribute('aria-invalid', 'true')
    expect(confirm).toHaveAccessibleDescription("Passwords don't match.")
    expect(calls.some((c) => c.path === '/setup/admin')).toBe(false)
  })

  it('shows field errors from the server', async () => {
    fakeApi({
      'GET /server': server(false),
      'POST /setup/admin': {
        status: 422,
        body: {
          error: {
            code: 'weak_password',
            message: '',
            details: [{ field: 'password', code: 'too_common' }],
          },
        },
      },
    })
    await renderApp('/setup')
    await screen.findByRole('heading', { name: 'Welcome to PLAID.' })
    await fill('password123')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Choose a longer or less common password.',
    )
    expect(screen.getByLabelText('Password')).toHaveAccessibleDescription(
      'Too common. Choose another one.',
    )
  })

  it('asks for the setup code from outside the local network', async () => {
    const calls = fakeApi({
      'GET /server': server(false),
      'POST /setup/admin': (_req, body) =>
        (body as { setupCode?: string }).setupCode
          ? apiError(409, 'setup_complete')
          : apiError(403, 'setup_code_required'),
    })
    await renderApp('/setup')
    await screen.findByRole('heading', { name: 'Welcome to PLAID.' })
    await fill()
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))
    await userEvent.type(await screen.findByLabelText('Setup code'), 'UW26W7AQ')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    expect(calls.filter((c) => c.path === '/setup/admin').at(-1)?.body).toMatchObject({
      setupCode: 'UW26W7AQ',
    })
  })

  it('is in French for a French browser', async () => {
    fakeApi({ 'GET /server': server(false) })
    await renderApp('/setup', 'fr-FR')
    expect(await screen.findByRole('heading', { name: 'Bienvenue sur PLAID.' })).toBeInTheDocument()
    expect(screen.getByText('Crée le compte administrateur de ce serveur.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Créer le compte' })).toBeInTheDocument()
  })
})
