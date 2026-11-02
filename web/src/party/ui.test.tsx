import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { fakeApi } from '../test/fakeApi'
import { renderApp } from '../test/renderApp'
import { renderUi } from '../test/renderUi'
import { PartyPanel } from './PartyPanel'
import { Notices } from './Notices'
import type { PartyApi, Room } from './useParty'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const room: Room = {
  partyId: 'p1',
  status: { k: 'in' },
  connected: true,
  me: 'u:mia',
  host: 'u:alex',
  mode: 'host',
  itemId: 'nosferatu',
  playback: { state: 'playing', positionMs: 2292000, atServerMs: 0, rate: 1 },
  people: [
    {
      id: 'u:alex',
      name: 'Alex',
      avatarColor: 1,
      host: true,
      guest: false,
      canControl: true,
      state: 'here',
      screen: 'tvos',
      quality: '4K',
      companion: true,
    },
    {
      id: 'u:mia',
      name: 'Mia',
      avatarColor: 2,
      host: false,
      guest: false,
      canControl: false,
      state: 'here',
      screen: 'web',
      quality: '1080p',
      companion: false,
    },
    {
      id: 'g:cam',
      name: 'Camille',
      avatarColor: 3,
      host: false,
      guest: true,
      canControl: false,
      state: 'loading',
      screen: 'browser',
      companion: false,
    },
  ],
  max: 8,
  queue: [],
  chat: [
    {
      id: 1,
      from: 'u:alex',
      name: 'Alex',
      text: 'Il arrive par la porte ou par le mur ?',
      positionMs: 2282000,
      atServerMs: 0,
    },
    {
      id: 2,
      system: 'joined',
      values: { name: 'Léo', device: 'ios', quality: '720p' },
      positionMs: 0,
      atServerMs: 0,
    },
  ],
  reactions: [],
  asks: [],
  waitingFor: null,
  pauseAtServerMs: null,
  next: null,
  limit: null,
  quality: null,
  retries: 0,
  waitTurn: false,
  notices: [{ key: 1, kind: 'caughtUp', seconds: 1.2 }],
}

function fakeParty(r: Room = room) {
  const send = vi.fn((_type: string, _data?: unknown) => Promise.resolve({ type: 'ack' }))
  const api = {
    room: r,
    send,
    attach: vi.fn(),
    report: vi.fn(),
    serverNow: () => 0,
    clearLimit: vi.fn(),
  }
  return { api: api as unknown as PartyApi, send }
}

describe('PartyPanel (Party13)', () => {
  it('shows who is here, the chat, and asks for a pause without control', async () => {
    const { api, send } = fakeParty()
    await renderUi(
      <PartyPanel
        party={api}
        code="K7P-4QX"
        title="Nosferatu"
        onPeople={() => {}}
        onClose={() => {}}
      />,
    )
    expect(screen.getByText('Camille')).toBeInTheDocument()
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.getByText('Apple TV · 4K')).toBeInTheDocument()
    expect(screen.getByText('Léo joined from the iPhone · 720p')).toBeInTheDocument()
    expect(screen.getByText('Alex · 38:02')).toBeInTheDocument()
    // Mia has no control in host mode: she asks.
    await userEvent.click(screen.getByRole('button', { name: 'Ask for a pause' }))
    expect(send).toHaveBeenCalledWith('party.pause.request')
    await userEvent.type(
      screen.getByRole('textbox', { name: 'Message' }),
      'Attends la scène{Enter}',
    )
    expect(send).toHaveBeenCalledWith('party.chat', { text: 'Attends la scène' })
    await userEvent.click(screen.getByRole('button', { name: 'Send 😱' }))
    expect(send).toHaveBeenCalledWith('party.react', { emoji: '😱' })
  })

  it('lets the host answer a pause request', async () => {
    const { api, send } = fakeParty({
      ...room,
      me: 'u:alex',
      asks: [{ requestId: 3, by: 'u:mia', name: 'Mia' }],
    })
    await renderUi(<PartyPanel party={api} onPeople={() => {}} onClose={() => {}} />)
    expect(screen.getByText('Mia asks for a pause')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'In 5 min' }))
    expect(send).toHaveBeenCalledWith('party.pause.answer', { requestId: 3, answer: 'in5' })
  })

  it('says how far behind it caught you up (Party15)', async () => {
    await renderUi(<Notices room={room} serverNow={() => 0} refused={false} />)
    expect(screen.getByText('You were 1.2 s behind. We caught you up gently.')).toBeInTheDocument()
  })
})

describe('party link (Party14)', () => {
  it('lets a guest in with a first name when the server allows it', async () => {
    const calls = fakeApi({
      'GET /server': {
        status: 200,
        body: { name: 'Remote test', version: 'dev', setupComplete: true },
      },
      'GET /me': { status: 401, body: { error: { code: 'unauthorized', message: '' } } },
      'GET /party-codes/K7P4QX': {
        status: 200,
        body: {
          code: 'K7P-4QX',
          status: 'live',
          hostName: 'Alex',
          watching: 3,
          full: false,
          guests: true,
          item: { id: 'n', kind: 'movie', title: 'Nosferatu' },
        },
      },
      'POST /party-codes/K7P4QX/guest': {
        status: 201,
        body: { partyId: 'p1', guestId: 'g1', firstName: 'Camille' },
      },
    })
    vi.stubGlobal(
      'WebSocket',
      class {
        static OPEN = 1
        close() {}
        send() {}
      },
    )
    await renderApp('/p/K7P4QX')
    expect(await screen.findByText('Alex invites you to a Plaid Party')).toBeInTheDocument()
    expect(screen.getByText('Nosferatu')).toBeInTheDocument()
    await userEvent.type(screen.getByRole('textbox', { name: 'Your first name' }), 'Camille')
    await userEvent.click(screen.getByRole('button', { name: 'Join' }))
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ firstName: 'Camille' }),
    )
  })
})

describe('Upcoming (PartiesWeb)', () => {
  it('lists invitations to answer and releases with their status', async () => {
    const party = {
      id: 'p2',
      code: 'ABC-DEF',
      link: 'x',
      status: 'scheduled',
      controlMode: 'host',
      createdAt: '2026-11-01T10:00:00Z',
      scheduledAt: '2026-11-13T19:30:00Z',
      organizer: { userId: 'mia', name: 'Mia', avatarColor: 2, avatar: { kind: 'initial' } },
      invites: [],
      myAnswer: 'pending',
      item: { id: 'bbb', kind: 'movie', title: 'Big Buck Bunny' },
    }
    const calls = fakeApi({
      'GET /upcoming': {
        status: 200,
        body: {
          invitations: [party],
          parties: [party],
          episodes: [
            {
              showId: 'cam',
              showTitle: 'Caminandes',
              season: 2,
              episode: 3,
              title: 'Llamigos',
              airDate: '2026-11-05',
              status: 'upcoming',
              alert: false,
            },
          ],
        },
      },
      'PUT /parties/p2/answer': { status: 204 },
      'PUT /shows/cam/alerts/2/3': { status: 204 },
    })
    vi.stubGlobal(
      'WebSocket',
      class {
        static OPEN = 1
        close() {}
        send() {}
      },
    )
    const { UpcomingPage } = await import('../upcoming/UpcomingPage')
    await renderUi(
      <MemoryRouter>
        <UpcomingPage />
      </MemoryRouter>,
    )
    expect(await screen.findByText('Mia invites you')).toBeInTheDocument()
    expect(screen.getByText('Caminandes · S2 E3')).toBeInTheDocument()
    expect(screen.getByText('Coming up')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: "I'll be there" }))
    await waitFor(() =>
      expect(
        calls.find((c) => c.method === 'PUT' && c.path === '/parties/p2/answer')?.body,
      ).toEqual({ answer: 'yes' }),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Notify me' }))
    await waitFor(() => expect(calls.some((c) => c.path === '/shows/cam/alerts/2/3')).toBe(true))
  })
})
