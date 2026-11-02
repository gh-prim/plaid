import { serverOrigin } from '../api/server'
import { Clock } from './sync'

// PLAID's WebSocket (F010-R44, api/events.md): one connection, JSON
// envelopes, an ack or an error for messages sent with an id. It
// reconnects with backoff and says so, so the room can join again.

export interface Envelope<T = unknown> {
  type: string
  data?: T
  ts?: number
  id?: string
}

export class SocketError extends Error {
  constructor(public code: string) {
    super(code)
  }
}

type Handler = (data: never, env: Envelope) => void

const backoff = [1000, 2000, 4000, 8000, 16000, 30000]

/** The socket URL, on the server the app talks to (hosted mode included). */
export function socketUrl(origin = serverOrigin()): string {
  return origin.replace(/^http/, 'ws') + '/api/v1/ws'
}

export class PartySocket {
  readonly clock = new Clock()
  private ws: WebSocket | null = null
  private handlers = new Map<string, Set<Handler>>()
  private pending = new Map<
    string,
    { resolve: (e: Envelope) => void; reject: (e: Error) => void }
  >()
  private seq = 0
  private attempt = 0
  private closed = false
  private pingTimer: ReturnType<typeof setInterval> | undefined
  private retryTimer: ReturnType<typeof setTimeout> | undefined
  private queue: string[] = []

  constructor(
    private url = socketUrl(),
    private make: (url: string) => WebSocket = (u) => new WebSocket(u),
  ) {}

  connect() {
    this.closed = false
    const ws = this.make(this.url)
    this.ws = ws
    ws.onopen = () => {
      this.attempt = 0
      for (const m of this.queue.splice(0)) ws.send(m)
      // Five clock pings at once, then one every 30 s (R9).
      for (let i = 0; i < 5; i++) setTimeout(() => this.ping(), i * 150)
      clearInterval(this.pingTimer)
      this.pingTimer = setInterval(() => this.ping(), 30_000)
      this.emit('open', {})
    }
    ws.onmessage = (ev) => {
      let env: Envelope
      try {
        env = JSON.parse(String(ev.data)) as Envelope
      } catch {
        return
      }
      if (env.id && this.pending.has(env.id)) {
        const p = this.pending.get(env.id)!
        this.pending.delete(env.id)
        if (env.type === 'error')
          p.reject(new SocketError((env.data as { code?: string })?.code ?? 'error'))
        else p.resolve(env)
        if (env.type === 'ack') return
      }
      this.emit(env.type, env.data, env)
    }
    ws.onclose = () => {
      clearInterval(this.pingTimer)
      for (const p of this.pending.values()) p.reject(new SocketError('disconnected'))
      this.pending.clear()
      if (this.closed) return
      this.emit('close', {})
      const delay = backoff[Math.min(this.attempt++, backoff.length - 1)]!
      this.retryTimer = setTimeout(() => this.connect(), delay)
    }
  }

  private emit(type: string, data: unknown, env: Envelope = { type }) {
    for (const h of this.handlers.get(type) ?? [])
      (h as (d: unknown, e: Envelope) => void)(data, env)
  }

  /** Listens to a message type; returns the unsubscribe. */
  on<T>(type: string, fn: (data: T, env: Envelope) => void): () => void {
    const set = this.handlers.get(type) ?? new Set()
    set.add(fn as Handler)
    this.handlers.set(type, set)
    return () => set.delete(fn as Handler)
  }

  /** Sends without waiting for an answer. */
  send(type: string, data?: unknown) {
    this.raw(JSON.stringify({ type, data }))
  }

  /** Sends and resolves on the ack (or the reply), rejects on an error. */
  request(type: string, data?: unknown): Promise<Envelope> {
    const id = `c${++this.seq}`
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      this.raw(JSON.stringify({ type, id, data }))
    })
  }

  private raw(msg: string) {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(msg)
    else this.queue.push(msg)
  }

  private ping() {
    const sent = Date.now()
    this.request('sync.ping', { t0: sent }).then(
      (env) => this.clock.add({ sentMs: sent, serverMs: env.ts ?? sent, receivedMs: Date.now() }),
      () => {},
    )
  }

  close() {
    this.closed = true
    clearInterval(this.pingTimer)
    clearTimeout(this.retryTimer)
    this.ws?.close()
  }
}
