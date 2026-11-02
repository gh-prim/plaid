import { useCallback, useEffect, useRef, useState } from 'react'
import { PartySocket, SocketError } from './socket'
import { VideoSync, type RoomPlayback, type ScreenState } from './sync'

// The live room as the web app sees it (F010): one socket, the room's
// snapshot kept up to date from its events, and the actions a person can
// take. Protocol: api/events.md.

export interface PresencePerson {
  id: string
  name: string
  avatarColor: number
  avatarUrl?: string
  host: boolean
  guest: boolean
  canControl: boolean
  state: 'here' | 'loading' | 'waiting' | 'away'
  screen?: string
  quality?: string
  companion: boolean
}

export interface ChatLine {
  id: number
  from?: string
  name?: string
  text?: string
  positionMs: number
  atServerMs: number
  system?: string
  values?: Record<string, string>
}

export interface Reaction {
  key: number
  from: string
  name: string
  emoji: string
}

export interface PauseAsk {
  requestId: number
  by: string
  name: string
}

export interface NextUp {
  itemId: string
  title: string
  season: number
  episode: number
  runtimeMs: number
  startsAtServerMs: number
  ready: number
  of: number
}

export interface TranscodeLimit {
  waiting: { id: string; name: string; quality?: string }[]
  sharedQuality: string
}

/** A passing line of Party15 (who paused, who seeked, the gentle catch-up). */
export type Notice =
  | { key: number; kind: 'paused' | 'resumed' | 'seeked'; name: string; positionMs: number }
  | { key: number; kind: 'caughtUp'; seconds: number }
  | { key: number; kind: 'pauseScheduled'; atServerMs: number }

/** A Notice before it gets its key. */
type WithoutKey<T> = T extends unknown ? Omit<T, 'key'> : never
type NoticeInput = WithoutKey<Notice>

export type Status =
  | { k: 'connecting' }
  | { k: 'in' }
  | { k: 'queued'; position: number }
  | { k: 'removed' }
  | { k: 'ended' }
  | { k: 'failed'; code: string }

export interface Room {
  partyId: string
  status: Status
  connected: boolean
  me: string
  host: string
  mode: 'host' | 'everyone'
  itemId: string
  playback: RoomPlayback | null
  people: PresencePerson[]
  max: number
  queue: string[]
  chat: ChatLine[]
  reactions: Reaction[]
  asks: PauseAsk[]
  waitingFor: { ids: string[]; names: string[]; resumeWithoutAtMs: number } | null
  pauseAtServerMs: number | null
  next: NextUp | null
  limit: TranscodeLimit | null
  /** The quality the host chose for everyone who converts (R26). */
  quality: string | null
  /** Bumped when it's this screen's turn to retry a transcode. */
  retries: number
  /** Told to wait for a free transcode. */
  waitTurn: boolean
  notices: Notice[]
}

const initial: Room = {
  partyId: '',
  status: { k: 'connecting' },
  connected: false,
  me: '',
  host: '',
  mode: 'host',
  itemId: '',
  playback: null,
  people: [],
  max: 8,
  queue: [],
  chat: [],
  reactions: [],
  asks: [],
  waitingFor: null,
  pauseAtServerMs: null,
  next: null,
  limit: null,
  quality: null,
  retries: 0,
  waitTurn: false,
  notices: [],
}

export interface JoinTarget {
  partyId?: string
  code?: string
  /** this (default), companion, or an Apple TV's session id. */
  screen?: string
}

let seq = 0

export function useParty(target: JoinTarget | null, make?: () => PartySocket) {
  const [room, setRoom] = useState<Room>(initial)
  const socket = useRef<PartySocket | null>(null)
  const sync = useRef<VideoSync | null>(null)
  const roomRef = useRef(room)
  useEffect(() => {
    roomRef.current = room
  }, [room])

  const notice = useCallback((n: NoticeInput) => {
    const key = ++seq
    setRoom((r) => ({ ...r, notices: [...r.notices, { ...n, key } as Notice].slice(-3) }))
    setTimeout(
      () => setRoom((r) => ({ ...r, notices: r.notices.filter((x) => x.key !== key) })),
      4000,
    )
  }, [])

  const key = target ? `${target.partyId ?? ''}|${target.code ?? ''}|${target.screen ?? ''}` : ''
  useEffect(() => {
    if (!target) return
    const s = make ? make() : new PartySocket()
    socket.current = s
    const off: (() => void)[] = []
    const on = <T>(type: string, fn: (d: T) => void) => off.push(s.on<T>(type, fn))
    const join = () =>
      s.request('party.join', target).catch((e: unknown) => {
        const code = e instanceof SocketError ? e.code : 'generic'
        if (code !== 'disconnected') setRoom((r) => ({ ...r, status: { k: 'failed', code } }))
      })
    on('open', () => {
      setRoom((r) => ({ ...r, connected: true }))
      void join()
    })
    on('close', () => setRoom((r) => ({ ...r, connected: false })))
    on<Record<string, unknown>>('party.snapshot', (d) => {
      const pres = d.presence as { people: PresencePerson[]; max: number }
      const pb = d.playback as RoomPlayback
      setRoom((r) => ({
        ...r,
        status: { k: 'in' },
        partyId: d.partyId as string,
        me: d.me as string,
        host: d.host as string,
        mode: d.mode as Room['mode'],
        itemId: d.itemId as string,
        playback: pb,
        people: pres.people,
        max: pres.max,
        chat: (d.chat as ChatLine[]) ?? [],
        asks: (d.pauseRequests as PauseAsk[]) ?? [],
        pauseAtServerMs: (d.pauseAtServerMs as number | undefined) ?? null,
      }))
      sync.current?.apply(pb)
    })
    on<{ people: PresencePerson[]; max: number }>('party.presence', (d) =>
      setRoom((r) => ({ ...r, people: d.people, max: d.max })),
    )
    on<RoomPlayback & { action: string; by: string; byName: string; positionMs: number }>(
      'party.playback',
      (d) => {
        setRoom((r) => ({
          ...r,
          playback: d,
          waitingFor: d.state === 'waiting' ? r.waitingFor : null,
          pauseAtServerMs: d.action === 'pause' ? null : r.pauseAtServerMs,
        }))
        sync.current?.apply(d)
        const mine = d.by === roomRef.current.me
        if (
          !mine &&
          d.byName &&
          (d.action === 'pause' || d.action === 'play' || d.action === 'seek')
        ) {
          const kind = d.action === 'pause' ? 'paused' : d.action === 'play' ? 'resumed' : 'seeked'
          notice({ kind, name: d.byName, positionMs: d.positionMs })
        }
      },
    )
    on<{ for: string[]; names: string[]; resumeWithoutAtMs: number }>('party.waiting', (d) =>
      setRoom((r) => ({
        ...r,
        waitingFor: { ids: d.for, names: d.names, resumeWithoutAtMs: d.resumeWithoutAtMs },
      })),
    )
    on<ChatLine>('party.chat', (d) => setRoom((r) => ({ ...r, chat: [...r.chat, d].slice(-200) })))
    on<Omit<Reaction, 'key'>>('party.reaction', (d) => {
      const k = ++seq
      setRoom((r) => ({ ...r, reactions: [...r.reactions, { ...d, key: k }].slice(-12) }))
      setTimeout(
        () => setRoom((r) => ({ ...r, reactions: r.reactions.filter((x) => x.key !== k) })),
        3000,
      )
    })
    on<PauseAsk>('party.pause.requested', (d) => setRoom((r) => ({ ...r, asks: [...r.asks, d] })))
    on<{ requestId: number }>('party.pause.answered', (d) =>
      setRoom((r) => ({ ...r, asks: r.asks.filter((a) => a.requestId !== d.requestId) })),
    )
    on<{ atServerMs: number }>('party.pause.scheduled', (d) => {
      setRoom((r) => ({ ...r, pauseAtServerMs: d.atServerMs }))
      notice({ kind: 'pauseScheduled', atServerMs: d.atServerMs })
    })
    on<{ host: string }>('party.host.changed', (d) => setRoom((r) => ({ ...r, host: d.host })))
    on<{ mode: Room['mode'] }>('party.mode', (d) => setRoom((r) => ({ ...r, mode: d.mode })))
    on<{ position: number }>('party.queued', (d) =>
      setRoom((r) => ({ ...r, status: { k: 'queued', position: d.position } })),
    )
    on<{ waiting: string[] }>('party.queue', (d) => setRoom((r) => ({ ...r, queue: d.waiting })))
    on('party.removed', () => setRoom((r) => ({ ...r, status: { k: 'removed' } })))
    on('party.ended', () => setRoom((r) => ({ ...r, status: { k: 'ended' } })))
    on<NextUp & { stopped?: boolean }>('party.next', (d) =>
      setRoom((r) => ({ ...r, next: d.stopped ? null : d })),
    )
    on<{ itemId: string }>('party.item', (d) =>
      setRoom((r) => ({ ...r, itemId: d.itemId, next: null })),
    )
    on<TranscodeLimit>('party.transcode.limit', (d) => setRoom((r) => ({ ...r, limit: d })))
    on<{ quality?: string; retry?: boolean }>('party.transcode.apply', (d) =>
      setRoom((r) => ({
        ...r,
        limit: null,
        waitTurn: false,
        quality: d.quality ?? r.quality,
        retries: d.retry ? r.retries + 1 : r.retries,
      })),
    )
    on('party.transcode.wait', () => setRoom((r) => ({ ...r, waitTurn: true })))
    s.connect()
    return () => {
      for (const f of off) f()
      s.close()
      socket.current = null
    }
    // The target's fields are the key.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  const send = useCallback((type: string, data?: unknown) => {
    const s = socket.current
    if (!s) return Promise.reject(new SocketError('disconnected'))
    return s.request(type, data)
  }, [])

  /** Hands the room the player's video: the room drives it from now on. */
  const attach = useCallback(
    (video: HTMLVideoElement, sessionId: string, quality: string) => {
      sync.current?.destroy()
      const report = (state: ScreenState, positionMs: number) =>
        socket.current?.send('party.report', {
          state,
          positionMs,
          playbackSessionId: sessionId,
          quality,
        })
      sync.current = new VideoSync(video, {
        serverNow: () => socket.current?.clock.now() ?? Date.now(),
        report,
        onCaughtUp: (behind) => {
          if (behind > 0) notice({ kind: 'caughtUp', seconds: Math.round(behind / 100) / 10 })
        },
      })
      report('loading', Math.round(video.currentTime * 1000))
      const pb = roomRef.current.playback
      if (pb) sync.current.apply(pb)
    },
    [notice],
  )

  useEffect(() => () => sync.current?.destroy(), [])

  const report = useCallback((state: ScreenState, extra: Record<string, unknown> = {}) => {
    socket.current?.send('party.report', { state, positionMs: 0, ...extra })
  }, [])

  const serverNow = useCallback(() => socket.current?.clock.now() ?? Date.now(), [])
  const clearLimit = useCallback(() => setRoom((r) => ({ ...r, limit: null })), [])

  return { room, send, attach, report, serverNow, clearLimit }
}

export type PartyApi = ReturnType<typeof useParty>

/** Whether `me` may play, pause and seek (R14). */
export function canControl(room: Room): boolean {
  if (room.mode === 'everyone' || room.me === room.host) return true
  return room.people.find((p) => p.id === room.me)?.canControl ?? false
}
