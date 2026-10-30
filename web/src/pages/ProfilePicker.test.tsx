import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { apiError, fakeApi, health, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada', { hasPin: true })
const kids = profile('p2', 'Kids', { maxAge: 7 })
const lea = profile('p3', 'Léa', { locale: 'fr' })

function setup(
  select: Parameters<typeof fakeApi>[0][string],
  opts: { profiles?: unknown[]; active?: unknown } = {},
) {
  const profiles = opts.profiles ?? [ada, kids]
  let active = opts.active
  return fakeApi({
    'GET /server': server(true),
    'GET /me': () => ({ status: 200, body: { user: user(), profiles, activeProfile: active } }),
    'POST /session/profile': async (req, body) => {
      const reply = typeof select === 'function' ? await select(req, body) : select
      if (reply.status === 200)
        active = profiles.find(
          (p) => (p as { id: string }).id === (body as { profileId: string }).profileId,
        )
      return reply
    },
    'GET /health': health,
  })
}

const ok = { status: 200, body: {} }
const menuFor = (name: string) =>
  screen.findAllByRole('button', { name: `Profile menu for ${name}` })

describe('ProfilePicker', () => {
  it('opens a profile without a PIN in one click', async () => {
    const calls = setup(ok)
    await renderApp('/profiles')
    expect(
      await screen.findByRole('heading', { name: 'Who’s under the plaid?' }),
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Kids' }))
    expect(await menuFor('Kids')).not.toHaveLength(0)
    expect(calls.find((c) => c.path === '/session/profile')?.body).toEqual({ profileId: 'p2' })
  })

  it('asks for the PIN, keyboard only, and sends it at the fourth digit', async () => {
    const calls = setup((_req, body) =>
      (body as { pin?: string }).pin === '1234' ? ok : apiError(403, 'invalid_pin'),
    )
    await renderApp('/profiles')
    await screen.findByRole('heading', { name: 'Who’s under the plaid?' })
    await userEvent.tab() // first tile: Ada, with a PIN
    expect(screen.getByRole('button', { name: 'Ada, PIN' })).toHaveFocus()
    await userEvent.keyboard('{Enter}')

    expect(await screen.findByLabelText('Enter Ada’s PIN')).toHaveFocus()
    await userEvent.keyboard('99a99')
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong PIN. Try again.')
    // A fresh, empty field after a wrong PIN, still focused.
    const again = screen.getByLabelText('Enter Ada’s PIN')
    expect(again).toHaveValue('')
    expect(again).toHaveFocus()

    await userEvent.keyboard('1234')
    expect(await menuFor('Ada')).not.toHaveLength(0)
    expect(
      calls
        .filter((c) => c.path === '/session/profile')
        .map((c) => (c.body as { pin: string }).pin),
    ).toEqual(['9999', '1234'])
  })

  it('says when a locked profile can be tried again, and goes back to the profiles', async () => {
    setup(apiError(423, 'profile_locked', { 'Retry-After': '300' }))
    await renderApp('/profiles')
    await userEvent.click(await screen.findByRole('button', { name: 'Ada, PIN' }))
    await userEvent.type(await screen.findByLabelText('Enter Ada’s PIN'), '1234')
    expect(await screen.findByText('Too many tries. Try again in 5 min.')).toBeInTheDocument()
    expect(screen.getByLabelText('Enter Ada’s PIN')).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Choose another profile' }))
    expect(screen.getByRole('heading', { name: 'Who’s under the plaid?' })).toBeInTheDocument()
  })

  it('asks once per tab when the account has several profiles, then goes where it was headed', async () => {
    setup(ok, { profiles: [kids, profile('p4', 'Sam'), profile('p5', 'Mia')], active: kids })
    await renderApp('/pair')
    expect(
      await screen.findByRole('heading', { name: 'Who’s under the plaid?' }),
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Sam' }))
    expect(await screen.findByRole('heading', { name: 'Pair a TV' })).toBeInTheDocument()
  })

  it('never shows on its own with one profile', async () => {
    setup(ok, { profiles: [kids], active: kids })
    await renderApp('/')
    expect(await menuFor('Kids')).not.toHaveLength(0)
    expect(
      screen.queryByRole('heading', { name: 'Who’s under the plaid?' }),
    ).not.toBeInTheDocument()
  })

  it('switches to the profile’s language without a reload', async () => {
    setup(ok, { profiles: [kids, lea] })
    await renderApp('/profiles')
    await userEvent.click(await screen.findByRole('button', { name: 'Léa' }))
    expect(await screen.findAllByRole('button', { name: 'Menu du profil Léa' })).not.toHaveLength(0)
    expect(document.documentElement.lang).toBe('fr')
  })
})
