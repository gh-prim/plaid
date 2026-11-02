import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { apiError, fakeApi, profile, server, user } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'
import { markProfilePicked } from '../auth/profileSession'

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
  // This tab already chose its profile (F008-R2).
  beforeEach(markProfilePicked)

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

    expect(await screen.findByRole('status')).toHaveTextContent('Living room TV is connected.')
    expect(screen.getByText(/It opens on Ada’s profile\./)).toBeInTheDocument()
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
    expect(screen.getByRole('group', { name: 'It will open on' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: 'Kids' }))
    expect(screen.getByRole('button', { name: 'Pair' })).toBeDisabled()
    await userEvent.type(screen.getByLabelText('Kids’s PIN'), '4321')
    await userEvent.click(screen.getByRole('button', { name: 'Pair' }))
    await screen.findByRole('status')
    expect(calls.find((c) => c.path === '/pairing/claim')?.body).toEqual({
      code: 'K7P4QX',
      profileId: 'p2',
      pin: '4321',
    })
  })

  it('keeps only the characters a code can have', async () => {
    signedIn({ status: 200, body: { id: 's9', deviceName: 'TV', deviceType: 'tvos' } })
    await renderApp('/pair')
    const field = await screen.findByLabelText('Code shown on the TV')
    await userEvent.type(field, 'k0o-1il7p4qx9')
    expect(field).toHaveValue('K7P4QX')
  })

  it('shows a wrong PIN under the PIN, not on the code', async () => {
    signedIn(apiError(403, 'invalid_pin'))
    await renderApp('/pair')
    await userEvent.type(await screen.findByLabelText('Code shown on the TV'), 'K7P4QX')
    await userEvent.click(screen.getByRole('radio', { name: 'Kids' }))
    await userEvent.type(screen.getByLabelText('Kids’s PIN'), '1111')
    await userEvent.click(screen.getByRole('button', { name: 'Pair' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong PIN. Try again.')
    expect(screen.getByLabelText('Code shown on the TV')).not.toHaveAttribute('aria-invalid')
    expect(screen.getByLabelText('Kids’s PIN')).toHaveValue('')
  })

  it('pairs another TV from the done screen', async () => {
    signedIn({ status: 200, body: { id: 's9', deviceName: 'TV', deviceType: 'tvos' } })
    await renderApp('/pair')
    await userEvent.type(await screen.findByLabelText('Code shown on the TV'), 'K7P4QX')
    await userEvent.click(screen.getByRole('button', { name: 'Pair' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Pair another TV' }))
    expect(screen.getByLabelText('Code shown on the TV')).toHaveValue('')
  })

  it('explains an expired code', async () => {
    signedIn(apiError(410, 'pairing_expired'))
    await renderApp('/pair')
    await userEvent.type(await screen.findByLabelText('Code shown on the TV'), 'K7P4QX')
    await userEvent.click(screen.getByRole('button', { name: 'Pair' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'This code expired. Show a new one on the TV.',
    )
  })

  it('takes the code from the TV QR code link', async () => {
    const calls = signedIn({
      status: 200,
      body: { id: 's9', deviceName: 'Living room TV', deviceType: 'tvos' },
    })
    await renderApp('/pair?code=k7p4qx')
    const pair = await screen.findByRole('button', { name: 'Pair' })
    expect(pair).toBeEnabled()
    await userEvent.click(pair)

    expect(await screen.findByRole('status')).toHaveTextContent('Living room TV is connected.')
    expect(calls.find((c) => c.path === '/pairing/claim')?.body).toEqual({
      code: 'K7P4QX',
      profileId: 'p1',
    })
  })
})
