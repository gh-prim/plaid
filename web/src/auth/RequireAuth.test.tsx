import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { apiError, fakeApi, health, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'
import { currentStep } from './RequireAuth'

const ready = (setupComplete: boolean, me: unknown) =>
  ({ status: 'ready', server: { name: 'nas', version: '1', setupComplete }, me }) as Parameters<
    typeof currentStep
  >[0]

describe('currentStep', () => {
  it('walks setup → sign-in → password change → profile → app', () => {
    const profiles = [profile('p1', 'Ada')]
    expect(currentStep(ready(false, null))).toBe('setup')
    expect(currentStep(ready(true, null))).toBe('login')
    expect(currentStep(ready(true, { user: user({ mustChangePassword: true }), profiles }))).toBe(
      'change-password',
    )
    expect(currentStep(ready(true, { user: user(), profiles }))).toBe('profiles')
    expect(currentStep(ready(true, { user: user(), profiles, activeProfile: profiles[0] }))).toBe(
      'app',
    )
  })
})

describe('route guards', () => {
  it('sends a fresh server to the setup page', async () => {
    fakeApi({ 'GET /server': server(false) })
    await renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Welcome to PLAID.' })).toBeInTheDocument()
  })

  it('sends a signed-out visitor to sign-in, even from /setup', async () => {
    fakeApi({ 'GET /server': server(true), 'GET /me': apiError(401, 'unauthorized') })
    await renderApp('/setup')
    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument()
  })

  it('sends a temporary password to the password change', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': {
        status: 200,
        body: { user: user({ mustChangePassword: true }), profiles: [profile('p1', 'Ada')] },
      },
    })
    await renderApp('/pair')
    expect(
      await screen.findByRole('heading', { name: 'Choose a new password' }),
    ).toBeInTheDocument()
  })

  it('asks who is watching when no profile is active', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': {
        status: 200,
        body: { user: user(), profiles: [profile('p1', 'Ada'), profile('p2', 'Kids')] },
      },
    })
    await renderApp('/')
    expect(await screen.findByRole('heading', { name: "Who's watching?" })).toBeInTheDocument()
  })

  it('shows the home page with an active profile', async () => {
    const p = profile('p1', 'Ada')
    fakeApi({
      'GET /server': server(true),
      'GET /me': { status: 200, body: { user: user(), profiles: [p], activeProfile: p } },
      'GET /health': health,
    })
    await renderApp('/login')
    expect(await screen.findByText('Watching as Ada')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
  })

  it('offers to retry when the server is unreachable', async () => {
    fakeApi({ 'GET /server': { status: 500 } })
    await renderApp('/')
    expect(await screen.findByRole('alert')).toHaveTextContent("Can't reach the server")
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })
})
