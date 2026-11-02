import { describe, expect, it, vi } from 'vitest'
import { Clock, correct, rateFast, rateSlow, targetAt, VideoSync } from './sync'
import { PartySocket, socketUrl } from './socket'

describe('sync rules (F010-R10)', () => {
  it('ignores small drift, catches up gently, seeks far', () => {
    expect(correct(10_000, 10_100, 1)).toEqual({ rate: 1 })
    expect(correct(10_000, 11_200, 1)).toEqual({ rate: rateFast })
    expect(correct(11_200, 10_000, 1)).toEqual({ rate: rateSlow })
    expect(correct(10_000, 10_100, rateFast)).toEqual({ rate: rateFast })
    expect(correct(10_000, 10_040, rateFast)).toEqual({ rate: 1 })
    expect(correct(10_000, 14_000, 1)).toEqual({ rate: 1, seekMs: 14_000 })
    // A player that can't change its rate seeks past 1 s.
    expect(correct(10_000, 11_200, 1, false)).toEqual({ rate: 1, seekMs: 11_200 })
    expect(correct(10_000, 10_800, 1, false)).toEqual({ rate: 1 })
  })

  it('projects the room position', () => {
    const pb = { state: 'playing' as const, positionMs: 1000, atServerMs: 5000, rate: 1 }
    expect(targetAt(pb, 7000)).toBe(3000)
    expect(targetAt({ ...pb, state: 'paused' }, 7000)).toBe(1000)
  })

  it('keeps the shortest round trip for the clock', () => {
    const c = new Clock()
    c.add({ sentMs: 0, serverMs: 1_000_300, receivedMs: 400 })
    c.add({ sentMs: 1000, serverMs: 1_001_040, receivedMs: 1080 })
    expect(c.offset).toBe(1_000_000)
    expect(c.now(2000)).toBe(1_002_000)
  })
})

describe('VideoSync', () => {
  it('drives the video from the room and reports buffering', () => {
    vi.useFakeTimers()
    const video = document.createElement('video')
    Object.defineProperty(video, 'readyState', { value: 4 })
    let paused = true
    Object.defineProperty(video, 'paused', { get: () => paused })
    video.play = vi.fn(() => {
      paused = false
      return Promise.resolve()
    })
    video.pause = vi.fn(() => {
      paused = true
    })
    const reports: string[] = []
    let now = 10_000
    const sync = new VideoSync(video, { serverNow: () => now, report: (s) => reports.push(s) })
    sync.apply({ state: 'playing', positionMs: 60_000, atServerMs: 10_000, rate: 1 })
    expect(video.play).toHaveBeenCalled()
    expect(video.currentTime).toBe(60) // far: seek

    // 1.2 s behind: catch up at 1.05.
    video.currentTime = 60
    now = 11_200
    vi.advanceTimersByTime(250)
    expect(video.playbackRate).toBe(rateFast)

    video.dispatchEvent(new Event('waiting'))
    expect(reports).toContain('buffering')
    sync.apply({ state: 'paused', positionMs: 61_000, atServerMs: 11_300, rate: 1 })
    expect(video.pause).toHaveBeenCalled()
    expect(video.playbackRate).toBe(1)
    sync.destroy()
    vi.useRealTimers()
  })
})

class FakeWS {
  static OPEN = 1
  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  send(m: string) {
    this.sent.push(m)
  }
  close() {
    this.onclose?.()
  }
  open() {
    this.readyState = 1
    this.onopen?.()
  }
  receive(m: unknown) {
    this.onmessage?.({ data: JSON.stringify(m) })
  }
}

describe('PartySocket', () => {
  it('queues until open, resolves acks, rejects errors, reconnects', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('WebSocket', FakeWS)
    const made: FakeWS[] = []
    const s = new PartySocket('ws://x/api/v1/ws', () => {
      const w = new FakeWS()
      made.push(w)
      return w as unknown as WebSocket
    })
    s.connect()
    const joined = s.request('party.join', { partyId: 'p' })
    expect(made[0]!.sent).toHaveLength(0)
    made[0]!.open()
    const join = JSON.parse(made[0]!.sent[0]!) as { id: string; type: string }
    expect(join.type).toBe('party.join')
    made[0]!.receive({ type: 'ack', id: join.id })
    await expect(joined).resolves.toMatchObject({ type: 'ack' })

    const ctl = s.request('party.control', { action: 'pause' })
    const sent = JSON.parse(made[0]!.sent.at(-1)!) as { id: string }
    made[0]!.receive({ type: 'error', id: sent.id, data: { code: 'no_control' } })
    await expect(ctl).rejects.toMatchObject({ code: 'no_control' })

    const seen: unknown[] = []
    s.on('party.chat', (d) => seen.push(d))
    made[0]!.receive({ type: 'party.chat', data: { text: 'hi' } })
    expect(seen).toEqual([{ text: 'hi' }])

    made[0]!.close()
    vi.advanceTimersByTime(1000)
    expect(made).toHaveLength(2)
    s.close()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('builds the socket URL on the server', () => {
    expect(socketUrl('https://abc.getplaid.direct')).toBe('wss://abc.getplaid.direct/api/v1/ws')
    expect(socketUrl('http://localhost:5173')).toBe('ws://localhost:5173/api/v1/ws')
  })
})
