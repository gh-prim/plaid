import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { apiError, fakeApi, health, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

describe('ChangePassword', () => {
  it('replaces a temporary password, then lets the user in', async () => {
    let changed = false
    const p = profile('p1', 'Bob')
    const calls = fakeApi({
      'GET /server': server(true),
      'GET /me': () => ({
        status: 200,
        body: { user: user({ mustChangePassword: !changed }), profiles: [p], activeProfile: p },
      }),
      'POST /me/password': () => {
        changed = true
        return { status: 204 }
      },
      'GET /health': health,
    })
    await renderApp('/')
    await userEvent.type(await screen.findByLabelText('Current password'), 'temporary password')
    await userEvent.type(screen.getByLabelText('New password'), 'bob has a long password')
    await userEvent.type(screen.getByLabelText('Confirm password'), 'bob has a long password')
    await userEvent.click(screen.getByRole('button', { name: 'Save password' }))

    expect(await screen.findAllByRole('button', { name: 'Profile menu for Bob' })).not.toHaveLength(
      0,
    )
    expect(calls.find((c) => c.path === '/me/password')?.body).toEqual({
      currentPassword: 'temporary password',
      newPassword: 'bob has a long password',
    })
  })

  it('shows a wrong current password', async () => {
    fakeApi({
      'GET /server': server(true),
      'GET /me': { status: 200, body: { user: user({ mustChangePassword: true }), profiles: [] } },
      'POST /me/password': apiError(403, 'invalid_credentials'),
    })
    await renderApp('/change-password')
    await userEvent.type(await screen.findByLabelText('Current password'), 'nope')
    await userEvent.type(screen.getByLabelText('New password'), 'bob has a long password')
    await userEvent.type(screen.getByLabelText('Confirm password'), 'bob has a long password')
    await userEvent.click(screen.getByRole('button', { name: 'Save password' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong username or password.')
  })
})
