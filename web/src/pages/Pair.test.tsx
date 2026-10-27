import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { apiError, fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'

const ada = profile('p1', 'Ada')
const kids = profile('p2', 'Kids', { hasPin: true })

function signedIn(claim: Parameters<typeof fakeApi>[0][string]) {
  return fakeApi({
    'GET /server': server(true),
    'GET /me': { status: 200, body: { user: user(), profiles: [ada, kids], activeProfile: ada } },
    'POST /pairing/claim': claim,
  })
}

describe('Pair', () => {
  it('claims a code on the chosen profile', async () => {
    const calls = signedIn({
      status: 200,
      body: { id: 's9', deviceName: 'Living room TV', deviceType: 'tvos' },
    })
    await renderApp('/pair')
    const pair = await screen.findByRole('button', { name: 'Pair' })
    expect(pair).toBeDisabled()
    await userEvent.type(screen.getByLabelText('Code shown on the TV'), 'k7p-4qx')
    await userEvent.click(pair)

    expect(await screen.findByRole('status')).toHaveTextContent('Living room TV is signed in.')
    expect(calls.find((c) => c.path === '/pairing/claim')?.body).toEqual({
      code: 'K7P4QX',
      profileId: 'p1',
    })
  })

  it('asks for the PIN of a protected profile', async () => {
    const calls = signedIn({
      status: 200,
      body: { id: 's9', deviceName: 'TV', deviceType: 'tvos' },
    })
    await renderApp('/pair')
    await userEvent.type(await screen.findByLabelText('Code shown on the TV'), 'K7P4QX')
    await userEvent.selectOptions(screen.getByLabelText('Start on profile'), 'p2')
    await userEvent.type(screen.getByLabelText('Enter the PIN for Kids'), '4321')
    await userEvent.click(screen.getByRole('button', { name: 'Pair' }))
    await screen.findByRole('status')
    expect(calls.find((c) => c.path === '/pairing/claim')?.body).toEqual({
      code: 'K7P4QX',
      profileId: 'p2',
      pin: '4321',
    })
  })

  it('explains an expired code', async () => {
    signedIn(apiError(410, 'pairing_expired'))
    await renderApp('/pair')
    await userEvent.type(await screen.findByLabelText('Code shown on the TV'), 'K7P4QX')
    await userEvent.click(screen.getByRole('button', { name: 'Pair' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'This code has expired. Ask the TV for a new one.',
    )
  })
})
