import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { apiError, fakeApi, health, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada', { hasPin: true })
const kids = profile('p2', 'Kids', { maxAge: 7 })

function setup(select: Parameters<typeof fakeApi>[0][string]) {
  let active: unknown
  return fakeApi({
    'GET /server': server(true),
    'GET /me': () => ({
      status: 200,
      body: { user: user(), profiles: [ada, kids], activeProfile: active },
    }),
    'POST /session/profile': async (req, body) => {
      const reply = typeof select === 'function' ? await select(req, body) : select
      if (reply.status === 200)
        active = (body as { profileId: string }).profileId === 'p1' ? ada : kids
      return reply
    },
    'GET /health': health,
  })
}

describe('ProfilePicker', () => {
  it('opens a profile without a PIN in one click', async () => {
    const calls = setup({ status: 200, body: {} })
    await renderApp('/profiles')
    await userEvent.click(await screen.findByRole('button', { name: /Kids/ }))
    expect(await screen.findByText('Watching as Kids')).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/session/profile')?.body).toEqual({ profileId: 'p2' })
  })

  it('asks for the PIN, keyboard only', async () => {
    const calls = setup((_req, body) =>
      (body as { pin?: string }).pin === '1234'
        ? { status: 200, body: {} }
        : apiError(403, 'invalid_pin'),
    )
    await renderApp('/profiles')
    await screen.findByRole('heading', { name: "Who's watching?" })
    await userEvent.tab() // first tile: Ada
    await userEvent.keyboard('{Enter}')

    const pin = await screen.findByLabelText('Enter the PIN for Ada')
    expect(pin).toHaveFocus()
    await userEvent.keyboard('99a99{Enter}')
    expect(pin).toHaveValue('')
    expect(pin).toHaveAccessibleDescription('Wrong PIN. Try again.')

    await userEvent.keyboard('1234{Enter}')
    expect(await screen.findByText('Watching as Ada')).toBeInTheDocument()
    expect(
      calls
        .filter((c) => c.path === '/session/profile')
        .map((c) => (c.body as { pin: string }).pin),
    ).toEqual(['9999', '1234'])
  })

  it('says when a locked profile can be tried again', async () => {
    setup(apiError(423, 'profile_locked', { 'Retry-After': '300' }))
    await renderApp('/profiles')
    await userEvent.click(await screen.findByRole('button', { name: /Ada/ }))
    await userEvent.type(await screen.findByLabelText('Enter the PIN for Ada'), '1234{Enter}')
    expect(await screen.findByText('Too many wrong PINs. Try again in 5 min.')).toBeInTheDocument()
  })
})
